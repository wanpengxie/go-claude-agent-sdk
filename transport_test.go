package claude

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestBuildCommandBasic(t *testing.T) {
	opts := &AgentOptions{}
	tr := newSubprocessTransport(opts)
	cmd := tr.buildCommand()

	// Should contain basic flags
	found := map[string]bool{
		"--output-format":   false,
		"stream-json":       false,
		"--verbose":         false,
		"--input-format":    false,
		"--system-prompt":   false,
		"--setting-sources": false,
	}

	for _, arg := range cmd {
		if _, ok := found[arg]; ok {
			found[arg] = true
		}
	}

	for flag, present := range found {
		if !present {
			t.Errorf("expected flag %s in command", flag)
		}
	}
}

func TestBuildCommandWithModel(t *testing.T) {
	opts := &AgentOptions{Model: "claude-sonnet-4-5"}
	tr := newSubprocessTransport(opts)
	cmd := tr.buildCommand()

	cmdStr := strings.Join(cmd, " ")
	if !strings.Contains(cmdStr, "--model claude-sonnet-4-5") {
		t.Errorf("expected --model flag in command: %s", cmdStr)
	}
}

func TestBuildCommandWithMaxTurns(t *testing.T) {
	opts := &AgentOptions{MaxTurns: 5}
	tr := newSubprocessTransport(opts)
	cmd := tr.buildCommand()

	cmdStr := strings.Join(cmd, " ")
	if !strings.Contains(cmdStr, "--max-turns 5") {
		t.Errorf("expected --max-turns 5 in command: %s", cmdStr)
	}
}

func TestBuildCommandWithPermissionMode(t *testing.T) {
	opts := &AgentOptions{PermissionMode: PermissionAcceptEdits}
	tr := newSubprocessTransport(opts)
	cmd := tr.buildCommand()

	cmdStr := strings.Join(cmd, " ")
	if !strings.Contains(cmdStr, "--permission-mode acceptEdits") {
		t.Errorf("expected --permission-mode in command: %s", cmdStr)
	}
}

func TestBuildCommandWithAllowedTools(t *testing.T) {
	opts := &AgentOptions{AllowedTools: []string{"Read", "Write", "Bash"}}
	tr := newSubprocessTransport(opts)
	cmd := tr.buildCommand()

	cmdStr := strings.Join(cmd, " ")
	if !strings.Contains(cmdStr, "--allowedTools Read,Write,Bash") {
		t.Errorf("expected --allowedTools in command: %s", cmdStr)
	}
}

func TestBuildCommandWithSystemPrompt(t *testing.T) {
	prompt := "You are a helpful assistant"
	opts := &AgentOptions{SystemPrompt: &prompt}
	tr := newSubprocessTransport(opts)
	cmd := tr.buildCommand()

	cmdStr := strings.Join(cmd, " ")
	if !strings.Contains(cmdStr, "--system-prompt "+prompt) {
		t.Errorf("expected --system-prompt in command: %s", cmdStr)
	}
}

func TestBuildCommandWithThinkingEnabled(t *testing.T) {
	opts := &AgentOptions{
		Thinking: &ThinkingConfigEnabled{BudgetTokens: 16000},
	}
	tr := newSubprocessTransport(opts)
	cmd := tr.buildCommand()

	cmdStr := strings.Join(cmd, " ")
	if !strings.Contains(cmdStr, "--max-thinking-tokens 16000") {
		t.Errorf("expected --max-thinking-tokens 16000 in command: %s", cmdStr)
	}
}

func TestBuildCommandWithThinkingDisabled(t *testing.T) {
	opts := &AgentOptions{
		Thinking: &ThinkingConfigDisabled{},
	}
	tr := newSubprocessTransport(opts)
	cmd := tr.buildCommand()

	cmdStr := strings.Join(cmd, " ")
	if !strings.Contains(cmdStr, "--max-thinking-tokens 0") {
		t.Errorf("expected --max-thinking-tokens 0 in command: %s", cmdStr)
	}
}

func TestBuildCommandWithEffort(t *testing.T) {
	opts := &AgentOptions{Effort: EffortHigh}
	tr := newSubprocessTransport(opts)
	cmd := tr.buildCommand()

	cmdStr := strings.Join(cmd, " ")
	if !strings.Contains(cmdStr, "--effort high") {
		t.Errorf("expected --effort high in command: %s", cmdStr)
	}
}

