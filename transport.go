package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

const defaultMaxBufferSize = 1024 * 1024 // 1MB buffer limit

// subprocessTransport implements Transport using the Claude Code CLI subprocess.
type subprocessTransport struct {
	options       *AgentOptions
	cliPath       string
	cwd           string
	process       *exec.Cmd
	stdin         io.WriteCloser
	stdout        io.ReadCloser
	stderr        io.ReadCloser
	msgChan       chan map[string]any
	errChan       chan error
	ready         bool
	maxBufferSize int
	writeMu       sync.Mutex
	cancel        context.CancelFunc

	exitErr error
	errMu   sync.Mutex
}

func newSubprocessTransport(options *AgentOptions) *subprocessTransport {
	cliPath := options.CLIPath
	if cliPath == "" {
		cliPath = findCLI()
	}

	maxBuf := options.MaxBufferSize
	if maxBuf <= 0 {
		maxBuf = defaultMaxBufferSize
	}

	return &subprocessTransport{
		options:       options,
		cliPath:       cliPath,
		cwd:           options.Cwd,
		maxBufferSize: maxBuf,
		msgChan:       make(chan map[string]any, 100),
		errChan:       make(chan error, 1),
	}
}

func findCLI() string {
	// Try which/where
	if path, err := exec.LookPath("claude"); err == nil {
		return path
	}

	// Common locations
	home, _ := os.UserHomeDir()
	locations := []string{
		filepath.Join(home, ".npm-global/bin/claude"),
		"/usr/local/bin/claude",
		filepath.Join(home, ".local/bin/claude"),
		filepath.Join(home, "node_modules/.bin/claude"),
		filepath.Join(home, ".yarn/bin/claude"),
		filepath.Join(home, ".claude/local/claude"),
	}

	for _, loc := range locations {
		if info, err := os.Stat(loc); err == nil && !info.IsDir() {
			return loc
		}
	}

	return "claude" // Will fail at connect time with clear error
}

