package agentfile_test

import (
	"strings"
	"testing"

	"github.com/agentcrate/agentfile"
)

// --- CheckPolicies: Valid configurations ---

func TestCheckPolicies_ValidFull(t *testing.T) {
	data := mustReadTestdata(t, "policy_valid.yaml")
	result, err := agentfile.Parse(data)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if !result.IsValid() {
		t.Fatalf("expected valid parse, got %d errors", len(result.Errors))
	}

	pr := agentfile.CheckPolicies(result.Agentfile)
	if !pr.Valid {
		t.Errorf("expected valid policy result, got invalid")
		for _, f := range pr.Findings {
			t.Logf("  %s: %s: %s", f.Severity, f.Field, f.Message)
		}
	}
	if len(pr.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(pr.Findings))
	}
}

func TestCheckPolicies_ValidFullFromFixture(t *testing.T) {
	// valid_full.yaml has policies with correct references.
	data := mustReadTestdata(t, "valid_full.yaml")
	result, err := agentfile.Parse(data)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if !result.IsValid() {
		t.Fatalf("expected valid parse, got %d errors", len(result.Errors))
	}

	pr := agentfile.CheckPolicies(result.Agentfile)
	if !pr.Valid {
		t.Errorf("expected valid policy result")
		for _, f := range pr.Findings {
			t.Logf("  %s: %s: %s", f.Severity, f.Field, f.Message)
		}
	}
}

// --- CheckPolicies: No policies section ---

func TestCheckPolicies_NoPolicies(t *testing.T) {
	data := mustReadTestdata(t, "policy_no_policies.yaml")
	result, err := agentfile.Parse(data)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if !result.IsValid() {
		t.Fatalf("expected valid parse, got %d errors", len(result.Errors))
	}

	pr := agentfile.CheckPolicies(result.Agentfile)
	// No policies → valid (warning, not error).
	if !pr.Valid {
		t.Error("expected valid result for missing policies (warning only)")
	}
	if len(pr.Findings) != 1 {
		t.Fatalf("expected 1 finding (warning), got %d", len(pr.Findings))
	}
	f := pr.Findings[0]
	if f.Severity != agentfile.PolicyWarning {
		t.Errorf("expected warning severity, got %s", f.Severity)
	}
	if !strings.Contains(f.Message, "No policies defined") {
		t.Errorf("expected 'No policies defined' message, got: %s", f.Message)
	}
}

// --- CheckPolicies: Unknown skill references ---

func TestCheckPolicies_UnknownSkillRef(t *testing.T) {
	data := mustReadTestdata(t, "policy_unknown_skill_ref.yaml")
	result, err := agentfile.Parse(data)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	pr := agentfile.CheckPolicies(result.Agentfile)
	if pr.Valid {
		t.Fatal("expected invalid result for unknown skill ref")
	}

	// Should have exactly one error about the unknown skill.
	errors := pr.Errors()
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
	e := errors[0]
	if e.Rule != "unknown-skill-ref" {
		t.Errorf("expected rule 'unknown-skill-ref', got %q", e.Rule)
	}
	if !strings.Contains(e.Message, "unknown-skill") {
		t.Errorf("expected error mentioning 'unknown-skill', got: %s", e.Message)
	}
	if !strings.Contains(e.Field, "tool_permissions") {
		t.Errorf("expected field containing 'tool_permissions', got: %s", e.Field)
	}
}

// --- CheckPolicies: Invalid HITL conditions ---

