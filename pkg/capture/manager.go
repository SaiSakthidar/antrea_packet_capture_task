package capture

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

const (
	CaptureAnnotation = "tcpdump.antrea.io"
	CaptureDir        = "/var/log/antrea-captures"
)

type session struct {
	cancel context.CancelFunc
}

type Manager struct {
	mu       sync.Mutex
	sessions map[string]*session
}

func NewManager() *Manager {
	if err := os.MkdirAll(CaptureDir, 0755); err != nil {
		klog.Errorf("Failed to create capture directory: %v", err)
	}
	return &Manager{
		sessions: make(map[string]*session),
	}
}

func (m *Manager) StartCapture(pod *corev1.Pod) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
	if _, exists := m.sessions[key]; exists {
		klog.V(2).Infof("Capture already running for pod %s", key)
		return nil
	}

	limit, ok := pod.Annotations[CaptureAnnotation]
	if !ok {
		return fmt.Errorf("capture annotation not found")
	}

	if _, err := strconv.Atoi(limit); err != nil {
		limit = "10"
		klog.Warningf("Invalid capture limit for pod %s, using default: 10", key)
	}

	if len(pod.Status.ContainerStatuses) == 0 {
		return fmt.Errorf("no container statuses found for pod %s", key)
	}

	containerID := pod.Status.ContainerStatuses[0].ContainerID
	parts := strings.Split(containerID, "://")
	if len(parts) < 2 {
		return fmt.Errorf("invalid container ID format: %s", containerID)
	}
	cid := parts[1]

	pid, err := findPidByContainerID(cid)
	if err != nil {
		return fmt.Errorf("failed to find PID for container %s: %w", cid, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.sessions[key] = &session{cancel: cancel}

	klog.Infof("Starting capture for pod %s (PID: %d, limit: %s)", key, pid, limit)
	go m.runTcpdump(ctx, pod, pid, limit, key)

	return nil
}

func (m *Manager) StopCapture(namespace, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	if sess, exists := m.sessions[key]; exists {
		klog.Infof("Stopping capture for pod %s", key)
		sess.cancel()
		delete(m.sessions, key)
		m.cleanupFiles(namespace, name)
	}
}

func (m *Manager) runTcpdump(ctx context.Context, pod *corev1.Pod, pid int, limit, key string) {
	pcapFile := filepath.Join(CaptureDir, fmt.Sprintf("capture-%s-%s.pcap", pod.Namespace, pod.Name))

	args := []string{
		"-t", fmt.Sprintf("%d", pid),
		"-n",
		"--",
		"tcpdump",
		"-Z", "root",
		"-i", "any",
		"-C", "1",
		"-W", limit,
		"-w", pcapFile,
	}

	klog.V(2).Infof("Executing: nsenter %v", args)
	cmd := exec.CommandContext(ctx, "nsenter", args...)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.Canceled {
			klog.V(2).Infof("Capture stopped gracefully for pod %s", key)
		} else {
			klog.Errorf("tcpdump exited with error for pod %s: %v", key, err)
		}
	}

	m.mu.Lock()
	delete(m.sessions, key)
	m.mu.Unlock()
}

func (m *Manager) cleanupFiles(namespace, podName string) {
	pattern := filepath.Join(CaptureDir, fmt.Sprintf("capture-%s-%s.pcap*", namespace, podName))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		klog.Errorf("Failed to glob cleanup files: %v", err)
		return
	}
	for _, f := range matches {
		if err := os.Remove(f); err != nil {
			klog.Errorf("Failed to remove capture file %s: %v", f, err)
		} else {
			klog.V(2).Infof("Removed capture file: %s", f)
		}
	}
}

func findPidByContainerID(containerID string) (int, error) {
	dirs, err := os.ReadDir("/proc")
	if err != nil {
		return 0, fmt.Errorf("failed to read /proc: %w", err)
	}

	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		var testPid int
		if _, err := fmt.Sscanf(d.Name(), "%d", &testPid); err != nil {
			continue
		}

		cgroupPath := filepath.Join("/proc", d.Name(), "cgroup")
		f, err := os.Open(cgroupPath)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(f)
		found := false
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), containerID) {
				found = true
				break
			}
		}
		f.Close()

		if found {
			return testPid, nil
		}
	}
	return 0, fmt.Errorf("pid not found for container %s", containerID)
}
