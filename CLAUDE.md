# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ClusterKit is a simplified Kubernetes platform for personal projects on GKE Autopilot with Cloudflare DNS integration. It automates infrastructure deployment, certificate management, and DNS configuration to minimize costs while maintaining full Kubernetes functionality.

**Key Technologies:**
- GKE Autopilot (managed Kubernetes)
- Terraform (infrastructure as code)
- Gateway API (shared load balancer)
- Google-managed SSL certificates
- ExternalDNS (Cloudflare integration)

**Target monthly cost:** ~£25-30 for infrastructure + multiple applications

## Architecture

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
- Centralized SSL termination
- ExternalDNS auto-creates DNS from HTTPRoute hostnames

### Two-Tier Terraform Structure

**1. Root Terraform** (`terraform/`):
- GKE Autopilot cluster
- Gateway API (Gateway, SSL certificates, ReferenceGrants)
- Static IP (`clusterkit-ingress-ip`)
- Cloudflare DNS records for all domains (`dns.tf`) — single source of truth
- Shared Cloud SQL instance (`clusterkit-db`) and proxy service account
- Workload Identity bindings for database access (torale, bananagraph, a2aregistry, prefect)
- Prefect database and user
- IAM (service accounts with Workload Identity)
- Logging optimization (project-level, includes ExternalDNS INFO exclusion)

**2. Project-Specific Terraform** (`terraform/projects/<project>/`):
- Project-specific databases and users (created in shared instance)
- Application-specific GCS buckets and service accounts
- Project resources separate from cluster

Both use the same GCP project (`baldmaninc`) but maintain separate Terraform states.

### Terraform Module Structure

Reusable modules in `terraform/modules/`:
- `gke/` - GKE Autopilot cluster with configurable logging/monitoring
- `gateway-api/` - Gateway with SSL certs and ReferenceGrants
- `ssl-certificate/` - Google-managed SSL certificates
- `cloudflare-dns/` - Cloudflare DNS record management
- `httproute/` - HTTPRoute template (for application use)
- `networking/` - Static IP addresses
- `iam/` - Service accounts with Workload Identity
- `logging/` - Cost-optimized Cloud Logging (with custom exclusion support)
- `cloudsql-instance/`, `cloudsql-proxy-sa/` - PostgreSQL instances
- `static-ip/` - Global static IP addresses

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

### Gateway API Conventions

**HTTPRoute Requirements:**
- MUST be in `clusterkit` namespace (Gateway namespace)
- MUST include annotation: `external-dns.alpha.kubernetes.io/cloudflare-proxied: "false"`
- Cross-namespace service refs: Add `namespace: <app-namespace>` to backendRefs
- ExternalDNS auto-creates DNS from `hostnames` field

**Example HTTPRoute:**
```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: app-prod
  namespace: clusterkit
  annotations:
    external-dns.alpha.kubernetes.io/cloudflare-proxied: "false"
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

Gateway, SSL certificates, and ReferenceGrants are managed by ClusterKit Terraform.

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
- ExternalDNS watches HTTPRoutes and auto-creates DNS records
- **Critical**: HTTPRoute annotation `cloudflare-proxied: false` required for GCP SSL to work

### SSL Certificate Limitations

Google-managed certificates:
- ✅ Up to 100 non-wildcard domains per certificate
- ❌ No wildcard domain support (`*.torale.ai` not supported)
- ❌ Updates not supported (must recreate for new domains)
- ✅ Automatic renewal before expiration
- Adding domain = cert recreation (~15 min with brief downtime)

### Cloudflare Integration

- DNS records managed via Terraform (`terraform/dns.tf`) — single source of truth for all domains
- Cloudflare provider configured in `versions.tf`, reads `CLOUDFLARE_API_TOKEN` from environment
- Zone IDs stored in `variables.tf` (`cloudflare_zone_ids` map)
- Domains managed: torale.ai, bananagraph.com, a2aregistry.org, repowire.io, feedforward.space
- ExternalDNS watches HTTPRoutes for gateway-related A records (TXT ownership records not in Terraform)
- DNS records MUST be "DNS only" (gray cloud), not "Proxied" (orange cloud) for gateway domains
- Orange cloud = Cloudflare terminates SSL → breaks GCP-managed certificates
- All gateway domains point to shared Gateway IP (34.149.49.202)

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

# Check SSL certificates
gcloud compute ssl-certificates list
gcloud compute ssl-certificates describe torale-prod-cert

# Check ExternalDNS
kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns

# Verify DNS
dig +short domain.com @1.1.1.1
```

## Common Workflows

### Adding New Subdomain

See `docs/maintenance.md#adding-domains` for detailed instructions.

**Quick steps:**
1. Add domain to SSL cert in `terraform/main.tf`
2. Add DNS record in `terraform/dns.tf`
3. Apply Terraform (15 min for cert provisioning)
4. Application team creates HTTPRoute

### Deploying New Application

See `docs/app-integration.md` for application developer guide.

**Infrastructure team:**
1. Add subdomain to SSL certificate (if new)
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
3. SSL cert status: `gcloud compute ssl-certificates describe <cert-name>`
4. DNS resolution: `dig +short domain.com @1.1.1.1` (should return 34.149.49.202)
5. ExternalDNS logs: `kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns`
6. Cloudflare DNS mode: Should be gray cloud (DNS-only), not orange (proxied)

## Documentation Structure

- **README.md** - Project overview, quick start, architecture
- **CLAUDE.md** (this file) - AI assistant context
- **docs/app-integration.md** - 1-page guide for application developers
- **docs/maintenance.md** - Comprehensive operator guide
- **docs/external-dns-values.yaml** - Helm values for ExternalDNS
- **docs/prefect-values.yaml** - Helm values for Prefect Server

## Project-Specific Notes

### Current Setup

- **Cluster:** GKE Autopilot in us-central1
- **Project:** baldmaninc
- **Domains:** Managed via Cloudflare
- **Gateway:** clusterkit-gateway (namespace: clusterkit, IP: 34.149.49.202)
- **HTTPRoutes:** All routes in `clusterkit` namespace with cross-namespace service refs
- **App Namespaces:** torale, torale-staging, bananagraph, a2aregistry, repowire, prefect
- **Database:** Cloud SQL PostgreSQL (db-f1-micro, PITR disabled) — shared by torale, bananagraph, a2aregistry, prefect
- **DNS:** All domains managed via Terraform Cloudflare provider (`terraform/dns.tf`)
- **Cost Savings:** £5/month saved by using single Gateway IP instead of 2 separate IPs

### Critical Operations

**Adding domain to SSL certificate:**
- Edit `terraform/main.tf` SSL cert module
- Add domain to `domains` list
- Add DNS record in `terraform/dns.tf`
- `terraform apply` (recreates cert, ~15 min downtime)

**Updating ExternalDNS:**
- Configuration in `docs/external-dns-values.yaml`
- Deploy: `helm upgrade external-dns external-dns/external-dns -n external-dns -f docs/external-dns-values.yaml`

**Gateway troubleshooting:**
- If PROGRAMMED: False, check SSL certs and static IP
- Force reconciliation: `kubectl annotate gateway clusterkit-gateway -n clusterkit reconcile="$(date +%s)" --overwrite`
- Check events: `kubectl describe gateway clusterkit-gateway -n clusterkit`
