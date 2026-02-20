package claude

import (
	"context"
	"testing"
)

func TestApplyOptions(t *testing.T) {
	budget := 5.0
	opts := applyOptions([]Option{
		WithModel("claude-sonnet-4-5"),
		WithMaxTurns(10),
		WithMaxBudgetUSD(budget),
		WithPermissionMode(PermissionAcceptEdits),
		WithAllowedTools("Read", "Write"),
		WithCwd("/tmp"),
		WithEffort(EffortHigh),
	})

	if opts.Model != "claude-sonnet-4-5" {
		t.Errorf("expected model 'claude-sonnet-4-5', got %s", opts.Model)
	}
	if opts.MaxTurns != 10 {
		t.Errorf("expected max_turns 10, got %d", opts.MaxTurns)
	}
	if opts.MaxBudgetUSD == nil || *opts.MaxBudgetUSD != 5.0 {
		t.Errorf("expected max_budget_usd 5.0, got %v", opts.MaxBudgetUSD)
	}
	if opts.PermissionMode != PermissionAcceptEdits {
		t.Errorf("expected permission mode 'acceptEdits', got %s", opts.PermissionMode)
	}
	if len(opts.AllowedTools) != 2 || opts.AllowedTools[0] != "Read" {
		t.Errorf("expected AllowedTools [Read, Write], got %v", opts.AllowedTools)
	}
	if opts.Cwd != "/tmp" {
		t.Errorf("expected cwd '/tmp', got %s", opts.Cwd)
	}
	if opts.Effort != EffortHigh {
		t.Errorf("expected effort 'high', got %s", opts.Effort)
	}
}

func TestWithSystemPrompt(t *testing.T) {
	opts := applyOptions([]Option{WithSystemPrompt("You are a helpful assistant")})
	if opts.SystemPrompt == nil || *opts.SystemPrompt != "You are a helpful assistant" {
		t.Errorf("expected system prompt, got %v", opts.SystemPrompt)
	}
}

func TestWithThinking(t *testing.T) {
	opts := applyOptions([]Option{WithThinking(&ThinkingConfigEnabled{BudgetTokens: 16000})})
	if opts.Thinking == nil {
		t.Fatal("expected thinking config")
	}
	tc, ok := opts.Thinking.(*ThinkingConfigEnabled)
	if !ok {
		t.Fatalf("expected *ThinkingConfigEnabled, got %T", opts.Thinking)
	}
	if tc.BudgetTokens != 16000 {
		t.Errorf("expected 16000 budget tokens, got %d", tc.BudgetTokens)
	}
}

func TestWithHooks(t *testing.T) {
	opts := applyOptions([]Option{
		WithHooks(map[HookEvent][]HookMatcher{
			HookPreToolUse: {
				{
					Matcher: "Bash",
					Hooks:   nil,
				},
			},
		}),
	})
	if opts.Hooks == nil {
		t.Fatal("expected hooks")
	}
	matchers, ok := opts.Hooks[HookPreToolUse]
	if !ok || len(matchers) != 1 {
		t.Fatalf("expected 1 PreToolUse matcher, got %v", matchers)
	}
	if matchers[0].Matcher != "Bash" {
		t.Errorf("expected matcher 'Bash', got %s", matchers[0].Matcher)
	}
}

func TestWithDisallowedTools(t *testing.T) {
	opts := applyOptions([]Option{WithDisallowedTools("Bash", "Write")})
	if len(opts.DisallowedTools) != 2 || opts.DisallowedTools[0] != "Bash" {
		t.Errorf("expected [Bash, Write], got %v", opts.DisallowedTools)
	}
}

func TestWithTools(t *testing.T) {
	opts := applyOptions([]Option{WithTools("Read", "Write", "Bash")})
	if len(opts.Tools) != 3 || opts.Tools[0] != "Read" {
		t.Errorf("expected [Read, Write, Bash], got %v", opts.Tools)
	}
}

func TestWithMcpServers(t *testing.T) {
	servers := map[string]McpServerConfig{
		"test": &McpStdioServerConfig{Type: "stdio", Command: "npx"},
	}
	opts := applyOptions([]Option{WithMcpServers(servers)})
	if len(opts.McpServers) != 1 {
		t.Errorf("expected 1 MCP server, got %d", len(opts.McpServers))
	}
}

