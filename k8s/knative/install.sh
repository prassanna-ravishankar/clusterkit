#!/bin/bash
set -euo pipefail

# ClusterKit - Knative Serving Installation Script
# This script installs Knative Serving v1.15.0 with Kourier networking

KNATIVE_VERSION="1.15.0"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "=================================="
echo "ClusterKit Knative Installation"
echo "Version: ${KNATIVE_VERSION}"
echo "=================================="
echo

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl is not installed or not in PATH"
    exit 1
fi

# Check if cluster is accessible
if ! kubectl cluster-info &> /dev/null; then
    echo "Error: Cannot connect to Kubernetes cluster"
    echo "Please configure kubectl first:"
    echo "  gcloud container clusters get-credentials <cluster-name> --region <region>"
    exit 1
fi

echo "Step 1: Installing Knative Serving CRDs..."
kubectl apply -f "${SCRIPT_DIR}/01-serving-crds.yaml"
echo "✓ CRDs installed"
echo

echo "Step 2: Waiting for CRDs to be established..."
sleep 5
kubectl wait --for condition=established --timeout=60s \
  crd/services.serving.knative.dev \
  crd/configurations.serving.knative.dev \
  crd/revisions.serving.knative.dev \
  crd/routes.serving.knative.dev
echo "✓ CRDs ready"
echo

echo "Step 3: Installing Knative Serving core components..."
kubectl apply -f https://github.com/knative/serving/releases/download/knative-v${KNATIVE_VERSION}/serving-core.yaml
echo "✓ Core components installed"
echo

echo "Step 4: Waiting for Knative Serving to be ready..."
kubectl wait --for=condition=ready pod \
  --selector=app.kubernetes.io/name=knative-serving \
  --namespace=knative-serving \
  --timeout=300s
echo "✓ Knative Serving ready"
echo

echo "Step 5: Installing Kourier networking layer..."
kubectl apply -f https://github.com/knative/net-kourier/releases/download/knative-v${KNATIVE_VERSION}/kourier.yaml
echo "✓ Kourier installed"
echo

echo "Step 6: Configuring Knative to use Kourier..."
kubectl patch configmap/config-network \
  --namespace knative-serving \
  --type merge \
  --patch '{"data":{"ingress-class":"kourier.ingress.networking.knative.dev"}}'
echo "✓ Kourier configured as ingress"
echo

echo "Step 7: Waiting for Kourier to be ready..."
kubectl wait --for=condition=ready pod \
  --selector=app=3scale-kourier-gateway \
  --namespace=kourier-system \
  --timeout=300s
echo "✓ Kourier ready"
echo

echo "Step 8: Configuring autoscaling parameters..."
kubectl apply -f "${SCRIPT_DIR}/05-config-autoscaler.yaml"
echo "✓ Autoscaling configured"
echo

echo "Step 9: Verifying installation..."
echo
echo "Knative Serving components:"
kubectl get pods -n knative-serving
echo
echo "Kourier components:"
kubectl get pods -n kourier-system
echo
echo "Kourier LoadBalancer service:"
kubectl get svc kourier -n kourier-system
echo

echo "=================================="
echo "✓ Installation Complete!"
echo "=================================="
echo
echo "Next steps:"
echo "1. Note the Kourier LoadBalancer external IP (may take a few minutes to provision)"
echo "2. Configure this IP in your NGINX ingress (Task 3)"
echo "3. Deploy a test Knative Service to verify functionality"
echo
echo "To deploy a test service:"
echo "  kubectl apply -f examples/hello-world.yaml"
echo
