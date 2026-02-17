package agentfile

import (
	"fmt"
	"sort"
	"strings"
)

// ResolveProfile merges the named profile's overrides onto the base Agentfile
// and returns a new Agentfile with the resolved configuration.
// If profileName is empty, the base Agentfile is returned unmodified.
// Returns an error if the profile is not defined.
func ResolveProfile(af *Agentfile, profileName string) (*Agentfile, error) {
	if profileName == "" || profileName == "default" {
		return af, nil
	}

	p, ok := af.Profiles[profileName]
	if !ok {
		return nil, &ProfileNotFoundError{
			Name:      profileName,
			Available: availableProfiles(af),
		}
	}

	// Shallow-copy the Agentfile so we don't mutate the original.
	resolved := *af

	// Merge brain.default override.
	if p.Brain != nil {
		brain := resolved.Brain
		brain.Default = p.Brain.Default
		resolved.Brain = brain
	}

	// Merge policies override (full replacement, not deep merge).
	if p.Policies != nil {
		resolved.Policies = p.Policies
	}

	// Clear profiles from the resolved output — the result is a flattened config.
	resolved.Profiles = nil

	return &resolved, nil
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
