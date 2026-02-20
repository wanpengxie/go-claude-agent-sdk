package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestParsePermissionUpdate(t *testing.T) {
	raw := map[string]any{
		"type":        "tool",
		"behavior":    "allow",
		"mode":        "bypassPermissions",
		"destination": "project",
		"directories": []any{"/tmp", "/home"},
		"rules": []any{
			map[string]any{"toolName": "Bash", "ruleContent": "echo *"},
		},
	}

	pu := parsePermissionUpdate(raw)

	if pu.Type != PermissionUpdateType("tool") {
		t.Errorf("expected type 'tool', got %q", pu.Type)
	}
	if pu.Behavior != PermissionBehavior("allow") {
		t.Errorf("expected behavior 'allow', got %q", pu.Behavior)
	}
	if pu.Mode != PermissionMode("bypassPermissions") {
		t.Errorf("expected mode 'bypassPermissions', got %q", pu.Mode)
	}
	if pu.Destination != PermissionUpdateDestination("project") {
		t.Errorf("expected destination 'project', got %q", pu.Destination)
	}
	if len(pu.Directories) != 2 || pu.Directories[0] != "/tmp" {
		t.Errorf("unexpected directories: %v", pu.Directories)
	}
	if len(pu.Rules) != 1 || pu.Rules[0].ToolName != "Bash" || pu.Rules[0].RuleContent != "echo *" {
		t.Errorf("unexpected rules: %v", pu.Rules)
	}
}

func TestParsePermissionUpdateEmpty(t *testing.T) {
	pu := parsePermissionUpdate(map[string]any{})
	if pu.Type != "" || pu.Behavior != "" || len(pu.Directories) != 0 {
		t.Errorf("expected empty PermissionUpdate, got %+v", pu)
	}
}

func TestParseHookInput(t *testing.T) {
	isInterrupt := true
	raw := map[string]any{
		"session_id":      "sess-123",
		"cwd":             "/tmp",
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": "ls"},
		"tool_use_id":     "tu-1",
		"is_interrupt":    isInterrupt,
		"hook_event_name": "PreToolUse",
		"prompt":          "test prompt",
	}

	input := parseHookInput(raw)

	if input.SessionID != "sess-123" {
		t.Errorf("expected session_id 'sess-123', got %q", input.SessionID)
	}
	if input.Cwd != "/tmp" {
		t.Errorf("expected cwd '/tmp', got %q", input.Cwd)
	}
	if input.ToolName != "Bash" {
		t.Errorf("expected tool_name 'Bash', got %q", input.ToolName)
	}
	if input.ToolInput == nil || input.ToolInput["command"] != "ls" {
		t.Errorf("unexpected tool_input: %v", input.ToolInput)
	}
	if input.IsInterrupt == nil || *input.IsInterrupt != true {
		t.Error("expected is_interrupt to be true")
	}
	if input.HookEventName != "PreToolUse" {
		t.Errorf("expected hook_event_name 'PreToolUse', got %q", input.HookEventName)
	}
	if input.Prompt != "test prompt" {
		t.Errorf("expected prompt 'test prompt', got %q", input.Prompt)
	}
}

func TestConvertHookOutputForCLI(t *testing.T) {
	cont := true
	suppress := false
	asyncTimeout := 30
	output := &HookJSONOutput{
		Continue:       &cont,
		SuppressOutput: &suppress,
		AsyncTimeout:   &asyncTimeout,
		StopReason:     "done",
		Decision:       "allow",
		Reason:         "safe command",
		HookSpecificOutput: &HookSpecificOutput{
			HookEventName:            "PreToolUse",
			PermissionDecision:       "allow",
			PermissionDecisionReason: "trusted",
			AdditionalContext:        "extra info",
		},
	}

	result := convertHookOutputForCLI(output)

	if result["continue"] != true {
		t.Error("expected continue=true")
	}
	if result["suppressOutput"] != false {
		t.Error("expected suppressOutput=false")
	}
	if result["asyncTimeout"] != 30 {
		t.Error("expected asyncTimeout=30")
	}
	if result["stopReason"] != "done" {
		t.Errorf("expected stopReason='done', got %v", result["stopReason"])
	}
	if result["decision"] != "allow" {
		t.Errorf("expected decision='allow', got %v", result["decision"])
	}
	if result["reason"] != "safe command" {
		t.Errorf("expected reason='safe command', got %v", result["reason"])
	}

	hso, ok := result["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("expected hookSpecificOutput to be a map")
	}
	if hso["hookEventName"] != "PreToolUse" {
		t.Errorf("expected hookEventName='PreToolUse', got %v", hso["hookEventName"])
	}
	if hso["permissionDecision"] != "allow" {
		t.Errorf("expected permissionDecision='allow', got %v", hso["permissionDecision"])
	}
	if hso["additionalContext"] != "extra info" {
		t.Errorf("expected additionalContext='extra info', got %v", hso["additionalContext"])
	}
}

func TestConvertHookOutputForCLINil(t *testing.T) {
	output := &HookJSONOutput{}
	result := convertHookOutputForCLI(output)
	if len(result) != 0 {
		t.Errorf("expected empty map for nil fields, got %v", result)
	}
}

// mockTransport is a test transport for queryHandler tests.
type mockTransport struct {
	msgChan chan map[string]any
	errChan chan error
	written []string
	mu      sync.Mutex
	closed  bool
	lastErr error
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		msgChan: make(chan map[string]any, 100),
		errChan: make(chan error, 10),
	}
}

