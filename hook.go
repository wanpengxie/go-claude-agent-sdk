package claude

import "context"

// HookEvent represents the type of hook event.
type HookEvent string

const (
	HookPreToolUse         HookEvent = "PreToolUse"
	HookPostToolUse        HookEvent = "PostToolUse"
	HookPostToolUseFailure HookEvent = "PostToolUseFailure"
	HookUserPromptSubmit   HookEvent = "UserPromptSubmit"
	HookStop               HookEvent = "Stop"
	HookSubagentStop       HookEvent = "SubagentStop"
	HookPreCompact         HookEvent = "PreCompact"
	HookNotification       HookEvent = "Notification"
	HookSubagentStart      HookEvent = "SubagentStart"
	HookPermissionRequest  HookEvent = "PermissionRequest"
)

// HookInput represents input data for hook callbacks.
// Use HookEventName field to determine the specific type.
type HookInput struct {
	// Common fields
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	PermissionMode string `json:"permission_mode,omitempty"`
	HookEventName  string `json:"hook_event_name"`

	// PreToolUse / PostToolUse / PostToolUseFailure / PermissionRequest
	ToolName  string         `json:"tool_name,omitempty"`
	ToolInput map[string]any `json:"tool_input,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`

	// PostToolUse
	ToolResponse any `json:"tool_response,omitempty"`

	// PostToolUseFailure
	ErrorMsg    string `json:"error,omitempty"`
	IsInterrupt *bool  `json:"is_interrupt,omitempty"`

	// UserPromptSubmit
	Prompt string `json:"prompt,omitempty"`

	// Stop / SubagentStop
	StopHookActive bool `json:"stop_hook_active,omitempty"`

	// SubagentStop / SubagentStart
	AgentID             string `json:"agent_id,omitempty"`
	AgentTranscriptPath string `json:"agent_transcript_path,omitempty"`
	AgentType           string `json:"agent_type,omitempty"`

	// PreCompact
	Trigger            string `json:"trigger,omitempty"`
	CustomInstructions string `json:"custom_instructions,omitempty"`

	// Notification
	NotificationMessage string `json:"message,omitempty"`
	Title               string `json:"title,omitempty"`
	NotificationType    string `json:"notification_type,omitempty"`

	// PermissionRequest
	PermissionSuggestions []any `json:"permission_suggestions,omitempty"`
}

// HookSpecificOutput represents hook-specific output fields.
type HookSpecificOutput struct {
	HookEventName string `json:"hookEventName"`

	// PreToolUse specific
	PermissionDecision       string         `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string         `json:"permissionDecisionReason,omitempty"`
	UpdatedInput             map[string]any `json:"updatedInput,omitempty"`

	// PostToolUse specific
	UpdatedMCPToolOutput any `json:"updatedMCPToolOutput,omitempty"`

	// Common
	AdditionalContext string `json:"additionalContext,omitempty"`

	// PermissionRequest specific
	Decision map[string]any `json:"decision,omitempty"`
}

// HookJSONOutput represents the output from a hook callback.
type HookJSONOutput struct {
	// Async mode
	Async        *bool `json:"async,omitempty"`
	AsyncTimeout *int  `json:"asyncTimeout,omitempty"`

	// Sync mode - common control fields
	Continue       *bool  `json:"continue,omitempty"`
	SuppressOutput *bool  `json:"suppressOutput,omitempty"`
	StopReason     string `json:"stopReason,omitempty"`

	// Decision fields
	Decision      string `json:"decision,omitempty"` // "block"
	SystemMessage string `json:"systemMessage,omitempty"`
	Reason        string `json:"reason,omitempty"`

	// Hook-specific output
	HookSpecificOutput *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

// HookContext provides context information for hook callbacks.
type HookContext struct {
	Signal any // Future: abort signal support
}

// HookCallback is the signature for hook callback functions.
type HookCallback func(
	ctx context.Context,
	input HookInput,
	toolUseID string,
	hookCtx HookContext,
) (*HookJSONOutput, error)

// HookMatcher defines a hook matcher configuration.
type HookMatcher struct {
	Matcher string // Tool name pattern (e.g., "Bash", "Write|Edit")
	Hooks   []HookCallback
	Timeout *float64 // Timeout in seconds
}
