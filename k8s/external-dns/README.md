# ExternalDNS for ClusterKit

Automatic DNS record management in Cloudflare based on Kubernetes resources.

## Features

- **Automatic DNS Records**: Creates A/CNAME records for Services and Ingresses
- **Cloudflare Integration**: Direct API integration with Cloudflare DNS
- **Knative Support**: Manages DNS for Knative Services
- **Safe by Default**: upsert-only policy prevents accidental deletion
- **TXT Record Ownership**: Tracks which records are managed by ExternalDNS
- **Cloudflare Proxy**: Optionally enable orange-cloud CDN/WAF

## Architecture

```
Kubernetes Resources (Services/Ingresses/Knative)
  ↓
ExternalDNS (watches for annotations)
  ↓
Cloudflare API
  ↓
DNS Records Created/Updated
  ↓
Internet Traffic → Cloudflare → Static IP → Cluster
```

## Installation

### Prerequisites

1. Cloudflare account with domain
2. Cloudflare API token ([Setup Guide](CLOUDFLARE_TOKEN_SETUP.md))
3. GKE cluster with NGINX Ingress (Task 3)

### Quick Install

```bash
# Create Cloudflare API token first (see CLOUDFLARE_TOKEN_SETUP.md)

# Set token environment variable
export CLOUDFLARE_API_TOKEN='your-token-here'

# Run install script
cd k8s/external-dns
./install.sh

# Enter your domain when prompted (e.g., example.com)
```

### Manual Installation

```bash
# Create namespace
kubectl create namespace external-dns

# Create secret with Cloudflare token
kubectl create secret generic cloudflare-api-token \
  --from-literal=apiToken="${CLOUDFLARE_API_TOKEN}" \
  --namespace=external-dns

# Update domain in deployment.yaml
# Change: --domain-filter=example.com
# To: --domain-filter=yourdomain.com

# Deploy
kubectl apply -f deployment.yaml
```

## Usage

### Method 1: Service with Annotation

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  annotations:
    external-dns.alpha.kubernetes.io/hostname: app.example.com
    external-dns.alpha.kubernetes.io/ttl: "300"  # Optional: 5 min TTL
spec:
  type: LoadBalancer
  # ... rest of service spec
```

### Method 2: Ingress (Automatic)

ExternalDNS automatically creates DNS for Ingress hosts:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  rules:
  - host: app.example.com  # DNS auto-created
    http:
      # ... routes
```

### Method 3: Knative Service

```yaml
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: my-app
  annotations:
    external-dns.alpha.kubernetes.io/hostname: app.example.com
spec:
  # ... Knative service spec
```

## Configuration Options

### Domain Filtering

Restrict ExternalDNS to specific domain(s):

```yaml
args:
- --domain-filter=example.com
- --domain-filter=another-example.com  # Multiple domains
```

### Cloudflare Proxy (Orange Cloud)

Enable Cloudflare CDN/WAF:

```yaml
args:
- --cloudflare-proxied  # Enable for all records
```

Or per-resource:

```yaml
metadata:
  annotations:
    external-dns.alpha.kubernetes.io/cloudflare-proxied: "true"
```

### TXT Record Ownership

ExternalDNS creates TXT records to track ownership:

```yaml
args:
- --txt-owner-id=clusterkit  # Unique identifier
- --txt-prefix=_externaldns.  # Prefix for TXT records
```

Example: For `app.example.com`, creates `_externaldns.app.example.com` TXT record.

### Policy Options

```yaml
args:
- --policy=upsert-only  # Never delete (safest)
# OR
- --policy=sync  # Delete records not in cluster (dangerous!)
```

**Recommendation**: Always use `upsert-only` in production.

## Testing

### 1. Deploy Test Service

```bash
kubectl apply -f examples/test-service.yaml

# Watch ExternalDNS logs
kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns -f
```

### 2. Verify DNS Record Creation

Check Cloudflare dashboard:
- Go to your domain → DNS → Records
- Look for the new A record

Or use CLI:

```bash
dig test.example.com
# Should return LoadBalancer IP
```

### 3. Test DNS Resolution

```bash
# Query DNS
nslookup test.example.com

# Should return cluster LoadBalancer IP
```

### 4. Test Cleanup