func (m *mockTransport) Write(data string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.written = append(m.written, data)
	return nil
}

func (m *mockTransport) Messages() <-chan map[string]any { return m.msgChan }
func (m *mockTransport) Errors() <-chan error            { return m.errChan }
func (m *mockTransport) LastError() error                { return m.lastErr }
func (m *mockTransport) Close() error                    { m.closed = true; return nil }
func (m *mockTransport) EndInput() error                 { return nil }
func (m *mockTransport) IsReady() bool                   { return true }

func (m *mockTransport) getWritten() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]string, len(m.written))
	copy(cp, m.written)
	return cp
}

func TestQueryHandlerCanUseTool(t *testing.T) {
	mt := newMockTransport()

	handler := newQueryHandler(mt, queryOptions{
		CanUseTool: func(ctx context.Context, toolName string, input map[string]any, permCtx ToolPermissionContext) (PermissionResult, error) {
			if toolName == "Bash" {
				return &PermissionResultDeny{Message: "denied"}, nil
			}
			return &PermissionResultAllow{}, nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = handler.start(ctx)
	defer handler.close()

	// Simulate can_use_tool control request from CLI
	mt.msgChan <- map[string]any{
		"type":       "control_request",
		"request_id": "req_1",
		"request": map[string]any{
			"subtype":   "can_use_tool",
			"tool_name": "Bash",
			"input":     map[string]any{"command": "rm -rf /"},
		},
	}

	// Wait for handler to process
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for deny response")
			return
		default:
		}
		time.Sleep(10 * time.Millisecond)
		written := mt.getWritten()
		if len(written) > 0 {
			var resp map[string]any
			_ = json.Unmarshal([]byte(written[0]), &resp)
			response, _ := resp["response"].(map[string]any)
			inner, _ := response["response"].(map[string]any)
			if inner != nil && inner["behavior"] == "deny" && inner["message"] == "denied" {
				return // success
			}
		}
	}
}

func TestQueryHandlerMcpMessage(t *testing.T) {
	addTool := NewMCPTool("add", "Add two numbers",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{"type": "number"},
				"b": map[string]any{"type": "number"},
			},
		},
		func(ctx context.Context, args map[string]any) (MCPToolResult, error) {
			a, _ := args["a"].(float64)
			b, _ := args["b"].(float64)
			return MCPToolResult{
				Content: []MCPContent{{Type: "text", Text: fmt.Sprintf("%.0f", a+b)}},
			}, nil
		},
	)
	serverConfig := CreateSdkMcpServer("calc", "1.0.0", addTool)

	mt := newMockTransport()
	handler := newQueryHandler(mt, queryOptions{
		SdkMcpServers: map[string]*McpServer{"calc": serverConfig.Instance},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = handler.start(ctx)
	defer handler.close()

	// Send MCP tools/list request
	mt.msgChan <- map[string]any{
		"type":       "control_request",
		"request_id": "req_mcp_1",
		"request": map[string]any{
			"subtype":     "mcp_message",
			"server_name": "calc",
			"message": map[string]any{
				"jsonrpc": "2.0",
				"id":      float64(1),
				"method":  "tools/list",
			},
		},
	}

	// Wait for response
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for MCP tools/list response")
			return
		default:
		}
		time.Sleep(10 * time.Millisecond)
		written := mt.getWritten()
		if len(written) > 0 {
			var resp map[string]any
			_ = json.Unmarshal([]byte(written[0]), &resp)
			response, _ := resp["response"].(map[string]any)
			inner, _ := response["response"].(map[string]any)
			if inner != nil {
				mcpResp, _ := inner["mcp_response"].(map[string]any)
				if mcpResp != nil {
					result, _ := mcpResp["result"].(map[string]any)
					tools, _ := result["tools"].([]any)
					if len(tools) == 1 {
						return // success
					}
				}
			}
		}
	}
}

func TestQueryHandlerSDKMessages(t *testing.T) {
	mt := newMockTransport()
	handler := newQueryHandler(mt, queryOptions{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = handler.start(ctx)

	// Send a regular SDK message
	mt.msgChan <- map[string]any{
		"type":    "assistant",
		"message": map[string]any{"role": "assistant"},
		"content": []any{map[string]any{"type": "text", "text": "hello"}},
	}

	// Read from receiveMessages
	select {
	case msg := <-handler.receiveMessages():
		if msg["type"] != "assistant" {
			t.Errorf("expected type 'assistant', got %v", msg["type"])
		}
	case <-ctx.Done():
		t.Fatal("context cancelled waiting for message")
	}

	handler.close()
}

func TestQueryHandlerPropagatesTransportError(t *testing.T) {
	mt := newMockTransport()
	handler := newQueryHandler(mt, queryOptions{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = handler.start(ctx)
	defer handler.close()

	mt.errChan <- errors.New("transport boom")

	select {
	case msg := <-handler.receiveMessages():
		if msg["type"] != "error" {
			t.Fatalf("expected error message type, got %v", msg["type"])
		}
		if msg["error"] != "transport boom" {
			t.Fatalf("expected error payload, got %v", msg["error"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for propagated transport error")
	}

	if handler.err() == nil {
		t.Fatal("expected handler.err() to be set")
	}
}
