package agentfile

import (
	"errors"
	"slices"
	"testing"
)

func baseAgentfile() *Agentfile {
	temp := 0.7
	maxTok := 4096
	return &Agentfile{
		Version: "1",
		Metadata: Metadata{
			Name:        "test-agent",
			Version:     "0.1.0",
			Description: "Test agent.",
		},
		Brain: Brain{
			Default: "gpt",
			Models: []ModelConfig{
				{Name: "gpt", Model: "openai/gpt-4o", Temperature: &temp, MaxTokens: &maxTok},
				{Name: "claude", Model: "anthropic/claude-sonnet-4-20250514", Temperature: &temp},
				{Name: "local", Model: "ollama/llama3"},
			},
		},
		Persona: Persona{
			SystemPrompt: "You are a test agent.",
		},
		Skills: []Skill{
			{Name: "web-search", Type: "mcp", Source: "cratehub.ai/tools/web-search"},
		},
		Profiles: map[string]Profile{
			"dev": {
				Brain: &ProfileBrain{Default: "local"},
			},
			"staging": {
				Brain: &ProfileBrain{Default: "claude"},
			},
			"prod": {
				Brain:    &ProfileBrain{Default: "gpt"},
				Policies: &Policies{MaxTokens: intPtr(8192)},
			},
		},
	}
}

func intPtr(v int) *int { return &v }

// ---------------------------------------------------------------------------
// ResolveProfile
// ---------------------------------------------------------------------------

