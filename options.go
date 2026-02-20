package claude

import (
	"io"
	"os"
)

// AgentOptions holds all configuration options for Claude SDK queries and clients.
type AgentOptions struct {
	// Tools specifies the base set of tools. Can be a list of tool names, or nil.
	Tools []string

	// ToolsPreset specifies a tools preset instead of a list.
	ToolsPreset *ToolsPreset

	// AllowedTools specifies additional allowed tools.
	AllowedTools []string

	// DisallowedTools specifies tools to disallow.
	DisallowedTools []string

	// SystemPrompt sets a custom system prompt. Empty string clears the default.
	SystemPrompt *string

	// SystemPromptPreset sets a system prompt preset.
	SystemPromptPreset *SystemPromptPreset

	// McpServers maps server names to their configurations.
	McpServers map[string]McpServerConfig

	// McpServersPath is an alternative path/JSON string for MCP config.
	McpServersPath string

	// PermissionMode controls tool execution permissions.
	PermissionMode PermissionMode

	// ContinueConversation continues the previous conversation.
	ContinueConversation bool

	// Resume resumes a specific session by ID.
	Resume string

	// MaxTurns limits the number of conversation turns.
	MaxTurns int

	// MaxBudgetUSD sets the maximum budget in USD.
	MaxBudgetUSD *float64

	// Model specifies the AI model to use.
	Model string

	// FallbackModel specifies a fallback model.
	FallbackModel string

	// Betas enables beta features.
	Betas []SdkBeta

	// PermissionPromptToolName sets the permission prompt tool name.
	PermissionPromptToolName string

	// Cwd sets the working directory.
	Cwd string

	// CLIPath sets the path to the Claude Code CLI.
	CLIPath string

	// Settings sets the settings file path or JSON string.
	Settings string

	// AddDirs specifies additional directories.
	AddDirs []string

	// Env sets additional environment variables.
	Env map[string]string

	// ExtraArgs passes arbitrary CLI flags (flag -> value, nil value for boolean flags).
	ExtraArgs map[string]*string

	// MaxBufferSize sets the maximum bytes when buffering CLI stdout.
	MaxBufferSize int

	// Stderr is a callback for stderr output from CLI.
	Stderr func(string)

	// DebugStderr is a deprecated fallback writer used when debug-to-stderr
	// is enabled and Stderr callback is not set.
	DebugStderr io.Writer

	// CanUseTool is a callback for tool permission requests.
	CanUseTool CanUseToolFunc

	// Hooks maps hook events to their matcher configurations.
	Hooks map[HookEvent][]HookMatcher

	// User sets the user for the CLI process.
	User string

	// IncludePartialMessages enables partial message streaming.
	IncludePartialMessages bool

	// ForkSession forks resumed sessions to a new session ID.
	ForkSession bool

	// Agents defines custom agent configurations.
	Agents map[string]AgentDefinition

	// SettingSources specifies which setting sources to load.
	SettingSources []SettingSource

	// Sandbox configures sandbox settings for bash command isolation.
	Sandbox *SandboxSettings

	// Plugins specifies plugin configurations.
	Plugins []SdkPluginConfig

	// MaxThinkingTokens is deprecated: use Thinking instead.
	MaxThinkingTokens *int

	// Thinking controls extended thinking behavior.
	Thinking ThinkingConfig

	// Effort sets the effort level for thinking depth.
	Effort Effort

	// OutputFormat for structured outputs (e.g., JSON schema).
	OutputFormat map[string]any

	// EnableFileCheckpointing enables file checkpointing.
	EnableFileCheckpointing bool
}

// Option is a functional option for configuring AgentOptions.
type Option func(*AgentOptions)

// WithTools sets the base set of tools.
func WithTools(tools ...string) Option {
	return func(o *AgentOptions) {
		o.Tools = tools
		o.ToolsPreset = nil
	}
}

// WithToolsPreset sets a tools preset instead of an explicit tool list.
func WithToolsPreset(preset ToolsPreset) Option {
	return func(o *AgentOptions) {
		o.ToolsPreset = &preset
		o.Tools = nil
	}
}

// WithAllowedTools sets additional allowed tools.
func WithAllowedTools(tools ...string) Option {
	return func(o *AgentOptions) { o.AllowedTools = tools }
}

// WithDisallowedTools sets tools to disallow.
func WithDisallowedTools(tools ...string) Option {
	return func(o *AgentOptions) { o.DisallowedTools = tools }
}

// WithSystemPrompt sets a custom system prompt.
func WithSystemPrompt(prompt string) Option {
	return func(o *AgentOptions) {
		o.SystemPrompt = &prompt
		o.SystemPromptPreset = nil
	}
}

// WithSystemPromptPreset sets a system prompt preset.
func WithSystemPromptPreset(preset SystemPromptPreset) Option {
	return func(o *AgentOptions) {
		o.SystemPromptPreset = &preset
		o.SystemPrompt = nil
	}
}

// WithMcpServers sets MCP server configurations.
func WithMcpServers(servers map[string]McpServerConfig) Option {
	return func(o *AgentOptions) {
		o.McpServers = servers
		o.McpServersPath = ""
	}
}

// WithMcpServersPath sets MCP server config as a path or raw JSON string.
func WithMcpServersPath(pathOrJSON string) Option {
	return func(o *AgentOptions) {
		o.McpServersPath = pathOrJSON
		o.McpServers = nil
	}
}

// WithPermissionMode sets the permission mode.
func WithPermissionMode(mode PermissionMode) Option {
	return func(o *AgentOptions) { o.PermissionMode = mode }
}

