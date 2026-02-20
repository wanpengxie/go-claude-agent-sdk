// Example: Interactive conversation using ClaudeClient.
package main

import (
	"context"
	"fmt"
	"log"

	claude "github.com/anthropics/go-claude-agent-sdk"
)

func main() {
	ctx := context.Background()

	client := claude.NewClient(
		claude.WithAllowedTools("Read", "Write"),
		claude.WithPermissionMode(claude.PermissionAcceptEdits),
	)
	defer client.Close()

	if err := client.Connect(ctx); err != nil {
		log.Fatal(err)
	}

	// First query
	if err := client.Query(ctx, "What's the capital of France?"); err != nil {
		log.Fatal(err)
	}
	for msg := range client.ReceiveResponse(ctx) {
		if m, ok := msg.(*claude.AssistantMessage); ok {
			for _, block := range m.Content {
				if tb, ok := block.(*claude.TextBlock); ok {
					fmt.Println("Claude:", tb.Text)
				}
			}
		}
	}

	// Follow-up query (same session)
	if err := client.Query(ctx, "What's its population?"); err != nil {
		log.Fatal(err)
	}
	for msg := range client.ReceiveResponse(ctx) {
		if m, ok := msg.(*claude.AssistantMessage); ok {
			for _, block := range m.Content {
				if tb, ok := block.(*claude.TextBlock); ok {
					fmt.Println("Claude:", tb.Text)
				}
			}
		}
	}
}