func TestResolveProfile_EmptyName(t *testing.T) {
	af := baseAgentfile()
	resolved, err := ResolveProfile(af, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ResolveProfile must return a defensive copy in all paths so callers can
	// safely mutate the result without corrupting the input. The empty/"default"
	// path used to alias the input pointer; the contract is now uniform.
	if resolved == af {
		t.Fatal("expected defensive copy for empty profile name; got aliased input pointer")
	}
	if resolved.Brain.Default != af.Brain.Default {
		t.Errorf("expected brain.default copied, got %q want %q", resolved.Brain.Default, af.Brain.Default)
	}
	// Mutating the returned copy must not affect the original.
	resolved.Brain.Default = "mutated"
	if af.Brain.Default == "mutated" {
		t.Error("mutating resolved leaked into original Agentfile")
	}
}

func TestResolveProfile_Default(t *testing.T) {
	af := baseAgentfile()
	resolved, err := ResolveProfile(af, "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "default" is a logical no-op but still returns a defensive copy.
	if resolved == af {
		t.Fatal("expected defensive copy for 'default' profile; got aliased input pointer")
	}
	resolved.Brain.Default = "mutated"
	if af.Brain.Default == "mutated" {
		t.Error("mutating resolved leaked into original Agentfile")
	}
}

// TestResolveProfile_ProfilePoliciesDeepCopied verifies that when a profile
// provides its own Policies block, the resolved Policies pointer is a deep
// copy of the profile's policies, not the same pointer (regression for the
// previously-aliased profile-supplied policies bug).
func TestResolveProfile_ProfilePoliciesDeepCopied(t *testing.T) {
	af := baseAgentfile()
	// Replace the prod profile with one whose policies have populated slices
	// so we can verify those slices are independent.
	prodPolicies := &Policies{
		AllowedDomains:  []string{"prod.example.com"},
		ToolPermissions: []ToolPermission{{Skill: "web-search", Allow: []string{"read"}}},
		HumanInTheLoop:  []HITLRule{{Skill: "web-search", Condition: HITLConditionAlways}},
	}
	af.Profiles["prod"] = Profile{
		Brain:    &ProfileBrain{Default: "gpt"},
		Policies: prodPolicies,
	}

	resolved, err := ResolveProfile(af, "prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Policies == prodPolicies {
		t.Fatal("expected deep copy; resolved.Policies aliases the profile's policies pointer")
	}

	// Mutating the resolved policies must not bleed into the original profile.
	resolved.Policies.AllowedDomains = append(resolved.Policies.AllowedDomains, "extra.example.com")
	resolved.Policies.ToolPermissions[0].Allow = append(resolved.Policies.ToolPermissions[0].Allow, "write")
	resolved.Policies.HumanInTheLoop[0].Condition = "never"

	if len(prodPolicies.AllowedDomains) != 1 || prodPolicies.AllowedDomains[0] != "prod.example.com" {
		t.Errorf("AllowedDomains leaked into original profile: %v", prodPolicies.AllowedDomains)
	}
	if len(prodPolicies.ToolPermissions[0].Allow) != 1 {
		t.Errorf("ToolPermissions[0].Allow leaked into original profile: %v", prodPolicies.ToolPermissions[0].Allow)
	}
	if prodPolicies.HumanInTheLoop[0].Condition != "always" {
		t.Errorf("HumanInTheLoop[0].Condition leaked into original profile: %q", prodPolicies.HumanInTheLoop[0].Condition)
	}
}

func TestResolveProfile_DevProfile(t *testing.T) {
	af := baseAgentfile()
	resolved, err := ResolveProfile(af, "dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Brain.Default != "local" {
		t.Errorf("expected brain.default='local', got %q", resolved.Brain.Default)
	}
	// Original should be unmodified.
	if af.Brain.Default != "gpt" {
		t.Error("original Agentfile was mutated")
	}
	// Profiles should be cleared in resolved.
	if resolved.Profiles != nil {
		t.Error("expected profiles to be nil in resolved output")
	}
	// Models should be preserved (not overridden).
	if len(resolved.Brain.Models) != 3 {
		t.Errorf("expected 3 models, got %d", len(resolved.Brain.Models))
	}
}

func TestResolveProfile_StagingProfile(t *testing.T) {
	af := baseAgentfile()
	resolved, err := ResolveProfile(af, "staging")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Brain.Default != "claude" {
		t.Errorf("expected brain.default='claude', got %q", resolved.Brain.Default)
	}
}

func TestResolveProfile_ProdProfileWithPolicies(t *testing.T) {
	af := baseAgentfile()
	resolved, err := ResolveProfile(af, "prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Brain.Default != "gpt" {
		t.Errorf("expected brain.default='gpt', got %q", resolved.Brain.Default)
	}
	if resolved.Policies == nil {
		t.Fatal("expected policies to be set from prod profile")
	}
	if resolved.Policies.MaxTokens == nil || *resolved.Policies.MaxTokens != 8192 {
		t.Error("expected policies.max_tokens=8192 from prod profile")
	}
}

func TestResolveProfile_PreservesBasePolicies(t *testing.T) {
	af := baseAgentfile()
	basePolicies := &Policies{
		AllowedDomains: []string{"example.com"},
	}
	af.Policies = basePolicies

	// Dev profile doesn't override policies.
	resolved, err := ResolveProfile(af, "dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Deep copy means different pointer, but values should match.
	if resolved.Policies == nil {
		t.Fatal("expected base policies to be preserved when profile doesn't override")
	}
	if len(resolved.Policies.AllowedDomains) != 1 || resolved.Policies.AllowedDomains[0] != "example.com" {
		t.Error("expected base policies values to be preserved when profile doesn't override")
	}
}

func TestResolveProfile_ProfileReplacesPolicies(t *testing.T) {
	af := baseAgentfile()
	af.Policies = &Policies{AllowedDomains: []string{"base.com"}}

	resolved, err := ResolveProfile(af, "prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Prod profile has its own policies; should fully replace base.
	if resolved.Policies.MaxTokens == nil || *resolved.Policies.MaxTokens != 8192 {
		t.Error("expected prod profile policies")
	}
	if resolved.Policies.AllowedDomains != nil {
		t.Error("expected base allowed_domains to be replaced, not merged")
	}
}

func TestResolveProfile_DoesNotMutateOriginal(t *testing.T) {
	af := baseAgentfile()
	original := af.Brain.Default

	_, err := ResolveProfile(af, "dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if af.Brain.Default != original {
		t.Errorf("original brain.default mutated: %q → %q", original, af.Brain.Default)
	}
	if af.Profiles == nil {
		t.Error("original profiles were cleared")
	}
}

// ---------------------------------------------------------------------------
// ProfileNotFoundError
// ---------------------------------------------------------------------------

func TestResolveProfile_NotFound(t *testing.T) {
	af := baseAgentfile()
	_, err := ResolveProfile(af, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent profile")
	}

	var pnf *ProfileNotFoundError
	if !errors.As(err, &pnf) {
		t.Fatalf("expected ProfileNotFoundError, got %T: %v", err, err)
	}
	if pnf.Name != "nonexistent" {
		t.Errorf("expected name 'nonexistent', got %q", pnf.Name)
	}
	// Available should be sorted and include "default".
	if len(pnf.Available) != 4 {
		t.Fatalf("expected 4 available profiles, got %d: %v", len(pnf.Available), pnf.Available)
	}
	if want := []string{"default", "dev", "prod", "staging"}; !slices.Equal(pnf.Available, want) {
		t.Errorf("expected %v, got %v", want, pnf.Available)
	}

	// Error message should be helpful.
	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestResolveProfile_NotFound_NoProfiles(t *testing.T) {
	af := &Agentfile{
		Version:  "1",
		Metadata: Metadata{Name: "no-profiles"},
		Brain:    Brain{Default: "gpt", Models: []ModelConfig{{Name: "gpt", Model: "openai/gpt-4o"}}},
		Persona:  Persona{SystemPrompt: "Test."},
	}
	_, err := ResolveProfile(af, "dev")
	if err == nil {
		t.Fatal("expected error")
	}
	var pnf *ProfileNotFoundError
	if !errors.As(err, &pnf) {
		t.Fatalf("expected ProfileNotFoundError, got %T", err)
	}
	// Should still have "default" even with no user-defined profiles.
	if len(pnf.Available) != 1 || pnf.Available[0] != "default" {
		t.Errorf("expected [default], got %v", pnf.Available)
	}
}

// ---------------------------------------------------------------------------
// availableProfiles
// ---------------------------------------------------------------------------

func TestResolveProfile_DeepCopySlices(t *testing.T) {
	af := baseAgentfile()
	af.Policies = &Policies{
		AllowedDomains: []string{"example.com"},
		ToolPermissions: []ToolPermission{
			{Skill: "web-search", Allow: []string{"read"}},
		},
		HumanInTheLoop: []HITLRule{
			{Skill: "web-search", Condition: HITLConditionAlways},
		},
	}

	resolved, err := ResolveProfile(af, "dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Mutate resolved copy's Skills slice.
	originalSkillsLen := len(af.Skills)
	resolved.Skills = append(resolved.Skills, Skill{Name: "new-skill", Type: "mcp", Source: "test"})
	if len(af.Skills) != originalSkillsLen {
		t.Errorf("appending to resolved.Skills mutated original: got len %d, want %d",
			len(af.Skills), originalSkillsLen)
	}

	// Mutate resolved copy's Brain.Models slice.
	originalModelsLen := len(af.Brain.Models)
	resolved.Brain.Models = append(resolved.Brain.Models, ModelConfig{Name: "new", Model: "test/new"})
	if len(af.Brain.Models) != originalModelsLen {
		t.Errorf("appending to resolved.Brain.Models mutated original: got len %d, want %d",
			len(af.Brain.Models), originalModelsLen)
	}

	// Mutate resolved copy's Build pointer.
	if af.Build == nil {
		af.Build = &Build{BaseImage: "original:latest"}
		resolved2, err := ResolveProfile(af, "staging")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		resolved2.Build.BaseImage = "mutated:latest"
		if af.Build.BaseImage != "original:latest" {
			t.Errorf("mutating resolved.Build mutated original: got %q, want %q",
				af.Build.BaseImage, "original:latest")
		}
	}
}

// TestResolveProfile_DeepCopySkillFields verifies that the per-Skill mutable
// reference fields (Args, Env, Config) are independently cloned, not aliased.
// Direct element assignment is the discriminating mutation: append() may
// reallocate and hide aliasing, but in-place writes cannot.
func TestResolveProfile_DeepCopySkillFields(t *testing.T) {
	af := baseAgentfile()
	af.Skills = []Skill{
		{
			Name:   "shell",
			Type:   "stdio",
			Source: "/bin/sh",
			Args:   []string{"-c", "echo hi"},
			Env:    []string{"FOO=bar"},
			Config: map[string]any{"timeout": 30},
		},
	}

	resolved, err := ResolveProfile(af, "dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// In-place writes on the resolved Skill's reference fields must not bleed
	// into the source Agentfile. append() can hide aliasing via reallocation;
	// these direct writes cannot.
	resolved.Skills[0].Args[0] = "mutated"
	resolved.Skills[0].Env[0] = "MUTATED=1"
	resolved.Skills[0].Config["timeout"] = 9999

	if af.Skills[0].Args[0] != "-c" {
		t.Errorf("Skill.Args aliased: original mutated to %q", af.Skills[0].Args[0])
	}
	if af.Skills[0].Env[0] != "FOO=bar" {
		t.Errorf("Skill.Env aliased: original mutated to %q", af.Skills[0].Env[0])
	}
	if v, _ := af.Skills[0].Config["timeout"].(int); v != 30 {
		t.Errorf("Skill.Config aliased: original mutated to %v", af.Skills[0].Config["timeout"])
	}
}

// TestResolveProfile_DeepCopyToolPermissionAllow verifies that each
// ToolPermission's Allow slice is independently cloned. The existing
// AllowedDomains test uses append() which can reallocate; in-place element
// assignment is the discriminating mutation that exposes aliasing.
func TestResolveProfile_DeepCopyToolPermissionAllow(t *testing.T) {
	af := baseAgentfile()
	prodPolicies := &Policies{
		ToolPermissions: []ToolPermission{
			{Skill: "web-search", Allow: []string{"read", "list"}},
		},
	}
	af.Profiles["prod"] = Profile{
		Brain:    &ProfileBrain{Default: "gpt"},
		Policies: prodPolicies,
	}

	resolved, err := ResolveProfile(af, "prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// In-place write into the resolved Allow slice — if it aliases the source,
	// the original profile's ToolPermissions[0].Allow[0] will flip.
	resolved.Policies.ToolPermissions[0].Allow[0] = "write"

	if prodPolicies.ToolPermissions[0].Allow[0] != "read" {
		t.Errorf("ToolPermission.Allow aliased: original mutated to %q",
			prodPolicies.ToolPermissions[0].Allow[0])
	}
}

// TestResolveProfile_DeepCopyBasePolicyToolPermissionAllow covers the same
// aliasing for base (non-profile) policies, which flow through
// deepCopyAgentfile → deepCopyPolicies.
func TestResolveProfile_DeepCopyBasePolicyToolPermissionAllow(t *testing.T) {
	af := baseAgentfile()
	af.Policies = &Policies{
		ToolPermissions: []ToolPermission{
			{Skill: "web-search", Allow: []string{"read", "list"}},
		},
	}

	// "dev" does not override policies, so resolved.Policies is a deep copy
	// of af.Policies.
	resolved, err := ResolveProfile(af, "dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resolved.Policies.ToolPermissions[0].Allow[0] = "write"

	if af.Policies.ToolPermissions[0].Allow[0] != "read" {
		t.Errorf("base ToolPermission.Allow aliased: original mutated to %q",
			af.Policies.ToolPermissions[0].Allow[0])
	}
}

func TestAvailableProfiles_Sorted(t *testing.T) {
	af := baseAgentfile()
	names := availableProfiles(af)
	// 3 user-defined + "default" = 4.
	if len(names) != 4 {
		t.Fatalf("expected 4 profiles, got %d: %v", len(names), names)
	}
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("profiles not sorted: %v", names)
			break
		}
	}
}

func TestAvailableProfiles_EmptyAgentfile(t *testing.T) {
	af := &Agentfile{}
	names := availableProfiles(af)
	// "default" is always present.
	if len(names) != 1 || names[0] != "default" {
		t.Errorf("expected [default], got %v", names)
	}
}

func TestResolveProfile_CheckPoliciesOnResolved(t *testing.T) {
	af := baseAgentfile()
	af.Skills = []Skill{
		{Name: "web-search", Type: "mcp", Source: "cratehub.ai/tools/web-search"},
	}
	af.Policies = &Policies{
		AllowedDomains: []string{"example.com"},
		ToolPermissions: []ToolPermission{
			{Skill: "web-search", Allow: []string{"read"}},
		},
	}

	resolved, err := ResolveProfile(af, "prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Prod profile replaces policies entirely (no tool_permissions),
	// so CheckPolicies should return valid (no stale skill references).
	pr := CheckPolicies(resolved)
	if pr.HasErrors() {
		t.Error("expected no errors on resolved profile")
		for _, f := range pr.Findings {
			t.Logf("  %s: %s: %s", f.Severity, f.Field, f.Message)
		}
	}
}
