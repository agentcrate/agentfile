package agentfile

// Agentfile represents a fully parsed Agentfile v1 specification.
// Declarative specification for packaging AI agents with AgentCrate.
type Agentfile struct {
	// Schema version. Must be "1" for v1 Agentfiles.
	Version string `yaml:"version" json:"version" jsonschema:"required,const=1"`
	// Agent identity and discoverability information.
	Metadata Metadata `yaml:"metadata" json:"metadata" jsonschema:"required"`
	// Model configurations and inference parameters.
	Brain Brain `yaml:"brain" json:"brain" jsonschema:"required"`
	// Agent identity and behavioral framing.
	Persona Persona `yaml:"persona" json:"persona" jsonschema:"required"`
	// Tools and capabilities available to the agent.
	Skills []Skill `yaml:"skills,omitempty" json:"skills,omitempty"`
	// Container image build configuration.
	Build *Build `yaml:"build,omitempty" json:"build,omitempty"`
	// Security constraints and governance rules.
	Policies *Policies `yaml:"policies,omitempty" json:"policies,omitempty"`
	// Environment-specific configuration overrides.
	Profiles map[string]Profile `yaml:"profiles,omitempty" json:"profiles,omitempty"`
}

// Metadata contains agent identity and discoverability information.
type Metadata struct {
	// Agent name. Lowercase alphanumeric with dots, hyphens, underscores. Max 128 chars.
	// NOTE: Commas in jsonschema patterns must be escaped as \, because the struct tag parser treats commas as delimiters.
	Name string `yaml:"name" json:"name" jsonschema:"required,pattern=^[a-z0-9][a-z0-9._-]{0\\,127}$"`
	// Semantic version (e.g., 1.0.0, 0.1.0-beta).
	Version string `yaml:"version" json:"version" jsonschema:"required,pattern=^(0|[1-9]\\d*)\\.(0|[1-9]\\d*)\\.(0|[1-9]\\d*)(-[a-zA-Z0-9]+)?(\\+[a-zA-Z0-9]+)?$"`
	// Human-readable description of the agent.
	Description string `yaml:"description" json:"description" jsonschema:"required,minLength=1,maxLength=500"`
	// Author name or organization.
	Author string `yaml:"author,omitempty" json:"author,omitempty"`
	// SPDX license identifier (e.g., MIT, Apache-2.0).
	License string `yaml:"license,omitempty" json:"license,omitempty"`
	// Source repository URL.
	Repository string `yaml:"repository,omitempty" json:"repository,omitempty" jsonschema:"format=uri"`
	// Discovery tags for CrateHub.
	Tags []string `yaml:"tags,omitempty" json:"tags,omitempty" jsonschema:"maxItems=20,uniqueItems=true"`
}

// Brain defines model configurations and the active default.
// Declares named model entries with per-model tuning; profiles switch the active model via default.
type Brain struct {
	// Name of the model configuration to use by default. Must match a name in the models array.
	Default string `yaml:"default" json:"default" jsonschema:"required,pattern=^[a-z0-9][a-z0-9-]{0\\,31}$"`
	// Array of model configurations. Each model has its own tuning parameters.
	Models []ModelConfig `yaml:"models" json:"models" jsonschema:"required,minItems=1"`
}

// ModelConfig defines a named model with provider-qualified identifier and tuning.
type ModelConfig struct {
	// Short identifier for this model configuration (e.g., sonnet, local, fast).
	Name string `yaml:"name" json:"name" jsonschema:"required,pattern=^[a-z0-9][a-z0-9-]{0\\,31}$"`
	// Provider-qualified model identifier (e.g., anthropic/claude-3.5-sonnet, openai/gpt-4o, ollama/llama3).
	// Provider and model names must be lowercase.
	Model string `yaml:"model" json:"model" jsonschema:"required,pattern=^[a-z0-9-]+/[a-z0-9._-]+$"`
	// Sampling temperature for this model.
	Temperature *float64 `yaml:"temperature,omitempty" json:"temperature,omitempty" jsonschema:"minimum=0,maximum=2"`
	// Maximum tokens per response for this model.
	MaxTokens *int `yaml:"max_tokens,omitempty" json:"max_tokens,omitempty" jsonschema:"minimum=1"`
	// Top-p (nucleus) sampling parameter for this model.
	TopP *float64 `yaml:"top_p,omitempty" json:"top_p,omitempty" jsonschema:"minimum=0,maximum=1"`
}

