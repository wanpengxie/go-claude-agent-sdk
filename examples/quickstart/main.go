// Example: Simple one-shot query to Claude Code.
package main

import (
	"context"
	"fmt"
	"log"

	claude "github.com/wanpengxie/go-claude-agent-sdk"
)

func main() {
	ctx := context.Background()

	msgs, errs := claude.Query(ctx, "What is 2+2?",
		claude.WithModel("claude-sonnet-4-5"),
		claude.WithMaxTurns(1),
		claude.WithPermissionMode(claude.PermissionDefault),
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
			if m.TotalCostUSD != nil {
				fmt.Printf("Cost: $%.4f\n", *m.TotalCostUSD)
			}
			fmt.Printf("Duration: %dms\n", m.DurationMS)
		}
	}
	if err := <-errs; err != nil {
		log.Fatal(err)
	}
}
