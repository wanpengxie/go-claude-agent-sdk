package claude

import (
	"testing"
)

func TestTextBlock(t *testing.T) {
	b := &TextBlock{Text: "hello"}
	if b.contentBlockType() != "text" {
		t.Errorf("expected type 'text', got %q", b.contentBlockType())
	}
	if b.Text != "hello" {
		t.Errorf("expected text 'hello', got %q", b.Text)
	}
}

func TestThinkingBlock(t *testing.T) {
	b := &ThinkingBlock{Thinking: "reasoning", Signature: "sig123"}
	if b.contentBlockType() != "thinking" {
		t.Errorf("expected type 'thinking', got %q", b.contentBlockType())
	}
	if b.Signature != "sig123" {
		t.Errorf("unexpected signature: %q", b.Signature)
	}
}

func TestToolUseBlock(t *testing.T) {
	b := &ToolUseBlock{
		ID:    "tu-1",
		Name:  "Bash",
		Input: map[string]any{"command": "ls"},
	}
	if b.contentBlockType() != "tool_use" {
		t.Errorf("expected type 'tool_use', got %q", b.contentBlockType())
	}
	if b.Name != "Bash" {
		t.Errorf("expected name 'Bash', got %q", b.Name)
	}
}

func TestToolResultBlock(t *testing.T) {
	isErr := true
	b := &ToolResultBlock{
		ToolUseID: "tu-1",
		Content:   "output",
		IsError:   &isErr,
	}
	if b.contentBlockType() != "tool_result" {
		t.Errorf("expected type 'tool_result', got %q", b.contentBlockType())
	}
	if b.IsError == nil || !*b.IsError {
		t.Error("expected IsError to be true")
	}
}

func TestUserMessage(t *testing.T) {
	msg := &UserMessage{
		Content: "hello",
	}
	if msg.messageType() != "user" {
		t.Errorf("expected type 'user', got %q", msg.messageType())
	}
}

func TestAssistantMessage(t *testing.T) {
	msg := &AssistantMessage{
		Content: []ContentBlock{
			&TextBlock{Text: "response"},
		},
	}
	if msg.messageType() != "assistant" {
		t.Errorf("expected type 'assistant', got %q", msg.messageType())
	}
	if len(msg.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(msg.Content))
	}
	tb, ok := msg.Content[0].(*TextBlock)
	if !ok {
		t.Fatal("expected TextBlock")
	}
	if tb.Text != "response" {
		t.Errorf("expected text 'response', got %q", tb.Text)
	}
}

func TestAssistantMessageWithThinking(t *testing.T) {
	msg := &AssistantMessage{
		Content: []ContentBlock{
			&ThinkingBlock{Thinking: "let me think", Signature: "sig-abc"},
			&TextBlock{Text: "answer"},
		},
	}
	if len(msg.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(msg.Content))
	}
	thinking, ok := msg.Content[0].(*ThinkingBlock)
	if !ok {
		t.Fatal("expected ThinkingBlock")
	}
	if thinking.Signature != "sig-abc" {
		t.Errorf("unexpected signature: %q", thinking.Signature)
	}
}

func TestResultMessage(t *testing.T) {
	cost := 0.05
	msg := &ResultMessage{
		Subtype:      "success",
		IsError:      false,
		DurationMS:   1500,
		NumTurns:     3,
		SessionID:    "sess-123",
		TotalCostUSD: &cost,
	}
	if msg.messageType() != "result" {
		t.Errorf("expected type 'result', got %q", msg.messageType())
	}
	if msg.NumTurns != 3 {
		t.Errorf("expected 3 turns, got %d", msg.NumTurns)
	}
	if msg.TotalCostUSD == nil || *msg.TotalCostUSD != 0.05 {
		t.Errorf("expected cost 0.05, got %v", msg.TotalCostUSD)
	}
}

func TestSystemMessage(t *testing.T) {
	msg := &SystemMessage{
		Subtype: "init",
	}
	if msg.messageType() != "system" {
		t.Errorf("expected type 'system', got %q", msg.messageType())
	}
}

func TestStreamEvent(t *testing.T) {
	msg := &StreamEvent{
		Event: map[string]any{"type": "content_block_delta"},
	}
	if msg.messageType() != "stream_event" {
		t.Errorf("expected type 'stream_event', got %q", msg.messageType())
	}
}

func TestMessageTypeSwitch(t *testing.T) {
	messages := []Message{
		&UserMessage{Content: "hi"},
		&AssistantMessage{},
		&SystemMessage{Subtype: "init"},
		&ResultMessage{Subtype: "success"},
		&StreamEvent{},
	}

	types := make([]string, len(messages))
	for i, msg := range messages {
		switch msg.(type) {
		case *UserMessage:
			types[i] = "user"
		case *AssistantMessage:
			types[i] = "assistant"
		case *SystemMessage:
			types[i] = "system"
		case *ResultMessage:
			types[i] = "result"
		case *StreamEvent:
			types[i] = "stream"
		}
	}

	expected := []string{"user", "assistant", "system", "result", "stream"}
	for i, exp := range expected {
		if types[i] != exp {
			t.Errorf("index %d: expected %q, got %q", i, exp, types[i])
		}
	}
}

