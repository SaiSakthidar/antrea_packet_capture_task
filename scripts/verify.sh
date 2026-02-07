#!/bin/bash
# Verification script for packet capture functionality

set -e

echo "=== Verifying Packet Capture Functionality ==="

# Deploy test pod
echo "Deploying test pod..."
kubectl apply -f deploy/test-pod.yaml

# Wait for test pod to be ready
echo "Waiting for test pod to be ready..."
kubectl wait --for=condition=Ready pod/test-pod --timeout=120s

# Get test pod node
NODE=$(kubectl get pod test-pod -o jsonpath='{.spec.nodeName}')
echo "Test pod is running on node: $NODE"

# Find controller pod on the same node
CONTROLLER_POD=$(kubectl get pod -l app=packet-capture-controller --field-selector spec.nodeName="$NODE" -o jsonpath='{.items[0].metadata.name}')
echo "Controller pod on same node: $CONTROLLER_POD"

# Add capture annotation
echo ""
echo "=== Starting packet capture (annotation: tcpdump.antrea.io=5) ==="
kubectl annotate pod test-pod tcpdump.antrea.io="5" --overwrite

# Wait for capture to start
echo "Waiting for capture to start..."
sleep 10

# Check for pcap files
echo ""
echo "=== Checking for pcap files ==="
kubectl exec "$CONTROLLER_POD" -- ls -lh /var/log/antrea-captures/ | tee capture-files.txt

# Verify files exist
if kubectl exec "$CONTROLLER_POD" -- ls /var/log/antrea-captures/capture-default-test-pod.pcap* >/dev/null 2>&1; then
    echo "✓ Capture files found!"
else
    echo "✗ No capture files found"
    exit 1
fi

# Wait a bit more for traffic
echo ""
echo "Waiting for traffic to be captured..."
sleep 15

# Save pod description
echo ""
echo "=== Saving pod description ==="
kubectl describe pod test-pod > pod-describe.txt
echo "Saved to pod-describe.txt"

# Save pods list
echo ""
echo "=== Saving pods list ==="
kubectl get pods -A > pods.txt
echo "Saved to pods.txt"

# Copy pcap file
echo ""
echo "=== Copying pcap file ==="
PCAP_FILE=$(kubectl exec "$CONTROLLER_POD" -- ls /var/log/antrea-captures/capture-default-test-pod.pcap* | head -1)
kubectl cp "$CONTROLLER_POD:$PCAP_FILE" ./capture.pcap
echo "Copied to ./capture.pcap"

# Analyze pcap file
echo ""
echo "=== Analyzing pcap file ==="
if command -v tcpdump >/dev/null 2>&1; then
    tcpdump -r ./capture.pcap -n | head -50 | tee capture-output.txt
    echo "Full output saved to capture-output.txt"
else
    echo "tcpdump not installed locally, analyzing inside container..."
    kubectl exec "$CONTROLLER_POD" -- tcpdump -r "$PCAP_FILE" -n | head -50 | tee capture-output.txt
fi

# Remove annotation
echo ""
echo "=== Stopping packet capture (removing annotation) ==="
kubectl annotate pod test-pod tcpdump.antrea.io-

# Wait for cleanup
echo "Waiting for cleanup..."
sleep 5

# Verify files are deleted
echo ""
echo "=== Verifying cleanup ==="
if kubectl exec "$CONTROLLER_POD" -- ls /var/log/antrea-captures/capture-default-test-pod.pcap* >/dev/null 2>&1; then
    echo "✗ Capture files still exist (cleanup failed)"
    exit 1
else
    echo "✓ Capture files cleaned up successfully!"
fi

echo ""
echo "=== Verification complete! ==="
echo ""
echo "Generated files:"
echo "  - pod-describe.txt: Pod description with annotation"
echo "  - pods.txt: List of all pods"
echo "  - capture-files.txt: Listing of capture files"
echo "  - capture.pcap: Extracted pcap file"
echo "  - capture-output.txt: Human-readable tcpdump output"
