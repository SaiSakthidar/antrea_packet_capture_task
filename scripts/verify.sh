#!/bin/bash

set -e

echo "=== Verifying Packet Capture Functionality ==="

echo "Deploying test pod..."
kubectl apply -f deploy/test-pod.yaml

echo "Waiting for test pod to be ready..."
kubectl wait --for=condition=Ready pod/test-pod --timeout=120s

NODE=$(kubectl get pod test-pod -o jsonpath='{.spec.nodeName}')
echo "Test pod is running on node: $NODE"

CONTROLLER_POD=$(kubectl get pod -l app=packet-capture-controller --field-selector spec.nodeName="$NODE" -o jsonpath='{.items[0].metadata.name}')
echo "Controller pod on same node: $CONTROLLER_POD"

echo ""
echo "=== Starting packet capture (annotation: tcpdump.antrea.io=5) ==="
kubectl annotate pod test-pod tcpdump.antrea.io="5" --overwrite

echo "Waiting for capture to start..."
sleep 10

echo ""
echo "=== Checking for pcap files ==="
kubectl exec "$CONTROLLER_POD" -- ls -lh /var/log/antrea-captures/ | tee capture-files.txt

if kubectl exec "$CONTROLLER_POD" -- ls /var/log/antrea-captures/capture-default-test-pod.pcap* >/dev/null 2>&1; then
    echo "✓ Capture files found!"
else
    echo "✗ No capture files found"
    exit 1
fi

echo ""
echo "Waiting for traffic to be captured..."
sleep 15

echo ""
echo "=== Saving pod description ==="
kubectl describe pod test-pod > pod-describe.txt
echo "Saved to pod-describe.txt"

echo ""
echo "=== Saving pods list ==="
kubectl get pods -A > pods.txt
echo "Saved to pods.txt"

echo ""
echo "=== Copying pcap file ==="
PCAP_FILE=$(kubectl exec "$CONTROLLER_POD" -- ls /var/log/antrea-captures/capture-default-test-pod.pcap* | head -1)
kubectl cp "$CONTROLLER_POD:$PCAP_FILE" ./capture.pcap
echo "Copied to ./capture.pcap"

echo ""
echo "=== Analyzing pcap file ==="
if command -v tcpdump >/dev/null 2>&1; then
    tcpdump -r ./capture.pcap -n | head -50 | tee capture-output.txt
    echo "Full output saved to capture-output.txt"
else
    echo "tcpdump not installed locally, analyzing inside container..."
    kubectl exec "$CONTROLLER_POD" -- tcpdump -r "$PCAP_FILE" -n | head -50 | tee capture-output.txt
fi

echo ""
echo "=== Stopping packet capture (removing annotation) ==="
kubectl annotate pod test-pod tcpdump.antrea.io-

echo "Waiting for cleanup..."
sleep 5

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
