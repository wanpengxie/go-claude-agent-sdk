package claude

// Control protocol request/response types for Claude Code SDK.
// These types represent the bidirectional control protocol between
// the SDK and the Claude Code CLI.

// ControlRequestType represents the type of control request.
type ControlRequestType string

const (
	ControlRequestInitialize        ControlRequestType = "initialize"
	ControlRequestCanUseTool        ControlRequestType = "can_use_tool"
	ControlRequestSetPermissionMode ControlRequestType = "set_permission_mode"
	ControlRequestHookCallback      ControlRequestType = "hook_callback"
	ControlRequestMcpMessage        ControlRequestType = "mcp_message"
	ControlRequestInterrupt         ControlRequestType = "interrupt"
	ControlRequestRewindFiles       ControlRequestType = "rewind_files"
	ControlRequestMcpStatus         ControlRequestType = "mcp_status"
	ControlRequestSetModel          ControlRequestType = "set_model"
)

// SDKControlRequest represents an incoming control request from CLI.
type SDKControlRequest struct {
	Type      string         `json:"type"` // "control_request"
	RequestID string         `json:"request_id"`
	Request   map[string]any `json:"request"`
}

// SDKControlResponse represents an outgoing control response to CLI.
type SDKControlResponse struct {
	Type     string              `json:"type"` // "control_response"
	Response ControlResponseBody `json:"response"`
}

// ControlResponseBody represents the body of a control response.
type ControlResponseBody struct {
	Subtype   string         `json:"subtype"` // "success" or "error"
	RequestID string         `json:"request_id"`
	Response  map[string]any `json:"response,omitempty"`
	Error     string         `json:"error,omitempty"`
}
