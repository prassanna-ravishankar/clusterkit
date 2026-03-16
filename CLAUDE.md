# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ClusterKit is a simplified Kubernetes platform for personal projects on GKE Autopilot with Cloudflare DNS integration. It automates infrastructure deployment, certificate management, and DNS configuration to minimize costs while maintaining full Kubernetes functionality.

**Key Technologies:**
- GKE Autopilot (managed Kubernetes)
- Terraform (infrastructure as code)
- Gateway API (shared load balancer)
- Cloudflare Origin CA wildcard SSL certificates
- Cloudflare CDN/WAF (Full Strict SSL mode)
- ExternalDNS (Cloudflare integration, proxied by default)

**Target monthly cost:** ~£25-30 for infrastructure + multiple applications

## Architecture

### SSL / Traffic Flow

```
Client → Cloudflare (edge SSL, CDN/WAF, orange cloud) → GKE Gateway (Origin CA cert) → HTTPRoute → Service → Pod
```

- Cloudflare terminates client-facing SSL at the edge
- GKE Gateway presents Cloudflare Origin CA wildcard cert
- Cloudflare zones set to **Full (Strict)** — validates Origin CA cert
- End-to-end encrypted, with Cloudflare CDN/WAF benefits

### Gateway API Pattern

ClusterKit uses **Gateway API** (successor to Ingress) for cost-effective multi-tenant routing:

```
clusterkit namespace (Gateway + all HTTPRoutes)
├── Gateway (clusterkit-gateway, IP: 34.149.49.202)
├── HTTPRoute (torale.ai) → Service (torale/torale-frontend)
├── HTTPRoute (api.torale.ai) → Service (torale/torale-api)
├── HTTPRoute (docs.torale.ai) → Service (torale/torale-docs)
├── HTTPRoute (staging.torale.ai) → Service (torale-staging/torale-frontend)
├── HTTPRoute (bananagraph.com) → Service (bananagraph/bananagraph-service)
├── HTTPRoute (beta.a2aregistry.org) → Service (a2aregistry/a2aregistry-api)
├── HTTPRoute (repowire.io) → Service (repowire/repowire-service)
└── HTTPRoute (relay.repowire.io) → Service (repowire/repowire-relay)
```

**Benefits:**
- Single load balancer IP (saves £5/month per environment)
- Cross-namespace routing (production + staging share Gateway)
- Centralized SSL termination via Origin CA wildcard certs
- ExternalDNS auto-creates proxied DNS from HTTPRoute hostnames
- Cloudflare CDN/WAF on all traffic by default

### Two-Tier Terraform Structure

**1. Root Terraform** (`terraform/`):
- GKE Autopilot cluster
- Gateway API (Gateway, Origin CA certs, ReferenceGrants)
- Cloudflare zone settings (Full Strict SSL per zone)
- Static IP (`clusterkit-ingress-ip`)
- Non-gateway DNS records (`dns.tf`) — email, verification, GitHub Pages only
- Shared Cloud SQL instance (`clusterkit-db`) and proxy service account
- Workload Identity bindings for database access (torale, bananagraph, a2aregistry)
- Logging optimization (project-level, includes ExternalDNS INFO exclusion)

**2. Project-Specific Terraform** (`terraform/projects/<project>/`):
- Project-specific databases and users (created in shared instance)
- Application-specific GCS buckets and service accounts
- Project resources separate from cluster

Both use the same GCP project (`baldmaninc`) but maintain separate Terraform states.

### DNS Record Ownership

- **ExternalDNS**: All gateway A records (created from HTTPRoutes, proxied/orange cloud)
- **Terraform** (`dns.tf`): Email (MX/DKIM/SPF), verification TXT, GitHub Pages, Cloudflare Pages

### Terraform Module Structure

Reusable modules in `terraform/modules/`:
- `gke/` - GKE Autopilot cluster with configurable logging/monitoring
- `gateway-api/` - Gateway with SSL certs and ReferenceGrants
- `cloudflare-dns/` - Cloudflare DNS record management
- `httproute/` - HTTPRoute template (for application use)
- `networking/` - Static IP addresses
- `logging/` - Cost-optimized Cloud Logging (with custom exclusion support)
- `cloudsql-instance/`, `cloudsql-proxy-sa/` - PostgreSQL instances

## Key Patterns and Conventions

### Terraform Best Practices

1. **Module Variables:** Always provide defaults for optional parameters
2. **Outputs:** Return useful information (IDs, emails, connection commands)
3. **Deletion Protection:** Enabled by default on critical resources
4. **Workload Identity:** Preferred over service account keys
5. **Lock Files:** Always commit `.terraform.lock.hcl`
6. **Provider Configuration:**
   - Kubernetes provider configured in root `versions.tf`
   - Uses GKE cluster credentials via `google_client_config`
