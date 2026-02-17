package agentfile

import (
	"fmt"
	"net/url"
	"strings"
)

// PolicySeverity indicates whether a finding is an error or a warning.
type PolicySeverity string

const (
	// PolicyError indicates a policy violation that should fail the check.
	PolicyError PolicySeverity = "error"
	// PolicyWarning indicates a non-fatal advisory finding.
	PolicyWarning PolicySeverity = "warning"
)

// PolicyFinding represents a single policy check finding.
type PolicyFinding struct {
	Severity PolicySeverity `json:"severity"`
	Rule     string         `json:"rule"`
	Field    string         `json:"field"`
	Message  string         `json:"message"`
	Value    string         `json:"value,omitempty"`
}

// PolicyResult contains the results of a policy check run.
type PolicyResult struct {
	Valid    bool            `json:"valid"`
	Findings []PolicyFinding `json:"findings,omitempty"`
}

// HasErrors returns true if any finding is an error.
func (r *PolicyResult) HasErrors() bool {
	for _, f := range r.Findings {
		if f.Severity == PolicyError {
			return true
		}
	}
	return false
}

// Errors returns only error-severity findings.
func (r *PolicyResult) Errors() []PolicyFinding {
	var errs []PolicyFinding
	for _, f := range r.Findings {
		if f.Severity == PolicyError {
			errs = append(errs, f)
		}
	}
	return errs
}

// Warnings returns only warning-severity findings.
func (r *PolicyResult) Warnings() []PolicyFinding {
	var warns []PolicyFinding
	for _, f := range r.Findings {
		if f.Severity == PolicyWarning {
			warns = append(warns, f)
		}
	}
	return warns
}

// validHITLConditions lists the recognized HITL condition keywords.
var validHITLConditions = map[string]bool{
	"always":       true,
	"never":        true,
	"on_failure":   true,
	"cost_above":   true,
	"side_effects": true,
}

// CheckPolicies performs policy consistency checks on a parsed Agentfile.
// It validates:
//   - Tool permissions reference declared skills
//   - HITL rules reference declared skills and have valid condition syntax
//   - Skill sources are within allowed domains (if allowed_domains is set)
//   - Missing policies section (advisory warning)
func CheckPolicies(af *Agentfile) *PolicyResult {
	result := &PolicyResult{Valid: true}

	// AC: No policy section → warning, not error.
	if af.Policies == nil {
		result.Findings = append(result.Findings, PolicyFinding{
			Severity: PolicyWarning,
			Rule:     "no-policies",
			Field:    "policies",
			Message:  "No policies defined. Consider adding security policies.",
		})
		return result
	}

	// Build skill name set for reference checks.
	skillNames := make(map[string]bool, len(af.Skills))
	for _, s := range af.Skills {
		skillNames[s.Name] = true
	}

	// Check tool_permissions reference declared skills.
	checkToolPermissions(af, skillNames, result)

	// Check human_in_the_loop references and condition syntax.
	checkHITLRules(af, skillNames, result)

	// Check skill sources are within allowed domains.
	checkAllowedDomains(af, result)

	result.Valid = !result.HasErrors()
	return result
}

// checkToolPermissions verifies that all tool_permissions reference declared skill names.
func checkToolPermissions(af *Agentfile, skillNames map[string]bool, result *PolicyResult) {
	for i, tp := range af.Policies.ToolPermissions {
		if !skillNames[tp.Skill] {
			result.Findings = append(result.Findings, PolicyFinding{
				Severity: PolicyError,
				Rule:     "unknown-skill-ref",
				Field:    fmt.Sprintf("policies.tool_permissions[%d].skill", i),
				Message:  fmt.Sprintf("Policy references unknown skill: '%s'", tp.Skill),
				Value:    tp.Skill,
			})
		}
	}
}

