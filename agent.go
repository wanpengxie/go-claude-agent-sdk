package claude

// SettingSource represents where settings are loaded from.
type SettingSource string

const (
	SettingSourceUser    SettingSource = "user"
	SettingSourceProject SettingSource = "project"
	SettingSourceLocal   SettingSource = "local"
)

// AgentDefinition represents an agent definition configuration.
type AgentDefinition struct {
	Description string   `json:"description"`
	Prompt      string   `json:"prompt"`
	Tools       []string `json:"tools,omitempty"`
	Model       string   `json:"model,omitempty"` // "sonnet", "opus", "haiku", "inherit"
}

// SystemPromptPreset represents a system prompt preset configuration.
type SystemPromptPreset struct {
	Type   string `json:"type"`   // "preset"
	Preset string `json:"preset"` // "claude_code"
	Append string `json:"append,omitempty"`
}

// ToolsPreset represents a tools preset configuration.
type ToolsPreset struct {
	Type   string `json:"type"`   // "preset"
	Preset string `json:"preset"` // "claude_code"
}

// SdkPluginConfig represents a plugin configuration.
type SdkPluginConfig struct {
	Type string `json:"type"` // "local"
	Path string `json:"path"`
}

// SdkBeta represents SDK beta features.
type SdkBeta string

const (
	SdkBetaContext1M SdkBeta = "context-1m-2025-08-07"
)

// ThinkingConfig represents configuration for extended thinking.
type ThinkingConfig interface {
	thinkingConfigType() string
}

// ThinkingConfigAdaptive uses adaptive thinking.
type ThinkingConfigAdaptive struct{}

func (c *ThinkingConfigAdaptive) thinkingConfigType() string { return "adaptive" }

// ThinkingConfigEnabled uses enabled thinking with a budget.
type ThinkingConfigEnabled struct {
	BudgetTokens int `json:"budget_tokens"`
}

func (c *ThinkingConfigEnabled) thinkingConfigType() string { return "enabled" }

// ThinkingConfigDisabled disables thinking.
type ThinkingConfigDisabled struct{}

func (c *ThinkingConfigDisabled) thinkingConfigType() string { return "disabled" }

// Effort represents the effort level for thinking depth.
type Effort string

const (
	EffortLow    Effort = "low"
	EffortMedium Effort = "medium"
	EffortHigh   Effort = "high"
	EffortMax    Effort = "max"
)