7. **Origin CA certs:** Generated automatically via `tls` + `cloudflare` providers (no manual steps)

### Gateway API Conventions

**HTTPRoute Requirements:**
- MUST be in `clusterkit` namespace (Gateway namespace)
- MUST include `external-dns.alpha.kubernetes.io/cloudflare-proxied: "true"` annotation
- Cross-namespace service refs: Add `namespace: <app-namespace>` to backendRefs
- ExternalDNS auto-creates proxied DNS from `hostnames` field when the annotation is present

**Example HTTPRoute:**
```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: app-prod
  namespace: clusterkit
  annotations:
    external-dns.alpha.kubernetes.io/cloudflare-proxied: "true"
spec:
  parentRefs:
  - name: clusterkit-gateway
    namespace: clusterkit
  hostnames:
  - "app.domain.com"
  rules:
  - backendRefs:
    - name: app-service
      namespace: myapp  # Cross-namespace reference to app's service
      port: 80
```

### Kubernetes Manifest Pattern

Standard app deployment requires 3 resources:
1. **Deployment** (with Spot Pod nodeSelector for cost savings)
2. **Service** (ClusterIP exposing pod ports)
3. **HTTPRoute** (routing rules, attaches to shared Gateway)

Gateway, Origin CA certificates, and ReferenceGrants are managed by ClusterKit Terraform.

**Spot Pod Configuration (60-91% savings):**
```yaml
nodeSelector:
  cloud.google.com/gke-spot: "true"
tolerations:
- effect: NoSchedule
  key: cloud.google.com/gke-spot
  operator: Equal
  value: "true"
```

## Cost Optimization Strategy

**Logging Optimizations** (`terraform/modules/logging/`):
- Log retention: 7 days (down from 30)
- Health check exclusion (~24% of logs)
- GKE noise exclusion (gcfs-snapshotter, gcfsd, container-runtime)
- ExternalDNS INFO log exclusion (custom exclusion)
- INFO log sampling at 10% (ERROR/WARNING kept at 100%)

**GKE Monitoring Optimizations** (`terraform/modules/gke/`):
- Monitoring components: `SYSTEM_COMPONENTS`, `POD`, `DEPLOYMENT` only
- Managed Prometheus: Cannot be disabled (GKE 1.25+, enforced by Google)
- Workload logging: Enabled (kept for debugging)

**Gateway API Cost Savings:**
- Single IP for all apps: £5/month (vs £10 for 2 IPs)
- Shared load balancer: No per-app LB costs
- Cross-namespace routing: Production + staging share Gateway

**Target Costs:**
- Cloud Monitoring: ~£3-5/month (down from £14/month)
- Infrastructure: ~£25-30/month total for side projects

## Important Caveats

### GKE Autopilot Limitations

- **Managed Prometheus:** Cannot be disabled (enforced in GKE 1.25+)
- **Node Management:** Fully automated, no node pool configuration
- **Resource Limits:** Automatic resource provisioning based on pod requests

### Gateway API Integration

- Uses single shared Gateway (`clusterkit-gateway`) in `clusterkit` namespace
- Gateway IP: `clusterkit-ingress-ip` (34.149.49.202)
- All HTTPRoutes live in `clusterkit` namespace (centralized routing)
- Cross-namespace routing via ReferenceGrants (HTTPRoutes in `clusterkit` → services in app namespaces)
- ExternalDNS watches HTTPRoutes and auto-creates proxied DNS records

### SSL Certificates

Cloudflare Origin CA wildcard certificates:
- One cert per domain covering `domain.com` + `*.domain.com`
- 15-year validity, generated automatically by Terraform (`tls` + `cloudflare` providers)
- Wildcard = no cert changes for new subdomains
- Cloudflare Full (Strict) validates Origin CA cert on Gateway
- `create_before_destroy` lifecycle for zero-downtime rotation
- Private keys generated as RSA 2048-bit, stored in Terraform state (ensure state is secured)

### Cloudflare Integration

- Cloudflare provider configured in `versions.tf`, reads `CLOUDFLARE_API_TOKEN` from environment
- Zone IDs looked up dynamically via `cloudflare_zones` data source (filtered by `cloudflare_domains` variable)
- Domains managed: torale.ai, bananagraph.com, a2aregistry.org, repowire.io, feedforward.space
- All gateway DNS records are **proxied** (orange cloud) — safe with Origin CA certs
- ExternalDNS creates proxied A records from HTTPRoutes
- Terraform (`dns.tf`) manages only non-gateway records: email, verification, GitHub Pages
- Cloudflare zones set to Full (Strict) SSL via Terraform

## Development Commands

### Terraform