func TestPermissionResultTypeSwitch(t *testing.T) {
	results := []PermissionResult{
		&PermissionResultAllow{},
		&PermissionResultDeny{Message: "denied"},
	}

	for i, r := range results {
		switch v := r.(type) {
		case *PermissionResultAllow:
			if i != 0 {
				t.Error("expected allow at index 0")
			}
		case *PermissionResultDeny:
			if i != 1 {
				t.Error("expected deny at index 1")
			}
			if v.Message != "denied" {
				t.Errorf("expected message 'denied', got %q", v.Message)
			}
		}
	}
}

func TestPermissionUpdateToDictWithRules(t *testing.T) {
	pu := PermissionUpdate{
		Type:        PermissionUpdateAddRules,
		Behavior:    PermissionBehavior("allow"),
		Destination: PermissionDestProjectSettings,
		Rules: []PermissionRuleValue{
			{ToolName: "Bash", RuleContent: "echo *"},
		},
	}

	d := pu.ToDict()
	if d["type"] != string(PermissionUpdateAddRules) {
		t.Errorf("expected type '%s', got %v", PermissionUpdateAddRules, d["type"])
	}
	if d["behavior"] != "allow" {
		t.Errorf("expected behavior 'allow', got %v", d["behavior"])
	}
	if d["destination"] != string(PermissionDestProjectSettings) {
		t.Errorf("expected destination '%s', got %v", PermissionDestProjectSettings, d["destination"])
	}
	rules, ok := d["rules"].([]map[string]any)
	if !ok || len(rules) != 1 {
		t.Fatalf("unexpected rules: %v", d["rules"])
	}
	if rules[0]["toolName"] != "Bash" {
		t.Errorf("expected toolName 'Bash', got %v", rules[0]["toolName"])
	}
}

func TestPermissionUpdateToDictWithDirectories(t *testing.T) {
	pu := PermissionUpdate{
		Type:        PermissionUpdateAddDirectories,
		Directories: []string{"/tmp", "/home"},
	}

	d := pu.ToDict()
	dirs, ok := d["directories"].([]string)
	if !ok || len(dirs) != 2 || dirs[0] != "/tmp" {
		t.Errorf("unexpected directories: %v", d["directories"])
	}
}

func TestPermissionUpdateToDictWithMode(t *testing.T) {
	pu := PermissionUpdate{
		Type: PermissionUpdateSetMode,
		Mode: PermissionBypassPermissions,
	}

	d := pu.ToDict()
	if d["mode"] != string(PermissionBypassPermissions) {
		t.Errorf("expected mode '%s', got %v", PermissionBypassPermissions, d["mode"])
	}
}

func TestMcpServerConfigTypeSwitch(t *testing.T) {
	configs := []McpServerConfig{
		&McpStdioServerConfig{Type: "stdio", Command: "npx"},
		&McpSSEServerConfig{Type: "sse", URL: "http://localhost:3000"},
		&McpHTTPServerConfig{Type: "http", URL: "http://localhost:3001"},
		&McpSdkServerConfig{Type: "sdk", Name: "calc"},
	}

	for i, cfg := range configs {
		switch cfg.(type) {
		case *McpStdioServerConfig:
			if i != 0 {
				t.Errorf("expected stdio at index 0, got index %d", i)
			}
		case *McpSSEServerConfig:
			if i != 1 {
				t.Errorf("expected sse at index 1, got index %d", i)
			}
		case *McpHTTPServerConfig:
			if i != 2 {
				t.Errorf("expected http at index 2, got index %d", i)
			}
		case *McpSdkServerConfig:
			if i != 3 {
				t.Errorf("expected sdk at index 3, got index %d", i)
			}
		}
	}
}

func TestThinkingConfigTypeSwitch(t *testing.T) {
	configs := []ThinkingConfig{
		&ThinkingConfigAdaptive{},
		&ThinkingConfigEnabled{BudgetTokens: 1000},
		&ThinkingConfigDisabled{},
	}

	for i, cfg := range configs {
		switch v := cfg.(type) {
		case *ThinkingConfigAdaptive:
			if i != 0 {
				t.Error("expected adaptive at index 0")
			}
		case *ThinkingConfigEnabled:
			if i != 1 {
				t.Error("expected enabled at index 1")
			}
			if v.BudgetTokens != 1000 {
				t.Errorf("expected 1000 tokens, got %d", v.BudgetTokens)
			}
		case *ThinkingConfigDisabled:
			if i != 2 {
				t.Error("expected disabled at index 2")
			}
		}
	}
}

func TestPermissionResultAllow(t *testing.T) {
	allow := &PermissionResultAllow{
		UpdatedInput: map[string]any{"command": "echo hi"},
		UpdatedPermissions: []PermissionUpdate{
			{Type: "tool", Behavior: "allow"},
		},
	}
	if allow.UpdatedInput["command"] != "echo hi" {
		t.Error("expected updated input preserved")
	}
	if len(allow.UpdatedPermissions) != 1 {
		t.Error("expected 1 updated permission")
	}
}

func TestPermissionResultDeny(t *testing.T) {
	deny := &PermissionResultDeny{
		Message:   "not allowed",
		Interrupt: true,
	}
	if deny.Message != "not allowed" {
		t.Errorf("expected message 'not allowed', got %q", deny.Message)
	}
	if !deny.Interrupt {
		t.Error("expected interrupt to be true")
	}
}
