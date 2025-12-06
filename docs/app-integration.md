# Application Integration Guide

**Quick guide for deploying your application to ClusterKit.**

## What You Need to Know

ClusterKit provides:
- **Shared Gateway**: Load balancer at `34.149.49.202` (all apps use this IP)
- **Automatic SSL**: Google-managed certificates for HTTPS
- **Automatic DNS**: ExternalDNS creates Cloudflare DNS records from your manifests
- **Cross-namespace routing**: Production and staging can share the Gateway

## Required Resources

Every application needs 3 Kubernetes resources:

1. **Deployment** - Your application pods
2. **Service** - ClusterIP exposing your pods
3. **HTTPRoute** - Routing rules (hostname → service)

## HTTPRoute Template

Copy and customize this template for your app:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: myapp-prod
  namespace: torale  # MUST be 'torale' (Gateway namespace)
  annotations:
    # REQUIRED: Disable Cloudflare proxy for GCP SSL to work
    external-dns.alpha.kubernetes.io/cloudflare-proxied: "false"
spec:
  parentRefs:
  - name: clusterkit-gateway  # Shared Gateway
    namespace: torale
  hostnames:
  - "myapp.yourdomain.com"  # Your domain
  rules:
  - backendRefs:
    - name: myapp-service  # Your Service name
      port: 80             # Your Service port
```

**For staging** (cross-namespace):
```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: myapp-staging
  namespace: torale  # HTTPRoute in torale namespace
  annotations:
    external-dns.alpha.kubernetes.io/cloudflare-proxied: "false"
spec:
  parentRefs:
  - name: clusterkit-gateway
    namespace: torale
  hostnames:
  - "staging.myapp.yourdomain.com"
  rules:
  - backendRefs:
    - name: myapp-service
      namespace: myapp-staging  # Service in different namespace
      port: 80
```

## Deployment Checklist

### Before You Deploy

1. **Request domain in SSL certificate**
   - Contact ClusterKit team to add your subdomain to SSL certificate
   - Wait for confirmation (cert provisioning takes ~15 min)

### Deploy Your App

2. **Create manifests**:
   ```bash
   kubectl apply -f deployment.yaml
   kubectl apply -f service.yaml
   kubectl apply -f httproute.yaml
   ```

3. **Verify deployment**:
   ```bash
   # Check HTTPRoute attached to Gateway
   kubectl describe httproute myapp-prod -n torale

   # Should show: "Accepted: True"
   ```

4. **Wait for DNS** (~2-5 minutes):
   ```bash
   # Check ExternalDNS created the record
   kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns | grep myapp

   # Verify DNS resolves
   dig +short myapp.yourdomain.com @1.1.1.1
   # Should return: 34.149.49.202
   ```

5. **Test your app**:
   ```bash
   curl https://myapp.yourdomain.com
   ```

## Common Issues

### HTTPRoute not attaching
- **Issue**: `kubectl describe httproute` shows `Accepted: False`
- **Fix**: Ensure `namespace: torale` in HTTPRoute metadata
- **Fix**: Ensure Gateway name is `clusterkit-gateway`

### SSL certificate warning
- **Issue**: Browser shows "Not Secure" or cert warning
- **Check**: Is your domain in the SSL certificate?
  ```bash
  gcloud compute ssl-certificates describe torale-prod-cert
  ```
- **Fix**: Contact ClusterKit team to add domain to certificate

### DNS not resolving
- **Issue**: `dig myapp.yourdomain.com` returns nothing
- **Check**: ExternalDNS logs for errors
  ```bash
  kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns
  ```
- **Fix**: Verify HTTPRoute has annotation: `cloudflare-proxied: "false"`

### Cloudflare proxy enabled (orange cloud)
- **Issue**: DNS resolves to Cloudflare IPs (104.x.x.x) instead of 34.149.49.202
- **Check**: Cloudflare dashboard shows orange cloud icon
- **Fix**: Ensure HTTPRoute has: `external-dns.alpha.kubernetes.io/cloudflare-proxied: "false"`
- **Fix**: Set Cloudflare DNS record to "DNS only" (gray cloud)

## Need Help?

1. **Check Gateway status**:
   ```bash
   kubectl get gateway clusterkit-gateway -n torale
   ```
   Should show `PROGRAMMED: True` and `ADDRESS: 34.149.49.202`

2. **Check HTTPRoute status**:
   ```bash
   kubectl describe httproute <your-route> -n torale
   ```
   Look for `Accepted: True` under Conditions

3. **Contact ClusterKit team** with:
   - Your app name
   - HTTPRoute name
   - Domain you're trying to use
   - Error messages from `kubectl describe`

## Reference

- Gateway name: `clusterkit-gateway`
- Gateway namespace: `torale`
- Gateway IP: `34.149.49.202`
- HTTPRoute namespace: `torale` (always, even for staging)
- Required annotation: `external-dns.alpha.kubernetes.io/cloudflare-proxied: "false"`

For operational/maintenance questions, see `docs/maintenance.md`.
