package claude

import (
	"context"
	"testing"
)

func TestCreateSdkMcpServer(t *testing.T) {
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
				Content: []MCPContent{{Type: "text", Text: "Result: " + string(rune(int(a+b)+'0'))}},
			}, nil
		},
	)

	server := CreateSdkMcpServer("calculator", "1.0.0", addTool)
	if server.Type != "sdk" {
		t.Errorf("expected type 'sdk', got %s", server.Type)
	}
	if server.Name != "calculator" {
		t.Errorf("expected name 'calculator', got %s", server.Name)
	}
	if server.Instance == nil {
		t.Fatal("expected non-nil Instance")
	}
	if len(server.Instance.Tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(server.Instance.Tools))
	}
}

func TestMcpServerHandleInitialize(t *testing.T) {
	server := CreateSdkMcpServer("test", "1.0.0")
	resp := server.Instance.HandleInitialize("init-1")
	result, _ := resp["result"].(map[string]any)
	if result == nil {
		t.Fatal("expected result")
	}
	serverInfo, _ := result["serverInfo"].(map[string]any)
	if name, _ := serverInfo["name"].(string); name != "test" {
		t.Errorf("expected name 'test', got %s", name)
	}
}

func TestMcpServerHandleListTools(t *testing.T) {
	tool := NewMCPTool("greet", "Greet someone",
		map[string]any{"type": "object", "properties": map[string]any{}},
		func(ctx context.Context, args map[string]any) (MCPToolResult, error) {
			return MCPToolResult{
				Content: []MCPContent{{Type: "text", Text: "Hello!"}},
			}, nil
		},
	)
	server := CreateSdkMcpServer("greeter", "1.0.0", tool)
	resp := server.Instance.HandleListTools("list-1")
	result, _ := resp["result"].(map[string]any)
	tools, _ := result["tools"].([]map[string]any)
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0]["name"] != "greet" {
		t.Errorf("expected tool name 'greet', got %v", tools[0]["name"])
	}
}

func TestMcpServerHandleCallTool(t *testing.T) {
	tool := NewMCPTool("echo", "Echo input",
		map[string]any{"type": "object", "properties": map[string]any{}},
		func(ctx context.Context, args map[string]any) (MCPToolResult, error) {
			text, _ := args["text"].(string)
			return MCPToolResult{
				Content: []MCPContent{{Type: "text", Text: text}},
			}, nil
		},
	)
	server := CreateSdkMcpServer("echo-server", "1.0.0", tool)
	resp := server.Instance.HandleCallTool(context.Background(), "call-1", "echo", map[string]any{"text": "hello"})
	result, _ := resp["result"].(map[string]any)
	content, _ := result["content"].([]map[string]any)
	if len(content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(content))
	}
	if content[0]["text"] != "hello" {
		t.Errorf("expected text 'hello', got %v", content[0]["text"])
	}
}

func TestMcpServerHandleCallToolNotFound(t *testing.T) {
	server := CreateSdkMcpServer("empty", "1.0.0")
	resp := server.Instance.HandleCallTool(context.Background(), "call-1", "nonexistent", map[string]any{})
	errObj, _ := resp["error"].(map[string]any)
	if errObj == nil {
		t.Fatal("expected error for nonexistent tool")
	}
}

func TestMcpServerHandleRequest(t *testing.T) {
	server := CreateSdkMcpServer("test", "1.0.0")

	// Test initialize
	resp := server.Instance.HandleRequest(context.Background(), map[string]any{
		"method": "initialize",
		"id":     "1",
	})
	if resp["error"] != nil {
		t.Errorf("unexpected error: %v", resp["error"])
	}

	// Test unknown method
	resp = server.Instance.HandleRequest(context.Background(), map[string]any{
		"method": "unknown/method",
		"id":     "2",
	})
	errObj, _ := resp["error"].(map[string]any)
	if errObj == nil {
		t.Error("expected error for unknown method")
	}
}