func TestCheckPolicies_BadHITL(t *testing.T) {
	data := mustReadTestdata(t, "policy_bad_hitl.yaml")
	result, err := agentfile.Parse(data)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	pr := agentfile.CheckPolicies(result.Agentfile)
	if pr.Valid {
		t.Fatal("expected invalid result for bad HITL conditions")
	}

	errors := pr.Errors()
	if len(errors) < 2 {
		t.Fatalf("expected at least 2 errors (invalid keyword + empty cost_above), got %d", len(errors))
	}

	// Check for the unknown keyword error.
	var foundUnknown, foundCostAbove bool
	for _, e := range errors {
		if e.Rule == "invalid-hitl-condition" && strings.Contains(e.Message, "invalid_keyword") {
			foundUnknown = true
		}
		if e.Rule == "invalid-hitl-condition" && strings.Contains(e.Message, "cost_above") {
			foundCostAbove = true
		}
	}
	if !foundUnknown {
		t.Error("expected error for unknown HITL keyword 'invalid_keyword'")
	}
	if !foundCostAbove {
		t.Error("expected error for empty cost_above threshold")
	}
}

// --- CheckPolicies: Domain violations ---

func TestCheckPolicies_DomainViolation(t *testing.T) {
	data := mustReadTestdata(t, "policy_domain_violation.yaml")
	result, err := agentfile.Parse(data)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	pr := agentfile.CheckPolicies(result.Agentfile)
	if pr.Valid {
		t.Fatal("expected invalid result for domain violation")
	}

	errors := pr.Errors()
	// Should flag evil.example.com but NOT sec.gov or stdio:// sources.
	if len(errors) != 1 {
		for _, e := range errors {
			t.Logf("  %s: %s", e.Field, e.Message)
		}
		t.Fatalf("expected exactly 1 domain error, got %d", len(errors))
	}
	e := errors[0]
	if e.Rule != "domain-not-allowed" {
		t.Errorf("expected rule 'domain-not-allowed', got %q", e.Rule)
	}
	if !strings.Contains(e.Message, "evil.example.com") {
		t.Errorf("expected error mentioning 'evil.example.com', got: %s", e.Message)
	}
}

// --- CheckPolicies: Subdomain matching ---

func TestCheckPolicies_SubdomainMatch(t *testing.T) {
	// mcp.sec.gov should match allowed domain "sec.gov".
	af := &agentfile.Agentfile{
		Skills: []agentfile.Skill{
			{Name: "sec-api", Type: "mcp", Source: "https://mcp.sec.gov/v1"},
		},
		Policies: &agentfile.Policies{
			AllowedDomains: []string{"sec.gov"},
		},
	}
	pr := agentfile.CheckPolicies(af)
	if !pr.Valid {
		t.Error("expected valid result — mcp.sec.gov is a subdomain of sec.gov")
		for _, f := range pr.Findings {
			t.Logf("  %s: %s", f.Field, f.Message)
		}
	}
}

// --- CheckPolicies: stdio:// skip domain check ---

func TestCheckPolicies_StdioSkipDomainCheck(t *testing.T) {
	af := &agentfile.Agentfile{
		Skills: []agentfile.Skill{
			{Name: "local-tool", Type: "mcp", Source: "stdio://my-server"},
		},
		Policies: &agentfile.Policies{
			AllowedDomains: []string{"example.com"},
		},
	}
	pr := agentfile.CheckPolicies(af)
	if !pr.Valid {
		t.Error("expected valid — stdio:// sources should not be domain-checked")
	}
}

// --- CheckPolicies: Valid HITL conditions ---

func TestCheckPolicies_ValidHITLConditions(t *testing.T) {
	conditions := []string{
		"always",
		"never",
		"on_failure",
		"side_effects",
		"cost_above:100",
		"cost_above:0.5",
	}
	for _, cond := range conditions {
		af := &agentfile.Agentfile{
			Skills: []agentfile.Skill{
				{Name: "tool", Type: "mcp", Source: "cratehub.ai/tools/test"},
			},
			Policies: &agentfile.Policies{
				HumanInTheLoop: []agentfile.HITLRule{
					{Tool: "tool", Condition: cond},
				},
			},
		}
		pr := agentfile.CheckPolicies(af)
		if !pr.Valid {
			t.Errorf("expected valid for HITL condition %q, got errors:", cond)
			for _, f := range pr.Findings {
				t.Logf("  %s: %s", f.Field, f.Message)
			}
		}
	}
}

