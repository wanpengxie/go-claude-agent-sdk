package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// TestIntegrationSimpleQueryResponse tests a complete query -> assistant message -> result flow.
func TestIntegrationSimpleQueryResponse(t *testing.T) {
	mt := newMockTransport()
	handler := newQueryHandler(mt, queryOptions{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = handler.start(ctx)

	// Simulate initialize handshake
	go func() {
		// Wait for initialize request from handler
		time.Sleep(20 * time.Millisecond)
		written := mt.getWritten()
		for _, w := range written {
			var req map[string]any
			_ = json.Unmarshal([]byte(w), &req)
			if req["type"] == "control_request" {
				r, _ := req["request"].(map[string]any)
				if r["subtype"] == "initialize" {
					reqID, _ := req["request_id"].(string)
					mt.msgChan <- map[string]any{
						"type": "control_response",
						"response": map[string]any{
							"subtype":    "success",
							"request_id": reqID,
							"response":   map[string]any{"version": "2.0.0"},
						},
					}
				}
			}
		}
	}()

	_, err := handler.initialize(ctx)
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}

	// Send SDK messages
	mt.msgChan <- map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"role":  "assistant",
			"model": "claude-sonnet-4-5",
			"content": []any{
				map[string]any{"type": "text", "text": "The answer is 4."},
			},
		},
	}
	mt.msgChan <- map[string]any{
		"type":            "result",
		"subtype":         "success",
		"is_error":        false,
		"duration_ms":     float64(200),
		"duration_api_ms": float64(150),
		"num_turns":       float64(1),
		"session_id":      "test-session",
	}

	// Collect messages
	var messages []Message
	timeout := time.After(2 * time.Second)
	for {
		select {
		case raw, ok := <-handler.receiveMessages():
			if !ok {
				goto done
			}
			msg, parseErr := parseMessage(raw)
			if parseErr != nil {
				t.Fatalf("parse error: %v", parseErr)
			}
			messages = append(messages, msg)
			if _, isResult := msg.(*ResultMessage); isResult {
				goto done
			}
		case <-timeout:
			t.Fatal("timeout waiting for messages")
		}
	}
done:
	handler.close()

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	// Verify assistant message
	am, ok := messages[0].(*AssistantMessage)
	if !ok {
		t.Fatalf("expected AssistantMessage, got %T", messages[0])
	}
	if len(am.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(am.Content))
	}
	tb, ok := am.Content[0].(*TextBlock)
	if !ok {
		t.Fatalf("expected TextBlock, got %T", am.Content[0])
	}
	if tb.Text != "The answer is 4." {
		t.Errorf("unexpected text: %q", tb.Text)
	}

	// Verify result message
	rm, ok := messages[1].(*ResultMessage)
	if !ok {
		t.Fatalf("expected ResultMessage, got %T", messages[1])
	}
	if rm.Subtype != "success" {
		t.Errorf("expected subtype 'success', got %q", rm.Subtype)
	}
	if rm.NumTurns != 1 {
		t.Errorf("expected 1 turn, got %d", rm.NumTurns)
	}
}

// TestIntegrationToolUseFlow tests tool use in messages.
func TestIntegrationToolUseFlow(t *testing.T) {
	mt := newMockTransport()
	handler := newQueryHandler(mt, queryOptions{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = handler.start(ctx)

	// Send assistant message with tool use
	mt.msgChan <- map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"role":  "assistant",
			"model": "claude-sonnet-4-5",
			"content": []any{
				map[string]any{"type": "text", "text": "Let me check that."},
				map[string]any{
					"type":  "tool_use",
					"id":    "tu-123",
					"name":  "Read",
					"input": map[string]any{"file_path": "/tmp/test.txt"},
				},
			},
		},
	}

	// Tool result as user message
	mt.msgChan <- map[string]any{
		"type": "user",
		"message": map[string]any{
			"role": "user",
			"content": []any{
				map[string]any{
					"type":        "tool_result",
					"tool_use_id": "tu-123",
					"content":     "file contents here",
				},
			},
		},
	}

	// Final assistant response
	mt.msgChan <- map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"role":  "assistant",
			"model": "claude-sonnet-4-5",
			"content": []any{
				map[string]any{"type": "text", "text": "The file contains: file contents here"},
			},
		},
	}

	mt.msgChan <- map[string]any{
		"type":            "result",
		"subtype":         "success",
		"is_error":        false,
		"duration_ms":     float64(200),
		"duration_api_ms": float64(150),
		"num_turns":       float64(1),
		"session_id":      "test-session",
	}

	var messages []Message
	timeout := time.After(2 * time.Second)
	for {
		select {
		case raw, ok := <-handler.receiveMessages():
			if !ok {
				goto done
			}
			msg, _ := parseMessage(raw)
			if msg != nil {
				messages = append(messages, msg)
			}
			if _, isResult := msg.(*ResultMessage); isResult {
				goto done
			}
		case <-timeout:
			t.Fatal("timeout")
		}
	}
