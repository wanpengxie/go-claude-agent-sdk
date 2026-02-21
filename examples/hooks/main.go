// Example: Using hooks to control tool execution.
package main

import (
	"context"
	"fmt"
	"log"

	claude "github.com/wanpengxie/go-claude-agent-sdk"
)

func main() {
	ctx := context.Background()

	// Hook that logs before any tool use
	logHook := func(ctx context.Context, input claude.HookInput, toolUseID string, hookCtx claude.HookContext) (*claude.HookJSONOutput, error) {
		fmt.Printf("[Hook] Tool %s called with input: %v\n", input.ToolName, input.ToolInput)
		cont := true
		return &claude.HookJSONOutput{Continue: &cont}, nil
	}

	msgs, errs := claude.Query(ctx, "List files in the current directory",
		claude.WithPermissionMode(claude.PermissionAcceptEdits),
		claude.WithMaxTurns(3),
		claude.WithHooks(map[claude.HookEvent][]claude.HookMatcher{
			claude.HookPreToolUse: {
				{
					Matcher: "Bash",
					Hooks:   []claude.HookCallback{logHook},
				},
			},
		}),
	)

	for msg := range msgs {
		if m, ok := msg.(*claude.AssistantMessage); ok {
			for _, block := range m.Content {
				if tb, ok := block.(*claude.TextBlock); ok {
					fmt.Println(tb.Text)
				}
			}
		}
	}
	if err := <-errs; err != nil {
		log.Fatal(err)
	}
}
