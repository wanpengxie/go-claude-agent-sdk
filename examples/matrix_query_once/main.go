// Example: matrix mode A - one-shot Query.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	claude "github.com/wanpengxie/go-claude-agent-sdk"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	msgs, errs := claude.Query(
		ctx,
		"请只输出数字：2+2等于几？",
		claude.WithModel("sonnet"),
		claude.WithMaxTurns(1),
		claude.WithPermissionMode(claude.PermissionDefault),
	)

	for msg := range msgs {
		switch m := msg.(type) {
		case *claude.AssistantMessage:
			for _, block := range m.Content {
				if tb, ok := block.(*claude.TextBlock); ok {
					fmt.Println("assistant:", tb.Text)
				}
			}
		case *claude.ResultMessage:
			fmt.Printf("result: subtype=%s session=%s turns=%d\n", m.Subtype, m.SessionID, m.NumTurns)
		case *claude.RateLimitEvent:
			fmt.Println("rate_limit_event")
		}
	}

	if err := <-errs; err != nil {
		log.Fatal(err)
	}
}
