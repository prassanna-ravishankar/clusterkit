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
- Shared Gateway and SSL certificates
- Static IPs
- IAM (service accounts)
- Logging configuration

**Project Terraform** (`terraform/projects/torale/`):
- Cloud SQL databases
- Project-specific resources

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

### Importing Existing Resources

If manually created resources need to be brought under Terraform management:

```bash
# Example: Import SSL certificate
terraform import -var="project_id=baldmaninc" \
  module.ssl_cert_torale_prod.google_compute_managed_ssl_certificate.cert \
  projects/baldmaninc/global/sslCertificates/torale-prod-cert

# Example: Import Gateway
terraform import -var="project_id=baldmaninc" \
  'module.gateway.kubernetes_manifest.gateway' \
  'apiVersion=gateway.networking.k8s.io/v1,kind=Gateway,namespace=torale,name=clusterkit-gateway'
```

## Adding Domains

### Adding Subdomain to Existing Domain

**Process:** Add to SSL cert → Create HTTPRoute → DNS auto-created

1. **Update SSL certificate** (15 min downtime):

   Edit `terraform/main.tf`:
   ```hcl
   module "ssl_cert_torale_prod" {
     domains = [
       "torale.ai",
       "api.torale.ai",
       "docs.torale.ai",
       "shop.torale.ai",  # NEW
     ]
   }
   ```

   Apply:
   ```bash
   cd terraform
   terraform apply -var="project_id=baldmaninc"
   ```

   **Note**: This recreates the certificate (~15 min provisioning time with brief downtime)

2. **Application team creates HTTPRoute**:
   - See `docs/app-integration.md` for template
   - DNS is automatically created by ExternalDNS

3. **Verify**:
   ```bash
   # Check cert provisioned
   gcloud compute ssl-certificates describe torale-prod-cert

   # Check DNS created
   dig +short shop.torale.ai @1.1.1.1
   ```

### Adding Completely New Domain

1. **Update Cloudflare API token**:
   - Go to https://dash.cloudflare.com/profile/api-tokens
   - Edit existing token
   - Add new domain to "Zone Resources"

2. **Update ExternalDNS secret** (if new token):
   ```bash
   kubectl create secret generic cloudflare-api-token \
     --from-literal=apiToken=YOUR_NEW_TOKEN \
     -n external-dns \
     --dry-run=client -o yaml | kubectl apply -f -

   kubectl rollout restart deployment external-dns -n external-dns
   ```

3. **Create SSL certificate module** in `terraform/main.tf`:
   ```hcl
   module "ssl_cert_newdomain_prod" {
     source = "./modules/ssl-certificate"

     project_id       = var.project_id
     certificate_name = "newdomain-prod-cert"
     domains          = ["newdomain.com", "api.newdomain.com"]
   }
   ```

4. **Update Gateway** to reference new cert:
   ```hcl
   module "gateway" {
     ssl_certificate_names = [
       module.ssl_cert_torale_prod.certificate_name,
       module.ssl_cert_torale_staging.certificate_name,
       module.ssl_cert_newdomain_prod.certificate_name,  # NEW
     ]
   }
   ```

5. **Apply Terraform**:
   ```bash
   terraform apply -var="project_id=baldmaninc"
   ```

## SSL Certificate Management

### Checking Certificate Status

```bash
# List all certificates
gcloud compute ssl-certificates list

# Check specific certificate
gcloud compute ssl-certificates describe torale-prod-cert

# Check which domains are covered
gcloud compute ssl-certificates describe torale-prod-cert \
  --format="value(managed.domains)"
```

Certificate status values:
- `PROVISIONING` - Certificate being created (~15 min)
- `ACTIVE` - Certificate ready and serving traffic
- `RENEWAL_FAILED` - Issue with renewal (check domain ownership)

### Certificate Limitations

Google-managed certificates:
- ✅ Up to 100 non-wildcard domains per certificate
- ❌ No wildcard domain support (`*.torale.ai` not supported)
- ❌ Updates not supported (must recreate for new domains)
- ✅ Automatic renewal before expiration

For wildcard support, migrate to cert-manager + Let's Encrypt (requires DNS-01 challenge setup).

### Troubleshooting Certificate Issues

