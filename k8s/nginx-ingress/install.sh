#!/bin/bash
set -euo pipefail

# ClusterKit - NGINX Ingress Controller Installation Script

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STATIC_IP_NAME="clusterkit-ingress-ip"  # From Terraform
NAMESPACE="ingress-nginx"

echo "=================================="
echo "ClusterKit NGINX Ingress Installation"
echo "=================================="
echo

# Check prerequisites
if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl not found"
    exit 1
fi

if ! command -v helm &> /dev/null; then
    echo "Error: helm not found"
    echo "Install with: curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash"
    exit 1
fi

# Get static IP from Terraform or GCP
echo "Step 1: Retrieving static IP address..."
STATIC_IP=$(gcloud compute addresses describe ${STATIC_IP_NAME} --global --format="get(address)" 2>/dev/null || echo "")

if [ -z "$STATIC_IP" ]; then
    echo "⚠ Warning: Could not find static IP '${STATIC_IP_NAME}'"
    echo "Please ensure Terraform (Task 1) has been applied"
    echo "Continuing without static IP assignment..."
else
    echo "✓ Found static IP: ${STATIC_IP}"
fi
echo

# Add Helm repository
echo "Step 2: Adding NGINX Ingress Helm repository..."
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update
echo "✓ Helm repository added"
echo

# Create namespace
echo "Step 3: Creating namespace..."
kubectl create namespace ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -
echo "✓ Namespace ready"
echo

# Prepare values file with static IP
VALUES_FILE="${SCRIPT_DIR}/values.yaml"
if [ -n "$STATIC_IP" ]; then
    echo "Step 4: Configuring static IP..."
    # Create temporary values file with static IP
    cat > /tmp/nginx-ingress-values.yaml <<EOF
$(cat ${VALUES_FILE})

  # Static IP configuration (added by install script)
  service:
    loadBalancerIP: ${STATIC_IP}
EOF
    VALUES_FILE="/tmp/nginx-ingress-values.yaml"
    echo "✓ Static IP configured"
else
    echo "Step 4: Using dynamic IP (no static IP configured)"
fi
echo

# Install NGINX Ingress Controller
echo "Step 5: Installing NGINX Ingress Controller..."
helm upgrade --install ingress-nginx ingress-nginx/ingress-nginx \
  --namespace ${NAMESPACE} \
  --values ${VALUES_FILE} \
  --wait \
  --timeout 5m
echo "✓ NGINX Ingress installed"
echo

# Wait for LoadBalancer IP
echo "Step 6: Waiting for LoadBalancer IP assignment..."
for i in {1..30}; do
    EXTERNAL_IP=$(kubectl get svc ingress-nginx-controller -n ${NAMESPACE} -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "")
    if [ -n "$EXTERNAL_IP" ]; then
        echo "✓ LoadBalancer IP assigned: ${EXTERNAL_IP}"
        break
    fi
    echo "  Waiting... ($i/30)"
    sleep 10
done

if [ -z "$EXTERNAL_IP" ]; then
    echo "⚠ Warning: LoadBalancer IP not assigned yet"
    echo "Check status with: kubectl get svc -n ${NAMESPACE}"
else
    if [ -n "$STATIC_IP" ] && [ "$EXTERNAL_IP" != "$STATIC_IP" ]; then
        echo "⚠ Warning: External IP ($EXTERNAL_IP) does not match static IP ($STATIC_IP)"
    fi
fi
echo

# Verify installation
echo "Step 7: Verifying installation..."
kubectl wait --namespace ${NAMESPACE} \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=300s
echo "✓ Controller pods ready"
echo

# Display status
echo "=================================="
echo "✓ Installation Complete!"
echo "=================================="
echo
echo "NGINX Ingress Controller Status:"
kubectl get pods -n ${NAMESPACE}
echo
echo "Service:"
kubectl get svc ingress-nginx-controller -n ${NAMESPACE}
echo
echo "IngressClass:"
kubectl get ingressclass
echo

if [ -n "$EXTERNAL_IP" ]; then
    echo "=================================="
    echo "Next Steps:"
    echo "=================================="
    echo "1. Configure Cloudflare DNS to point to: ${EXTERNAL_IP}"
    echo "2. Install cert-manager (Task 4)"
    echo "3. Setup ExternalDNS (Task 5)"
    echo "4. Test with sample ingress resource"
    echo
fi

# Cleanup temp file
[ -f "/tmp/nginx-ingress-values.yaml" ] && rm /tmp/nginx-ingress-values.yaml

echo "To test ingress:"
echo "  kubectl apply -f examples/test-ingress.yaml"
echo
