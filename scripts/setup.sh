#!/bin/bash

set -e

echo "=== Setting up Kind cluster with Antrea ==="

command -v kind >/dev/null 2>&1 || { echo "Error: kind is not installed"; exit 1; }
command -v kubectl >/dev/null 2>&1 || { echo "Error: kubectl is not installed"; exit 1; }
command -v docker >/dev/null 2>&1 || { echo "Error: docker is not installed"; exit 1; }

CLUSTER_NAME="packet-capture-test"

echo "Creating Kind cluster..."
kind create cluster --config kind-config.yaml --name "$CLUSTER_NAME"

echo "Waiting for cluster to be ready..."
kubectl wait --for=condition=Ready nodes --all --timeout=120s

echo "Installing Antrea..."
if ! command -v helm >/dev/null 2>&1; then
    echo "Helm not found, installing Antrea using kubectl..."
    kubectl apply -f https://github.com/antrea-io/antrea/releases/download/v2.2.0/antrea.yml
else
    helm repo add antrea https://charts.antrea.io
    helm repo update
    helm install antrea antrea/antrea --namespace kube-system
fi

echo "Waiting for Antrea to be ready..."
kubectl wait --for=condition=Ready pods -l app=antrea -n kube-system --timeout=300s

echo "Building controller image..."
docker build -t packet-capture-controller:latest .

echo "Loading controller image into Kind..."
kind load docker-image packet-capture-controller:latest --name "$CLUSTER_NAME"

echo "Deploying packet capture controller..."
make deploy

echo "Waiting for controller pods to be ready..."
kubectl wait --for=condition=Ready pods -l app=packet-capture-controller --timeout=120s

echo ""
echo "=== Setup complete! ==="
echo ""
echo "Cluster: $CLUSTER_NAME"
echo "Nodes:"
kubectl get nodes
echo ""
echo "Controller pods:"
kubectl get pods -l app=packet-capture-controller
echo ""
echo "Next steps:"
echo "  1. Deploy test pod: make deploy-test-pod"
echo "  2. Run verification: ./scripts/verify.sh"