// Persona defines the agent's identity and behavioral framing.
type Persona struct {
	// The system-level instruction that defines agent behavior.
	SystemPrompt string `yaml:"system_prompt" json:"system_prompt" jsonschema:"required,minLength=1"`
	// Display name for the agent persona.
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	// Role description (e.g., Code Reviewer, Research Assistant).
	Role string `yaml:"role,omitempty" json:"role,omitempty"`
}

// Skill represents a tool or capability available to the agent.
type Skill struct {
	// Unique skill identifier.
	Name string `yaml:"name" json:"name" jsonschema:"required,pattern=^[a-z0-9][a-z0-9._-]{0\\,63}$"`
	// Skill transport type. 'mcp' for registry tools, 'stdio' for local binaries, 'http' for Streamable HTTP endpoints, 'sse' for SSE endpoints.
	Type string `yaml:"type" json:"type" jsonschema:"required,enum=mcp,enum=stdio,enum=http,enum=sse"`
	// Skill source. For mcp: registry skill name. For stdio: local binary path (if no command). For http/sse: endpoint URL.
	Source string `yaml:"source,omitempty" json:"source,omitempty"`
	// Human-readable description of the skill.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	// Skill-specific configuration key-value pairs.
	// Arbitrary key-value configuration. Server-side validation should enforce maxProperties and depth limits.
	Config map[string]any `yaml:"config,omitempty" json:"config,omitempty"`
	// Binary to execute for stdio skills (e.g., "npx"). If empty, source is used as the binary path.
	Command string `yaml:"command,omitempty" json:"command,omitempty"`
	// Arguments passed to command for stdio skills (e.g., ["-y", "@modelcontextprotocol/server-everything"]).
	Args []string `yaml:"args,omitempty" json:"args,omitempty"`
	// Required environment variables. The runtime validates these are set at startup.
	Env []string `yaml:"env,omitempty" json:"env,omitempty"`
}

// Build configures the container image build process.
// Use this to customize how crate build produces agent images.
type Build struct {
	// Override the default base image (agentcrate/base:latest). Required when
	// the build section is present.
	// Docker image reference. Server-side validation should enforce format (registry/image:tag) and max length.
	BaseImage string `yaml:"base_image" json:"base_image"`
}

// Policies defines security constraints and governance rules.
type Policies struct {
	// List of allowed network domains the agent can access.
	AllowedDomains []string `yaml:"allowed_domains,omitempty" json:"allowed_domains,omitempty"`
	// Human-in-the-loop approval requirements.
	HumanInTheLoop []HITLRule `yaml:"human_in_the_loop,omitempty" json:"human_in_the_loop,omitempty"`
	// Fine-grained permissions per skill.
	ToolPermissions []ToolPermission `yaml:"tool_permissions,omitempty" json:"tool_permissions,omitempty"`
	// Global token budget per session.
	MaxTokens *int `yaml:"max_tokens,omitempty" json:"max_tokens,omitempty" jsonschema:"minimum=1"`
	// Maximum conversation turns per session.
	MaxTurns *int `yaml:"max_turns,omitempty" json:"max_turns,omitempty" jsonschema:"minimum=1"`
}

// HITLRule defines a human-in-the-loop approval requirement.
type HITLRule struct {
	// Skill name that requires human approval.
	Tool string `yaml:"tool" json:"tool" jsonschema:"required"`
	// Condition expression for when HITL is required (e.g., "always", "cost > 100").
	Condition string `yaml:"condition" json:"condition" jsonschema:"required"`
}

// ToolPermission defines fine-grained permissions for a skill.
type ToolPermission struct {
	// Skill name to assign permissions to.
	Skill string `yaml:"skill" json:"skill" jsonschema:"required"`
	// List of allowed operations for this skill.
	Allow []string `yaml:"allow" json:"allow" jsonschema:"required"`
}

// ProfileBrain allows profile overrides to switch the active model.
type ProfileBrain struct {
	// Name of the model configuration to activate for this profile. Must match a name in brain.models.
	Default string `yaml:"default" json:"default" jsonschema:"required,pattern=^[a-z0-9][a-z0-9-]{0\\,31}$"`
}

// Profile represents environment-specific configuration overrides (dev/staging/prod).
// Profile brain overrides can only switch the default model; they cannot add new models.
type Profile struct {
	// Profile brain override. Can only switch the active model via default.
	Brain *ProfileBrain `yaml:"brain,omitempty" json:"brain,omitempty"`
	// Profile-specific policy overrides.
	Policies *Policies `yaml:"policies,omitempty" json:"policies,omitempty"`
}
