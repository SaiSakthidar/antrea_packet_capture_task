#!/bin/bash

set -e

CLUSTER_NAME="packet-capture-test"

echo "=== Cleaning up packet capture test environment ==="

echo "Deleting test pod..."
kubectl delete pod test-pod --ignore-not-found=true

echo "Undeploying controller..."
make undeploy || true

echo "Deleting Kind cluster..."
kind delete cluster --name "$CLUSTER_NAME"

echo "Cleaning up generated files..."
rm -f pod-describe.txt pods.txt capture-files.txt capture.pcap capture-output.txt

echo ""
echo "=== Cleanup complete! ===
