#!/bin/bash
set -euo pipefail

# ClusterKit - ExternalDNS Installation Script

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NAMESPACE="external-dns"

echo "=================================="
echo "ClusterKit ExternalDNS Installation"
echo "Provider: Cloudflare"
echo "=================================="
echo

# Check prerequisites
if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl not found"
    exit 1
fi

# Check for Cloudflare API token
echo "Step 1: Checking for Cloudflare API token..."
if [ -z "${CLOUDFLARE_API_TOKEN:-}" ]; then
    echo
    echo "⚠ CLOUDFLARE_API_TOKEN environment variable not set"
    echo
    echo "To create a Cloudflare API token:"
    echo "1. Go to: https://dash.cloudflare.com/profile/api-tokens"
    echo "2. Click 'Create Token'"
    echo "3. Use 'Edit zone DNS' template or create custom token with:"
    echo "   - Permissions: Zone:Zone:Read, Zone:DNS:Edit"
    echo "   - Zone Resources: Include > Specific zone > [your domain]"
    echo "4. Copy the token"
    echo
    echo "Then run:"
    echo "  export CLOUDFLARE_API_TOKEN='your-token-here'"
    echo "  ./install.sh"
    echo
    exit 1
fi
echo "✓ Cloudflare API token found"
echo

# Create namespace
echo "Step 2: Creating namespace..."
kubectl create namespace ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -
echo "✓ Namespace ready"
echo

# Create secret
echo "Step 3: Creating Cloudflare API token secret..."
kubectl create secret generic cloudflare-api-token \
  --from-literal=apiToken="${CLOUDFLARE_API_TOKEN}" \
  --namespace=${NAMESPACE} \
  --dry-run=client -o yaml | kubectl apply -f -
echo "✓ Secret created"
echo

# Prompt for domain configuration
echo "Step 4: Configuring domain filter..."
read -p "Enter your domain to manage (e.g., example.com): " DOMAIN
if [ -z "$DOMAIN" ]; then
    echo "Error: Domain cannot be empty"
    exit 1
fi

# Update deployment with domain
echo "Updating domain filter to: ${DOMAIN}"
sed "s/example.com/${DOMAIN}/g" ${SCRIPT_DIR}/deployment.yaml > /tmp/external-dns-deployment.yaml
echo "✓ Domain configured"
echo

# Deploy ExternalDNS
echo "Step 5: Deploying ExternalDNS..."
kubectl apply -f /tmp/external-dns-deployment.yaml
rm /tmp/external-dns-deployment.yaml
echo "✓ ExternalDNS deployed"
echo

# Wait for deployment
echo "Step 6: Waiting for ExternalDNS to be ready..."
kubectl wait --namespace ${NAMESPACE} \
  --for=condition=available deployment/external-dns \
  --timeout=180s
echo "✓ ExternalDNS ready"
echo

# Display status
echo "=================================="
echo "✓ Installation Complete!"
echo "=================================="
echo
echo "ExternalDNS Status:"
kubectl get pods -n ${NAMESPACE}
echo
echo "Logs (last 20 lines):"
kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/name=external-dns --tail=20
echo

echo "=================================="
echo "Next Steps:"
echo "=================================="
echo "1. Verify Cloudflare connection in logs above"
echo "2. Deploy test service with DNS annotation:"
echo "   kubectl apply -f examples/test-service.yaml"
echo "3. Check Cloudflare dashboard for DNS record creation"
echo "4. Ready to build ClusterKit CLI (Task 6)"
echo

echo "To monitor ExternalDNS:"
echo "  kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/name=external-dns -f"
echo
