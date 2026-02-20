package claude

import "fmt"

// SDKError is the base error type for all Claude SDK errors.
type SDKError struct {
	Message string
	Cause   error
}

func (e *SDKError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *SDKError) Unwrap() error {
	return e.Cause
}

// CLIConnectionError is raised when unable to connect to Claude Code.
type CLIConnectionError struct {
	SDKError
}

// CLINotFoundError is raised when Claude Code is not found or not installed.
type CLINotFoundError struct {
	CLIConnectionError
	CLIPath string
}

// ProcessError is raised when the CLI process fails.
type ProcessError struct {
	SDKError
	ExitCode int
	Stderr   string
}

func NewProcessError(message string, exitCode int, stderr string) *ProcessError {
	msg := message
	if exitCode != 0 {
		msg = fmt.Sprintf("%s (exit code: %d)", msg, exitCode)
	}
	if stderr != "" {
		msg = fmt.Sprintf("%s\nError output: %s", msg, stderr)
	}
	return &ProcessError{
		SDKError: SDKError{Message: msg},
		ExitCode: exitCode,
		Stderr:   stderr,
	}
}

// CLIJSONDecodeError is raised when unable to decode JSON from CLI output.
type CLIJSONDecodeError struct {
	SDKError
	Line string
}

// MessageParseError is raised when unable to parse a message from CLI output.
type MessageParseError struct {
	SDKError
	Data map[string]any
}
