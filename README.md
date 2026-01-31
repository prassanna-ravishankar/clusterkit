# ClusterKit

**Simplified Kubernetes platform for personal projects on GKE Autopilot with Cloudflare.**

Automates setup of:
- GKE Autopilot cluster (managed Kubernetes)
- Gateway API (shared HTTP(S) load balancer)
- Cloudflare Origin CA SSL certificates (wildcard, 15-year)
- ExternalDNS (automatic Cloudflare DNS updates, proxied by default)
- Cost optimizations (logging, monitoring, Spot Pods)

**Target cost: ~£25-30/month** for infrastructure + multiple applications.

## What You Get

- **Shared Gateway**: One load balancer IP for all applications (saves £5/month per environment)
- **Automatic HTTPS**: Cloudflare Origin CA wildcard certs — no cert changes for new subdomains
- **Automatic DNS**: Deploy an app, ExternalDNS creates a proxied Cloudflare DNS record
- **Cloudflare CDN/WAF**: All traffic proxied through Cloudflare (orange cloud) by default
- **Cross-namespace routing**: Production and staging share the same Gateway
- **Cost optimized**: Logging retention, sampling, Spot Pods (60-91% savings)

## Architecture

```
┌─────────────────┐
│   Cloudflare    │  DNS: app.yourdomain.com → Cloudflare edge (proxied)
│  CDN / WAF      │  SSL terminated at Cloudflare (Full Strict mode)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Gateway API    │  Shared Gateway (1 IP, multiple apps)
│  (GKE Gateway)  │  Origin CA wildcard cert, hostname routing
└────────┬────────┘
         │
         ├──→ HTTPRoute (app1.domain.com) ──→ Service ──→ Pods
         ├──→ HTTPRoute (app2.domain.com) ──→ Service ──→ Pods
         └──→ HTTPRoute (staging.app.com)  ──→ Service ──→ Pods (different namespace)
```

**SSL chain:** Client → Cloudflare (edge cert) → GKE Gateway (Origin CA cert) → Pod

## Quick Start

### Prerequisites

- Google Cloud account with billing enabled
- Cloudflare account with a domain
- Tools: `gcloud`, `kubectl`, `terraform`

### 1. Bootstrap Infrastructure

```bash
# Clone repository
git clone https://github.com/yourusername/clusterkit
cd clusterkit

# Configure GCP
gcloud auth login
gcloud config set project YOUR_PROJECT_ID

# Deploy infrastructure (Origin CA certs are generated automatically)
cd terraform
terraform init
terraform apply -var="project_id=YOUR_PROJECT_ID"
```

This creates:
- GKE Autopilot cluster
- Shared Gateway with Origin CA wildcard certs
- Static IP
- IAM service accounts
- Optimized logging configuration
- Cloudflare Full (Strict) SSL mode per zone

### 2. Configure ExternalDNS

Get Cloudflare API token: https://dash.cloudflare.com/profile/api-tokens

Deploy ExternalDNS:
```bash
# Install via Helm
helm repo add external-dns https://kubernetes-sigs.github.io/external-dns/
helm upgrade --install external-dns external-dns/external-dns \
  --namespace external-dns --create-namespace \
  -f docs/external-dns-values.yaml \
  --set cloudflare.apiToken=YOUR_CLOUDFLARE_TOKEN
```

### 3. Deploy Your First App

Create `myapp.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
  namespace: clusterkit
spec:
  replicas: 1
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      nodeSelector:
        cloud.google.com/gke-spot: "true"  # Use Spot Pods (60-91% savings)
      containers:
      - name: app
        image: nginx:alpine
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: myapp
  namespace: clusterkit
spec:
  selector:
    app: myapp
  ports:
  - port: 80
    targetPort: 80
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: myapp-prod
  namespace: clusterkit
  annotations:
    external-dns.alpha.kubernetes.io/cloudflare-proxied: "true"
spec:
  parentRefs:
  - name: clusterkit-gateway
    namespace: clusterkit
  hostnames:
  - "myapp.yourdomain.com"
  rules:
  - backendRefs:
    - name: myapp
      port: 80
```

Deploy:
```bash
kubectl apply -f myapp.yaml
```

Your app will be live at `https://myapp.yourdomain.com` with automatic SSL and DNS. The wildcard Origin CA cert covers any subdomain — no Terraform changes needed.

## Documentation

- **[Application Integration Guide](docs/app-integration.md)** - For app developers deploying to ClusterKit
- **[Maintenance Guide](docs/maintenance.md)** - For ClusterKit operators
- **[CLAUDE.md](CLAUDE.md)** - Architecture and patterns for AI assistants

## How It Works

### Gateway API

ClusterKit uses Kubernetes **Gateway API** instead of traditional Ingress:

- **One Gateway** = One load balancer IP
- **HTTPRoutes** = Routing rules (hostname → service)
- **Cross-namespace routing** = Production and staging share the Gateway

Benefits:
- Cost savings: £5/month per environment (vs separate IPs)
- Simpler: No per-app load balancers
- Flexible: HTTPRoutes can reference services in any namespace

### SSL Certificates