```bash
# Root infrastructure (cluster, Gateway, logging)
cd terraform
terraform init
terraform plan -var="project_id=baldmaninc"
terraform apply -var="project_id=baldmaninc"

# Project-specific (Cloud SQL, app resources)
cd terraform/projects/torale
terraform init
terraform plan
terraform apply
```

### Kubernetes Operations

```bash
# Connect to cluster
gcloud container clusters get-credentials clusterkit --region us-central1 --project baldmaninc

# Check Gateway
kubectl get gateway clusterkit-gateway -n clusterkit
kubectl describe gateway clusterkit-gateway -n clusterkit

# Check HTTPRoutes
kubectl get httproute -n clusterkit
kubectl describe httproute <name> -n clusterkit

# Check SSL certificates (Origin CA certs are self-managed)
gcloud compute ssl-certificates list

# Check ExternalDNS
kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns

# Verify DNS (should return Cloudflare IPs since proxied)
dig +short domain.com @1.1.1.1

# Verify SSL chain (issuer should be Cloudflare)
echo | openssl s_client -connect domain.com:443 2>/dev/null | openssl x509 -noout -issuer
```

## Common Workflows

### Adding New Subdomain

Wildcard Origin CA certs cover all subdomains — no Terraform changes needed.

**Quick steps:**
1. Application team creates HTTPRoute in `clusterkit` namespace
2. ExternalDNS auto-creates proxied DNS record
3. Done.

### Adding New Domain

See `docs/maintenance.md#adding-domains` for detailed instructions.

**Quick steps:**
1. Add domain to `origin_ca_domains` in `terraform/variables.tf`
2. Add domain to `cloudflare_domains` in `terraform/variables.tf`
3. Apply Terraform (looks up zone ID, generates cert, adds to Gateway, sets Full Strict SSL)
4. Application team creates HTTPRoute

### Deploying New Application

See `docs/app-integration.md` for application developer guide.

**Infrastructure team:**
1. Ensure domain has Origin CA cert on Gateway
2. Ensure ReferenceGrant exists (if cross-namespace)

**Application team:**
1. Create Deployment, Service, HTTPRoute manifests
2. Deploy to cluster
3. Verify HTTPRoute attached and DNS created

### Troubleshooting Deployments

See `docs/maintenance.md#troubleshooting` for comprehensive guide.

**Quick checks:**
1. Gateway status: `kubectl get gateway clusterkit-gateway -n clusterkit` (should show PROGRAMMED: True)
2. HTTPRoute status: `kubectl describe httproute <name> -n clusterkit` (should show Accepted: True)
3. SSL cert status: `gcloud compute ssl-certificates list`
4. DNS resolution: `dig +short domain.com @1.1.1.1` (should return Cloudflare IPs)
5. ExternalDNS logs: `kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns`
6. Cloudflare SSL mode: Should be Full (Strict) per zone

## Documentation Structure

- **README.md** - Project overview, quick start, architecture
- **CLAUDE.md** (this file) - AI assistant context
- **docs/app-integration.md** - 1-page guide for application developers
- **docs/maintenance.md** - Comprehensive operator guide
- **docs/external-dns-values.yaml** - Helm values for ExternalDNS

## Project-Specific Notes

### Current Setup

- **Cluster:** GKE Autopilot in us-central1
- **Project:** baldmaninc
- **Domains:** Managed via Cloudflare
- **Gateway:** clusterkit-gateway (namespace: clusterkit, IP: 34.149.49.202)
- **SSL:** Cloudflare Origin CA wildcard certs (Full Strict mode)
- **HTTPRoutes:** All routes in `clusterkit` namespace with cross-namespace service refs
- **App Namespaces:** torale, torale-staging, bananagraph, a2aregistry, repowire
- **Database:** Cloud SQL PostgreSQL (db-f1-micro, PITR disabled) — shared by torale, bananagraph, a2aregistry
- **DNS split:** ExternalDNS owns gateway A records (proxied), Terraform owns email/verification/Pages
- **Cost Savings:** £5/month saved by using single Gateway IP instead of 2 separate IPs

### Critical Operations

**Adding new domain (Origin CA cert):**
- Add domain to `origin_ca_domains` list in `terraform/variables.tf`
- Add domain to `cloudflare_domains` in `terraform/variables.tf`
- `terraform apply` — generates cert, adds to Gateway, sets Full Strict SSL

**Updating ExternalDNS:**
- Configuration in `docs/external-dns-values.yaml`
- Deploy: `helm upgrade external-dns external-dns/external-dns -n external-dns -f docs/external-dns-values.yaml`

**Gateway troubleshooting:**
- If PROGRAMMED: False, check SSL certs and static IP
- Force reconciliation: `kubectl annotate gateway clusterkit-gateway -n clusterkit reconcile="$(date +%s)" --overwrite`
- Check events: `kubectl describe gateway clusterkit-gateway -n clusterkit`
