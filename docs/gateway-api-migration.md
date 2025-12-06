# Gateway API Migration Guide

**Goal:** Migrate from 2 separate GKE Ingresses to 1 shared Gateway with cross-namespace routing

**Benefits:**
- Save £5/month (£60/year) by deleting staging static IP
- Keep namespace isolation (torale vs torale-staging)
- Modern, future-proof Gateway API
- Single shared IP for all domains

**Estimated Time:** 4-6 hours
**Downtime:** <5 minutes (DNS update only)

---

## Prerequisites

- [x] GKE Autopilot cluster (Gateway API enabled by default)
- [x] Two namespaces: `torale` (prod), `torale-staging` (staging)
- [x] Static IP: `clusterkit-ingress-ip` (34.149.49.202)
- [x] Cloudflare DNS access

---

## Architecture Overview

### Current (Ingress)
```
Ingress (torale namespace)
├── IP: clusterkit-ingress-ip (34.149.49.202)
├── Domains: torale.ai, api.torale.ai, docs.torale.ai
└── Certificate: torale-cert (ManagedCertificate)

Ingress (torale-staging namespace)
├── IP: torale-staging-ip (35.186.213.216)  ← DELETE THIS
├── Domains: staging.torale.ai, api.staging.torale.ai
└── Certificate: torale-staging-cert (ManagedCertificate)
```

### Target (Gateway API)
```
Gateway (torale namespace)
├── IP: clusterkit-ingress-ip (34.149.49.202)  ← SHARED
├── SSL: Google-managed certificates
└── Accepts HTTPRoutes from: torale, torale-staging

HTTPRoute (torale namespace)
├── Routes: torale.ai → torale-frontend
│          api.torale.ai → torale-api
│          docs.torale.ai → torale-docs
└── Services: torale namespace

HTTPRoute (torale-staging namespace)
├── Routes: staging.torale.ai → torale-frontend
│          api.staging.torale.ai → torale-api
└── Services: torale-staging namespace

ReferenceGrant (torale-staging namespace)
└── Allows Gateway to access torale-staging services
```

---

## Phase 1: Create Google-Managed SSL Certificates

### Step 1.1: Create Production Certificate

```bash
gcloud compute ssl-certificates create torale-prod-cert \
  --domains=torale.ai,api.torale.ai,docs.torale.ai \
  --global
```

**Output:**
```
Created [https://www.googleapis.com/compute/v1/projects/baldmaninc/global/sslCertificates/torale-prod-cert].
NAME              TYPE     CREATION_TIMESTAMP             EXPIRE_TIME  MANAGED_STATUS
torale-prod-cert  MANAGED  2025-12-06T10:30:00.000-08:00               PROVISIONING
```

### Step 1.2: Create Staging Certificate

```bash
gcloud compute ssl-certificates create torale-staging-cert \
  --domains=staging.torale.ai,api.staging.torale.ai \
  --global
```

### Step 1.3: Verify Certificates

```bash
gcloud compute ssl-certificates list
```

**Note:** Certificates will show `MANAGED_STATUS: PROVISIONING` until the Gateway is created and DNS is pointed to it. This is normal.

---

## Phase 2: Create Gateway Resource

### Step 2.1: Create Gateway Manifest

Save as `k8s/gateway/gateway.yaml`:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: clusterkit-gateway
  namespace: torale
  annotations:
    # Reference Google-managed SSL certificates
    networking.gke.io/pre-shared-certs: "torale-prod-cert,torale-staging-cert"
spec:
  gatewayClassName: gke-l7-global-external-managed
  listeners:
  - name: https
    protocol: HTTPS
    port: 443
    # Allow HTTPRoutes from any namespace (with proper ReferenceGrants)
    allowedRoutes:
      namespaces:
        from: All
  addresses:
  - type: NamedAddress
    value: clusterkit-ingress-ip  # Use existing static IP
```

**Key points:**
- `gatewayClassName: gke-l7-global-external-managed` → Global external Application Load Balancer
- `networking.gke.io/pre-shared-certs` → References Google-managed SSL certs
- `allowedRoutes.namespaces.from: All` → Accept HTTPRoutes from any namespace
- `addresses.value: clusterkit-ingress-ip` → Use existing static IP

### Step 2.2: Apply Gateway

```bash
kubectl apply -f k8s/gateway/gateway.yaml
```

### Step 2.3: Wait for Gateway Provisioning

```bash
kubectl get gateway clusterkit-gateway -n torale -w
```

**Wait for:**
```
NAME                  CLASS                              ADDRESS          PROGRAMMED   AGE
clusterkit-gateway   gke-l7-global-external-managed     34.149.49.202    True         5m
```

**Note:** This may take 2-5 minutes as GKE provisions the load balancer.

---

## Phase 3: Create HTTPRoute for Production

### Step 3.1: Create Production HTTPRoute

Save as `k8s/gateway/httproute-prod.yaml`:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: torale-prod
  namespace: torale
spec:
  parentRefs:
  - name: clusterkit-gateway
    namespace: torale
  hostnames:
  - "torale.ai"
  - "api.torale.ai"
  - "docs.torale.ai"
  rules:
  # Main site
  - matches:
    - headers:
      - name: ":authority"
        value: "torale.ai"
    backendRefs:
    - name: torale-frontend
      port: 80

  # API
  - matches:
    - headers:
      - name: ":authority"
        value: "api.torale.ai"
    backendRefs:
    - name: torale-api
      port: 80

  # Docs
  - matches:
    - headers:
      - name: ":authority"
        value: "docs.torale.ai"
    backendRefs:
    - name: torale-docs
      port: 80
```