**Certificate stuck in PROVISIONING**:
1. Check DNS points to Gateway IP:
   ```bash
   dig +short yourdomain.com @1.1.1.1
   # Should return: 34.149.49.202
   ```

2. Check HTTPRoute is attached:
   ```bash
   kubectl describe httproute <name> -n torale
   # Should show: Accepted: True
   ```

3. Wait up to 15 minutes for initial provisioning

**Certificate shows RENEWAL_FAILED**:
1. Verify domain still resolves correctly
2. Check Gateway is still using the certificate
3. May need to delete and recreate certificate

## Gateway API Operations

### Gateway Status

```bash
# Check Gateway
kubectl get gateway clusterkit-gateway -n torale

# Detailed status
kubectl describe gateway clusterkit-gateway -n torale

# Should show:
# - ADDRESS: 34.149.49.202
# - PROGRAMMED: True
# - Attached Routes: 5 (or current count)
```

### Listing HTTPRoutes

```bash
# All HTTPRoutes
kubectl get httproute -n torale

# Detailed status
kubectl describe httproute <name> -n torale

# Check which routes are attached to Gateway
kubectl get gateway clusterkit-gateway -n torale -o jsonpath='{.status.listeners[0].attachedRoutes}'
```

### ReferenceGrant Management

ReferenceGrants allow HTTPRoutes in `torale` namespace to reference services in other namespaces (e.g., `torale-staging`).

```bash
# Check ReferenceGrants
kubectl get referencegrant -n torale-staging

# Verify permissions
kubectl describe referencegrant allow-torale-to-torale-staging-services -n torale-staging
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
   kubectl describe gateway clusterkit-gateway -n torale
   ```

2. Look for errors in events section

3. Common issue: Static IP already in use
   ```bash
   # Check IP assignment
   gcloud compute addresses describe clusterkit-ingress-ip --global
   ```

**Gateway shows PROGRAMMED: False**:
1. Check SSL certificates exist and are ACTIVE
2. Verify static IP exists: `gcloud compute addresses list --global`
3. Force reconciliation:
   ```bash
   kubectl annotate gateway clusterkit-gateway -n torale reconcile="$(date +%s)" --overwrite
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

ExternalDNS watches HTTPRoutes and creates corresponding DNS records in Cloudflare.

**How it works:**
1. HTTPRoute with `hostnames: ["api.torale.ai"]` is created
2. ExternalDNS detects the hostname
3. Creates A record: `api.torale.ai` → `34.149.49.202`
4. Creates TXT record: `api.torale.ai` → `"heritage=external-dns,external-dns/owner=clusterkit"`

**Viewing managed DNS records**:
```bash
# Filter logs for specific domain
kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns | grep torale.ai

# Check Cloudflare directly (requires CF_API_TOKEN)
curl -X GET "https://api.cloudflare.com/client/v4/zones/ZONE_ID/dns_records" \
  -H "Authorization: Bearer $CF_API_TOKEN" \
  -H "Content-Type: application/json" | jq '.result[] | select(.name | contains("torale"))'
```

### Cloudflare Proxy Mode

**CRITICAL**: DNS records MUST be "DNS only" (gray cloud), NOT "Proxied" (orange cloud).

Orange cloud = Cloudflare terminates SSL → breaks GCP-managed certificates.

**Fixing Cloudflare proxy issues**:
1. Ensure HTTPRoute has annotation:
   ```yaml
   annotations:
     external-dns.alpha.kubernetes.io/cloudflare-proxied: "false"
   ```

2. Check ExternalDNS config (should have `--cloudflare-proxied` flag):
   ```bash
   kubectl get deployment external-dns -n external-dns -o yaml | grep cloudflare-proxied
   ```

3. Manually disable in Cloudflare if needed:
   - Go to Cloudflare DNS dashboard
   - Click orange cloud icon → turns to gray cloud
   - Set to "DNS only"

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
- Policy: `upsert-only` (creates/updates, never deletes)
- TXT registry: Tracks ownership with TXT records

## Troubleshooting

### Common Issues

**HTTPRoute not attaching to Gateway**:
```bash
kubectl describe httproute <name> -n torale