func (t *subprocessTransport) buildCommand() []string {
	cmd := []string{t.cliPath, "--output-format", "stream-json", "--verbose"}

	opts := t.options

	// System prompt
	if opts.SystemPrompt != nil {
		cmd = append(cmd, "--system-prompt", *opts.SystemPrompt)
	} else if opts.SystemPromptPreset != nil {
		if opts.SystemPromptPreset.Append != "" {
			cmd = append(cmd, "--append-system-prompt", opts.SystemPromptPreset.Append)
		}
	} else {
		cmd = append(cmd, "--system-prompt", "")
	}

	// Tools
	if opts.ToolsPreset != nil {
		cmd = append(cmd, "--tools", "default")
	} else if opts.Tools != nil {
		if len(opts.Tools) == 0 {
			cmd = append(cmd, "--tools", "")
		} else {
			cmd = append(cmd, "--tools", strings.Join(opts.Tools, ","))
		}
	}

	if len(opts.AllowedTools) > 0 {
		cmd = append(cmd, "--allowedTools", strings.Join(opts.AllowedTools, ","))
	}

	if opts.MaxTurns > 0 {
		cmd = append(cmd, "--max-turns", strconv.Itoa(opts.MaxTurns))
	}

	if opts.MaxBudgetUSD != nil {
		cmd = append(cmd, "--max-budget-usd", fmt.Sprintf("%g", *opts.MaxBudgetUSD))
	}

	if len(opts.DisallowedTools) > 0 {
		cmd = append(cmd, "--disallowedTools", strings.Join(opts.DisallowedTools, ","))
	}

	if opts.Model != "" {
		cmd = append(cmd, "--model", opts.Model)
	}

	if opts.FallbackModel != "" {
		cmd = append(cmd, "--fallback-model", opts.FallbackModel)
	}

	if len(opts.Betas) > 0 {
		betas := make([]string, len(opts.Betas))
		for i, b := range opts.Betas {
			betas[i] = string(b)
		}
		cmd = append(cmd, "--betas", strings.Join(betas, ","))
	}

	if opts.PermissionPromptToolName != "" {
		cmd = append(cmd, "--permission-prompt-tool", opts.PermissionPromptToolName)
	}

	if opts.PermissionMode != "" {
		cmd = append(cmd, "--permission-mode", string(opts.PermissionMode))
	}

	if opts.ContinueConversation {
		cmd = append(cmd, "--continue")
	}

	if opts.Resume != "" {
		cmd = append(cmd, "--resume", opts.Resume)
	}

	// Settings + sandbox merging
	settingsValue := t.buildSettingsValue()
	if settingsValue != "" {
		cmd = append(cmd, "--settings", settingsValue)
	}

	for _, dir := range opts.AddDirs {
		cmd = append(cmd, "--add-dir", dir)
	}

	// MCP servers
	if len(opts.McpServers) > 0 {
		serversForCLI := make(map[string]any, len(opts.McpServers))
		for name, config := range opts.McpServers {
			switch cfg := config.(type) {
			case *McpSdkServerConfig:
				// Strip instance field for CLI
				serversForCLI[name] = map[string]any{
					"type": cfg.Type,
					"name": cfg.Name,
				}
			case *McpStdioServerConfig:
				serversForCLI[name] = cfg
			case *McpSSEServerConfig:
				serversForCLI[name] = cfg
			case *McpHTTPServerConfig:
				serversForCLI[name] = cfg
			}
		}
		if len(serversForCLI) > 0 {
			data, _ := json.Marshal(map[string]any{"mcpServers": serversForCLI})
			cmd = append(cmd, "--mcp-config", string(data))
		}
	} else if opts.McpServersPath != "" {
		cmd = append(cmd, "--mcp-config", opts.McpServersPath)
	}

	if opts.IncludePartialMessages {
		cmd = append(cmd, "--include-partial-messages")
	}

	if opts.ForkSession {
		cmd = append(cmd, "--fork-session")
	}

	// Setting sources
	if opts.SettingSources != nil {
		sources := make([]string, len(opts.SettingSources))
		for i, s := range opts.SettingSources {
			sources[i] = string(s)
		}
		cmd = append(cmd, "--setting-sources", strings.Join(sources, ","))
	} else {
		cmd = append(cmd, "--setting-sources", "")
	}

	// Plugins
	for _, plugin := range opts.Plugins {
		if plugin.Type == "local" {
			cmd = append(cmd, "--plugin-dir", plugin.Path)
		}
	}

	// Extra args
	for flag, value := range opts.ExtraArgs {
		normalizedFlag := flag
		if !strings.HasPrefix(normalizedFlag, "--") {
			normalizedFlag = "--" + normalizedFlag
		}
		if value == nil {
			cmd = append(cmd, normalizedFlag)
		} else {
			cmd = append(cmd, normalizedFlag, *value)
		}
	}

	// Thinking config
	var maxThinkingTokens *int
	if opts.Thinking != nil {
		switch tc := opts.Thinking.(type) {
		case *ThinkingConfigAdaptive:
			if opts.MaxThinkingTokens == nil {
				v := 32000
				maxThinkingTokens = &v
			} else {
				maxThinkingTokens = opts.MaxThinkingTokens
			}
		case *ThinkingConfigEnabled:
			maxThinkingTokens = &tc.BudgetTokens
		case *ThinkingConfigDisabled:
			v := 0
			maxThinkingTokens = &v
		}
	} else {
		maxThinkingTokens = opts.MaxThinkingTokens
	}
	if maxThinkingTokens != nil {
		cmd = append(cmd, "--max-thinking-tokens", strconv.Itoa(*maxThinkingTokens))
	}

	if opts.Effort != "" {
		cmd = append(cmd, "--effort", string(opts.Effort))
	}

	// Output format → JSON schema
	if opts.OutputFormat != nil {
		if t, _ := opts.OutputFormat["type"].(string); t == "json_schema" {
			if schema, ok := opts.OutputFormat["schema"]; ok {
				data, _ := json.Marshal(schema)
				cmd = append(cmd, "--json-schema", string(data))
			}
		}
	}

	// Always use streaming mode
	cmd = append(cmd, "--input-format", "stream-json")

	return cmd
}

func (t *subprocessTransport) buildSettingsValue() string {
	hasSettings := t.options.Settings != ""
	hasSandbox := t.options.Sandbox != nil

	if !hasSettings && !hasSandbox {
		return ""
	}

	if hasSettings && !hasSandbox {
		return t.options.Settings
	}

	// Need to merge sandbox into settings
	settingsObj := map[string]any{}

	if hasSettings {
		s := strings.TrimSpace(t.options.Settings)
		if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
			_ = json.Unmarshal([]byte(s), &settingsObj)
		} else {
			// Try as file path
			data, err := os.ReadFile(s)
			if err == nil {
				_ = json.Unmarshal(data, &settingsObj)
			}
		}
	}

	if hasSandbox {
		settingsObj["sandbox"] = t.options.Sandbox
	}

	data, _ := json.Marshal(settingsObj)
	return string(data)
}

