package claude

// ContentBlock is a sealed interface representing content within messages.
// Use type switch to handle specific content types.
type ContentBlock interface {
	contentBlockType() string
}

// TextBlock represents a text content block.
type TextBlock struct {
	Text string `json:"text"`
}

func (b *TextBlock) contentBlockType() string { return "text" }

// ThinkingBlock represents a thinking content block.
type ThinkingBlock struct {
	Thinking  string `json:"thinking"`
	Signature string `json:"signature"`
}

func (b *ThinkingBlock) contentBlockType() string { return "thinking" }

// ToolUseBlock represents a tool use content block.
type ToolUseBlock struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

func (b *ToolUseBlock) contentBlockType() string { return "tool_use" }

// ToolResultBlock represents a tool result content block.
type ToolResultBlock struct {
	ToolUseID string `json:"tool_use_id"`
	Content   any    `json:"content,omitempty"` // string | []map[string]any | nil
	IsError   *bool  `json:"is_error,omitempty"`
}

func (b *ToolResultBlock) contentBlockType() string { return "tool_result" }