func TestWithContinueConversation(t *testing.T) {
	opts := applyOptions([]Option{WithContinueConversation()})
	if !opts.ContinueConversation {
		t.Error("expected ContinueConversation=true")
	}
}

func TestWithResume(t *testing.T) {
	opts := applyOptions([]Option{WithResume("sess-123")})
	if opts.Resume != "sess-123" {
		t.Errorf("expected 'sess-123', got %q", opts.Resume)
	}
}

func TestWithFallbackModel(t *testing.T) {
	opts := applyOptions([]Option{WithFallbackModel("claude-haiku")})
	if opts.FallbackModel != "claude-haiku" {
		t.Errorf("expected 'claude-haiku', got %q", opts.FallbackModel)
	}
}

func TestWithBetas(t *testing.T) {
	opts := applyOptions([]Option{WithBetas("feature-x", "feature-y")})
	if len(opts.Betas) != 2 || opts.Betas[0] != "feature-x" {
		t.Errorf("unexpected betas: %v", opts.Betas)
	}
}

func TestWithCLIPath(t *testing.T) {
	opts := applyOptions([]Option{WithCLIPath("/usr/local/bin/claude")})
	if opts.CLIPath != "/usr/local/bin/claude" {
		t.Errorf("expected CLI path, got %q", opts.CLIPath)
	}
}

func TestWithSettings(t *testing.T) {
	opts := applyOptions([]Option{WithSettings(`{"verbose": true}`)})
	if opts.Settings != `{"verbose": true}` {
		t.Error("expected settings string")
	}
}

func TestWithAddDirs(t *testing.T) {
	opts := applyOptions([]Option{WithAddDirs("/tmp", "/home")})
	if len(opts.AddDirs) != 2 {
		t.Errorf("expected 2 dirs, got %d", len(opts.AddDirs))
	}
}

func TestWithEnv(t *testing.T) {
	env := map[string]string{"FOO": "bar"}
	opts := applyOptions([]Option{WithEnv(env)})
	if opts.Env["FOO"] != "bar" {
		t.Error("expected FOO=bar in env")
	}
}

func TestWithExtraArgs(t *testing.T) {
	val := "true"
	args := map[string]*string{"--verbose": &val, "--debug": nil}
	opts := applyOptions([]Option{WithExtraArgs(args)})
	if len(opts.ExtraArgs) != 2 {
		t.Errorf("expected 2 extra args, got %d", len(opts.ExtraArgs))
	}
}

func TestWithStderr(t *testing.T) {
	called := false
	fn := func(s string) { called = true }
	opts := applyOptions([]Option{WithStderr(fn)})
	if opts.Stderr == nil {
		t.Fatal("expected stderr callback")
	}
	opts.Stderr("test")
	if !called {
		t.Error("expected stderr callback to be invoked")
	}
}

func TestWithCanUseTool(t *testing.T) {
	fn := func(ctx context.Context, toolName string, input map[string]any, permCtx ToolPermissionContext) (PermissionResult, error) {
		return &PermissionResultAllow{}, nil
	}
	opts := applyOptions([]Option{WithCanUseTool(fn)})
	if opts.CanUseTool == nil {
		t.Error("expected CanUseTool callback")
	}
}

func TestWithIncludePartialMessages(t *testing.T) {
	opts := applyOptions([]Option{WithIncludePartialMessages()})
	if !opts.IncludePartialMessages {
		t.Error("expected IncludePartialMessages=true")
	}
}

func TestWithForkSession(t *testing.T) {
	opts := applyOptions([]Option{WithForkSession()})
	if !opts.ForkSession {
		t.Error("expected ForkSession=true")
	}
}

func TestWithAgents(t *testing.T) {
	agents := map[string]AgentDefinition{
		"test-agent": {Description: "A test agent", Prompt: "Be helpful"},
	}
	opts := applyOptions([]Option{WithAgents(agents)})
	if len(opts.Agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(opts.Agents))
	}
	if opts.Agents["test-agent"].Description != "A test agent" {
		t.Error("unexpected agent description")
	}
}

func TestWithSandbox(t *testing.T) {
	enabled := true
	sandbox := &SandboxSettings{
		Enabled: &enabled,
	}
	opts := applyOptions([]Option{WithSandbox(sandbox)})
	if opts.Sandbox == nil {
		t.Fatal("expected sandbox settings")
	}
	if opts.Sandbox.Enabled == nil || !*opts.Sandbox.Enabled {
		t.Error("expected Enabled=true")
	}
}