// checkHITLRules validates human_in_the_loop rules: tool references and condition syntax.
func checkHITLRules(af *Agentfile, skillNames map[string]bool, result *PolicyResult) {
	for i, hitl := range af.Policies.HumanInTheLoop {
		// Check tool reference.
		if !skillNames[hitl.Tool] {
			result.Findings = append(result.Findings, PolicyFinding{
				Severity: PolicyError,
				Rule:     "unknown-skill-ref",
				Field:    fmt.Sprintf("policies.human_in_the_loop[%d].tool", i),
				Message:  fmt.Sprintf("Policy references unknown skill: '%s'", hitl.Tool),
				Value:    hitl.Tool,
			})
		}

		// Validate condition syntax.
		if err := validateHITLCondition(hitl.Condition); err != nil {
			result.Findings = append(result.Findings, PolicyFinding{
				Severity: PolicyError,
				Rule:     "invalid-hitl-condition",
				Field:    fmt.Sprintf("policies.human_in_the_loop[%d].condition", i),
				Message:  err.Error(),
				Value:    hitl.Condition,
			})
		}
	}
}

// validateHITLCondition checks that a HITL condition string has valid syntax.
// Conditions are keyword-based: "always", "never", "on_failure", "side_effects",
// or parameterized like "cost_above:100".
func validateHITLCondition(condition string) error {
	if condition == "" {
		return fmt.Errorf("HITL condition must not be empty")
	}

	// Check for parameterized conditions (e.g., "cost_above:100").
	parts := strings.SplitN(condition, ":", 2)
	keyword := parts[0]

	if !validHITLConditions[keyword] {
		return fmt.Errorf("unknown HITL condition keyword: '%s' (valid: %s)",
			keyword, validHITLKeywords())
	}

	// Parameterized conditions must have a non-empty value.
	if keyword == "cost_above" {
		if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
			return fmt.Errorf("'cost_above' condition requires a numeric threshold (e.g., cost_above:100)")
		}
	}

	return nil
}

// validHITLKeywords returns a comma-separated list of valid HITL condition keywords.
func validHITLKeywords() string {
	keys := make([]string, 0, len(validHITLConditions))
	for k := range validHITLConditions {
		keys = append(keys, k)
	}
	return strings.Join(keys, ", ")
}

// checkAllowedDomains verifies that network skill sources fall within allowed_domains.
func checkAllowedDomains(af *Agentfile, result *PolicyResult) {
	if len(af.Policies.AllowedDomains) == 0 {
		return
	}

	// Build allowed domain set.
	allowed := make(map[string]bool, len(af.Policies.AllowedDomains))
	for _, d := range af.Policies.AllowedDomains {
		allowed[strings.ToLower(d)] = true
	}

	for i, s := range af.Skills {
		// Only check skills with network-accessible sources (http/sse types).
		if s.Type != "http" && s.Type != "sse" {
			continue
		}
		host := extractHost(s.Source)
		if host == "" {
			continue
		}
		if !isDomainAllowed(host, allowed) {
			result.Findings = append(result.Findings, PolicyFinding{
				Severity: PolicyError,
				Rule:     "domain-not-allowed",
				Field:    fmt.Sprintf("skills[%d].source", i),
				Message:  fmt.Sprintf("Skill source domain '%s' is not in allowed_domains", host),
				Value:    s.Source,
			})
		}
	}
}

// extractHost extracts the hostname from a URL. Returns "" for non-network URIs.
func extractHost(source string) string {
	parsed, err := url.Parse(source)
	if err != nil {
		return ""
	}
	scheme := strings.ToLower(parsed.Scheme)
	// Only check network schemes.
	if scheme != "http" && scheme != "https" {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}

// isDomainAllowed checks if a host matches any allowed domain.
// Supports subdomain matching: "mcp.sec.gov" matches "sec.gov".
func isDomainAllowed(host string, allowed map[string]bool) bool {
	// Exact match.
	if allowed[host] {
		return true
	}
	// Subdomain match: check if host ends with ".domain".
	for domain := range allowed {
		if strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}
