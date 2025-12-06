# ClusterKit

**Simplified Kubernetes platform for personal projects on GKE Autopilot with Cloudflare.**

Automates setup of:
- GKE Autopilot cluster (managed Kubernetes)
- Gateway API (shared HTTP(S) load balancer)
- Google-managed SSL certificates (automatic TLS)
- ExternalDNS (automatic Cloudflare DNS updates)
- Cost optimizations (logging, monitoring, Spot Pods)

**Target cost: ~£25-30/month** for infrastructure + multiple applications.

## What You Get

- **Shared Gateway**: One load balancer IP for all applications (saves £5/month per environment)
- **Automatic HTTPS**: Google provisions and renews SSL certificates
- **Automatic DNS**: Deploy an app, DNS record is created in Cloudflare
- **Cross-namespace routing**: Production and staging share the same Gateway
- **Cost optimized**: Logging retention, sampling, Spot Pods (60-91% savings)

## Architecture

```
┌─────────────────┐
│   Cloudflare    │  DNS: app.yourdomain.com → 34.149.49.202
│  (ExternalDNS)  │  (auto-created from HTTPRoutes)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Gateway API    │  Shared Gateway (1 IP, multiple apps)
│  (GKE Gateway)  │  SSL termination, hostname routing
└────────┬────────┘
         │
         ├──→ HTTPRoute (app1.domain.com) ──→ Service ──→ Pods
         ├──→ HTTPRoute (app2.domain.com) ──→ Service ──→ Pods
         └──→ HTTPRoute (staging.app.com)  ──→ Service ──→ Pods (different namespace)
```

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

# Deploy infrastructure
cd terraform
terraform init
terraform apply -var="project_id=YOUR_PROJECT_ID"
```

This creates:
- GKE Autopilot cluster
- Shared Gateway with SSL certificates
- Static IP
- IAM service accounts
- Optimized logging configuration

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
  namespace: torale
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
  namespace: torale
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
  namespace: torale
  annotations:
    external-dns.alpha.kubernetes.io/cloudflare-proxied: "false"
spec:
  parentRefs:
  - name: clusterkit-gateway
    namespace: torale
  hostnames:
  - "myapp.yourdomain.com"
  rules:
  - backendRefs:
    - name: myapp
      port: 80
```

**Before deploying**, add your domain to the SSL certificate:

Edit `terraform/main.tf`:
```hcl
module "ssl_cert_torale_prod" {
  domains = [
    "torale.ai",
    "myapp.yourdomain.com",  # Add your domain
  ]
}
```

Apply Terraform, then deploy your app:
```bash
terraform apply -var="project_id=YOUR_PROJECT_ID"
kubectl apply -f myapp.yaml
```

Your app will be live at `https://myapp.yourdomain.com` with automatic SSL and DNS!

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

Google-managed SSL certificates:
- Created via Terraform
- Auto-renewed by Google
- Attached to Gateway
- Up to 100 domains per certificate
- No wildcard support (each subdomain needs explicit entry)

### DNS Automation

ExternalDNS watches HTTPRoutes:
1. HTTPRoute created with `hostnames: ["app.domain.com"]`
2. ExternalDNS detects the hostname
3. Creates A record in Cloudflare: `app.domain.com` → Gateway IP
4. No manual DNS management needed

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
kubectl get gateway clusterkit-gateway -n torale
# Should show: PROGRAMMED: True, ADDRESS: 34.149.49.202
```

### List HTTPRoutes
```bash
kubectl get httproute -n torale
```

### Check ExternalDNS
```bash
kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns
```

### Verify DNS
```bash
dig +short myapp.yourdomain.com @1.1.1.1
# Should return: 34.149.49.202
```

### Add New Domain

See [Maintenance Guide](docs/maintenance.md#adding-domains) for detailed instructions.

Quick steps:
1. Add domain to SSL certificate in Terraform
2. Apply Terraform (15 min for cert provisioning)
3. Create HTTPRoute
4. DNS auto-created by ExternalDNS

## Troubleshooting

See [Maintenance Guide](docs/maintenance.md#troubleshooting) for comprehensive troubleshooting.

Common issues:
- **HTTPRoute not attaching**: Check namespace is `torale`
- **DNS not resolving**: Check ExternalDNS logs
- **SSL warning**: Ensure domain is in SSL certificate
- **Cloudflare proxy**: Ensure annotation `cloudflare-proxied: "false"`

## Project Structure

```
.
├── terraform/                  # Infrastructure as Code
│   ├── main.tf                 # Root config (cluster, Gateway, SSL)
│   ├── modules/                # Reusable modules
│   │   ├── gke/                # GKE Autopilot cluster
│   │   ├── gateway-api/        # Gateway + ReferenceGrants
│   │   ├── ssl-certificate/    # Google-managed SSL
│   │   └── ...
│   └── projects/torale/        # Project-specific (Cloud SQL, etc.)
├── docs/
│   ├── app-integration.md      # For app developers
│   ├── maintenance.md          # For operators
│   └── external-dns-values.yaml # Helm values
└── CLAUDE.md                   # AI assistant context
```

## FAQ

**Q: Why Gateway API instead of Ingress?**
A: Gateway API is the successor to Ingress, with better multi-tenancy, cross-namespace routing, and cost savings (shared Gateway IP).

**Q: Can I use wildcard certificates?**
A: No, Google-managed certificates don't support wildcards. Each subdomain needs explicit entry. For wildcards, migrate to cert-manager + Let's Encrypt.

**Q: How do staging environments work?**
A: HTTPRoutes in `torale` namespace can reference services in `torale-staging` namespace via ReferenceGrants. Both share the same Gateway IP.

**Q: What are Spot Pods?**
A: Discounted pods (60-91% off) that can be preempted. Great for web apps where brief downtime is acceptable. Kubernetes auto-reschedules preempted pods.

**Q: Can I deploy multiple domains?**
A: Yes! ExternalDNS supports unlimited domains. Add each domain to the SSL certificate and create HTTPRoutes.

**Q: How do I add a new application?**
A: See [Application Integration Guide](docs/app-integration.md) for step-by-step instructions.

## Contributing

This is a personal project optimized for cost-effective Kubernetes hosting. Feel free to fork and adapt for your needs.

## License

MIT