// --- CheckPolicies: Empty HITL condition ---

func TestCheckPolicies_EmptyHITLCondition(t *testing.T) {
	af := &agentfile.Agentfile{
		Skills: []agentfile.Skill{
			{Name: "tool", Type: "mcp", Source: "cratehub.ai/tools/test"},
		},
		Policies: &agentfile.Policies{
			HumanInTheLoop: []agentfile.HITLRule{
				{Tool: "tool", Condition: ""},
			},
		},
	}
	pr := agentfile.CheckPolicies(af)
	if pr.Valid {
		t.Fatal("expected invalid for empty HITL condition")
	}
	found := false
	for _, e := range pr.Errors() {
		if strings.Contains(e.Message, "must not be empty") {
			found = true
		}
	}
	if !found {
		t.Error("expected error about empty HITL condition")
	}
}

// --- CheckPolicies: Multiple errors combined ---

func TestCheckPolicies_MultipleErrors(t *testing.T) {
	af := &agentfile.Agentfile{
		Skills: []agentfile.Skill{
			{Name: "real-skill", Type: "mcp", Source: "https://evil.com/v1"},
		},
		Policies: &agentfile.Policies{
			AllowedDomains: []string{"example.com"},
			ToolPermissions: []agentfile.ToolPermission{
				{Skill: "fake-skill", Allow: []string{"read"}},
			},
			HumanInTheLoop: []agentfile.HITLRule{
				{Tool: "also-fake", Condition: "garbage"},
			},
		},
	}
	pr := agentfile.CheckPolicies(af)
	if pr.Valid {
		t.Fatal("expected invalid for multiple errors")
	}
	// Should have at least 3 errors: unknown skill ref (tool perm),
	// unknown skill ref + bad condition (HITL), and domain violation.
	if len(pr.Errors()) < 3 {
		t.Errorf("expected at least 3 errors, got %d", len(pr.Errors()))
		for _, e := range pr.Errors() {
			t.Logf("  %s: %s: %s", e.Rule, e.Field, e.Message)
		}
	}
}

// --- PolicyResult helper methods ---

func TestPolicyResult_HasErrors(t *testing.T) {
	r := &agentfile.PolicyResult{
		Valid: true,
		Findings: []agentfile.PolicyFinding{
			{Severity: agentfile.PolicyWarning, Message: "warning"},
		},
	}
	if r.HasErrors() {
		t.Error("expected HasErrors() = false for warnings only")
	}

	r.Findings = append(r.Findings, agentfile.PolicyFinding{
		Severity: agentfile.PolicyError, Message: "error",
	})
	if !r.HasErrors() {
		t.Error("expected HasErrors() = true when errors exist")
	}
}

func TestPolicyResult_ErrorsAndWarnings(t *testing.T) {
	r := &agentfile.PolicyResult{
		Findings: []agentfile.PolicyFinding{
			{Severity: agentfile.PolicyError, Message: "err1"},
			{Severity: agentfile.PolicyWarning, Message: "warn1"},
			{Severity: agentfile.PolicyError, Message: "err2"},
		},
	}
	if len(r.Errors()) != 2 {
		t.Errorf("expected 2 errors, got %d", len(r.Errors()))
	}
	if len(r.Warnings()) != 1 {
		t.Errorf("expected 1 warning, got %d", len(r.Warnings()))
	}
}

// --- Policies present but empty (no rules) ---

func TestCheckPolicies_EmptyPolicies(t *testing.T) {
	af := &agentfile.Agentfile{
		Skills: []agentfile.Skill{
			{Name: "tool", Type: "mcp", Source: "cratehub.ai/tools/test"},
		},
		Policies: &agentfile.Policies{},
	}
	pr := agentfile.CheckPolicies(af)
	if !pr.Valid {
		t.Error("expected valid for empty (but present) policies section")
	}
	if len(pr.Findings) != 0 {
		t.Errorf("expected 0 findings for empty policies, got %d", len(pr.Findings))
	}
}
