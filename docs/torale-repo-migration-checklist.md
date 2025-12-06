# Torale Repo Migration Checklist

Use this checklist to migrate the torale application repository from Ingress to Gateway API.

## Pre-Migration

- [ ] Backup current Ingress manifests
- [ ] Document current DNS records in Cloudflare
- [ ] Verify Gateway is running in ClusterKit:
  ```bash
  kubectl get gateway clusterkit-gateway -n torale
  ```
- [ ] Verify SSL certificates are active:
  ```bash
  gcloud compute ssl-certificates list
  ```

## Migration Steps

### 1. Create HTTPRoute Manifests

- [ ] Create `manifests/httproutes/` directory
- [ ] Convert production Ingress → HTTPRoute:
  - [ ] `torale.ai` → `torale-prod-main.yaml`
  - [ ] `api.torale.ai` → `torale-prod-api.yaml`
  - [ ] `docs.torale.ai` → `torale-prod-docs.yaml`
- [ ] Convert staging Ingress → HTTPRoute:
  - [ ] `staging.torale.ai` → `torale-staging-main.yaml`
  - [ ] `api.staging.torale.ai` → `torale-staging-api.yaml`
- [ ] Add `cloudflare-proxied: false` annotation to ALL HTTPRoutes
- [ ] For staging routes: Add `namespace: torale-staging` to backendRefs

### 2. Update CI/CD Pipeline

- [ ] Remove Ingress deployment steps
- [ ] Add HTTPRoute deployment:
  ```yaml
  kubectl apply -f manifests/httproutes/
  ```
- [ ] Add safety check:
  ```bash
  kubectl get gateway clusterkit-gateway -n torale || exit 1
  ```
- [ ] Test CI pipeline in staging first

### 3. Deploy HTTPRoutes

- [ ] Deploy to staging first:
  ```bash
  kubectl apply -f manifests/httproutes/staging.yaml
  ```
- [ ] Verify staging HTTPRoute attached:
  ```bash
  kubectl describe httproute torale-staging-main -n torale
  ```
- [ ] Test staging URLs work
- [ ] Deploy to production:
  ```bash
  kubectl apply -f manifests/httproutes/production.yaml
  ```
- [ ] Verify production HTTPRoutes attached
- [ ] Test production URLs work

### 4. Cleanup Old Resources

- [ ] Delete old Ingress manifests from repo
- [ ] Remove ManagedCertificate manifests (ClusterKit manages these)
- [ ] Update README to reference Gateway API
- [ ] Commit and push changes

### 5. Verify Everything Works

- [ ] Test all production URLs:
  - [ ] https://torale.ai
  - [ ] https://api.torale.ai
  - [ ] https://docs.torale.ai
- [ ] Test all staging URLs:
  - [ ] https://staging.torale.ai
  - [ ] https://api.staging.torale.ai
- [ ] Check SSL certificates (no warnings)
- [ ] Verify Cloudflare DNS records are gray cloud (DNS-only)
- [ ] Check ExternalDNS created DNS records:
  ```bash
  kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns | grep torale
  ```

## Post-Migration

- [ ] Monitor application logs for any issues
- [ ] Update team documentation
- [ ] Archive old Ingress manifests (don't delete yet)
- [ ] Schedule review after 1 week

## Rollback Plan (If Needed)

If something goes wrong:

1. **Redeploy old Ingresses:**
   ```bash
   kubectl apply -f manifests/ingress-backup/
   ```

2. **Recreate staging static IP:**
   ```bash
   cd terraform/projects/torale
   # Uncomment the static IP module
   terraform apply
   ```

3. **Update Cloudflare DNS** back to old IPs

4. **Delete HTTPRoutes:**
   ```bash
   kubectl delete httproute --all -n torale
   ```

## Notes

- HTTPRoutes for both prod and staging live in `torale` namespace
- Staging HTTPRoutes use cross-namespace service references
- DNS is managed automatically by ExternalDNS
- SSL certificates are managed by ClusterKit Terraform

## Reference

See `docs/torale-repo-integration.md` for detailed integration guide.
