# Torale Repository Integration Guide

This guide explains how to integrate the torale application repository with ClusterKit's Gateway API infrastructure.

## Overview

ClusterKit now uses **Gateway API** instead of separate Ingresses for each environment. This provides:
- **Shared IP address**: Both production and staging use `34.149.49.202` (saves £5/month)
- **Cross-namespace routing**: HTTPRoutes in `torale` namespace route to services in both `torale` and `torale-staging`
- **Automatic DNS**: ExternalDNS manages Cloudflare DNS from HTTPRoute hostnames
- **Centralized TLS**: Google-managed SSL certificates cover all domains

## Infrastructure Ownership

### ClusterKit Terraform Manages (DO NOT CHANGE IN TORALE REPO)

1. **Gateway** (`clusterkit-gateway` in `torale` namespace)
   - Shared load balancer for all routes
   - Attached to static IP `clusterkit-ingress-ip`
   - SSL certificates: `torale-prod-cert`, `torale-staging-cert`

2. **SSL Certificates** (Google-managed)
   - Production: `torale.ai`, `api.torale.ai`, `docs.torale.ai`
   - Staging: `staging.torale.ai`, `api.staging.torale.ai`

3. **ReferenceGrant** (in `torale-staging` namespace)
   - Allows HTTPRoutes in `torale` namespace to access services in `torale-staging`

4. **ExternalDNS Configuration**
   - Watches HTTPRoutes for DNS automation
   - Creates/updates Cloudflare DNS records

### Torale Repo Manages

1. **HTTPRoutes** (routing rules for your application)
2. **Services** (ClusterIP exposing pods)
3. **Deployments** (your application workloads)
4. **ConfigMaps, Secrets** (application config)

## HTTPRoute Examples

### Production HTTPRoute (torale namespace → torale service)

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: torale-prod-main
  namespace: torale  # Same namespace as Gateway
  annotations:
    # REQUIRED: Disable Cloudflare proxy for GCP SSL to work
    external-dns.alpha.kubernetes.io/cloudflare-proxied: "false"
spec:
  parentRefs:
  - name: clusterkit-gateway  # Gateway name from ClusterKit
    namespace: torale
  hostnames:
  - "torale.ai"
  rules:
  - backendRefs:
    - name: torale-frontend  # Service in same namespace
      port: 80
```

### Staging HTTPRoute (torale namespace → torale-staging service)

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: torale-staging-main
  namespace: torale  # HTTPRoute lives in torale namespace
  annotations:
    external-dns.alpha.kubernetes.io/cloudflare-proxied: "false"
spec:
  parentRefs:
  - name: clusterkit-gateway
  hostnames:
  - "staging.torale.ai"
  rules:
  - backendRefs:
    - name: torale-frontend
      namespace: torale-staging  # Cross-namespace reference
      port: 80
```

## Migration from Ingress to HTTPRoute

### OLD (Ingress - DELETE THIS)

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: torale
  namespace: torale
  annotations:
    kubernetes.io/ingress.class: "gce"
    networking.gke.io/managed-certificates: torale-prod-cert
spec:
  rules:
  - host: torale.ai
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: torale-frontend
            port:
              number: 80
```

### NEW (HTTPRoute - USE THIS)

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: torale-prod-main
  namespace: torale
  annotations:
    external-dns.alpha.kubernetes.io/cloudflare-proxied: "false"
spec:
  parentRefs:
  - name: clusterkit-gateway
    namespace: torale
  hostnames:
  - "torale.ai"
  rules:
  - backendRefs:
    - name: torale-frontend
      port: 80
```

**Key Differences:**
1. `kind: HTTPRoute` instead of `kind: Ingress`
2. `parentRefs` points to Gateway instead of `ingress.class` annotation
3. No ManagedCertificate annotation (Gateway handles SSL)
4. Must add `cloudflare-proxied: false` annotation
5. DNS auto-created by ExternalDNS (no manual Cloudflare updates)

## CI/CD Integration

### Important: Prevent CI from Breaking Infrastructure

Your CI/CD pipeline must **NOT**:
- Create/delete Gateway resources
- Modify SSL certificates
- Change ReferenceGrants
- Delete HTTPRoutes managed by Terraform

### Recommended CI/CD Pattern

**Option 1: Torale repo owns HTTPRoutes (RECOMMENDED)**

```yaml
# .github/workflows/deploy.yml
- name: Deploy HTTPRoutes
  run: |
    kubectl apply -f manifests/httproutes/

- name: Deploy Application
  run: |
    kubectl apply -f manifests/deployments/
    kubectl apply -f manifests/services/
```

**Directory Structure:**
```
torale-repo/
├── manifests/
│   ├── httproutes/
│   │   ├── production.yaml  # HTTPRoutes for prod
│   │   └── staging.yaml     # HTTPRoutes for staging
│   ├── deployments/
│   │   ├── frontend.yaml
│   │   └── api.yaml
│   └── services/
│       ├── frontend.yaml
│       └── api.yaml
```