func (t *subprocessTransport) Connect(ctx context.Context) error {
	if t.process != nil {
		return nil
	}

	cmd := t.buildCommand()
	t.process = exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	if err := setProcessUser(t.process, t.options.User); err != nil {
		return &CLIConnectionError{
			SDKError: SDKError{
				Message: "Failed to configure process user",
				Cause:   err,
			},
		}
	}

	// Set environment
	env := os.Environ()
	for k, v := range t.options.Env {
		env = append(env, k+"="+v)
	}
	env = append(env,
		"CLAUDE_CODE_ENTRYPOINT=sdk-go",
		"CLAUDE_AGENT_SDK_VERSION="+Version,
	)
	if t.options.EnableFileCheckpointing {
		env = append(env, "CLAUDE_CODE_ENABLE_SDK_FILE_CHECKPOINTING=true")
	}
	t.process.Env = env

	if t.cwd != "" {
		t.process.Dir = t.cwd
		// Also set PWD
		t.process.Env = append(t.process.Env, "PWD="+t.cwd)
	}

	var err error
	t.stdin, err = t.process.StdinPipe()
	if err != nil {
		return &CLIConnectionError{SDKError: SDKError{Message: "Failed to create stdin pipe", Cause: err}}
	}

	t.stdout, err = t.process.StdoutPipe()
	if err != nil {
		return &CLIConnectionError{SDKError: SDKError{Message: "Failed to create stdout pipe", Cause: err}}
	}

	// Pipe stderr if callback is set or debug mode is enabled
	shouldPipeStderr := t.options.Stderr != nil || t.hasExtraArg("debug-to-stderr")
	if shouldPipeStderr {
		t.stderr, err = t.process.StderrPipe()
		if err != nil {
			return &CLIConnectionError{SDKError: SDKError{Message: "Failed to create stderr pipe", Cause: err}}
		}
	} else {
		if runtime.GOOS != "windows" {
			devNull, _ := os.Open(os.DevNull)
			t.process.Stderr = devNull
		}
	}

	if err := t.process.Start(); err != nil {
		if os.IsNotExist(err) {
			return &CLINotFoundError{
				CLIConnectionError: CLIConnectionError{SDKError: SDKError{Message: "Claude Code not found at: " + t.cliPath, Cause: err}},
				CLIPath:            t.cliPath,
			}
		}
		return &CLIConnectionError{SDKError: SDKError{Message: "Failed to start Claude Code", Cause: err}}
	}

	var readCtx context.Context
	readCtx, t.cancel = context.WithCancel(context.Background())

	// Start stderr reader
	if t.stderr != nil {
		go t.readStderr()
	}

	// Start stdout reader
	go t.readMessages(readCtx)

	t.ready = true
	return nil
}

func (t *subprocessTransport) readStderr() {
	if t.stderr == nil {
		return
	}
	scanner := bufio.NewScanner(t.stderr)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if t.options.Stderr != nil {
			t.options.Stderr(line)
		} else if t.hasExtraArg("debug-to-stderr") && t.options.DebugStderr != nil {
			_, _ = io.WriteString(t.options.DebugStderr, line+"\n")
			switch w := t.options.DebugStderr.(type) {
			case interface{ Flush() error }:
				_ = w.Flush()
			case interface{ Flush() }:
				w.Flush()
			}
		}
	}
}

