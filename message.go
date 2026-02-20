package claude

// Message is a sealed interface representing messages from Claude Code.
// Use type switch to handle specific message types.
type Message interface {
	messageType() string
}

// AssistantMessageError represents possible error types in assistant messages.
type AssistantMessageError string

const (
	AssistantErrorAuthenticationFailed AssistantMessageError = "authentication_failed"
	AssistantErrorBillingError         AssistantMessageError = "billing_error"
	AssistantErrorRateLimit            AssistantMessageError = "rate_limit"
	AssistantErrorInvalidRequest       AssistantMessageError = "invalid_request"
	AssistantErrorServerError          AssistantMessageError = "server_error"
	AssistantErrorUnknown              AssistantMessageError = "unknown"
)

// UserMessage represents a user message.
type UserMessage struct {
	Content         any            `json:"content"` // string | []ContentBlock
	UUID            string         `json:"uuid,omitempty"`
	ParentToolUseID string         `json:"parent_tool_use_id,omitempty"`
	ToolUseResult   map[string]any `json:"tool_use_result,omitempty"`
}

func (m *UserMessage) messageType() string { return "user" }

// AssistantMessage represents an assistant message with content blocks.
type AssistantMessage struct {
	Content         []ContentBlock        `json:"content"`
	Model           string                `json:"model"`
	ParentToolUseID string                `json:"parent_tool_use_id,omitempty"`
	Error           AssistantMessageError `json:"error,omitempty"`
}

func (m *AssistantMessage) messageType() string { return "assistant" }

// SystemMessage represents a system message with metadata.
type SystemMessage struct {
	Subtype string         `json:"subtype"`
	Data    map[string]any `json:"data"`
}

func (m *SystemMessage) messageType() string { return "system" }

// ResultMessage represents a result message with cost and usage information.
type ResultMessage struct {
	Subtype          string         `json:"subtype"`
	DurationMS       int            `json:"duration_ms"`
	DurationAPIMS    int            `json:"duration_api_ms"`
	IsError          bool           `json:"is_error"`
	NumTurns         int            `json:"num_turns"`
	SessionID        string         `json:"session_id"`
	TotalCostUSD     *float64       `json:"total_cost_usd,omitempty"`
	Usage            map[string]any `json:"usage,omitempty"`
	Result           string         `json:"result,omitempty"`
	StructuredOutput any            `json:"structured_output,omitempty"`
}

func (m *ResultMessage) messageType() string { return "result" }

// StreamEvent represents a stream event for partial message updates during streaming.
type StreamEvent struct {
	UUID            string         `json:"uuid"`
	SessionID       string         `json:"session_id"`
	Event           map[string]any `json:"event"`
	ParentToolUseID string         `json:"parent_tool_use_id,omitempty"`
}

func (m *StreamEvent) messageType() string { return "stream_event" }

// RateLimitEvent represents rate limit metadata events emitted by Claude CLI.
// Keep the payload raw for forward compatibility with CLI changes.
type RateLimitEvent struct {
	Data map[string]any `json:"data"`
}

func (m *RateLimitEvent) messageType() string { return "rate_limit_event" }
