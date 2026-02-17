package agentfile

// Agentfile represents a fully parsed Agentfile v1 specification.
type Agentfile struct {
	Version  string             `yaml:"version"           json:"version"`
	Metadata Metadata           `yaml:"metadata"          json:"metadata"`
	Brain    Brain              `yaml:"brain"             json:"brain"`
	Persona  Persona            `yaml:"persona"           json:"persona"`
	Skills   []Skill            `yaml:"skills,omitempty"  json:"skills,omitempty"`
	Build    *Build             `yaml:"build,omitempty"   json:"build,omitempty"`
	Policies *Policies          `yaml:"policies,omitempty" json:"policies,omitempty"`
	Profiles map[string]Profile `yaml:"profiles,omitempty" json:"profiles,omitempty"`
}

// Metadata contains agent identity and discoverability information.
type Metadata struct {
	Name        string   `yaml:"name"                  json:"name"`
	Version     string   `yaml:"version"               json:"version"`
	Description string   `yaml:"description"           json:"description"`
	Author      string   `yaml:"author,omitempty"      json:"author,omitempty"`
	License     string   `yaml:"license,omitempty"     json:"license,omitempty"`
	Repository  string   `yaml:"repository,omitempty"  json:"repository,omitempty"`
	Tags        []string `yaml:"tags,omitempty"        json:"tags,omitempty"`
}

// Brain defines model configurations and the active default.
type Brain struct {
	Default string        `yaml:"default"  json:"default"`
	Models  []ModelConfig `yaml:"models"   json:"models"`
}

// ModelConfig defines a named model with provider-qualified identifier and tuning.
type ModelConfig struct {
	Name        string   `yaml:"name"                  json:"name"`
	Model       string   `yaml:"model"                 json:"model"`
	Temperature *float64 `yaml:"temperature,omitempty" json:"temperature,omitempty"`
	MaxTokens   *int     `yaml:"max_tokens,omitempty"  json:"max_tokens,omitempty"`
	TopP        *float64 `yaml:"top_p,omitempty"       json:"top_p,omitempty"`
}

// Persona defines the agent's identity and behavioral framing.
type Persona struct {
	SystemPrompt string `yaml:"system_prompt"         json:"system_prompt"`
	Name         string `yaml:"name,omitempty"        json:"name,omitempty"`
	Role         string `yaml:"role,omitempty"        json:"role,omitempty"`
}

// Skill represents a tool or capability available to the agent.
type Skill struct {
	Name        string         `yaml:"name"                  json:"name"`
	Type        string         `yaml:"type"                  json:"type"`
	Source      string         `yaml:"source"                json:"source"`
	Description string         `yaml:"description,omitempty" json:"description,omitempty"`
	Config      map[string]any `yaml:"config,omitempty"      json:"config,omitempty"`
}

// Build configures the container image build process.
type Build struct {
	// BaseImage overrides the default base image (agentcrate/base:latest).
	// Use this to specify a custom base image with additional dependencies.
	BaseImage string `yaml:"base_image" json:"base_image"`
}

// Policies defines security constraints and governance rules.
type Policies struct {
	AllowedDomains  []string         `yaml:"allowed_domains,omitempty"  json:"allowed_domains,omitempty"`
	HumanInTheLoop  []HITLRule       `yaml:"human_in_the_loop,omitempty" json:"human_in_the_loop,omitempty"`
	ToolPermissions []ToolPermission `yaml:"tool_permissions,omitempty" json:"tool_permissions,omitempty"`
	MaxTokens       *int             `yaml:"max_tokens,omitempty"       json:"max_tokens,omitempty"`
	MaxTurns        *int             `yaml:"max_turns,omitempty"        json:"max_turns,omitempty"`
}

// HITLRule defines a human-in-the-loop approval requirement.
type HITLRule struct {
	Tool      string `yaml:"tool"      json:"tool"`
	Condition string `yaml:"condition" json:"condition"`
}

// ToolPermission defines fine-grained permissions for a skill.
type ToolPermission struct {
	Skill string   `yaml:"skill" json:"skill"`
	Allow []string `yaml:"allow" json:"allow"`
}

// ProfileBrain allows profile overrides to switch the active model.
type ProfileBrain struct {
	Default string `yaml:"default" json:"default"`
}

// Profile represents environment-specific configuration overrides.
type Profile struct {
	Brain    *ProfileBrain `yaml:"brain,omitempty"    json:"brain,omitempty"`
	Policies *Policies     `yaml:"policies,omitempty" json:"policies,omitempty"`
}
