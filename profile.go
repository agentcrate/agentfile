package agentfile

import (
	"fmt"
	"sort"
	"strings"
)

// ResolveProfile merges the named profile's overrides onto the base Agentfile
// and returns a new Agentfile with the resolved configuration.
//
// A defensive deep copy is returned in every path — including the empty
// profile name and the built-in "default" profile — so callers can freely
// mutate the result without corrupting the input Agentfile.
//
// Returns an error if a non-default profile is requested but not defined.
func ResolveProfile(af *Agentfile, profileName string) (*Agentfile, error) {
	if profileName == "" || profileName == "default" {
		resolved := deepCopyAgentfile(af)
		// The flattened view exposes a single environment, so profiles are
		// cleared even for the no-op path to keep the contract uniform.
		resolved.Profiles = nil
		return resolved, nil
	}

	p, ok := af.Profiles[profileName]
	if !ok {
		return nil, &ProfileNotFoundError{
			Name:      profileName,
			Available: availableProfiles(af),
		}
	}

	resolved := deepCopyAgentfile(af)

	// Merge brain.default override.
	if p.Brain != nil {
		resolved.Brain.Default = p.Brain.Default
	}

	// Merge policies override (full replacement, not deep merge). Deep-copy so
	// the resolved Policies don't alias the profile entry on the original
	// Agentfile — mutating one must never bleed into the other.
	if p.Policies != nil {
		resolved.Policies = deepCopyPolicies(p.Policies)
	}

	// Clear profiles from the resolved output — the result is a flattened config.
	resolved.Profiles = nil

	return resolved, nil
}

// deepCopyAgentfile returns a deep copy of af with mutable slices and pointer
// fields cloned so callers can mutate the result independently of the input.
// The Profiles map is shallow-copied (entries are reused) because callers
// either replace it (in ResolveProfile) or treat the map as read-only.
func deepCopyAgentfile(af *Agentfile) *Agentfile {
	resolved := *af
	if af.Build != nil {
		b := *af.Build
		resolved.Build = &b
	}
	if af.Skills != nil {
		skills := make([]Skill, len(af.Skills))
		for i := range af.Skills {
			skills[i] = af.Skills[i]
			deepCopySkillRefs(&skills[i])
		}
		resolved.Skills = skills
	}
	resolved.Brain.Models = append([]ModelConfig(nil), af.Brain.Models...)
	if af.Policies != nil {
		resolved.Policies = deepCopyPolicies(af.Policies)
	}
	return &resolved
}

// deepCopySkillRefs clones a Skill's mutable reference fields (Args, Env,
// Config) in place so the Skill can be mutated without affecting the source.
// The Config map values are not recursively cloned — callers must treat nested
// map/slice values as immutable, matching the JSON-style "arbitrary config"
// contract. Takes a pointer because Skill is heavy (~136 bytes).
func deepCopySkillRefs(s *Skill) {
	s.Args = append([]string(nil), s.Args...)
	s.Env = append([]string(nil), s.Env...)
	if s.Config != nil {
		cfg := make(map[string]any, len(s.Config))
		for k, v := range s.Config {
			cfg[k] = v
		}
		s.Config = cfg
	}
}

// deepCopyPolicies clones a Policies value and all of its slice fields,
// including each ToolPermission's Allow slice (which is itself a reference
// type and must be cloned to break aliasing on element-level writes).
// MaxTokens/MaxTurns are *int — these are not deep-copied because the pointed-to
// int is treated as immutable by callers; if that changes, deep-copy here too.
func deepCopyPolicies(p *Policies) *Policies {
	if p == nil {
		return nil
	}
	cp := *p
	cp.AllowedDomains = append([]string(nil), p.AllowedDomains...)
	cp.HumanInTheLoop = append([]HITLRule(nil), p.HumanInTheLoop...)
	if p.ToolPermissions != nil {
		tps := make([]ToolPermission, len(p.ToolPermissions))
		for i, tp := range p.ToolPermissions {
			tp.Allow = append([]string(nil), tp.Allow...)
			tps[i] = tp
		}
		cp.ToolPermissions = tps
	}
	return &cp
}

// ProfileNotFoundError is returned when a requested profile doesn't exist.
type ProfileNotFoundError struct {
	Name      string
	Available []string
}

func (e *ProfileNotFoundError) Error() string {
	return fmt.Sprintf(
		"profile %q not found in Agentfile. Available profiles: %s",
		e.Name,
		strings.Join(e.Available, ", "),
	)
}

// availableProfiles returns a sorted list of profile names.
// "default" is always included as a built-in profile.
func availableProfiles(af *Agentfile) []string {
	names := make([]string, 0, len(af.Profiles)+1)
	names = append(names, "default")
	for name := range af.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