func TestWithPlugins(t *testing.T) {
	plugin := SdkPluginConfig{Type: "local", Path: "/path/to/plugin"}
	opts := applyOptions([]Option{WithPlugins(plugin)})
	if len(opts.Plugins) != 1 || opts.Plugins[0].Path != "/path/to/plugin" {
		t.Errorf("unexpected plugins: %v", opts.Plugins)
	}
}

func TestWithOutputFormat(t *testing.T) {
	format := map[string]any{"type": "json_schema"}
	opts := applyOptions([]Option{WithOutputFormat(format)})
	if opts.OutputFormat["type"] != "json_schema" {
		t.Errorf("expected 'json_schema', got %v", opts.OutputFormat["type"])
	}
}

func TestWithEnableFileCheckpointing(t *testing.T) {
	opts := applyOptions([]Option{WithEnableFileCheckpointing()})
	if !opts.EnableFileCheckpointing {
		t.Error("expected EnableFileCheckpointing=true")
	}
}

func TestWithSettingSources(t *testing.T) {
	opts := applyOptions([]Option{WithSettingSources("user", "project")})
	if len(opts.SettingSources) != 2 {
		t.Errorf("expected 2 setting sources, got %d", len(opts.SettingSources))
	}
}

func TestWithMcpServersPath(t *testing.T) {
	opts := applyOptions([]Option{WithMcpServersPath("/tmp/mcp.json")})
	if opts.McpServersPath != "/tmp/mcp.json" {
		t.Errorf("expected mcp servers path to be set, got %q", opts.McpServersPath)
	}
	if opts.McpServers != nil {
		t.Errorf("expected McpServers to be cleared, got %v", opts.McpServers)
	}
}

func TestWithPermissionPromptToolName(t *testing.T) {
	opts := applyOptions([]Option{WithPermissionPromptToolName("stdio")})
	if opts.PermissionPromptToolName != "stdio" {
		t.Errorf("expected permission prompt tool name 'stdio', got %q", opts.PermissionPromptToolName)
	}
}

func TestWithMaxBufferSize(t *testing.T) {
	opts := applyOptions([]Option{WithMaxBufferSize(2048)})
	if opts.MaxBufferSize != 2048 {
		t.Errorf("expected max buffer size 2048, got %d", opts.MaxBufferSize)
	}
}

func TestWithUser(t *testing.T) {
	opts := applyOptions([]Option{WithUser("nobody")})
	if opts.User != "nobody" {
		t.Errorf("expected user 'nobody', got %q", opts.User)
	}
}

func TestWithMaxThinkingTokens(t *testing.T) {
	opts := applyOptions([]Option{WithMaxThinkingTokens(1234)})
	if opts.MaxThinkingTokens == nil || *opts.MaxThinkingTokens != 1234 {
		t.Errorf("expected max thinking tokens 1234, got %v", opts.MaxThinkingTokens)
	}
}

func TestWithToolsPreset(t *testing.T) {
	preset := ToolsPreset{Type: "preset", Preset: "claude_code"}
	opts := applyOptions([]Option{WithTools("Read"), WithToolsPreset(preset)})
	if opts.ToolsPreset == nil || opts.ToolsPreset.Preset != "claude_code" {
		t.Fatalf("expected tools preset to be set, got %+v", opts.ToolsPreset)
	}
	if opts.Tools != nil {
		t.Errorf("expected tools list to be cleared, got %v", opts.Tools)
	}
}

func TestWithSystemPromptPreset(t *testing.T) {
	preset := SystemPromptPreset{Type: "preset", Preset: "claude_code", Append: "extra"}
	opts := applyOptions([]Option{WithSystemPrompt("custom"), WithSystemPromptPreset(preset)})
	if opts.SystemPromptPreset == nil || opts.SystemPromptPreset.Preset != "claude_code" {
		t.Fatalf("expected system prompt preset to be set, got %+v", opts.SystemPromptPreset)
	}
	if opts.SystemPrompt != nil {
		t.Errorf("expected custom system prompt to be cleared, got %v", *opts.SystemPrompt)
	}
}
