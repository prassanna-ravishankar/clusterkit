# Cloudflare API Token Setup Guide

This guide walks you through creating a Cloudflare API token for ExternalDNS.

## Why API Token (not API Key)

- **API Tokens** are scoped with specific permissions (recommended)
- **API Keys** have full account access (not recommended)

ExternalDNS only needs:
- **Zone:Zone:Read** - Read zone information
- **Zone:DNS:Edit** - Create/update/delete DNS records

## Step-by-Step Instructions

### 1. Access Cloudflare Dashboard

Go to: https://dash.cloudflare.com/profile/api-tokens

### 2. Create Token

Click **"Create Token"** button

### 3. Choose Template or Custom

**Option A: Use Template (Recommended)**
1. Find "Edit zone DNS" template
2. Click **"Use template"**
3. Skip to step 4

**Option B: Create Custom Token**
1. Click **"Create Custom Token"**
2. Set Token name: `clusterkit-external-dns`
3. Configure Permissions:
   ```
   Zone - Zone - Read
   Zone - DNS - Edit
   ```

### 4. Configure Zone Resources

Under "Zone Resources":

**Option A: All Zones**
```
Include - All zones
```

**Option B: Specific Zone (Recommended)**
```
Include - Specific zone - example.com
```

This limits the token to only manage DNS for your domain.

### 5. Configure IP Filtering (Optional)

For additional security, restrict token usage to cluster IPs:

```
Client IP Address Filtering
  Is in - YOUR_CLUSTER_EXTERNAL_IP/32
```

Get cluster IP:
```bash
kubectl get svc ingress-nginx-controller -n ingress-nginx -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
```

### 6. Set TTL (Optional)

```
TTL: Start - [Today]
     End - [Leave blank for no expiration]
```

Or set expiration date for token rotation.

### 7. Review and Create

1. Review permissions summary
2. Click **"Continue to summary"**
3. Click **"Create Token"**

### 8. Copy Token

**IMPORTANT**: Copy the token immediately! You won't be able to see it again.

```
YOUR_TOKEN_WILL_LOOK_LIKE_THIS_aBcD1234eFgH5678iJkL
```

### 9. Test Token (Optional)

Test the token works:

```bash
# Set token
export CF_API_TOKEN='your-token-here'

# List zones
curl -X GET "https://api.cloudflare.com/client/v4/zones" \
  -H "Authorization: Bearer ${CF_API_TOKEN}" \
  -H "Content-Type: application/json"

# Should return your zones without errors
```

### 10. Use Token with ExternalDNS

```bash
# Set environment variable
export CLOUDFLARE_API_TOKEN='your-token-here'

# Run install script
cd k8s/external-dns
./install.sh
```

Or create secret manually:

```bash
kubectl create namespace external-dns

kubectl create secret generic cloudflare-api-token \
  --from-literal=apiToken="${CLOUDFLARE_API_TOKEN}" \
  --namespace=external-dns
```

## Token Permissions Summary

Minimum required permissions:

| Permission | Access | Reason |
|------------|--------|--------|
| Zone:Zone:Read | Read | List zones and zone details |
| Zone:DNS:Edit | Edit | Create, update, delete DNS records |

## Security Best Practices

1. **Scope to specific zones** - Don't use "All zones" unless necessary
2. **Use IP filtering** - Restrict to cluster IPs if possible
3. **Set expiration** - Rotate tokens periodically (e.g., every 90 days)
4. **Don't commit to git** - Never commit the token to version control
5. **Use Kubernetes Secrets** - Store in cluster, not in config files
6. **Monitor usage** - Check Cloudflare audit logs regularly

## Troubleshooting

### "Invalid API Token" Error

Check:
1. Token was copied correctly (no extra spaces)
2. Token has required permissions
3. Token hasn't expired
4. Zone ID is correct

### "Authentication Error"

The token might need both:
- `Zone:Zone:Read` (to list zones)
- `Zone:DNS:Edit` (to manage records)

### "Zone Not Found"

If using zone-specific token, ensure:
- Correct zone is configured in token
- domain-filter in ExternalDNS matches zone

## Token Rotation

To rotate tokens:

1. Create new token (follow steps above)
2. Update Kubernetes secret:
   ```bash
   kubectl create secret generic cloudflare-api-token \
     --from-literal=apiToken="NEW_TOKEN_HERE" \
     --namespace=external-dns \
     --dry-run=client -o yaml | kubectl apply -f -
   ```
3. Restart ExternalDNS:
   ```bash
   kubectl rollout restart deployment/external-dns -n external-dns
   ```
4. Verify it works
5. Delete old token from Cloudflare dashboard

## Reference

- [Cloudflare API Token Documentation](https://developers.cloudflare.com/fundamentals/api/get-started/create-token/)
- [ExternalDNS Cloudflare Provider](https://github.com/kubernetes-sigs/external-dns/blob/master/docs/tutorials/cloudflare.md)