# Check for:
# - Accepted: True
# - ResolvedRefs: True
# - Namespace matches Gateway (must be 'torale')
```

**DNS not resolving**:
```bash
# Check ExternalDNS created the record
kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns | grep your-domain

# Check DNS propagation
dig +short your-domain.com @1.1.1.1
# Should return: 34.149.49.202

# Check Cloudflare dashboard
# Verify record exists and is "DNS only" (gray cloud)
```

**SSL certificate warning in browser**:
```bash
# Verify domain is in certificate
gcloud compute ssl-certificates describe torale-prod-cert \
  --format="value(managed.domains)"

# Check certificate status
gcloud compute ssl-certificates describe torale-prod-cert \
  --format="value(managed.status)"
# Should be: ACTIVE

# Test SSL
curl -vI https://your-domain.com 2>&1 | grep -A 5 "SSL certificate"
```

**Cross-namespace routing not working (staging)**:
```bash
# Check ReferenceGrant exists
kubectl get referencegrant -n torale-staging

# Verify HTTPRoute has correct backendRef
kubectl get httproute <name> -n torale -o yaml | grep -A 5 backendRefs

# Should show:
#   - name: service-name
#     namespace: torale-staging
#     port: 80
```

### Emergency Procedures

**Gateway completely down**:
1. Check Gateway status:
   ```bash
   kubectl describe gateway clusterkit-gateway -n torale
   ```

2. If PROGRAMMED: False, delete and recreate:
   ```bash
   kubectl delete gateway clusterkit-gateway -n torale
   terraform apply -var="project_id=baldmaninc"
   ```

3. If IP not assigned, check static IP:
   ```bash
   gcloud compute addresses describe clusterkit-ingress-ip --global
   ```

**ExternalDNS failing to create records**:
1. Check Cloudflare API token:
   ```bash
   # Get current secret
   kubectl get secret cloudflare-api-token -n external-dns -o jsonpath='{.data.apiToken}' | base64 -d

   # Test token
   curl -X GET "https://api.cloudflare.com/client/v4/zones" \
     -H "Authorization: Bearer YOUR_TOKEN" \
     -H "Content-Type: application/json"
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
# View billing account
gcloud billing accounts list

# View project costs (requires billing account access)
gcloud billing projects describe baldmaninc

# GCP Console cost breakdown
# https://console.cloud.google.com/billing/reports?project=baldmaninc
```

### Cost Reduction Strategies

If costs exceed budget:

1. **Reduce logging**:
   - Edit `terraform/modules/logging/` variables
   - Increase INFO sampling rate (currently 0.1 = 10%)
   - Reduce retention days (currently 7)

2. **Right-size Cloud SQL**:
   - Current: db-f1-micro (cheapest)
   - If not heavily used, consider Cloud Run PostgreSQL

3. **Audit workloads**:
   - Check for overprovisioned resource requests
   - Ensure Spot pods are used where possible

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
kubectl get httproute -n torale -o yaml > backup-httproutes.yaml

# Backup Gateway
kubectl get gateway clusterkit-gateway -n torale -o yaml > backup-gateway.yaml

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
kubectl get gateway clusterkit-gateway -n torale

# HTTPRoutes attached
kubectl get httproute -n torale

# DNS resolving
dig +short torale.ai @1.1.1.1
```

### Complete Cluster Rebuild

If GKE cluster is lost:

1. **Run Terraform**:
   ```bash
   cd terraform
   terraform apply -var="project_id=baldmaninc"
   ```

2. **Verify Gateway**:
   ```bash
   kubectl get gateway clusterkit-gateway -n torale
   ```

3. **Restore application HTTPRoutes** (application teams responsible for their apps)

4. **Verify DNS and SSL** working

**Note**: ExternalDNS will automatically recreate DNS records from HTTPRoutes.

## Reference

- **Project**: baldmaninc
- **Region**: us-central1
- **Cluster**: clusterkit
- **Gateway**: clusterkit-gateway (namespace: torale)
- **Gateway IP**: 34.149.49.202
- **Static IP name**: clusterkit-ingress-ip
- **ExternalDNS namespace**: external-dns
- **Terraform states**:
  - Root: `terraform/terraform.tfstate`
  - Torale: `terraform/projects/torale/terraform.tfstate`