Cloudflare Origin CA wildcard certificates:
- One cert per domain covering `domain.com` + `*.domain.com`
- Generated automatically by Terraform (no manual steps)
- 15-year validity, no renewal hassle
- New subdomains covered automatically (wildcard)
- Cloudflare Full (Strict) mode = end-to-end encryption
- No Terraform changes needed for new subdomains

### DNS Automation

ExternalDNS watches HTTPRoutes:
1. HTTPRoute created with `hostnames: ["app.domain.com"]`
2. ExternalDNS detects the hostname
3. Creates proxied A record in Cloudflare: `app.domain.com` → Gateway IP (orange cloud)
4. Cloudflare CDN/WAF automatically active
5. No manual DNS management needed

### Cost Optimizations

**Logging** (~£10/month savings):
- 7-day retention (down from 30)
- Health check exclusions (~24% of logs)
- INFO log sampling at 10%

**Monitoring** (~£5/month savings):
- Minimal monitoring components
- Managed Prometheus (cannot be disabled in Autopilot)

**Spot Pods** (60-91% savings):
- Discounted pods for fault-tolerant workloads
- Auto-rescheduled on preemption

**Shared Gateway** (£5/month per environment):
- Single IP for all apps
- Cross-namespace routing for staging

## Cost Breakdown

**Monthly costs for personal projects:**

| Resource | Cost |
|----------|------|
| GKE cluster management | £0 (free tier) |
| Load Balancer (Gateway) | ~£5 |
| Cloud Monitoring | ~£3-5 |
| Cloud SQL (torale) | ~£6-8 |
| Workloads (Spot Pods) | ~£5-10 |
| Static IP | ~£5 |
| **Total** | **~£25-30** |

Compare to:
- Cloud Run: £0-10/month (less control)
- Traditional GKE: £75+/month
- Managed k8s elsewhere: £50+/month

## Operations

### Check Gateway Status
```bash
kubectl get gateway clusterkit-gateway -n clusterkit
# Should show: PROGRAMMED: True, ADDRESS: 34.149.49.202
```

### List HTTPRoutes
```bash
kubectl get httproute -n clusterkit
```

### Check ExternalDNS
```bash
kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns
```

### Verify DNS (should return Cloudflare IPs, not Gateway IP)
```bash
dig +short myapp.yourdomain.com @1.1.1.1
# Should return Cloudflare IPs (104.x.x.x) since records are proxied
```

### Add New Subdomain

Just deploy an HTTPRoute — ExternalDNS creates the DNS record, wildcard cert covers it:
1. Create HTTPRoute with your subdomain
2. Deploy: `kubectl apply -f httproute.yaml`
3. Done. No Terraform changes needed.

### Add New Domain

See [Maintenance Guide](docs/maintenance.md#adding-domains) for detailed instructions.

## Troubleshooting

See [Maintenance Guide](docs/maintenance.md#troubleshooting) for comprehensive troubleshooting.

Common issues:
- **HTTPRoute not attaching**: Check namespace is `clusterkit`
- **DNS not resolving**: Check ExternalDNS logs
- **SSL warning**: Verify Cloudflare zone SSL mode is "Full (Strict)"

## Project Structure

```
.
├── terraform/                  # Infrastructure as Code
│   ├── main.tf                 # Root config (cluster, Gateway, Origin CA certs, Cloud SQL)
│   ├── dns.tf                  # Cloudflare DNS records (email, verification only)
│   ├── modules/                # Reusable modules
│   │   ├── gke/                # GKE Autopilot cluster
│   │   ├── gateway-api/        # Gateway + ReferenceGrants
│   │   ├── cloudflare-dns/     # Cloudflare DNS records
│   │   └── ...
│   └── projects/               # Project-specific (torale, bananagraph)
├── docs/
│   ├── app-integration.md      # For app developers
│   ├── maintenance.md          # For operators
│   ├── external-dns-values.yaml # Helm values
│   └── prefect-values.yaml     # Prefect Server Helm values
└── CLAUDE.md                   # AI assistant context
```

## FAQ

**Q: Why Gateway API instead of Ingress?**
A: Gateway API is the successor to Ingress, with better multi-tenancy, cross-namespace routing, and cost savings (shared Gateway IP).

**Q: Do I need to update certificates for new subdomains?**
A: No. Origin CA wildcard certs cover `*.domain.com` automatically. Just deploy an HTTPRoute.

**Q: How does SSL work?**
A: Cloudflare terminates client-facing SSL at the edge, then connects to GKE Gateway using the Origin CA cert (Full Strict mode). End-to-end encrypted.

**Q: How do staging environments work?**
A: HTTPRoutes in `clusterkit` namespace can reference services in `torale-staging` namespace via ReferenceGrants. Both share the same Gateway IP.

**Q: What are Spot Pods?**
A: Discounted pods (60-91% off) that can be preempted. Great for web apps where brief downtime is acceptable. Kubernetes auto-reschedules preempted pods.

**Q: Can I deploy multiple domains?**
A: Yes! Add the domain to `origin_ca_domains` and its zone ID to `cloudflare_zone_ids` in variables.tf, then `terraform apply`. Terraform generates the Origin CA cert automatically.

**Q: How do I add a new application?**
A: See [Application Integration Guide](docs/app-integration.md) for step-by-step instructions.

## Contributing

This is a personal project optimized for cost-effective Kubernetes hosting. Feel free to fork and adapt for your needs.

## License

MIT