**Option 2: ClusterKit Terraform owns HTTPRoutes**

If you prefer ClusterKit to manage HTTPRoutes:
- Remove HTTPRoute manifests from torale repo
- Add HTTPRoutes to ClusterKit Terraform
- CI only deploys Deployments and Services

### Safety Checks

Add these checks to your CI pipeline:

```bash
# Ensure Gateway exists (ClusterKit managed)
kubectl get gateway clusterkit-gateway -n torale || exit 1

# Ensure SSL certs exist (ClusterKit managed)
gcloud compute ssl-certificates describe torale-prod-cert || exit 1

# Deploy only application resources
kubectl apply -f manifests/deployments/
kubectl apply -f manifests/services/
kubectl apply -f manifests/httproutes/  # Only if torale owns these
```

## DNS Management

### Automatic (via ExternalDNS)

ExternalDNS watches HTTPRoutes and creates DNS records automatically:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: my-new-route
  namespace: torale
  annotations:
    external-dns.alpha.kubernetes.io/cloudflare-proxied: "false"
spec:
  hostnames:
  - "shop.torale.ai"  # ExternalDNS will create this A record → 34.149.49.202
  # ...
```

**After applying HTTPRoute:**
1. ExternalDNS detects new hostname
2. Creates Cloudflare A record: `shop.torale.ai` → `34.149.49.202`
3. Creates TXT record for ownership tracking
4. Sets DNS-only mode (gray cloud, not orange)

### Manual (if needed)

If you need to manually manage DNS:
1. Go to Cloudflare dashboard
2. Add A record: `subdomain.torale.ai` → `34.149.49.202`
3. **IMPORTANT**: Set to "DNS only" (gray cloud), NOT "Proxied" (orange cloud)

## Adding New Subdomains

### Steps:

1. **Add domain to SSL certificate** (ClusterKit Terraform)

   Edit `terraform/main.tf`:
   ```hcl
   module "ssl_cert_torale_prod" {
     source = "./modules/ssl-certificate"

     project_id       = var.project_id
     certificate_name = "torale-prod-cert"
     domains          = [
       "torale.ai",
       "api.torale.ai",
       "docs.torale.ai",
       "shop.torale.ai",  # NEW SUBDOMAIN
     ]
   }
   ```

   Apply: `terraform apply -var="project_id=baldmaninc"`

   **WARNING**: This recreates the certificate (15 min downtime)

2. **Create HTTPRoute** (torale repo)

   ```yaml
   apiVersion: gateway.networking.k8s.io/v1
   kind: HTTPRoute
   metadata:
     name: torale-prod-shop
     namespace: torale
     annotations:
       external-dns.alpha.kubernetes.io/cloudflare-proxied: "false"
   spec:
     parentRefs:
     - name: clusterkit-gateway
       namespace: torale
     hostnames:
     - "shop.torale.ai"
     rules:
     - backendRefs:
       - name: torale-shop
         port: 80
   ```

3. **DNS is automatic** - ExternalDNS creates the record

## Troubleshooting

### HTTPRoute not attaching to Gateway

```bash
# Check HTTPRoute status
kubectl describe httproute <name> -n torale

# Common issue: Wrong namespace
# HTTPRoutes must be in 'torale' namespace (same as Gateway)
```

### SSL certificate warnings

```bash
# Check cert status
gcloud compute ssl-certificates describe torale-prod-cert

# Ensure Cloudflare DNS is gray cloud (DNS-only)
# Orange cloud = Cloudflare SSL (breaks GCP certs)
```

### DNS not updating

```bash
# Check ExternalDNS logs
kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns

# Verify HTTPRoute annotation
kubectl get httproute <name> -n torale -o yaml | grep cloudflare-proxied
# Should show: "false"
```

### Cross-namespace routing not working (staging)

```bash
# Check ReferenceGrant exists
kubectl get referencegrant -n torale-staging

# Should see: allow-torale-to-torale-staging-services
```

## Cost Savings

This setup saves money by:
- **Single static IP**: £5/month (was £10 with 2 IPs)
- **Shared Gateway**: No duplicate load balancer costs
- **Google-managed SSL**: Free (vs. paid cert management)

## Next Steps for Torale Repo

1. **Remove old Ingress manifests** from your repo
2. **Create HTTPRoute manifests** using examples above
3. **Update CI/CD** to deploy HTTPRoutes
4. **Test in staging** before production
5. **Update documentation** in torale repo

## Questions?

- ClusterKit infrastructure: Check `/Users/prass/development/projects/clusterkit/terraform/`
- Gateway config: `terraform/modules/gateway-api/`
- HTTPRoute module (if needed): `terraform/modules/httproute/`
