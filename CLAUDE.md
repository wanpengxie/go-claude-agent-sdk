# Go Claude Agent SDK

## Build & Test

```bash
# Build
go build ./...

# Run all tests
go test ./...

# Run tests with race detection
go test -race ./...

# Run tests with coverage
go test -cover ./...

# Run specific test
go test -run TestParseAssistantMessage ./...
```

## Module

- Module path: `github.com/wanpengxie/go-claude-agent-sdk`
- Go version: 1.24+
- Package name: `claude`

## Architecture

Single-package design (`package claude`) to avoid import cycles. All public types are at the root level. Implementation details use unexported types.

### Key files

| File | Purpose |
|------|---------|
| `claude.go` | `Query()` one-shot function, package docs |
| `client.go` | `ClaudeClient` bidirectional client |
| `options.go` | `AgentOptions` + `With*` functional options |
| `message.go` | `Message` sealed interface + 5 message types |
| `content.go` | `ContentBlock` sealed interface + 4 content types |
| `permission.go` | Permission types + `CanUseToolFunc` |
| `hook.go` | Hook events, matchers, callbacks |
| `mcp.go` | MCP server configs + `CreateSdkMcpServer` |
| `errors.go` | Error type hierarchy |
| `parser.go` | JSON -> typed Message parsing |
| `transport.go` | Claude Code CLI subprocess management |
| `query_handler.go` | Bidirectional control protocol router |

### Patterns

- **Sealed interfaces**: Message, ContentBlock, PermissionResult, McpServerConfig, ThinkingConfig — use unexported marker methods, handle via type switch
- **Functional options**: `claude.WithModel("...")`, `claude.WithMaxTurns(5)` etc.
- **Channels for async**: `Query()` returns `(<-chan Message, <-chan error)`
- **Context propagation**: All public methods accept `context.Context`
  - `ClaudeClient.Connect(ctx)` uses `ctx` for connect/initialize timeout only; client stream lifecycle is owned by `client.Close()`

## Conventions

- Do not add `internal/` packages — all SDK code is in the root `claude` package
- Unexported types for implementation (e.g., `queryHandler`, `subprocessTransport`)
- Table-driven tests preferred
- JSON field names use snake_case matching the CLI protocol

## Usage Matrix

- Matrix docs: `docs/usage-matrix.md`
- Matrix examples:
  - `examples/matrix_query_once/main.go`
  - `examples/matrix_query_stream/main.go`
  - `examples/matrix_client_sessions/main.go`