// WithContinueConversation enables continuing the previous conversation.
func WithContinueConversation() Option {
	return func(o *AgentOptions) { o.ContinueConversation = true }
}

// WithResume resumes a specific session by ID.
func WithResume(sessionID string) Option {
	return func(o *AgentOptions) { o.Resume = sessionID }
}

// WithMaxTurns sets the maximum number of turns.
func WithMaxTurns(n int) Option {
	return func(o *AgentOptions) { o.MaxTurns = n }
}

// WithMaxBudgetUSD sets the maximum budget in USD.
func WithMaxBudgetUSD(budget float64) Option {
	return func(o *AgentOptions) { o.MaxBudgetUSD = &budget }
}

// WithModel sets the AI model.
func WithModel(model string) Option {
	return func(o *AgentOptions) { o.Model = model }
}

// WithFallbackModel sets a fallback model.
func WithFallbackModel(model string) Option {
	return func(o *AgentOptions) { o.FallbackModel = model }
}

// WithBetas enables beta features.
func WithBetas(betas ...SdkBeta) Option {
	return func(o *AgentOptions) { o.Betas = betas }
}

// WithPermissionPromptToolName sets the permission prompt tool name.
func WithPermissionPromptToolName(name string) Option {
	return func(o *AgentOptions) { o.PermissionPromptToolName = name }
}

// WithCwd sets the working directory.
func WithCwd(cwd string) Option {
	return func(o *AgentOptions) { o.Cwd = cwd }
}

// WithCLIPath sets the path to the Claude Code CLI.
func WithCLIPath(path string) Option {
	return func(o *AgentOptions) { o.CLIPath = path }
}

// WithSettings sets the settings file path or JSON string.
func WithSettings(settings string) Option {
	return func(o *AgentOptions) { o.Settings = settings }
}

// WithAddDirs sets additional directories.
func WithAddDirs(dirs ...string) Option {
	return func(o *AgentOptions) { o.AddDirs = dirs }
}

// WithEnv sets additional environment variables.
func WithEnv(env map[string]string) Option {
	return func(o *AgentOptions) { o.Env = env }
}

// WithExtraArgs passes arbitrary CLI flags.
func WithExtraArgs(args map[string]*string) Option {
	return func(o *AgentOptions) { o.ExtraArgs = args }
}

// WithMaxBufferSize sets max bytes used when buffering CLI stdout JSON.
func WithMaxBufferSize(size int) Option {
	return func(o *AgentOptions) { o.MaxBufferSize = size }
}

// WithStderr sets a callback for stderr output.
func WithStderr(fn func(string)) Option {
	return func(o *AgentOptions) { o.Stderr = fn }
}

// WithDebugStderr sets deprecated fallback writer for debug-to-stderr output.
func WithDebugStderr(w io.Writer) Option {
	return func(o *AgentOptions) { o.DebugStderr = w }
}

// WithCanUseTool sets the tool permission callback.
func WithCanUseTool(fn CanUseToolFunc) Option {
	return func(o *AgentOptions) { o.CanUseTool = fn }
}

// WithHooks sets hook configurations.
func WithHooks(hooks map[HookEvent][]HookMatcher) Option {
	return func(o *AgentOptions) { o.Hooks = hooks }
}

// WithUser sets the OS user used to run the CLI subprocess.
func WithUser(user string) Option {
	return func(o *AgentOptions) { o.User = user }
}

// WithIncludePartialMessages enables partial message streaming.
func WithIncludePartialMessages() Option {
	return func(o *AgentOptions) { o.IncludePartialMessages = true }
}

// WithForkSession enables forking resumed sessions.
func WithForkSession() Option {
	return func(o *AgentOptions) { o.ForkSession = true }
}

// WithAgents sets custom agent configurations.
func WithAgents(agents map[string]AgentDefinition) Option {
	return func(o *AgentOptions) { o.Agents = agents }
}

// WithSettingSources specifies setting sources to load.
func WithSettingSources(sources ...SettingSource) Option {
	return func(o *AgentOptions) { o.SettingSources = sources }
}

// WithSandbox sets sandbox configuration.
func WithSandbox(sandbox *SandboxSettings) Option {
	return func(o *AgentOptions) { o.Sandbox = sandbox }
}

// WithPlugins sets plugin configurations.
func WithPlugins(plugins ...SdkPluginConfig) Option {
	return func(o *AgentOptions) { o.Plugins = plugins }
}

// WithMaxThinkingTokens sets deprecated max-thinking-tokens directly.
func WithMaxThinkingTokens(tokens int) Option {
	return func(o *AgentOptions) { o.MaxThinkingTokens = &tokens }
}

// WithThinking sets the thinking configuration.
func WithThinking(config ThinkingConfig) Option {
	return func(o *AgentOptions) { o.Thinking = config }
}

// WithEffort sets the effort level.
func WithEffort(effort Effort) Option {
	return func(o *AgentOptions) { o.Effort = effort }
}

// WithOutputFormat sets the output format for structured outputs.
func WithOutputFormat(format map[string]any) Option {
	return func(o *AgentOptions) { o.OutputFormat = format }
}

// WithEnableFileCheckpointing enables file checkpointing.
func WithEnableFileCheckpointing() Option {
	return func(o *AgentOptions) { o.EnableFileCheckpointing = true }
}

// applyOptions creates AgentOptions from functional options.
func applyOptions(opts []Option) *AgentOptions {
	o := &AgentOptions{}
	for _, opt := range opts {
		opt(o)
	}
	if o.DebugStderr == nil {
		o.DebugStderr = os.Stderr
	}
	return o
}
