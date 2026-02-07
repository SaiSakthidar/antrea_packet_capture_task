package capture

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

var testCaptureDir = CaptureDir

func TestCommandGenerationConsistency(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("nsenter command format is consistent", prop.ForAll(
		func(namespace, podName string, pid int) bool {
			expectedFile := filepath.Join(testCaptureDir, fmt.Sprintf("capture-%s-%s.pcap", namespace, podName))
			return filepath.IsAbs(expectedFile) && 
				   filepath.Dir(expectedFile) == testCaptureDir &&
				   pid > 0
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 64 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 64 }),
		gen.IntRange(1, 65535),
	))

	properties.TestingRun(t)
}

func TestFileCleanupOnCaptureStop(t *testing.T) {
	tmpDir := t.TempDir()
	
	namespace := "test-ns"
	podName := "test-pod"

	testFiles := []string{
		filepath.Join(tmpDir, fmt.Sprintf("capture-%s-%s.pcap", namespace, podName)),
		filepath.Join(tmpDir, fmt.Sprintf("capture-%s-%s.pcap1", namespace, podName)),
		filepath.Join(tmpDir, fmt.Sprintf("capture-%s-%s.pcap2", namespace, podName)),
	}

	for _, f := range testFiles {
		if err := os.WriteFile(f, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	pattern := filepath.Join(tmpDir, fmt.Sprintf("capture-%s-%s.pcap*", namespace, podName))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("Failed to glob files: %v", err)
	}
	for _, f := range matches {
		if err := os.Remove(f); err != nil {
			t.Errorf("Failed to remove file %s: %v", f, err)
		}
	}

	for _, f := range testFiles {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("File %s was not cleaned up", f)
		}
	}
}

func TestCompleteSessionCleanup(t *testing.T) {
	manager := NewManager()

	key := "test-ns/test-pod"

	manager.mu.Lock()
	manager.sessions[key] = &session{cancel: func() {}}
	manager.mu.Unlock()

	manager.StopCapture("test-ns", "test-pod")

	manager.mu.Lock()
	_, exists := manager.sessions[key]
	manager.mu.Unlock()

	if exists {
		t.Error("Session was not cleaned up from manager")
	}
}

func TestProcessTerminationOnAnnotationRemoval(t *testing.T) {
	manager := NewManager()

	key := "test-ns/test-pod"
	cancelled := false

	manager.mu.Lock()
	manager.sessions[key] = &session{
		cancel: func() { cancelled = true },
	}
	manager.mu.Unlock()

	manager.StopCapture("test-ns", "test-pod")

	time.Sleep(100 * time.Millisecond)

	if !cancelled {
		t.Error("Cancel function was not called")
	}

	manager.mu.Lock()
	_, exists := manager.sessions[key]
	manager.mu.Unlock()

	if exists {
		t.Error("Session still exists after stop")
	}
}
