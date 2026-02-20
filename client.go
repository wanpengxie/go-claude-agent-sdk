package claude

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"sync"
)

// ClaudeClient provides bidirectional, interactive conversations with Claude Code.
//
// Key features:
//   - Bidirectional: Send and receive messages at any time
//   - Stateful: Maintains conversation context across messages
//   - Interactive: Send follow-ups based on responses
//   - Control flow: Support for interrupts and session management
type ClaudeClient struct {
	options *AgentOptions

	transport *subprocessTransport
	query     *queryHandler

	mu     sync.Mutex
	closed bool
}

// NewClient creates a new ClaudeClient with the given options.
func NewClient(opts ...Option) *ClaudeClient {
	return &ClaudeClient{
		options: applyOptions(opts),
	}
}

// Connect establishes the connection to Claude Code.
func (c *ClaudeClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	os.Setenv("CLAUDE_CODE_ENTRYPOINT", "sdk-go-client")

	// Configure permission settings
	configuredOptions := *c.options
	if configuredOptions.CanUseTool != nil {
		if configuredOptions.PermissionPromptToolName != "" {
			return &SDKError{Message: "can_use_tool callback cannot be used with permission_prompt_tool_name"}
		}
		configuredOptions.PermissionPromptToolName = "stdio"
	}

	c.transport = newSubprocessTransport(&configuredOptions)
	if err := c.transport.Connect(ctx); err != nil {
		return err
	}

	// Extract SDK MCP servers
	sdkMcpServers := make(map[string]*McpServer)
	for name, config := range configuredOptions.McpServers {
		if sdkCfg, ok := config.(*McpSdkServerConfig); ok {
			sdkMcpServers[name] = sdkCfg.Instance
		}
	}

	// Convert agents
	var agentsMap map[string]map[string]any
	if len(configuredOptions.Agents) > 0 {
		agentsMap = make(map[string]map[string]any)
		for name, agent := range configuredOptions.Agents {
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

	c.query = newQueryHandler(c.transport, queryOptions{
		CanUseTool:        configuredOptions.CanUseTool,
		Hooks:             convertHooks(configuredOptions.Hooks),
		SdkMcpServers:     sdkMcpServers,
		InitializeTimeout: resolveInitializeTimeout(),
		Agents:            agentsMap,
	})

	if err := c.query.start(ctx); err != nil {
		return err
	}

	if _, err := c.query.initialize(ctx); err != nil {
		return err
	}

	return nil
}

// Query sends a new message in the conversation.
func (c *ClaudeClient) Query(ctx context.Context, prompt string) error {
	return c.QueryWithSession(ctx, prompt, "default")
}

// QueryWithSession sends a new string prompt with explicit session ID.
func (c *ClaudeClient) QueryWithSession(ctx context.Context, prompt string, sessionID string) error {
	c.mu.Lock()
	if c.query == nil || c.transport == nil {
		c.mu.Unlock()
		return &CLIConnectionError{SDKError: SDKError{Message: "Not connected. Call Connect() first."}}
	}
	c.mu.Unlock()
	if sessionID == "" {
		sessionID = "default"
	}

	message := map[string]any{
		"type":               "user",
		"message":            map[string]any{"role": "user", "content": prompt},
		"parent_tool_use_id": nil,
		"session_id":         sessionID,
	}
	data, _ := json.Marshal(message)
	return c.transport.Write(string(data) + "\n")
}

// QueryStream sends streaming messages with optional default session ID.
// Existing session_id on each message is preserved.
func (c *ClaudeClient) QueryStream(ctx context.Context, messages <-chan map[string]any, defaultSessionID string) error {
	c.mu.Lock()
	if c.query == nil || c.transport == nil {
		c.mu.Unlock()
		return &CLIConnectionError{SDKError: SDKError{Message: "Not connected. Call Connect() first."}}
	}
	c.mu.Unlock()
	if defaultSessionID == "" {
		defaultSessionID = "default"
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-messages:
			if !ok {
				return nil
			}
			if msg == nil {
				continue
			}
			if _, exists := msg["session_id"]; !exists {
				msg["session_id"] = defaultSessionID
			}
			data, _ := json.Marshal(msg)
			if err := c.transport.Write(string(data) + "\n"); err != nil {
				return err
			}
		}
	}
}

// ReceiveMessages returns a channel that yields all messages from Claude.
func (c *ClaudeClient) ReceiveMessages(ctx context.Context) <-chan Message {
	msgChan, _ := c.ReceiveMessagesWithErrors(ctx)
	return msgChan
}

// ReceiveMessagesWithErrors returns messages and a terminal error channel.
func (c *ClaudeClient) ReceiveMessagesWithErrors(ctx context.Context) (<-chan Message, <-chan error) {
	msgChan := make(chan Message, 100)
	errChan := make(chan error, 1)
	go func() {
		defer close(msgChan)
		defer close(errChan)
		if c.query == nil {
			errChan <- &CLIConnectionError{SDKError: SDKError{Message: "Not connected. Call Connect() first."}}
			return
		}
		for rawMsg := range c.query.receiveMessages() {
			if rawType, _ := rawMsg["type"].(string); rawType == "error" {
				errText, _ := rawMsg["error"].(string)
				if errText == "" {
					errText = "unknown stream error"
				}
				errChan <- &SDKError{Message: errText}
				return
			}
			msg, err := parseMessage(rawMsg)
			if err != nil {
				errChan <- err
				return
			}
			select {
			case msgChan <- msg:
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			}
		}
		if err := c.query.err(); err != nil {
			errChan <- err
		}
	}()
	return msgChan, errChan
}

// ReceiveResponse receives messages until a ResultMessage is received.
// The ResultMessage IS included in the yielded messages.
func (c *ClaudeClient) ReceiveResponse(ctx context.Context) <-chan Message {
	msgChan, _ := c.ReceiveResponseWithErrors(ctx)
	return msgChan
}

// ReceiveResponseWithErrors receives until ResultMessage and returns terminal error channel.
func (c *ClaudeClient) ReceiveResponseWithErrors(ctx context.Context) (<-chan Message, <-chan error) {
	msgChan := make(chan Message, 100)
	errChan := make(chan error, 1)
	go func() {
		defer close(msgChan)
		defer close(errChan)
		if c.query == nil {
			errChan <- &CLIConnectionError{SDKError: SDKError{Message: "Not connected. Call Connect() first."}}
			return
		}
		for rawMsg := range c.query.receiveMessages() {
			if rawType, _ := rawMsg["type"].(string); rawType == "error" {
				errText, _ := rawMsg["error"].(string)
				if errText == "" {
					errText = "unknown stream error"
				}
				errChan <- &SDKError{Message: errText}
				return
			}
			msg, err := parseMessage(rawMsg)
			if err != nil {
				errChan <- err
				return
			}
			select {
			case msgChan <- msg:
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			}
			if _, ok := msg.(*ResultMessage); ok {
				return
			}
		}
		if err := c.query.err(); err != nil {
			errChan <- err
		}
	}()
	return msgChan, errChan
}

// Interrupt sends an interrupt signal.
func (c *ClaudeClient) Interrupt(ctx context.Context) error {
	if c.query == nil {
		return &CLIConnectionError{SDKError: SDKError{Message: "Not connected. Call Connect() first."}}
	}
	return c.query.interrupt(ctx)
}

// SetPermissionMode changes the permission mode during conversation.
func (c *ClaudeClient) SetPermissionMode(ctx context.Context, mode PermissionMode) error {
	if c.query == nil {
		return &CLIConnectionError{SDKError: SDKError{Message: "Not connected. Call Connect() first."}}
	}
	return c.query.setPermissionMode(ctx, string(mode))
}

// SetModel changes the AI model during conversation.
func (c *ClaudeClient) SetModel(ctx context.Context, model string) error {
	return c.SetModelOptional(ctx, &model)
}

// SetModelOptional changes model; nil means reset to CLI default model.
func (c *ClaudeClient) SetModelOptional(ctx context.Context, model *string) error {
	if c.query == nil {
		return &CLIConnectionError{SDKError: SDKError{Message: "Not connected. Call Connect() first."}}
	}
	if model == nil {
		return c.query.setModelOptional(ctx, nil)
	}
	return c.query.setModelOptional(ctx, *model)
}

// RewindFiles rewinds tracked files to a specific user message state.
func (c *ClaudeClient) RewindFiles(ctx context.Context, userMessageID string) error {
	if c.query == nil {
		return &CLIConnectionError{SDKError: SDKError{Message: "Not connected. Call Connect() first."}}
	}
	return c.query.rewindFiles(ctx, userMessageID)
}

// GetMCPStatus gets current MCP server connection status.
func (c *ClaudeClient) GetMCPStatus(ctx context.Context) (map[string]any, error) {
	if c.query == nil {
		return nil, &CLIConnectionError{SDKError: SDKError{Message: "Not connected. Call Connect() first."}}
	}
	return c.query.getMcpStatus(ctx)
}

// Close disconnects from Claude Code and cleans up resources.
func (c *ClaudeClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	if c.query != nil {
		c.query.close()
		c.query = nil
	}
	c.transport = nil
	return nil
}

func resolveInitializeTimeout() float64 {
	const minTimeoutSeconds = 60.0
	raw := os.Getenv("CLAUDE_CODE_STREAM_CLOSE_TIMEOUT")
	if raw == "" {
		return minTimeoutSeconds
	}
	ms, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return minTimeoutSeconds
	}
	timeoutSeconds := ms / 1000.0
	if timeoutSeconds < minTimeoutSeconds {
		return minTimeoutSeconds
	}
	return timeoutSeconds
}
