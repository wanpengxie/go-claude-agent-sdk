package claude

import (
	"errors"
	"testing"
)

func TestSDKError(t *testing.T) {
	err := &SDKError{Message: "something went wrong"}
	if err.Error() != "something went wrong" {
		t.Errorf("expected 'something went wrong', got %q", err.Error())
	}
}

func TestSDKErrorWithCause(t *testing.T) {
	cause := errors.New("root cause")
	err := &SDKError{Message: "wrapped", Cause: cause}
	if err.Unwrap() != cause {
		t.Error("Unwrap should return the cause")
	}
	if !errors.Is(err, cause) {
		t.Error("errors.Is should match the cause")
	}
}

func TestCLIConnectionError(t *testing.T) {
	err := &CLIConnectionError{SDKError: SDKError{Message: "connection failed"}}
	if err.Error() != "connection failed" {
		t.Errorf("unexpected error message: %q", err.Error())
	}

	var connErr *CLIConnectionError
	if !errors.As(err, &connErr) {
		t.Error("errors.As should match CLIConnectionError")
	}

	// In Go, embedded struct doesn't make errors.As match parent types
	// but we can access the embedded SDKError directly
	if err.SDKError.Message != "connection failed" {
		t.Error("should access embedded SDKError")
	}
}

func TestCLINotFoundError(t *testing.T) {
	err := &CLINotFoundError{
		CLIConnectionError: CLIConnectionError{SDKError: SDKError{Message: "not found"}},
		CLIPath:            "/usr/local/bin/claude",
	}
	if err.CLIPath != "/usr/local/bin/claude" {
		t.Errorf("unexpected CLIPath: %q", err.CLIPath)
	}

	var notFoundErr *CLINotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Error("errors.As should match CLINotFoundError")
	}

	// Access embedded parent types directly
	if err.CLIConnectionError.SDKError.Message != "not found" {
		t.Error("should access embedded CLIConnectionError")
	}
}

func TestProcessError(t *testing.T) {
	err := &ProcessError{
		SDKError: SDKError{Message: "process failed"},
		ExitCode: 1,
		Stderr:   "error output",
	}
	if err.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", err.ExitCode)
	}
	if err.Stderr != "error output" {
		t.Errorf("unexpected stderr: %q", err.Stderr)
	}
}

func TestCLIJSONDecodeError(t *testing.T) {
	err := &CLIJSONDecodeError{
		SDKError: SDKError{Message: "bad json"},
		Line:     `{"broken":`,
	}
	if err.Line != `{"broken":` {
		t.Errorf("unexpected line: %q", err.Line)
	}
}

func TestMessageParseError(t *testing.T) {
	data := map[string]any{"type": "unknown"}
	err := &MessageParseError{
		SDKError: SDKError{Message: "parse failed"},
		Data:     data,
	}
	if err.Data["type"] != "unknown" {
		t.Error("expected data to be preserved")
	}
}

func TestErrorHierarchyIs(t *testing.T) {
	cause := errors.New("underlying")
	procErr := &ProcessError{
		SDKError: SDKError{Message: "proc", Cause: cause},
		ExitCode: 2,
	}

	if !errors.Is(procErr, cause) {
		t.Error("ProcessError should chain to its cause via errors.Is")
	}
}
