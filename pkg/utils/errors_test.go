package utils

import (
	"errors"
	"strings"
	"testing"
)

// TestErrorMessageClarity validates Property 8: Error message clarity
// For any error condition, the system should produce error messages that clearly
// describe what went wrong and suggest potential solutions
func TestErrorMessageClarity(t *testing.T) {
	tests := []struct {
		name          string
		errorFunc     func() error
		wantOperation string
		wantReason    bool
		wantHint      bool
	}{
		{
			name: "container not found error has clear message",
			errorFunc: func() error {
				return NewContainerNotFoundError("test-pod", errors.New("no containers"))
			},
			wantOperation: "Container discovery",
			wantReason:    true,
			wantHint:      true,
		},
		{
			name: "process not found error has clear message",
			errorFunc: func() error {
				return NewProcessNotFoundError("abc123", errors.New("pid not found"))
			},
			wantOperation: "Process discovery",
			wantReason:    true,
			wantHint:      true,
		},
		{
			name: "tcpdump execution error has clear message",
			errorFunc: func() error {
				return NewTcpdumpExecutionError("test-pod", errors.New("permission denied"))
			},
			wantOperation: "Tcpdump execution",
			wantReason:    true,
			wantHint:      true,
		},
		{
			name: "file cleanup error has clear message",
			errorFunc: func() error {
				return NewFileCleanupError("/tmp/test.pcap", errors.New("file not found"))
			},
			wantOperation: "File cleanup",
			wantReason:    true,
			wantHint:      true,
		},
		{
			name: "annotation parse error has clear message",
			errorFunc: func() error {
				return NewAnnotationParseError("invalid", errors.New("not a number"))
			},
			wantOperation: "Annotation parsing",
			wantReason:    true,
			wantHint:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.errorFunc()
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			errMsg := err.Error()

			// Verify operation is mentioned
			if !strings.Contains(errMsg, tt.wantOperation) {
				t.Errorf("error message should contain operation '%s', got: %s", tt.wantOperation, errMsg)
			}

			// Verify reason is present (should contain "failed:")
			if tt.wantReason && !strings.Contains(errMsg, "failed:") {
				t.Errorf("error message should contain reason (failed:), got: %s", errMsg)
			}

			// Verify hint is present (hints typically contain actionable words)
			if tt.wantHint {
				hasHint := strings.Contains(errMsg, "Ensure") ||
					strings.Contains(errMsg, "Check") ||
					strings.Contains(errMsg, "Verify") ||
					strings.Contains(errMsg, "Example:")
				if !hasHint {
					t.Errorf("error message should contain actionable hint, got: %s", errMsg)
				}
			}

			// Verify error is not empty or too short
			if len(errMsg) < 20 {
				t.Errorf("error message too short to be helpful: %s", errMsg)
			}

			// Verify error can be unwrapped
			captureErr, ok := err.(*CaptureError)
			if !ok {
				t.Errorf("error should be of type *CaptureError")
			}

			if captureErr.Unwrap() == nil {
				t.Errorf("error should wrap underlying error")
			}
		})
	}
}

// TestErrorMessageFormat validates that all error messages follow a consistent format
func TestErrorMessageFormat(t *testing.T) {
	testErr := errors.New("underlying error")

	errors := []error{
		NewContainerNotFoundError("pod1", testErr),
		NewProcessNotFoundError("container1", testErr),
		NewTcpdumpExecutionError("pod2", testErr),
		NewFileCleanupError("/path/file", testErr),
		NewAnnotationParseError("bad-value", testErr),
	}

	for _, err := range errors {
		msg := err.Error()

		// All errors should follow format: "<Operation> failed: <Reason>. <Hint>"
		if !strings.Contains(msg, "failed:") {
			t.Errorf("error message should contain 'failed:', got: %s", msg)
		}

		// Should have at least 3 parts separated by punctuation
		parts := strings.Split(msg, ".")
		if len(parts) < 2 {
			t.Errorf("error message should have multiple sentences, got: %s", msg)
		}
	}
}

// TestErrorWrapping validates that errors properly wrap underlying errors
func TestErrorWrapping(t *testing.T) {
	underlyingErr := errors.New("original error")
	captureErr := NewContainerNotFoundError("test-pod", underlyingErr)

	// Test Unwrap
	unwrapped := errors.Unwrap(captureErr)
	if unwrapped != underlyingErr {
		t.Errorf("Unwrap() should return underlying error")
	}

	// Test errors.Is
	if !errors.Is(captureErr, underlyingErr) {
		t.Errorf("errors.Is should work with wrapped errors")
	}
}
