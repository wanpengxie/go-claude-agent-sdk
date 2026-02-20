// Example: matrix mode B - QueryStream with multiple input messages.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	claude "github.com/anthropics/go-claude-agent-sdk"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	input := make(chan map[string]any, 4)

	msgs, errs := claude.QueryStream(
		ctx,
		input,
		claude.WithModel("sonnet"),
		claude.WithPermissionMode(claude.PermissionDefault),
		claude.WithMaxTurns(2),
	)

	go func() {
		defer close(input)
		sessionID := "matrix-stream-demo"
		input <- map[string]any{
			"type":               "user",
			"session_id":         sessionID,
			"message":            map[string]any{"role": "user", "content": "请只输出字母A"},
			"parent_tool_use_id": nil,
		}
		input <- map[string]any{
			"type":               "user",
			"session_id":         sessionID,
			"message":            map[string]any{"role": "user", "content": "请只输出字母B"},
			"parent_tool_use_id": nil,
		}
	}()

	round := 1
	for msg := range msgs {
		switch m := msg.(type) {
		case *claude.AssistantMessage:
			for _, block := range m.Content {
				if tb, ok := block.(*claude.TextBlock); ok {
					fmt.Printf("[round %d] assistant: %s\n", round, tb.Text)
				}
			}
		case *claude.ResultMessage:
			fmt.Printf("[round %d] result: subtype=%s session=%s\n", round, m.Subtype, m.SessionID)
			round++
		case *claude.RateLimitEvent:
			fmt.Println("rate_limit_event")
		}
	}

	if err := <-errs; err != nil {
		log.Fatal(err)
	}
}
