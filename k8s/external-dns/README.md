# ExternalDNS for ClusterKit

Automatic DNS record management in Cloudflare based on Kubernetes HTTPRoutes.

## Architecture

```
HTTPRoutes (clusterkit namespace)
  ↓
ExternalDNS (watches hostnames)
  ↓
Cloudflare API
  ↓
Proxied A Records (orange cloud)
  ↓
Client → Cloudflare CDN/WAF → GKE Gateway (Origin CA cert) → Pod
```

## How It Works

1. Application deploys HTTPRoute with `hostnames: ["app.domain.com"]`
2. ExternalDNS detects the hostname
3. Creates proxied A record in Cloudflare: `app.domain.com` → Gateway IP (orange cloud)
4. Creates TXT ownership record for tracking
5. Cloudflare CDN/WAF is automatically active

**DNS record ownership split:**
- **ExternalDNS**: All gateway A records (created from HTTPRoutes)
- **Terraform** (`dns.tf`): Email (MX/DKIM/SPF), verification TXT, GitHub Pages, Cloudflare Pages

## Installation

### Prerequisites

1. Cloudflare account with domain
2. Cloudflare API token with Zone:Zone:Read + Zone:DNS:Edit permissions

### Install via Helm

```bash
# Create namespace and secret
kubectl create namespace external-dns
kubectl create secret generic external-dns \
  --from-literal=cloudflare_api_token="${CLOUDFLARE_API_TOKEN}" \
  --namespace=external-dns

# Install via Helm
helm repo add external-dns https://kubernetes-sigs.github.io/external-dns/
helm upgrade --install external-dns external-dns/external-dns \
  --namespace external-dns \
  -f docs/external-dns-values.yaml
```

## Configuration

Configuration is in `docs/external-dns-values.yaml`.

Key settings:
- **Sources**: `service`, `ingress`, `gateway-httproute`
- **Provider**: `cloudflare`
- **Proxied**: `true` — creates orange cloud (proxied) records by default
- **Policy**: `upsert-only` — creates/updates, never deletes
- **TXT registry**: Tracks record ownership with TXT records

### Updating Configuration

```bash
helm upgrade external-dns external-dns/external-dns \
  --namespace external-dns \
  -f docs/external-dns-values.yaml
```

## Cloudflare Proxy (Orange Cloud)

All DNS records created by ExternalDNS are **proxied** (orange cloud) by default. This is safe because:
- GKE Gateway uses Cloudflare Origin CA certs
- Cloudflare zones are set to Full (Strict) SSL mode
- End-to-end encryption: Client → Cloudflare → Gateway → Pod

Benefits of proxied mode:
- CDN caching for static assets
- DDoS protection
- WAF (Web Application Firewall)
- Cloudflare analytics

Every HTTPRoute **must** include the proxied annotation:
```yaml
metadata:
  annotations:
    external-dns.alpha.kubernetes.io/cloudflare-proxied: "true"
```

To bypass Cloudflare proxy for a specific route (e.g., WebSocket-heavy services):
```yaml
    external-dns.alpha.kubernetes.io/cloudflare-proxied: "false"
```

## Monitoring

```bash
# Check ExternalDNS pods
kubectl get pods -n external-dns

# View logs
kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns

# Follow logs in real-time
kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns -f

# Filter for specific domain
kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns | grep torale.ai
```

## Troubleshooting

### DNS Records Not Created

1. Check ExternalDNS logs for errors
2. Verify Cloudflare API token is valid
3. Verify HTTPRoute is accepted by Gateway (`kubectl describe httproute <name> -n clusterkit`)

### ExternalDNS Crash-Looping

1. Check logs: `kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns`
2. Common causes: expired API token, RBAC issues, connectivity problems
3. Restart: `kubectl rollout restart deployment external-dns -n external-dns`

### Records Not Updating

ExternalDNS sync interval is 1 minute. Wait 1-2 minutes and check logs.

Force sync by restarting:
```bash
kubectl rollout restart deployment/external-dns -n external-dns
```

## Resources

- [ExternalDNS Documentation](https://github.com/kubernetes-sigs/external-dns)
- [Cloudflare Provider Guide](https://github.com/kubernetes-sigs/external-dns/blob/master/docs/tutorials/cloudflare.md)