### Step 3.2: Apply Production HTTPRoute

```bash
kubectl apply -f k8s/gateway/httproute-prod.yaml
```

### Step 3.3: Verify HTTPRoute Attached

```bash
kubectl get httproute torale-prod -n torale
```

---

## Phase 4: Create HTTPRoute for Staging (Cross-Namespace)

### Step 4.1: Create ReferenceGrant

**IMPORTANT:** This allows the Gateway in `torale` namespace to access Services in `torale-staging` namespace.

Save as `k8s/gateway/referencegrant-staging.yaml`:

```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
kind: ReferenceGrant
metadata:
  name: allow-gateway-to-staging
  namespace: torale-staging
spec:
  from:
  - group: gateway.networking.k8s.io
    kind: HTTPRoute
    namespace: torale-staging
  to:
  - group: ""
    kind: Service
```

**What this does:**
- `from.namespace: torale-staging` → HTTPRoutes in torale-staging namespace
- `to.kind: Service` → Can reference Services in torale-staging namespace
- Lives in `torale-staging` namespace (target namespace)

### Step 4.2: Apply ReferenceGrant

```bash
kubectl apply -f k8s/gateway/referencegrant-staging.yaml
```

### Step 4.3: Create Staging HTTPRoute

Save as `k8s/gateway/httproute-staging.yaml`:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: torale-staging
  namespace: torale-staging
spec:
  parentRefs:
  - name: clusterkit-gateway
    namespace: torale  # ← Cross-namespace reference to Gateway
  hostnames:
  - "staging.torale.ai"
  - "api.staging.torale.ai"
  rules:
  # Main site
  - matches:
    - headers:
      - name: ":authority"
        value: "staging.torale.ai"
    backendRefs:
    - name: torale-frontend
      port: 80

  # API
  - matches:
    - headers:
      - name: ":authority"
        value: "api.staging.torale.ai"
    backendRefs:
    - name: torale-api
      port: 80
```

### Step 4.4: Apply Staging HTTPRoute

```bash
kubectl apply -f k8s/gateway/httproute-staging.yaml
```

---

## Phase 5: Testing (Parallel to Existing Ingress)

### Step 5.1: Verify Gateway IP

```bash
kubectl get gateway clusterkit-gateway -n torale -o jsonpath='{.status.addresses[0].value}'
```

**Expected:** `34.149.49.202`

### Step 5.2: Test Production Routes

```bash
# Test torale.ai
curl -I https://torale.ai --resolve torale.ai:443:34.149.49.202

# Test api.torale.ai
curl -I https://api.torale.ai --resolve api.torale.ai:443:34.149.49.202

# Test docs.torale.ai
curl -I https://docs.torale.ai --resolve docs.torale.ai:443:34.149.49.202
```

**Expected:** HTTP 200 with valid TLS

### Step 5.3: Test Staging Routes

```bash
# Test staging.torale.ai
curl -I https://staging.torale.ai --resolve staging.torale.ai:443:34.149.49.202

# Test api.staging.torale.ai
curl -I https://api.staging.torale.ai --resolve api.staging.torale.ai:443:34.149.49.202
```

**Expected:** HTTP 200 with valid TLS

### Step 5.4: Verify SSL Certificates Provisioned

```bash
gcloud compute ssl-certificates describe torale-prod-cert --global
gcloud compute ssl-certificates describe torale-staging-cert --global
```

**Look for:**
```
managed:
  status: ACTIVE
