// Example: Using SDK MCP tools for a calculator.
package main

import (
	"context"
	"fmt"
	"log"

	claude "github.com/anthropics/go-claude-agent-sdk"
)

func main() {
	ctx := context.Background()

	addTool := claude.NewMCPTool("add", "Add two numbers",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{"type": "number", "description": "First number"},
				"b": map[string]any{"type": "number", "description": "Second number"},
			},
			"required": []string{"a", "b"},
		},
		func(ctx context.Context, args map[string]any) (claude.MCPToolResult, error) {
			a, _ := args["a"].(float64)
			b, _ := args["b"].(float64)
			return claude.MCPToolResult{
				Content: []claude.MCPContent{
					{Type: "text", Text: fmt.Sprintf("%.2f + %.2f = %.2f", a, b, a+b)},
				},
			}, nil
		},
	)

	multiplyTool := claude.NewMCPTool("multiply", "Multiply two numbers",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{"type": "number", "description": "First number"},
				"b": map[string]any{"type": "number", "description": "Second number"},
			},
			"required": []string{"a", "b"},
		},
		func(ctx context.Context, args map[string]any) (claude.MCPToolResult, error) {
			a, _ := args["a"].(float64)
			b, _ := args["b"].(float64)
			return claude.MCPToolResult{
				Content: []claude.MCPContent{
					{Type: "text", Text: fmt.Sprintf("%.2f * %.2f = %.2f", a, b, a*b)},
				},
			}, nil
		},
	)

	server := claude.CreateSdkMcpServer("calculator", "1.0.0", addTool, multiplyTool)

	msgs, errs := claude.Query(ctx, "What is 42 + 17? And what is 6 * 7?",
		claude.WithMcpServers(map[string]claude.McpServerConfig{
			"calc": server,
		}),
		claude.WithAllowedTools("mcp__calc__add", "mcp__calc__multiply"),
		claude.WithMaxTurns(5),
	)

	for msg := range msgs {
		switch m := msg.(type) {
		case *claude.AssistantMessage:
			for _, block := range m.Content {
				if tb, ok := block.(*claude.TextBlock); ok {
					fmt.Println(tb.Text)
				}
			}
		case *claude.ResultMessage:
			fmt.Printf("\nDone! Turns: %d\n", m.NumTurns)
		}
	}
	if err := <-errs; err != nil {
		log.Fatal(err)
	}
}
