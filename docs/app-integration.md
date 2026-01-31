# Application Integration Guide

**Quick guide for deploying your application to ClusterKit.**

## What You Need to Know

ClusterKit provides:
- **Shared Gateway**: Load balancer at `34.149.49.202` (all apps use this IP)
- **Automatic SSL**: Cloudflare Origin CA wildcard certs — new subdomains work instantly
- **Automatic DNS**: ExternalDNS creates proxied Cloudflare DNS records from your HTTPRoutes
- **Cloudflare CDN/WAF**: All traffic routed through Cloudflare by default
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
  namespace: clusterkit  # MUST be 'clusterkit' (Gateway namespace)
  annotations:
    external-dns.alpha.kubernetes.io/cloudflare-proxied: "true"  # Required for Cloudflare CDN/WAF
spec:
  parentRefs:
  - name: clusterkit-gateway  # Shared Gateway
    namespace: clusterkit
  hostnames:
  - "myapp.yourdomain.com"  # Your domain
  rules:
  - backendRefs:
    - name: myapp-service    # Your Service name
      namespace: myapp       # Your app's namespace (cross-namespace ref)
      port: 80               # Your Service port
```

**For staging** (same pattern, different service namespace):
```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: myapp-staging
  namespace: clusterkit  # HTTPRoute in clusterkit namespace
  annotations:
    external-dns.alpha.kubernetes.io/cloudflare-proxied: "true"
spec:
  parentRefs:
  - name: clusterkit-gateway
    namespace: clusterkit
  hostnames:
  - "staging.myapp.yourdomain.com"
  rules:
  - backendRefs:
    - name: myapp-service
      namespace: myapp-staging  # Service in staging namespace
      port: 80
```

## Deployment Checklist

### Before You Deploy

1. **Confirm your domain has an Origin CA cert** in the Gateway
   - Existing domains (torale.ai, bananagraph.com, a2aregistry.org, repowire.io) already have wildcard certs
   - New domains: contact ClusterKit team to add domain to `origin_ca_domains` and run `terraform apply`

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
   kubectl describe httproute myapp-prod -n clusterkit

   # Should show: "Accepted: True"
   ```

4. **Wait for DNS** (~2-5 minutes):
   ```bash
   # Check ExternalDNS created the record
   kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns | grep myapp

   # Verify DNS resolves (returns Cloudflare IPs since records are proxied)
   dig +short myapp.yourdomain.com @1.1.1.1
   ```

5. **Test your app**:
   ```bash
   curl https://myapp.yourdomain.com
   ```

## Common Issues

### HTTPRoute not attaching
- **Issue**: `kubectl describe httproute` shows `Accepted: False`
- **Fix**: Ensure `namespace: clusterkit` in HTTPRoute metadata
- **Fix**: Ensure Gateway name is `clusterkit-gateway`

### SSL certificate warning
- **Issue**: Browser shows cert warning
- **Check**: Is Cloudflare SSL mode set to "Full (Strict)" for the zone?
- **Check**: Does the Gateway have an Origin CA cert for this domain?

### DNS not resolving
- **Issue**: `dig myapp.yourdomain.com` returns nothing
- **Check**: ExternalDNS logs for errors
  ```bash
  kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns
  ```
- **Fix**: Verify HTTPRoute is accepted by the Gateway

## Need Help?

1. **Check Gateway status**:
   ```bash
   kubectl get gateway clusterkit-gateway -n clusterkit
   ```
   Should show `PROGRAMMED: True` and `ADDRESS: 34.149.49.202`

2. **Check HTTPRoute status**:
   ```bash
   kubectl describe httproute <your-route> -n clusterkit
   ```
   Look for `Accepted: True` under Conditions

3. **Contact ClusterKit team** with:
   - Your app name
   - HTTPRoute name
   - Domain you're trying to use
   - Error messages from `kubectl describe`

## Reference

- Gateway name: `clusterkit-gateway`
- Gateway namespace: `clusterkit`
- Gateway IP: `34.149.49.202`
- HTTPRoute namespace: `clusterkit` (all apps, centralized routing)
- SSL: Cloudflare Origin CA wildcard certs (Full Strict mode)
- DNS: ExternalDNS creates proxied records from the `cloudflare-proxied: "true"` annotation

For operational/maintenance questions, see `docs/maintenance.md`.
