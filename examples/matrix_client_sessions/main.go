// Example: matrix mode C - ClaudeClient with explicit session separation.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	claude "github.com/anthropics/go-claude-agent-sdk"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()

	client := claude.NewClient(
		claude.WithModel("sonnet"),
		claude.WithPermissionMode(claude.PermissionDefault),
	)
	defer client.Close()

	if err := client.Connect(ctx); err != nil {
		log.Fatal(err)
	}

	mustAsk(ctx, client, "sess-A", "你是谁？只输出一句")
	mustAsk(ctx, client, "sess-B", "2+2=? 只输出数字")
	mustAsk(ctx, client, "sess-A", "重复上一条你的回答，不要解释")
}

func mustAsk(ctx context.Context, client *claude.ClaudeClient, sessionID, prompt string) {
	if err := client.QueryWithSession(ctx, prompt, sessionID); err != nil {
		log.Fatalf("query failed for %s: %v", sessionID, err)
	}

	msgs, errs := client.ReceiveResponseWithErrors(ctx)
	for msg := range msgs {
		switch m := msg.(type) {
		case *claude.AssistantMessage:
			for _, block := range m.Content {
				if tb, ok := block.(*claude.TextBlock); ok {
					fmt.Printf("[%s] assistant: %s\n", sessionID, tb.Text)
				}
			}
		case *claude.ResultMessage:
			fmt.Printf("[%s] result: subtype=%s session=%s\n", sessionID, m.Subtype, m.SessionID)
		}
	}
	if err := <-errs; err != nil {
		log.Fatalf("receive failed for %s: %v", sessionID, err)
	}
}