```bash
# Delete service
kubectl delete -f examples/test-service.yaml

# DNS record should remain (upsert-only policy)
# To remove, delete manually from Cloudflare or use policy=sync
```

## Complete Example: Ingress + TLS + DNS

This creates everything automatically:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  annotations:
    # cert-manager creates TLS certificate
    cert-manager.io/cluster-issuer: letsencrypt-prod

    # ExternalDNS settings
    external-dns.alpha.kubernetes.io/ttl: "300"
    external-dns.alpha.kubernetes.io/cloudflare-proxied: "true"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - app.example.com
    secretName: app-tls
  rules:
  - host: app.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: my-app
            port:
              number: 80
```

Result:
1. ExternalDNS creates DNS A record → LoadBalancer IP
2. cert-manager creates TLS certificate via Let's Encrypt
3. NGINX Ingress serves traffic with TLS
4. Cloudflare CDN/WAF in front (if proxied enabled)

## Monitoring

### Check ExternalDNS Logs

```bash
# Real-time logs
kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns -f

# Last 100 lines
kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns --tail=100
```

### Check Managed Records

```bash
# View all resources ExternalDNS is watching
kubectl get services,ingresses -A

# Check annotations
kubectl describe service <name> | grep external-dns
```

### Cloudflare Dashboard

Monitor in Cloudflare:
- DNS → Records (see created records)
- Analytics → Traffic (view DNS queries)
- Audit Log (see API changes)

## Troubleshooting

### DNS Records Not Created

1. **Check ExternalDNS logs**:
   ```bash
   kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns
   ```

2. **Common issues**:
   - Cloudflare API token invalid/expired
   - Domain not in domain-filter
   - Service/Ingress missing hostname annotation
   - LoadBalancer IP not assigned yet

3. **Verify token**:
   ```bash
   kubectl get secret cloudflare-api-token -n external-dns -o yaml
   ```

### "Authentication Failure" Error

Check Cloudflare API token has correct permissions:
- Zone:Zone:Read
- Zone:DNS:Edit

Recreate token and update secret.

### Records Not Updating

ExternalDNS sync interval is 1 minute. Wait 1-2 minutes and check logs.

Force sync by restarting:

```bash
kubectl rollout restart deployment/external-dns -n external-dns
```

### Wrong IP Address

Verify LoadBalancer has external IP:

```bash
kubectl get svc -A | grep LoadBalancer
```

If pending, check cloud provider quota/permissions.

### Multiple ExternalDNS Instances

If running multiple instances, ensure different:
- `--txt-owner-id` values
- `--domain-filter` ranges (no overlap)

## Cloudflare Integration Details

### DNS Record Types

ExternalDNS creates:
- **A Records**: Service type LoadBalancer
- **CNAME Records**: Ingress (if alias to service)
- **TXT Records**: Ownership tracking

### Cloudflare Proxy

When `--cloudflare-proxied` is enabled:
- Traffic routes through Cloudflare edge
- Benefits: CDN, DDoS protection, WAF, caching
- Drawbacks: True client IP needs CF-Connecting-IP header (already configured in Task 3)

### Rate Limits

Cloudflare API rate limits:
- 1,200 requests per 5 minutes
- ExternalDNS is conservative with API calls
- Typical usage: <100 requests/hour

## Security Considerations

- **API Token Scope**: Limit to specific zones
- **upsert-only Policy**: Prevents accidental deletion
- **TXT Record Ownership**: Prevents conflicts with other tools
- **Secret Storage**: Token stored in Kubernetes Secret
- **RBAC**: Limited to read services/ingresses

## Cost

**Free**:
- ExternalDNS (open source)
- Cloudflare DNS (unlimited DNS queries)

**Resource Cost**:
- ExternalDNS pod: ~$2-3/month on GKE Autopilot

## Next Steps

1. Deploy test service and verify DNS creation
2. Combine with cert-manager for automatic TLS
3. Build ClusterKit CLI (Task 6) to automate this flow
4. Deploy production applications with automatic DNS+TLS

## Resources

- [ExternalDNS Documentation](https://github.com/kubernetes-sigs/external-dns)
- [Cloudflare Provider Guide](https://github.com/kubernetes-sigs/external-dns/blob/master/docs/tutorials/cloudflare.md)
- [Cloudflare API Documentation](https://developers.cloudflare.com/api/)
