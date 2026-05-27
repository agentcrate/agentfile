package agentfile

import (
	"fmt"
	"net/url"
	"sort"
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

// validHITLConditions is the set of recognized HITL condition keywords.
// Using map[HITLCondition]struct{} expresses set semantics without per-entry
// waste. Sourced from the HITLCondition* constants in types.go so the
// validator and the public constants cannot drift; the JSON Schema enum on
// HITLRule.Condition is derived from the same set via struct-tag annotations.
var validHITLConditions = map[HITLCondition]struct{}{
	HITLConditionAlways:      {},
	HITLConditionNever:       {},
	HITLConditionOnFailure:   {},
	HITLConditionCostAbove:   {},
	HITLConditionSideEffects: {},
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
	skillNames := buildSkillNameSet(af.Skills)

	// Check tool_permissions reference declared skills.
	checkToolPermissions(af, skillNames, result)

	// Check human_in_the_loop references and condition syntax.
	checkHITLRules(af, skillNames, result)

	// Check skill sources are within allowed domains.
	checkAllowedDomains(af, result)

	result.Valid = !result.HasErrors()
	return result
}

// buildSkillNameSet returns a set of declared skill names for O(1) membership checks.
func buildSkillNameSet(skills []Skill) map[string]struct{} {
	names := make(map[string]struct{}, len(skills))
	for i := range skills {
		names[skills[i].Name] = struct{}{}
	}
	return names
}

// checkToolPermissions verifies that all tool_permissions reference declared skill names.
func checkToolPermissions(af *Agentfile, skillNames map[string]struct{}, result *PolicyResult) {
	for i, tp := range af.Policies.ToolPermissions {
		if _, ok := skillNames[tp.Skill]; !ok {
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

// checkHITLRules validates human_in_the_loop rules: skill references and condition syntax.
func checkHITLRules(af *Agentfile, skillNames map[string]struct{}, result *PolicyResult) {
	for i, hitl := range af.Policies.HumanInTheLoop {
		// Check skill reference.
		if _, ok := skillNames[hitl.Skill]; !ok {
			result.Findings = append(result.Findings, PolicyFinding{
				Severity: PolicyError,
				Rule:     "unknown-skill-ref",
				Field:    fmt.Sprintf("policies.human_in_the_loop[%d].skill", i),
				Message:  fmt.Sprintf("Policy references unknown skill: '%s'", hitl.Skill),
				Value:    hitl.Skill,
			})
		}

		// Validate condition value.
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

// validateHITLCondition checks that a HITL condition is one of the five
// recognized keywords. The schema enforces the same enum at parse time, but
// this function exists so an Agentfile constructed directly in Go (bypassing
// parse) is still validated.
func validateHITLCondition(condition string) error {
	if condition == "" {
		return fmt.Errorf("HITL condition must not be empty")
	}
	if _, ok := validHITLConditions[HITLCondition(condition)]; !ok {
		return fmt.Errorf("unknown HITL condition: %q (valid: %s)",
			condition, validHITLKeywords())
	}
	return nil
}

// validHITLKeywords returns a comma-separated list of valid HITL condition keywords.
func validHITLKeywords() string {
	keys := make([]string, 0, len(validHITLConditions))
	for k := range validHITLConditions {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

// checkAllowedDomains verifies that network skill sources fall within allowed_domains.
func checkAllowedDomains(af *Agentfile, result *PolicyResult) {
	if len(af.Policies.AllowedDomains) == 0 {
		return
	}

	// Build allowed domain set. map[string]struct{} makes set semantics explicit
	// and avoids a false-negative when the zero value (false) is looked up.
	allowed := make(map[string]struct{}, len(af.Policies.AllowedDomains))
	for _, d := range af.Policies.AllowedDomains {
		allowed[strings.ToLower(d)] = struct{}{}
	}

	for i := range af.Skills {
		// Only check skills with network-accessible sources (http/sse types).
		if af.Skills[i].Type != "http" && af.Skills[i].Type != "sse" {
			continue
		}
		host := extractHost(af.Skills[i].Source)
		if host == "" {
			continue
		}
		if !isDomainAllowed(host, allowed) {
			result.Findings = append(result.Findings, PolicyFinding{
				Severity: PolicyError,
				Rule:     "domain-not-allowed",
				Field:    fmt.Sprintf("skills[%d].source", i),
				Message:  fmt.Sprintf("Skill source domain '%s' is not in allowed_domains", host),
				Value:    af.Skills[i].Source,
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
//
// The subdomain loop is O(N) over allowed_domains. In practice Agentfile
// allowed_domains lists are small (single digits), so the linear scan is
// acceptable. If large lists become common, consider a sorted-prefix index.
func isDomainAllowed(host string, allowed map[string]struct{}) bool {
	host = strings.ToLower(host)
	// Exact match.
	if _, ok := allowed[host]; ok {
		return true
	}
	// Subdomain match: check if host ends with ".domain".
	// Keys in allowed are already lowercased at construction time.
	for domain := range allowed {
		if strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}
