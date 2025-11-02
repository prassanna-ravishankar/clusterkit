# NGINX Ingress Controller for ClusterKit

Cluster-level ingress controller that routes external traffic to Knative services (via Kourier) and traditional workloads.

## Architecture

```
Internet
  ↓
Cloudflare CDN/WAF
  ↓
Static IP (from Terraform)
  ↓
NGINX Ingress Controller (this)
  ├→ Kourier → Knative Services (serverless)
  └→ Traditional Services (always-on)
```

## Features

- **Static IP Integration**: Uses static IP from Terraform (Task 1)
- **Cloudflare Compatibility**: Proper CF-Connecting-IP header handling
- **Real IP Forwarding**: Preserves client IPs through Cloudflare proxy
- **SSL/TLS Support**: TLS termination and passthrough modes
- **High Availability**: 2 replicas with anti-affinity
- **WebSocket Support**: Proper timeouts and buffer sizes
- **GKE Autopilot Optimized**: Appropriate resource requests/limits

## Installation

### Prerequisites

1. GKE Autopilot cluster (Task 1)
2. Static IP created via Terraform
3. kubectl and helm installed
4. Cluster admin access

### Quick Install

```bash
cd k8s/nginx-ingress
./install.sh
```

### Manual Installation

```bash
# Add Helm repository
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update

# Install with custom values
helm upgrade --install ingress-nginx ingress-nginx/ingress-nginx \
  --namespace ingress-nginx \
  --create-namespace \
  --values values.yaml \
  --wait
```

## Configuration

### Static IP Assignment

The static IP is configured in Terraform and automatically assigned during installation. To manually assign:

```yaml
# In values.yaml
controller:
  service:
    loadBalancerIP: "YOUR_STATIC_IP"  # From terraform output
```

### Cloudflare Integration

Cloudflare IP ranges are pre-configured in `values.yaml`. To update:

```yaml
controller:
  config:
    proxy-real-ip-cidr: "CLOUDFLARE_IP_RANGES"
```

Get latest Cloudflare IPs: https://www.cloudflare.com/ips/

### SSL/TLS Configuration

**TLS Termination** (default):
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
spec:
  tls:
  - hosts:
    - app.example.com
    secretName: app-tls-cert  # Created by cert-manager
```

**SSL Passthrough**:
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  annotations:
    nginx.ingress.kubernetes.io/ssl-passthrough: "true"
spec:
  # Service handles TLS
```

## Testing

### Deploy Test Service

```bash
kubectl apply -f examples/test-ingress.yaml
```

### Verify Ingress

```bash
# Check controller status
kubectl get pods -n ingress-nginx

# Check service and external IP
kubectl get svc ingress-nginx-controller -n ingress-nginx

# Check ingress resource
kubectl get ingress -A

# Test HTTP access
curl -H "Host: test.clusterkit.local" http://<EXTERNAL-IP>/
```

### Test Cloudflare IP Forwarding

```bash
# Deploy test service that echoes headers
kubectl apply -f examples/test-ingress.yaml

# Access through Cloudflare (after DNS is configured)
curl https://test.yourdomain.com

# Check logs for CF-Connecting-IP
kubectl logs -n ingress-nginx -l app.kubernetes.io/component=controller | grep CF-Connecting-IP
```

## Integration with Knative

NGINX Ingress routes to Kourier for Knative services:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: knative-ingress
  annotations:
    nginx.ingress.kubernetes.io/backend-protocol: "HTTP"
spec:
  ingressClassName: nginx
  rules:
  - host: "*.knative.example.com"
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: kourier
            namespace: kourier-system
            port:
              number: 80
```

## Monitoring

### Metrics

NGINX Ingress exports Prometheus metrics on port 10254:

```bash
kubectl port-forward -n ingress-nginx \
  svc/ingress-nginx-controller-metrics 10254:10254
```

Access metrics at: http://localhost:10254/metrics

### Logs

```bash
# Controller logs
kubectl logs -n ingress-nginx -l app.kubernetes.io/component=controller --tail=100 -f

# Access logs (includes CF-Connecting-IP)
kubectl logs -n ingress-nginx -l app.kubernetes.io/component=controller | grep "GET\|POST"
```

## Troubleshooting

### External IP Not Assigned

```bash
# Check service events
kubectl describe svc ingress-nginx-controller -n ingress-nginx

# Verify LoadBalancer quota
gcloud compute project-info describe --project=<PROJECT_ID>
```

### Client IP Not Preserved

```bash
# Verify externalTrafficPolicy
kubectl get svc ingress-nginx-controller -n ingress-nginx -o yaml | grep externalTrafficPolicy
# Should be: Local

# Check Cloudflare IP ranges
kubectl get cm ingress-nginx-controller -n ingress-nginx -o yaml | grep proxy-real-ip-cidr
```

### Certificate Issues

```bash
# Check TLS secret
kubectl get secret <tls-secret-name> -o yaml

# Verify ingress annotations
kubectl describe ingress <ingress-name>
```

### 502/503 Errors

```bash
# Check backend service
kubectl get svc <backend-service>

# Check backend pods
kubectl get pods -l app=<backend-app>

# Check ingress controller logs
kubectl logs -n ingress-nginx -l app.kubernetes.io/component=controller | grep "upstream"
```

## Performance Tuning

### For High Traffic

```yaml
controller:
  resources:
    requests:
      cpu: 500m
      memory: 512Mi
    limits:
      cpu: 2000m
      memory: 1Gi

  replicaCount: 3

  config:
    worker-processes: "4"
    max-worker-connections: "32768"
```

### For WebSockets

```yaml
controller:
  config:
    proxy-read-timeout: "3600"
    proxy-send-timeout: "3600"
```

## Cost Considerations

**Monthly Costs** (us-central1):
- Load Balancer: ~$5-8/month
- NGINX Ingress pods (2 replicas): ~$10-15/month
- **Total**: ~$15-23/month

Compare to: Cloud Load Balancer + SSL certificates = $18-30/month

## Next Steps

1. **Configure Cloudflare DNS** to point to LoadBalancer IP
2. **Install cert-manager** (Task 4) for automatic TLS certificates
3. **Setup ExternalDNS** (Task 5) for automatic DNS records
4. **Create routing rules** for Knative services via Kourier

## Resources

- [NGINX Ingress Documentation](https://kubernetes.github.io/ingress-nginx/)
- [Cloudflare IP Ranges](https://www.cloudflare.com/ips/)
- [GKE Ingress Guide](https://cloud.google.com/kubernetes-engine/docs/concepts/ingress)