func TestBuildCommandWithContinue(t *testing.T) {
	opts := &AgentOptions{ContinueConversation: true}
	tr := newSubprocessTransport(opts)
	cmd := tr.buildCommand()

	found := false
	for _, arg := range cmd {
		if arg == "--continue" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected --continue flag in command")
	}
}

func TestBuildCommandWithExtraArgs(t *testing.T) {
	val := "value1"
	opts := &AgentOptions{
		ExtraArgs: map[string]*string{
			"debug-to-stderr": nil,
			"custom-flag":     &val,
		},
	}
	tr := newSubprocessTransport(opts)
	cmd := tr.buildCommand()

	cmdStr := strings.Join(cmd, " ")
	if !strings.Contains(cmdStr, "--debug-to-stderr") {
		t.Errorf("expected --debug-to-stderr in command: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "--custom-flag value1") {
		t.Errorf("expected --custom-flag value1 in command: %s", cmdStr)
	}
}

func TestBuildSettingsValueEmpty(t *testing.T) {
	tr := &subprocessTransport{options: &AgentOptions{}}
	val := tr.buildSettingsValue()
	if val != "" {
		t.Errorf("expected empty, got %s", val)
	}
}

func TestBuildSettingsValueSettingsOnly(t *testing.T) {
	tr := &subprocessTransport{options: &AgentOptions{Settings: "/path/to/settings.json"}}
	val := tr.buildSettingsValue()
	if val != "/path/to/settings.json" {
		t.Errorf("expected path, got %s", val)
	}
}

func TestBuildSettingsValueSandboxOnly(t *testing.T) {
	enabled := true
	tr := &subprocessTransport{options: &AgentOptions{
		Sandbox: &SandboxSettings{Enabled: &enabled},
	}}
	val := tr.buildSettingsValue()
	if val == "" {
		t.Error("expected non-empty settings value")
	}
	if !strings.Contains(val, "sandbox") {
		t.Errorf("expected sandbox in settings: %s", val)
	}
}

func TestBuildCommandWithExtraArgsLeadingDashes(t *testing.T) {
	val := "1"
	opts := &AgentOptions{
		ExtraArgs: map[string]*string{
			"--already-prefixed": &val,
		},
	}
	tr := newSubprocessTransport(opts)
	cmd := tr.buildCommand()
	cmdStr := strings.Join(cmd, " ")
	if strings.Contains(cmdStr, "----already-prefixed") {
		t.Fatalf("unexpected duplicated dashes: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "--already-prefixed 1") {
		t.Fatalf("expected --already-prefixed 1 in command: %s", cmdStr)
	}
}

func TestReadMessagesBufferOverflowReturnsError(t *testing.T) {
	opts := &AgentOptions{MaxBufferSize: 16}
	tr := newSubprocessTransport(opts)
	tr.stdout = io.NopCloser(strings.NewReader(`{"type":"assistant","message":{"content":[` + "\n"))

	tr.readMessages(context.Background())

	err, ok := <-tr.Errors()
	if !ok {
		t.Fatal("expected an error value before channel close")
	}
	var decodeErr *CLIJSONDecodeError
	if !errors.As(err, &decodeErr) {
		t.Fatalf("expected CLIJSONDecodeError, got %T (%v)", err, err)
	}
	if tr.LastError() == nil {
		t.Fatal("expected transport last error to be recorded")
	}
}

func TestReadMessagesSkipsNonJSONPrelude(t *testing.T) {
	opts := &AgentOptions{}
	tr := newSubprocessTransport(opts)
	tr.stdout = io.NopCloser(strings.NewReader("[claude-wrapper] prelude\n{\"type\":\"system\",\"subtype\":\"init\"}\n"))

	tr.readMessages(context.Background())

	var got []map[string]any
	for msg := range tr.Messages() {
		got = append(got, msg)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 parsed message, got %d", len(got))
	}
	if typ, _ := got[0]["type"].(string); typ != "system" {
		t.Fatalf("expected parsed message type system, got %q", typ)
	}
	if sub, _ := got[0]["subtype"].(string); sub != "init" {
		t.Fatalf("expected subtype init, got %q", sub)
	}
	if err := tr.LastError(); err != nil {
		t.Fatalf("expected no transport error, got %v", err)
	}
}
