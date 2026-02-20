package claude

import "context"

// McpStdioServerConfig represents an MCP stdio server configuration.
type McpStdioServerConfig struct {
	Type    string            `json:"type,omitempty"` // "stdio" or empty
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

func (c *McpStdioServerConfig) mcpServerConfigType() string { return "stdio" }

// McpSSEServerConfig represents an MCP SSE server configuration.
type McpSSEServerConfig struct {
	Type    string            `json:"type"` // "sse"
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

func (c *McpSSEServerConfig) mcpServerConfigType() string { return "sse" }

// McpHTTPServerConfig represents an MCP HTTP server configuration.
type McpHTTPServerConfig struct {
	Type    string            `json:"type"` // "http"
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

func (c *McpHTTPServerConfig) mcpServerConfigType() string { return "http" }

// McpSdkServerConfig represents an SDK MCP server configuration.
type McpSdkServerConfig struct {
	Type     string     `json:"type"` // "sdk"
	Name     string     `json:"name"`
	Instance *McpServer `json:"-"` // Not serialized to JSON
}

func (c *McpSdkServerConfig) mcpServerConfigType() string { return "sdk" }

// McpServerConfig is a sealed interface for MCP server configurations.
type McpServerConfig interface {
	mcpServerConfigType() string
}

// MCPContent represents content in an MCP tool result.
type MCPContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// MCPToolResult represents the result from an MCP tool execution.
type MCPToolResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"is_error,omitempty"`
}

// MCPToolHandler is the function signature for MCP tool handlers.
type MCPToolHandler func(ctx context.Context, args map[string]any) (MCPToolResult, error)

// MCPToolAnnotations represents optional tool annotations.
type MCPToolAnnotations struct {
	Title           string `json:"title,omitempty"`
	ReadOnlyHint    *bool  `json:"readOnlyHint,omitempty"`
	DestructiveHint *bool  `json:"destructiveHint,omitempty"`
	IdempotentHint  *bool  `json:"idempotentHint,omitempty"`
	OpenWorldHint   *bool  `json:"openWorldHint,omitempty"`
}

// SdkMcpTool represents a tool definition for an SDK MCP server.
type SdkMcpTool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Handler     MCPToolHandler
	Annotations *MCPToolAnnotations
}

// NewMCPTool creates a new SDK MCP tool definition.
func NewMCPTool(name, description string, inputSchema map[string]any, handler MCPToolHandler) *SdkMcpTool {
	return &SdkMcpTool{
		Name:        name,
		Description: description,
		InputSchema: inputSchema,
		Handler:     handler,
	}
}

// McpServer represents an in-process MCP server that handles tool calls.
type McpServer struct {
	Name    string
	Version string
	Tools   []*SdkMcpTool
	toolMap map[string]*SdkMcpTool
}

// HandleInitialize handles the MCP initialize request.
func (s *McpServer) HandleInitialize(id any) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    s.Name,
				"version": s.Version,
			},
		},
	}
}

// HandleListTools handles the MCP tools/list request.
func (s *McpServer) HandleListTools(id any) map[string]any {
	tools := make([]map[string]any, 0, len(s.Tools))
	for _, t := range s.Tools {
		schema := t.InputSchema
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		toolData := map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": schema,
		}
		if t.Annotations != nil {
			toolData["annotations"] = t.Annotations
		}
		tools = append(tools, toolData)
	}
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  map[string]any{"tools": tools},
	}
}

// HandleCallTool handles the MCP tools/call request.
func (s *McpServer) HandleCallTool(ctx context.Context, id any, name string, arguments map[string]any) map[string]any {
	tool, ok := s.toolMap[name]
	if !ok {
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"error": map[string]any{
				"code":    -32601,
				"message": "Tool '" + name + "' not found",
			},
		}
	}

	result, err := tool.Handler(ctx, arguments)
	if err != nil {
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"error": map[string]any{
				"code":    -32603,
				"message": err.Error(),
			},
		}
	}

	content := make([]map[string]any, 0, len(result.Content))
	for _, item := range result.Content {
		c := map[string]any{"type": item.Type}
		if item.Type == "text" {
			c["text"] = item.Text
		} else if item.Type == "image" {
			c["data"] = item.Data
			c["mimeType"] = item.MimeType
		}
		content = append(content, c)
	}

	responseData := map[string]any{"content": content}
	if result.IsError {
		responseData["is_error"] = true
	}

	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  responseData,
	}
}

// HandleRequest dispatches an MCP JSONRPC request to the appropriate handler.
func (s *McpServer) HandleRequest(ctx context.Context, message map[string]any) map[string]any {
	method, _ := message["method"].(string)
	id := message["id"]
	params, _ := message["params"].(map[string]any)
	if params == nil {
		params = map[string]any{}
	}

	switch method {
	case "initialize":
		return s.HandleInitialize(id)
	case "tools/list":
		return s.HandleListTools(id)
	case "tools/call":
		name, _ := params["name"].(string)
		args, _ := params["arguments"].(map[string]any)
		if args == nil {
			args = map[string]any{}
		}
		return s.HandleCallTool(ctx, id, name, args)
	case "notifications/initialized":
		return map[string]any{"jsonrpc": "2.0", "result": map[string]any{}}
	default:
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"error": map[string]any{
				"code":    -32601,
				"message": "Method '" + method + "' not found",
			},
		}
	}
}

// CreateSdkMcpServer creates an in-process MCP server configuration.
func CreateSdkMcpServer(name string, version string, tools ...*SdkMcpTool) *McpSdkServerConfig {
	server := &McpServer{
		Name:    name,
		Version: version,
		Tools:   tools,
		toolMap: make(map[string]*SdkMcpTool, len(tools)),
	}
	for _, t := range tools {
		server.toolMap[t.Name] = t
	}
	return &McpSdkServerConfig{
		Type:     "sdk",
		Name:     name,
		Instance: server,
	}
}