done:
	handler.close()

	if len(messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(messages))
	}

	// Verify tool use in first assistant message
	am := messages[0].(*AssistantMessage)
	if len(am.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(am.Content))
	}
	tu, ok := am.Content[1].(*ToolUseBlock)
	if !ok {
		t.Fatalf("expected ToolUseBlock, got %T", am.Content[1])
	}
	if tu.Name != "Read" {
		t.Errorf("expected tool name 'Read', got %q", tu.Name)
	}
	if tu.ID != "tu-123" {
		t.Errorf("expected tool ID 'tu-123', got %q", tu.ID)
	}
}

// TestIntegrationPermissionCallback tests the full permission callback flow.
func TestIntegrationPermissionCallback(t *testing.T) {
	var capturedToolName string
	var capturedInput map[string]any

	mt := newMockTransport()
	handler := newQueryHandler(mt, queryOptions{
		CanUseTool: func(ctx context.Context, toolName string, input map[string]any, permCtx ToolPermissionContext) (PermissionResult, error) {
			capturedToolName = toolName
			capturedInput = input

			if toolName == "Bash" {
				cmd, _ := input["command"].(string)
				if len(cmd) >= 2 && cmd[:2] == "rm" {
					return &PermissionResultDeny{
						Message:   "rm commands are not allowed",
						Interrupt: true,
					}, nil
				}
			}
			return &PermissionResultAllow{}, nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = handler.start(ctx)

	// Send a can_use_tool control request
	mt.msgChan <- map[string]any{
		"type":       "control_request",
		"request_id": "perm_1",
		"request": map[string]any{
			"subtype":   "can_use_tool",
			"tool_name": "Bash",
			"input":     map[string]any{"command": "rm -rf /"},
		},
	}

	// Wait for response
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for permission response")
		default:
		}
		time.Sleep(10 * time.Millisecond)
		written := mt.getWritten()
		if len(written) > 0 {
			var resp map[string]any
			_ = json.Unmarshal([]byte(written[0]), &resp)
			response, _ := resp["response"].(map[string]any)
			inner, _ := response["response"].(map[string]any)
			if inner != nil && inner["behavior"] == "deny" {
				// Verify callback was invoked
				if capturedToolName != "Bash" {
					t.Errorf("expected tool name 'Bash', got %q", capturedToolName)
				}
				if capturedInput["command"] != "rm -rf /" {
					t.Errorf("unexpected input: %v", capturedInput)
				}
				// Verify response
				if inner["message"] != "rm commands are not allowed" {
					t.Errorf("unexpected message: %v", inner["message"])
				}
				if inner["interrupt"] != true {
					t.Error("expected interrupt=true")
				}
				handler.close()
				return
			}
		}
	}
}

// TestIntegrationPermissionCallbackAllow tests the allow path of permission callbacks.
func TestIntegrationPermissionCallbackAllow(t *testing.T) {
	mt := newMockTransport()
	handler := newQueryHandler(mt, queryOptions{
		CanUseTool: func(ctx context.Context, toolName string, input map[string]any, permCtx ToolPermissionContext) (PermissionResult, error) {
			return &PermissionResultAllow{
				UpdatedInput: map[string]any{"command": "echo hello"},
			}, nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = handler.start(ctx)

	mt.msgChan <- map[string]any{
		"type":       "control_request",
		"request_id": "perm_allow",
		"request": map[string]any{
			"subtype":   "can_use_tool",
			"tool_name": "Bash",
			"input":     map[string]any{"command": "ls"},
		},
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for allow response")
		default:
		}
		time.Sleep(10 * time.Millisecond)
		written := mt.getWritten()
		if len(written) > 0 {
			var resp map[string]any
			_ = json.Unmarshal([]byte(written[0]), &resp)
			response, _ := resp["response"].(map[string]any)
			inner, _ := response["response"].(map[string]any)
			if inner != nil && inner["behavior"] == "allow" {
				updatedInput, _ := inner["updatedInput"].(map[string]any)
				if updatedInput["command"] != "echo hello" {
					t.Errorf("expected updated command 'echo hello', got %v", updatedInput["command"])
				}
				handler.close()
				return
			}
		}
	}
}

// TestIntegrationHookCallback tests the full hook callback flow.
func TestIntegrationHookCallback(t *testing.T) {
	var capturedHookInput HookInput

	hookCB := func(ctx context.Context, input HookInput, toolUseID string, hookCtx HookContext) (*HookJSONOutput, error) {
		capturedHookInput = input
		cont := true
		return &HookJSONOutput{
			Continue: &cont,
			Reason:   "all good",
		}, nil
	}

	mt := newMockTransport()
	handler := newQueryHandler(mt, queryOptions{
		Hooks: map[string][]hookMatcherConfig{
			"PreToolUse": {
				{
					Matcher: "Bash",
					Hooks:   []HookCallback{hookCB},
				},
			},
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = handler.start(ctx)

	// Register hooks manually (simulating initialize)
	handler.hookCallbacks["hook_0"] = hookCB

	// Send hook callback request
	mt.msgChan <- map[string]any{
		"type":       "control_request",
		"request_id": "hook_1",
		"request": map[string]any{
			"subtype":     "hook_callback",
			"callback_id": "hook_0",
			"tool_use_id": "tu-abc",
			"input": map[string]any{
				"tool_name":       "Bash",
				"tool_input":      map[string]any{"command": "ls"},
				"hook_event_name": "PreToolUse",
				"cwd":             "/tmp",
			},
		},
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for hook response")
		default:
		}
		time.Sleep(10 * time.Millisecond)
		written := mt.getWritten()
		if len(written) > 0 {
			var resp map[string]any
			_ = json.Unmarshal([]byte(written[0]), &resp)
			response, _ := resp["response"].(map[string]any)
			if response["subtype"] == "success" {
				inner, _ := response["response"].(map[string]any)
				if inner != nil {
					if inner["continue"] != true {
						t.Errorf("expected continue=true, got %v", inner["continue"])
					}
					if inner["reason"] != "all good" {
						t.Errorf("expected reason='all good', got %v", inner["reason"])
					}
					// Verify captured input
					if capturedHookInput.ToolName != "Bash" {
						t.Errorf("expected tool name 'Bash', got %q", capturedHookInput.ToolName)
					}
					if capturedHookInput.Cwd != "/tmp" {
						t.Errorf("expected cwd '/tmp', got %q", capturedHookInput.Cwd)
					}
					handler.close()
					return
				}
			}
		}
	}
}

// TestIntegrationMCPToolCall tests the full MCP tool call flow.
func TestIntegrationMCPToolCall(t *testing.T) {
	addTool := NewMCPTool("add", "Add two numbers",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{"type": "number"},
				"b": map[string]any{"type": "number"},
			},
			"required": []string{"a", "b"},
		},
		func(ctx context.Context, args map[string]any) (MCPToolResult, error) {
			a, _ := args["a"].(float64)
			b, _ := args["b"].(float64)
			return MCPToolResult{
				Content: []MCPContent{
					{Type: "text", Text: formatFloat(a + b)},
				},
			}, nil
		},
	)

	serverCfg := CreateSdkMcpServer("calculator", "1.0.0", addTool)

	mt := newMockTransport()
	handler := newQueryHandler(mt, queryOptions{
		SdkMcpServers: map[string]*McpServer{"calc": serverCfg.Instance},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = handler.start(ctx)

	// Send MCP tools/call request
	mt.msgChan <- map[string]any{
		"type":       "control_request",
		"request_id": "mcp_call_1",
		"request": map[string]any{
			"subtype":     "mcp_message",
			"server_name": "calc",
			"message": map[string]any{
				"jsonrpc": "2.0",
				"id":      float64(42),
				"method":  "tools/call",
				"params": map[string]any{
					"name":      "add",
					"arguments": map[string]any{"a": float64(17), "b": float64(25)},
				},
			},
		},
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for MCP tool call response")
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
					content, _ := result["content"].([]any)
					if len(content) > 0 {
						item, _ := content[0].(map[string]any)
						if item["text"] == "42" {
							handler.close()
							return
						}
					}
				}
			}
		}
	}
}

// TestIntegrationMCPServerNotFound tests MCP request to unknown server.
func TestIntegrationMCPServerNotFound(t *testing.T) {
	mt := newMockTransport()
	handler := newQueryHandler(mt, queryOptions{
		SdkMcpServers: map[string]*McpServer{},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = handler.start(ctx)

	mt.msgChan <- map[string]any{
		"type":       "control_request",
		"request_id": "mcp_notfound",
		"request": map[string]any{
			"subtype":     "mcp_message",
			"server_name": "nonexistent",
			"message": map[string]any{
				"jsonrpc": "2.0",
				"id":      float64(1),
				"method":  "tools/list",
			},
		},
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for MCP not found response")
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
					errData, _ := mcpResp["error"].(map[string]any)
					if errData != nil {
						handler.close()
						return
					}
				}
			}
		}
	}
}

// TestIntegrationConvertHooks tests convertHooks utility.
func TestIntegrationConvertHooks(t *testing.T) {
	timeout := 5.0
	hooks := map[HookEvent][]HookMatcher{
		HookPreToolUse: {
			{
				Matcher: "Bash",
				Hooks: []HookCallback{
					func(ctx context.Context, input HookInput, toolUseID string, hookCtx HookContext) (*HookJSONOutput, error) {
						return nil, nil
					},
				},
				Timeout: &timeout,
			},
		},
	}

	converted := convertHooks(hooks)
	if converted == nil {
		t.Fatal("expected non-nil result")
	}
	matchers, ok := converted["PreToolUse"]
	if !ok || len(matchers) != 1 {
		t.Fatal("expected 1 matcher for PreToolUse")
	}
	if matchers[0].Matcher != "Bash" {
		t.Errorf("expected matcher 'Bash', got %q", matchers[0].Matcher)
	}
	if matchers[0].Timeout == nil || *matchers[0].Timeout != 5.0 {
		t.Error("expected timeout 5.0")
	}
}

func TestIntegrationConvertHooksNil(t *testing.T) {
	result := convertHooks(nil)
	if result != nil {
		t.Error("expected nil for nil hooks")
	}
}

// formatFloat is a simple helper for formatting floats as integers.
func formatFloat(f float64) string {
	return fmt.Sprintf("%.0f", f)
}
