#!/bin/bash
# Cleanup script to remove Kind cluster and resources

set -e

CLUSTER_NAME="packet-capture-test"

echo "=== Cleaning up packet capture test environment ==="

# Delete test pod
echo "Deleting test pod..."
kubectl delete pod test-pod --ignore-not-found=true

# Undeploy controller
echo "Undeploying controller..."
make undeploy || true

# Delete Kind cluster
echo "Deleting Kind cluster..."
sudo kind delete cluster --name "$CLUSTER_NAME"

# Clean up local files
echo "Cleaning up generated files..."
rm -f pod-describe.txt pods.txt capture-files.txt capture.pcap capture-output.txt

echo ""
echo "=== Cleanup complete! ==="
