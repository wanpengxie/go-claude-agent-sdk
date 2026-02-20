// Package claude provides a Go SDK for interacting with Claude Code.
//
// It offers both a simple one-shot Query function for stateless queries
// and a ClaudeClient for interactive, bidirectional conversations.
//
// Quick start:
//
//	msgs, errs := claude.Query(ctx, "What is 2+2?",
//	    claude.WithModel("claude-sonnet-4-5"),
//	)
//	for msg := range msgs {
//	    if m, ok := msg.(*claude.AssistantMessage); ok {
//	        for _, block := range m.Content {
//	            if tb, ok := block.(*claude.TextBlock); ok {
//	                fmt.Println(tb.Text)
//	            }
//	        }
//	    }
//	}
//	if err := <-errs; err != nil {
//	    log.Fatal(err)
//	}
package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// Version is the SDK version.
const Version = "0.1.0"

// MinimumClaudeCodeVersion is the minimum required Claude Code version.
const MinimumClaudeCodeVersion = "2.0.0"

// Query performs a one-shot query to Claude Code and returns channels for
// messages and a final error.
//
// The messages channel yields Message values as they arrive. The error channel
// yields at most one error after all messages have been sent. Always drain the
// messages channel before reading the error channel.
func Query(ctx context.Context, prompt string, opts ...Option) (<-chan Message, <-chan error) {
	return runQuery(ctx, &prompt, nil, opts...)
}

// QueryStream performs a query with streaming input messages.
// This matches Python SDK's AsyncIterable prompt mode.
func QueryStream(ctx context.Context, input <-chan map[string]any, opts ...Option) (<-chan Message, <-chan error) {
	return runQuery(ctx, nil, input, opts...)
}

func runQuery(ctx context.Context, prompt *string, input <-chan map[string]any, opts ...Option) (<-chan Message, <-chan error) {
	msgChan := make(chan Message, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(msgChan)
		defer close(errChan)

		options := applyOptions(opts)
		os.Setenv("CLAUDE_CODE_ENTRYPOINT", "sdk-go")

		// Configure permission settings
		if options.CanUseTool != nil {
			if prompt != nil {
				errChan <- &SDKError{Message: "can_use_tool callback requires streaming input; use QueryStream instead of Query"}
				return
			}
			if options.PermissionPromptToolName != "" {
				errChan <- &SDKError{Message: "can_use_tool callback cannot be used with permission_prompt_tool_name"}
				return
			}
			options.PermissionPromptToolName = "stdio"
		}

		t := newSubprocessTransport(options)
		if err := t.Connect(ctx); err != nil {
			errChan <- err
			return
		}

		// Extract SDK MCP servers
		sdkMcpServers := make(map[string]*McpServer)
		for name, config := range options.McpServers {
			if sdkCfg, ok := config.(*McpSdkServerConfig); ok {
				sdkMcpServers[name] = sdkCfg.Instance
			}
		}

		// Convert agents
		var agentsMap map[string]map[string]any
		if len(options.Agents) > 0 {
			agentsMap = make(map[string]map[string]any)
			for name, agent := range options.Agents {
				a := map[string]any{
					"description": agent.Description,
					"prompt":      agent.Prompt,
				}
				if len(agent.Tools) > 0 {
					a["tools"] = agent.Tools
				}
				if agent.Model != "" {
					a["model"] = agent.Model
				}
				agentsMap[name] = a
			}
		}

		q := newQueryHandler(t, queryOptions{
			CanUseTool:        options.CanUseTool,
			Hooks:             convertHooks(options.Hooks),
			SdkMcpServers:     sdkMcpServers,
			InitializeTimeout: 60,
			Agents:            agentsMap,
		})
		started := false
		defer func() {
			if started {
				q.close()
			}
		}()

		if err := q.start(ctx); err != nil {
			errChan <- err
			return
		}
		started = true

		if _, err := q.initialize(ctx); err != nil {
			errChan <- err
			return
		}

		if prompt != nil {
			userMsg := map[string]any{
				"type":               "user",
				"session_id":         "",
				"message":            map[string]any{"role": "user", "content": *prompt},
				"parent_tool_use_id": nil,
			}
			data, _ := json.Marshal(userMsg)
			if err := t.Write(string(data) + "\n"); err != nil {
				errChan <- err
				return
			}
			_ = t.EndInput()
		} else if input != nil {
			go q.streamInput(ctx, input)
		}

		// Read and forward messages
		hadError := false
		for rawMsg := range q.receiveMessages() {
			if msgType, _ := rawMsg["type"].(string); msgType == "error" {
				errText, _ := rawMsg["error"].(string)
				if errText == "" {
					errText = "unknown transport error"
				}
				errChan <- fmt.Errorf("%s", errText)
				hadError = true
				break
			}
			msg, err := parseMessage(rawMsg)
			if err != nil {
				errChan <- err
				hadError = true
				break
			}
			select {
			case msgChan <- msg:
			case <-ctx.Done():
				errChan <- ctx.Err()
				hadError = true
				return
			}
		}
		if !hadError {
			if transportErr := q.err(); transportErr != nil {
				errChan <- transportErr
			}
		}
	}()

	return msgChan, errChan
}

// convertHooks converts public hook types to the internal format.
func convertHooks(hooks map[HookEvent][]HookMatcher) map[string][]hookMatcherConfig {
	if len(hooks) == 0 {
		return nil
	}
	result := make(map[string][]hookMatcherConfig, len(hooks))
	for event, matchers := range hooks {
		configs := make([]hookMatcherConfig, len(matchers))
		for i, m := range matchers {
			configs[i] = hookMatcherConfig{
				Matcher: m.Matcher,
				Hooks:   m.Hooks,
				Timeout: m.Timeout,
			}
		}
		result[string(event)] = configs
	}
	return result
}
