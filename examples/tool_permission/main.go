// Example: Using custom tool permission callbacks.
package main

import (
	"context"
	"fmt"
	"log"

	claude "github.com/anthropics/go-claude-agent-sdk"
)

func main() {
	ctx := context.Background()

	// Permission callback that denies dangerous bash commands
	canUseTool := func(ctx context.Context, toolName string, input map[string]any, permCtx claude.ToolPermissionContext) (claude.PermissionResult, error) {
		if toolName == "Bash" {
			cmd, _ := input["command"].(string)
			fmt.Printf("[Permission] Bash command requested: %s\n", cmd)
			// Deny rm commands
			if len(cmd) >= 2 && cmd[:2] == "rm" {
				return &claude.PermissionResultDeny{
					Message: "rm commands are not allowed",
				}, nil
			}
		}
		// Allow everything else
		return &claude.PermissionResultAllow{}, nil
	}

	client := claude.NewClient(
		claude.WithCanUseTool(canUseTool),
		claude.WithMaxTurns(3),
	)
	defer client.Close()

	if err := client.Connect(ctx); err != nil {
		log.Fatal(err)
	}

	if err := client.Query(ctx, "List files in /tmp using bash"); err != nil {
		log.Fatal(err)
	}

	for msg := range client.ReceiveResponse(ctx) {
		if m, ok := msg.(*claude.AssistantMessage); ok {
			for _, block := range m.Content {
				if tb, ok := block.(*claude.TextBlock); ok {
					fmt.Println(tb.Text)
				}
			}
		}
	}
}