func (t *subprocessTransport) readMessages(ctx context.Context) {
	defer close(t.msgChan)
	defer close(t.errChan)

	scanner := bufio.NewScanner(t.stdout)
	scanner.Buffer(make([]byte, 256*1024), t.maxBufferSize)

	jsonBuffer := ""

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Split on newlines (TextReceiveStream equivalent)
		jsonLines := strings.Split(line, "\n")
		for _, jsonLine := range jsonLines {
			jsonLine = strings.TrimSpace(jsonLine)
			if jsonLine == "" {
				continue
			}

			// Some wrapper scripts may print informational lines to stdout before
			// CLI JSON payloads. Ignore those lines when we are not buffering JSON.
			if jsonBuffer == "" {
				firstBrace := strings.Index(jsonLine, "{")
				if firstBrace < 0 {
					continue
				}
				if firstBrace > 0 {
					jsonLine = strings.TrimSpace(jsonLine[firstBrace:])
					if jsonLine == "" {
						continue
					}
				}
			}

			jsonBuffer += jsonLine

			if len(jsonBuffer) > t.maxBufferSize {
				err := &CLIJSONDecodeError{
					SDKError: SDKError{
						Message: fmt.Sprintf("JSON message exceeded maximum buffer size of %d bytes", t.maxBufferSize),
						Cause:   fmt.Errorf("buffer size %d exceeds limit %d", len(jsonBuffer), t.maxBufferSize),
					},
					Line: jsonBuffer,
				}
				t.setExitError(err)
				t.signalError(err)
				return
			}

			var data map[string]any
			if err := json.Unmarshal([]byte(jsonBuffer), &data); err != nil {
				// Accumulate more data
				continue
			}
			jsonBuffer = ""

			select {
			case t.msgChan <- data:
			case <-ctx.Done():
				return
			}
		}
	}

	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		decodeErr := &CLIJSONDecodeError{
			SDKError: SDKError{
				Message: "Failed reading JSON stream from CLI",
				Cause:   err,
			},
			Line: jsonBuffer,
		}
		t.setExitError(decodeErr)
		t.signalError(decodeErr)
		return
	}

	// Wait for process to finish
	if t.process != nil {
		if err := t.process.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				t.setExitError(NewProcessError(
					fmt.Sprintf("Command failed with exit code %d", exitErr.ExitCode()),
					exitErr.ExitCode(),
					"Check stderr output for details",
				))
			} else {
				t.setExitError(&ProcessError{
					SDKError: SDKError{Message: "Claude Code process failed", Cause: err},
				})
			}
		}
	}
	if err := t.LastError(); err != nil {
		t.signalError(err)
	}
}

func (t *subprocessTransport) Write(data string) error {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	if !t.ready || t.stdin == nil {
		return &CLIConnectionError{SDKError: SDKError{Message: "Transport is not ready for writing"}}
	}
	if exitErr := t.LastError(); exitErr != nil {
		return &CLIConnectionError{
			SDKError: SDKError{
				Message: "Cannot write to process that exited with error",
				Cause:   exitErr,
			},
		}
	}

	_, err := io.WriteString(t.stdin, data)
	if err != nil {
		t.ready = false
		return &CLIConnectionError{SDKError: SDKError{Message: "Failed to write to process stdin", Cause: err}}
	}
	return nil
}

func (t *subprocessTransport) Messages() <-chan map[string]any {
	return t.msgChan
}

func (t *subprocessTransport) Errors() <-chan error {
	return t.errChan
}

func (t *subprocessTransport) LastError() error {
	t.errMu.Lock()
	defer t.errMu.Unlock()
	return t.exitErr
}

func (t *subprocessTransport) EndInput() error {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	if t.stdin != nil {
		err := t.stdin.Close()
		t.stdin = nil
		return err
	}
	return nil
}

func (t *subprocessTransport) IsReady() bool {
	return t.ready
}

func (t *subprocessTransport) Close() error {
	t.writeMu.Lock()
	t.ready = false
	if t.stdin != nil {
		t.stdin.Close()
		t.stdin = nil
	}
	t.writeMu.Unlock()

	if t.cancel != nil {
		t.cancel()
	}

	if t.process != nil && t.process.Process != nil {
		_ = t.process.Process.Kill()
		_, _ = t.process.Process.Wait()
	}

	return nil
}

func (t *subprocessTransport) hasExtraArg(flag string) bool {
	if t.options == nil || t.options.ExtraArgs == nil {
		return false
	}
	if _, ok := t.options.ExtraArgs[flag]; ok {
		return true
	}
	if _, ok := t.options.ExtraArgs["--"+flag]; ok {
		return true
	}
	return false
}

func (t *subprocessTransport) setExitError(err error) {
	if err == nil {
		return
	}
	t.errMu.Lock()
	defer t.errMu.Unlock()
	if t.exitErr == nil {
		t.exitErr = err
	}
}

func (t *subprocessTransport) signalError(err error) {
	if err == nil {
		return
	}
	select {
	case t.errChan <- err:
	default:
	}
}
