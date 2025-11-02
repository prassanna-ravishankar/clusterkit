#!/bin/bash
set -euo pipefail

# ClusterKit - cert-manager Installation Script

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CERT_MANAGER_VERSION="v1.14.4"  # Latest stable as of 2025
NAMESPACE="cert-manager"

echo "=================================="
echo "ClusterKit cert-manager Installation"
echo "Version: ${CERT_MANAGER_VERSION}"
echo "=================================="
echo

# Check prerequisites
if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl not found"
    exit 1
fi

if ! command -v helm &> /dev/null; then
    echo "Error: helm not found"
    exit 1
fi

# Check if NGINX Ingress is installed
echo "Step 1: Verifying NGINX Ingress Controller..."
if ! kubectl get ingressclass nginx &> /dev/null; then
    echo "⚠ Warning: NGINX Ingress Controller not found"
    echo "Please install NGINX Ingress (Task 3) first"
    echo "Continuing anyway..."
else
    echo "✓ NGINX Ingress Controller found"
fi
echo

# Add Helm repository
echo "Step 2: Adding cert-manager Helm repository..."
helm repo add jetstack https://charts.jetstack.io
helm repo update
echo "✓ Helm repository added"
echo

# Install cert-manager
echo "Step 3: Installing cert-manager ${CERT_MANAGER_VERSION}..."
helm upgrade --install cert-manager jetstack/cert-manager \
  --namespace ${NAMESPACE} \
  --create-namespace \
  --version ${CERT_MANAGER_VERSION} \
  --values ${SCRIPT_DIR}/values.yaml \
  --wait \
  --timeout 5m
echo "✓ cert-manager installed"
echo

# Wait for cert-manager to be ready
echo "Step 4: Waiting for cert-manager pods to be ready..."
kubectl wait --namespace ${NAMESPACE} \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/instance=cert-manager \
  --timeout=300s
echo "✓ cert-manager pods ready"
echo

# Verify CRDs are installed
echo "Step 5: Verifying CRDs..."
CRDS=(
  "certificates.cert-manager.io"
  "certificaterequests.cert-manager.io"
  "clusterissuers.cert-manager.io"
  "issuers.cert-manager.io"
  "challenges.acme.cert-manager.io"
  "orders.acme.cert-manager.io"
)

for crd in "${CRDS[@]}"; do
    if kubectl get crd ${crd} &> /dev/null; then
        echo "  ✓ ${crd}"
    else
        echo "  ✗ ${crd} - NOT FOUND"
    fi
done
echo

# Install ClusterIssuers
echo "Step 6: Creating ClusterIssuers..."
echo "  Installing Let's Encrypt staging issuer..."
kubectl apply -f ${SCRIPT_DIR}/clusterissuer-staging.yaml
echo "  Installing Let's Encrypt production issuer..."
kubectl apply -f ${SCRIPT_DIR}/clusterissuer-production.yaml
echo "✓ ClusterIssuers created"
echo

# Wait for ClusterIssuers to be ready
echo "Step 7: Waiting for ClusterIssuers to be ready..."
for i in {1..30}; do
    STAGING_READY=$(kubectl get clusterissuer letsencrypt-staging -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "")
    PROD_READY=$(kubectl get clusterissuer letsencrypt-prod -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "")

    if [[ "${STAGING_READY}" == "True" ]] && [[ "${PROD_READY}" == "True" ]]; then
        echo "✓ Both ClusterIssuers are ready"
        break
    fi

    echo "  Waiting... ($i/30) - Staging: ${STAGING_READY:-Pending}, Prod: ${PROD_READY:-Pending}"
    sleep 5
done
echo

# Display status
echo "=================================="
echo "✓ Installation Complete!"
echo "=================================="
echo
echo "cert-manager Pods:"
kubectl get pods -n ${NAMESPACE}
echo
echo "ClusterIssuers:"
kubectl get clusterissuer
echo

echo "=================================="
echo "Next Steps:"
echo "=================================="
echo "1. Update ClusterIssuer email addresses:"
echo "   Edit clusterissuer-*.yaml files and replace ops@clusterkit.example.com"
echo
echo "2. Test certificate issuance:"
echo "   kubectl apply -f examples/test-certificate.yaml"
echo
echo "3. Install ExternalDNS (Task 5)"
echo
echo "4. Deploy your first Knative Service with automatic TLS"
echo

echo "To test certificate creation:"
echo "  kubectl apply -f ${SCRIPT_DIR}/examples/test-certificate.yaml"
echo "  kubectl describe certificate test-certificate"
echo
