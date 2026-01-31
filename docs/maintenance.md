# ClusterKit Maintenance Guide

**Operational guide for managing ClusterKit infrastructure.**

## Table of Contents

- [Infrastructure Management](#infrastructure-management)
- [Adding Domains](#adding-domains)
- [SSL Certificate Management](#ssl-certificate-management)
- [Gateway API Operations](#gateway-api-operations)
- [ExternalDNS Management](#externaldns-management)
- [Troubleshooting](#troubleshooting)
- [Cost Monitoring](#cost-monitoring)
- [Disaster Recovery](#disaster-recovery)

## Infrastructure Management

### Terraform Workflows

ClusterKit uses two separate Terraform states:

**Root Terraform** (`terraform/`):
- GKE Autopilot cluster
- Shared Gateway and Origin CA wildcard SSL certificates
- Cloudflare zone settings (Full Strict SSL)
- Static IPs
- Shared Cloud SQL instance (`clusterkit-db`) and proxy service account
- IAM (service accounts)
- Logging configuration

**Project Terraform** (`terraform/projects/<project>/`):
- Project-specific databases and users (created in shared instance)
- Project-specific GCS buckets and service accounts

### Applying Infrastructure Changes

```bash
# Root infrastructure
cd terraform
terraform plan -var="project_id=baldmaninc"
terraform apply -var="project_id=baldmaninc"

# Project-specific
cd terraform/projects/torale
terraform plan   # Uses default project_id from variables.tf
terraform apply
```

## Adding Domains

### Adding Subdomain to Existing Domain

No infrastructure changes needed — wildcard Origin CA certs cover all subdomains.

1. **Application team creates HTTPRoute**:
   - See `docs/app-integration.md` for template
   - Deploy to `clusterkit` namespace

2. **ExternalDNS auto-creates DNS** (proxied A record in Cloudflare)

3. **Verify**:
   ```bash
   # Check DNS (returns Cloudflare IPs since proxied)
   dig +short shop.torale.ai @1.1.1.1

   # Check HTTPRoute attached
   kubectl describe httproute <name> -n clusterkit
   ```

### Adding Completely New Domain

1. **Update Cloudflare API token** (if needed):
   - Go to https://dash.cloudflare.com/profile/api-tokens
   - Edit existing token
   - Add new domain to "Zone Resources"

2. **Add domain** to `terraform/variables.tf`:
   ```hcl
   variable "origin_ca_domains" {
     default = [
       # ... existing domains ...
       "newdomain.com",
     ]
   }

   variable "cloudflare_domains" {
     default = [
       # ... existing domains ...
       "newdomain.com",
     ]
   }
   ```

3. **Apply Terraform**:
   ```bash
   terraform apply -var="project_id=baldmaninc"
   ```
   Terraform looks up the zone ID automatically, generates an Origin CA cert, adds it to the Gateway, and sets Full (Strict) SSL.

6. **Add ReferenceGrant** if the app lives in a new namespace:
   ```hcl
   module "gateway" {
     allowed_route_namespaces = [
       # ... existing namespaces ...
       "newapp",  # Add here
     ]
   }
   ```

7. **Application team deploys HTTPRoute** — ExternalDNS handles DNS automatically.

## SSL Certificate Management

### Current Setup

Cloudflare Origin CA wildcard certificates per domain, generated automatically by Terraform:
- `torale.ai` + `*.torale.ai`
- `bananagraph.com` + `*.bananagraph.com`
- `a2aregistry.org` + `*.a2aregistry.org`
- `repowire.io` + `*.repowire.io`

SSL mode: **Full (Strict)** per zone — Cloudflare validates the Origin CA cert on the Gateway.

Certs are generated via `tls_private_key` + `tls_cert_request` + `cloudflare_origin_ca_certificate` resources. Private keys are RSA 2048-bit and stored in Terraform state.

### Checking Certificate Status

```bash
# List all GCP SSL certificates (Origin CA certs are self-managed)
gcloud compute ssl-certificates list

# Check specific certificate
gcloud compute ssl-certificates describe torale-ai-origin-cert
```

### Rotating Origin CA Certs

Origin CA certs are valid for 15 years. To force rotation:

1. Taint the cert: `terraform taint 'cloudflare_origin_ca_certificate.origin_ca["domain.com"]'`
2. `terraform apply` — `create_before_destroy` ensures zero-downtime rotation

## Gateway API Operations

### Gateway Status

```bash
# Check Gateway
kubectl get gateway clusterkit-gateway -n clusterkit

# Detailed status
kubectl describe gateway clusterkit-gateway -n clusterkit

# Should show:
# - ADDRESS: 34.149.49.202
# - PROGRAMMED: True
# - Attached Routes: N (current count)
```

### Listing HTTPRoutes

```bash
# All HTTPRoutes
kubectl get httproute -n clusterkit

# Detailed status
kubectl describe httproute <name> -n clusterkit

# Check which routes are attached to Gateway
kubectl get gateway clusterkit-gateway -n clusterkit -o jsonpath='{.status.listeners[0].attachedRoutes}'
```

### ReferenceGrant Management

ReferenceGrants allow HTTPRoutes in `clusterkit` namespace to reference services in app namespaces.

```bash
# Check ReferenceGrants
kubectl get referencegrant --all-namespaces
```

**Adding new namespace for cross-namespace routing**:

Edit `terraform/main.tf`:
```hcl
module "gateway" {
  allowed_route_namespaces = [
    "torale-staging",
    "new-namespace",  # Add here
  ]
}
```

Apply Terraform to create ReferenceGrant in the new namespace.

### Gateway Troubleshooting

**Gateway not getting IP address**:
1. Check Gateway events:
   ```bash
   kubectl describe gateway clusterkit-gateway -n clusterkit
   ```

2. Common issue: Static IP already in use
   ```bash
   gcloud compute addresses describe clusterkit-ingress-ip --global
   ```

**Gateway shows PROGRAMMED: False**:
1. Check SSL certificates exist: `gcloud compute ssl-certificates list`
2. Verify static IP exists: `gcloud compute addresses list --global`
3. Force reconciliation:
   ```bash
   kubectl annotate gateway clusterkit-gateway -n clusterkit reconcile="$(date +%s)" --overwrite
   ```

## ExternalDNS Management

### ExternalDNS Status

```bash
# Check ExternalDNS pods
kubectl get pods -n external-dns

# View logs
kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns

# Follow logs in real-time
kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns -f
```

### DNS Record Management

ExternalDNS watches HTTPRoutes and creates proxied A records in Cloudflare.

**How it works:**
1. HTTPRoute with `hostnames: ["api.torale.ai"]` is created
2. ExternalDNS detects the hostname
3. Creates proxied A record: `api.torale.ai` → `34.149.49.202` (orange cloud)
4. Creates TXT record: `api.torale.ai` → `"heritage=external-dns,external-dns/owner=clusterkit"`
5. Cloudflare CDN/WAF automatically active for the record

### What Terraform Manages vs ExternalDNS

- **ExternalDNS**: All gateway A records (created from HTTPRoutes, proxied)
- **Terraform** (`dns.tf`): Email (MX/DKIM/SPF), verification TXT, GitHub Pages, Cloudflare Pages

### ExternalDNS Configuration

ExternalDNS is deployed via Helm. Configuration is in `docs/external-dns-values.yaml`.

**Updating ExternalDNS**:
```bash
helm upgrade external-dns external-dns/external-dns \
  --namespace external-dns \
  -f docs/external-dns-values.yaml
```

**Current configuration**:
- Sources: `service`, `ingress`, `gateway-httproute`
- Provider: `cloudflare`
- Proxied: `true` (orange cloud by default)
- Policy: `upsert-only` (creates/updates, never deletes)
- TXT registry: Tracks ownership with TXT records

## Troubleshooting

### Common Issues

**HTTPRoute not attaching to Gateway**:
```bash
kubectl describe httproute <name> -n clusterkit

# Check for:
# - Accepted: True
# - ResolvedRefs: True
# - Namespace matches Gateway (must be 'clusterkit')
```

**DNS not resolving**:
```bash
# Check ExternalDNS created the record
kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns | grep your-domain

# Check DNS propagation (should return Cloudflare IPs since proxied)
dig +short your-domain.com @1.1.1.1
```

**SSL certificate warning in browser**:
```bash
# Verify Cloudflare SSL mode is Full (Strict) — check in Cloudflare dashboard or Terraform state
# Verify Origin CA cert is attached to Gateway
gcloud compute ssl-certificates list

# Test SSL chain (issuer should be Cloudflare)
echo | openssl s_client -connect your-domain.com:443 2>/dev/null | openssl x509 -noout -issuer
```

**Cross-namespace routing not working (staging)**:
```bash
# Check ReferenceGrant exists
kubectl get referencegrant -A

# Verify HTTPRoute has correct backendRef
kubectl get httproute <name> -n clusterkit -o yaml | grep -A 5 backendRefs
```

### Emergency Procedures

**Gateway completely down**:
1. Check Gateway status:
   ```bash
   kubectl describe gateway clusterkit-gateway -n clusterkit
   ```

2. If PROGRAMMED: False, delete and recreate:
   ```bash
   kubectl delete gateway clusterkit-gateway -n clusterkit
   terraform apply -var="project_id=baldmaninc"
   ```

3. If IP not assigned, check static IP:
   ```bash
   gcloud compute addresses describe clusterkit-ingress-ip --global
   ```

**ExternalDNS failing to create records**:
1. Check Cloudflare API token:
   ```bash
   kubectl get secret external-dns -n external-dns -o jsonpath='{.data.cloudflare_api_token}' | base64 -d
   ```

2. Restart ExternalDNS:
   ```bash
   kubectl rollout restart deployment external-dns -n external-dns
   ```

## Cost Monitoring

### Monthly Cost Breakdown

**Current costs** (~£30/month):
- GKE Autopilot cluster management: £0 (covered by $74.40 free tier)
- Load Balancer (Gateway): ~£5/month (1 IP, 5 forwarding rules)
- Cloud Monitoring: ~£3-5/month (optimized logging)
- Cloud SQL (torale): ~£6-8/month (db-f1-micro)
- Workload pods: ~£5-10/month (Spot pods)
- Static IP: ~£5/month (clusterkit-ingress-ip)

**Cost optimizations in place**:
- Logging retention: 7 days (down from 30)
- INFO log sampling: 10% (ERROR/WARNING: 100%)
- Health check exclusions: Saves ~24% of logs
- Spot pods: 60-91% savings on workload costs
- Single shared Gateway IP: Saves £5/month (vs separate IPs per environment)

### Checking Costs

```bash
# GCP Console cost breakdown
# https://console.cloud.google.com/billing/reports?project=baldmaninc
```

## Disaster Recovery

### Backup Critical Resources

**Terraform state** (already backed up to GCS or local):
```bash
cd terraform
terraform state pull > backup-state.json
```

**Kubernetes resources**:
```bash
# Backup all HTTPRoutes
kubectl get httproute -n clusterkit -o yaml > backup-httproutes.yaml

# Backup Gateway
kubectl get gateway clusterkit-gateway -n clusterkit -o yaml > backup-gateway.yaml

# Backup ReferenceGrants
kubectl get referencegrant --all-namespaces -o yaml > backup-referencegrants.yaml
```

### Recovery Procedures

**Recreate Gateway from Terraform**:
```bash
cd terraform
terraform apply -var="project_id=baldmaninc"
```

**Recreate HTTPRoutes**:
```bash
kubectl apply -f backup-httproutes.yaml
```

**Verify after recovery**:
```bash
# Gateway programmed
kubectl get gateway clusterkit-gateway -n clusterkit

# HTTPRoutes attached
kubectl get httproute -n clusterkit

# DNS resolving (should return Cloudflare IPs)
dig +short torale.ai @1.1.1.1
```

## Reference

- **Project**: baldmaninc
- **Region**: us-central1
- **Cluster**: clusterkit
- **Gateway**: clusterkit-gateway (namespace: clusterkit)
- **Gateway IP**: 34.149.49.202
- **Static IP name**: clusterkit-ingress-ip
- **SSL**: Cloudflare Origin CA wildcard certs (Full Strict mode)
- **ExternalDNS namespace**: external-dns
- **DNS split**: ExternalDNS owns gateway A records, Terraform owns email/verification/Pages records
- **Terraform states**:
  - Root: `terraform/terraform.tfstate`
  - Torale: `terraform/projects/torale/terraform.tfstate`
