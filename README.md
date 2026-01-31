# ClusterKit

**Simplified Kubernetes platform for personal projects on GKE Autopilot with Cloudflare.**

Automates setup of a GKE Autopilot cluster, shared Gateway API load balancer, Cloudflare Origin CA wildcard SSL, automatic DNS via ExternalDNS, and cost optimizations — all for ~£25-30/month.

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

## Quick Start

### Prerequisites

- Google Cloud account with billing enabled
- Cloudflare account with a domain
- Tools: `gcloud`, `kubectl`, `terraform`

### 1. Bootstrap Infrastructure

```bash
git clone https://github.com/yourusername/clusterkit
cd clusterkit/terraform
gcloud auth login
gcloud config set project YOUR_PROJECT_ID
terraform init
terraform apply -var="project_id=YOUR_PROJECT_ID"
```

### 2. Configure ExternalDNS

```bash
helm repo add external-dns https://kubernetes-sigs.github.io/external-dns/
helm upgrade --install external-dns external-dns/external-dns \
  --namespace external-dns --create-namespace \
  -f docs/external-dns-values.yaml \
  --set cloudflare.apiToken=YOUR_CLOUDFLARE_TOKEN
```

### 3. Deploy an App

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
        cloud.google.com/gke-spot: "true"
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

```bash
kubectl apply -f myapp.yaml
```

Your app is live at `https://myapp.yourdomain.com` — SSL and DNS are automatic.

## Project Structure

```
terraform/
├── main.tf                 # Cluster, Gateway, Origin CA certs, Cloud SQL
├── dns.tf                  # Cloudflare DNS (email, verification only)
├── modules/                # Reusable Terraform modules
└── projects/               # Per-app resources (databases, buckets)
docs/
├── app-integration.md      # Guide for app developers
├── maintenance.md          # Operator guide (adding domains, troubleshooting)
├── external-dns-values.yaml
└── prefect-values.yaml
```

## Documentation

- **[Application Integration Guide](docs/app-integration.md)** — deploying apps to ClusterKit
- **[Maintenance Guide](docs/maintenance.md)** — operations, adding domains, troubleshooting

## License

MIT
