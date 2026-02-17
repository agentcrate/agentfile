package agentfile

import (
	"errors"
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
	// Should return the same pointer unmodified.
	if resolved != af {
		t.Error("expected same pointer for empty profile name")
	}
}

func TestResolveProfile_Default(t *testing.T) {
	af := baseAgentfile()
	resolved, err := ResolveProfile(af, "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "default" is a no-op — returns base config.
	if resolved != af {
		t.Error("expected same pointer for 'default' profile")
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
	if resolved.Policies != basePolicies {
		t.Error("expected base policies to be preserved when profile doesn't override")
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
	if pnf.Available[0] != "default" || pnf.Available[1] != "dev" || pnf.Available[2] != "prod" || pnf.Available[3] != "staging" {
		t.Errorf("expected [default, dev, prod, staging], got %v", pnf.Available)
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