```

**Note:** Certificates may take 30-60 minutes to provision after DNS is pointed to the Gateway IP.

---

## Phase 6: Cutover (DNS Update)

**IMPORTANT:** Only proceed when SSL certificates show `status: ACTIVE`

### Step 6.1: Update Cloudflare DNS for Staging

Go to Cloudflare DNS dashboard and update:

| Record | Old Value | New Value |
|--------|-----------|-----------|
| `staging.torale.ai` | `35.186.213.216` | `34.149.49.202` |
| `api.staging.torale.ai` | `35.186.213.216` | `34.149.49.202` |

**DNS propagation:** 1-5 minutes

### Step 6.2: Verify Staging Works on New IP

```bash
# Wait 2-3 minutes, then test
curl -I https://staging.torale.ai
curl -I https://api.staging.torale.ai
```

**Expected:** HTTP 200 with valid TLS (no certificate errors)

### Step 6.3: Monitor Traffic

**Production should be unaffected** (still using same IP `34.149.49.202`)

```bash
# Check Gateway status
kubectl describe gateway clusterkit-gateway -n torale

# Check HTTPRoute status
kubectl get httproute -A
```

---

## Phase 7: Cleanup

### Step 7.1: Delete Old Staging Ingress

**WAIT at least 24 hours** to ensure everything works.

```bash
# In your torale repo
kubectl delete ingress torale -n torale-staging
kubectl delete managedcertificate torale-staging-cert -n torale-staging
```

### Step 7.2: Delete Staging Static IP (ClusterKit Repo)

```bash
cd terraform/projects/torale
terraform apply  # We already removed the module earlier
```

**Expected:**
```
Plan: 0 to add, 0 to change, 1 to destroy.
  - module.static_ip_staging.google_compute_global_address.ingress
```

**Savings:** £5/month (£60/year)

### Step 7.3: Optional - Delete Production Ingress

**After staging proven stable** (1-2 weeks), delete production Ingress:

```bash
# In your torale repo
kubectl delete ingress torale -n torale
kubectl delete managedcertificate torale-cert -n torale
```

---

## Rollback Procedure

### If Issues During Testing (Before DNS Update)

**Simple:** Just delete the Gateway resources, old Ingresses still work:

```bash
kubectl delete gateway clusterkit-gateway -n torale
kubectl delete httproute torale-prod -n torale
kubectl delete httproute torale-staging -n torale-staging
kubectl delete referencegrant allow-gateway-to-staging -n torale-staging

# Delete SSL certificates
gcloud compute ssl-certificates delete torale-prod-cert --global
gcloud compute ssl-certificates delete torale-staging-cert --global
```

### If Issues After DNS Update

1. **Revert DNS in Cloudflare:**
   - Change staging domains back to `35.186.213.216`
   - Wait 5 minutes for propagation

2. **Keep both systems running** until diagnosis complete

3. **Delete Gateway resources** (see above)

---

## Troubleshooting

### Gateway Shows PROGRAMMED: False

**Check:**
```bash
kubectl describe gateway clusterkit-gateway -n torale
```

**Common issues:**
- Static IP already in use by old Ingress (delete Ingress first, or use different IP for testing)
- Invalid SSL certificate reference
- Gateway class not available

### HTTPRoute Not Attaching

**Check:**
```bash
kubectl describe httproute torale-staging -n torale-staging
```

**Common issues:**
- ReferenceGrant missing or incorrect namespace
- Gateway `allowedRoutes` doesn't permit the namespace
- Service doesn't exist in referenced namespace

### SSL Certificates Stuck in PROVISIONING

**Requirements:**
- DNS must point to Gateway IP (34.149.49.202)
- Gateway must be PROGRAMMED
- Port 443 must be open
- Can take 30-60 minutes

**Check:**
```bash
gcloud compute ssl-certificates describe torale-prod-cert --global --format="value(managed.status,managed.domainStatus)"
```

### Cross-Namespace Routing Not Working

**Verify ReferenceGrant:**
```bash
kubectl get referencegrant -n torale-staging
kubectl describe referencegrant allow-gateway-to-staging -n torale-staging
```

**Verify HTTPRoute can see services:**
```bash
kubectl get svc -n torale-staging
```

---

## Summary

**Before:**
- 2 Ingresses (torale, torale-staging)
- 2 Static IPs (£10/month)
- 2 ManagedCertificate CRDs

**After:**
- 1 Gateway (torale namespace)
- 1 Static IP (£5/month) ← **£60/year savings**
- 2 HTTPRoutes (one per namespace)
- 1 ReferenceGrant (staging namespace)
- 2 Google-managed SSL certificates

**Benefits:**
- ✅ Save £5/month
- ✅ Namespace isolation maintained
- ✅ Future-proof (Gateway API is the standard)
- ✅ Automatic SSL renewal
- ✅ One IP for all domains

---

## References

- [GKE Gateway API Documentation](https://cloud.google.com/kubernetes-engine/docs/concepts/gateway-api)
- [Cross-Namespace Routing Guide](https://gateway-api.sigs.k8s.io/guides/multiple-ns/)
- [Secure a Gateway with SSL](https://cloud.google.com/kubernetes-engine/docs/how-to/secure-gateway)
- [ReferenceGrant Specification](https://gateway-api.sigs.k8s.io/api-types/referencegrant/)
