package utils

import "fmt"

// CaptureError represents errors that occur during packet capture operations
type CaptureError struct {
	Operation string
	Reason    string
	Hint      string
	Err       error
}

func (e *CaptureError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s failed: %s. %s (underlying error: %v)", e.Operation, e.Reason, e.Hint, e.Err)
	}
	return fmt.Sprintf("%s failed: %s. %s", e.Operation, e.Reason, e.Hint)
}

func (e *CaptureError) Unwrap() error {
	return e.Err
}

// NewCaptureError creates a new CaptureError with helpful context
func NewCaptureError(operation, reason, hint string, err error) *CaptureError {
	return &CaptureError{
		Operation: operation,
		Reason:    reason,
		Hint:      hint,
		Err:       err,
	}
}

// Common error constructors with helpful hints

func NewContainerNotFoundError(podName string, err error) *CaptureError {
	return NewCaptureError(
		"Container discovery",
		fmt.Sprintf("Could not find container for pod %s", podName),
		"Ensure the pod is running and has at least one container. Check pod status with: kubectl describe pod "+podName,
		err,
	)
}

func NewProcessNotFoundError(containerID string, err error) *CaptureError {
	return NewCaptureError(
		"Process discovery",
		fmt.Sprintf("Could not find process for container %s", containerID),
		"The container may have just started or terminated. Verify with: kubectl get pod -o jsonpath='{.status.containerStatuses[0].state}'",
		err,
	)
}

func NewTcpdumpExecutionError(podName string, err error) *CaptureError {
	return NewCaptureError(
		"Tcpdump execution",
		fmt.Sprintf("Failed to start tcpdump for pod %s", podName),
		"Ensure the controller has privileged access and tcpdump is installed. Check security context in DaemonSet manifest.",
		err,
	)
}

func NewFileCleanupError(filePath string, err error) *CaptureError {
	return NewCaptureError(
		"File cleanup",
		fmt.Sprintf("Failed to remove capture file %s", filePath),
		"Check file permissions and disk space. The file may have been manually deleted.",
		err,
	)
}

func NewAnnotationParseError(value string, err error) *CaptureError {
	return NewCaptureError(
		"Annotation parsing",
		fmt.Sprintf("Invalid annotation value: %s", value),
		"The annotation value must be a positive integer representing max capture files. Example: kubectl annotate pod <name> tcpdump.antrea.io=\"5\"",
		err,
	)
}
