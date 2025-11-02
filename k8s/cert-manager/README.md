# cert-manager for ClusterKit

Automatic TLS certificate management using Let's Encrypt with HTTP-01 challenges.

## Features

- **Automatic Certificate Issuance**: Creates TLS certificates automatically via Let's Encrypt
- **HTTP-01 Challenge**: Uses NGINX Ingress for ACME challenge completion
- **Auto-Renewal**: Certificates renew automatically 15 days before expiration
- **Staging & Production**: Separate issuers to avoid rate limits during testing
- **GKE Autopilot Optimized**: Proper resource requests for cost efficiency

## Architecture

```
cert-manager
  ↓
Let's Encrypt ACME Server
  ↓
HTTP-01 Challenge
  ↓
NGINX Ingress (serves challenge)
  ↓
Certificate issued & stored in Secret
  ↓
Ingress uses Secret for TLS
```

## Installation

### Prerequisites

1. GKE Autopilot cluster (Task 1)
2. NGINX Ingress Controller installed (Task 3)
3. Domain with DNS pointing to cluster (or use staging for testing)
4. kubectl and helm installed

### Quick Install

```bash
cd k8s/cert-manager
./install.sh
```

### Update Email Address

**IMPORTANT**: Before using in production, update the email address in ClusterIssuer files:

```bash
# Edit both files
vim clusterissuer-staging.yaml
vim clusterissuer-production.yaml

# Replace:
email: ops@clusterkit.example.com
# With your actual email:
email: your-email@example.com

# Re-apply
kubectl apply -f clusterissuer-staging.yaml
kubectl apply -f clusterissuer-production.yaml
```

## Usage

### Method 1: Certificate Resource (Explicit)

Create a Certificate resource directly:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: my-app-cert
  namespace: default
spec:
  secretName: my-app-tls
  duration: 2160h  # 90 days
  renewBefore: 360h  # Renew 15 days before expiration
  dnsNames:
  - app.example.com
  - www.app.example.com
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
```

### Method 2: Ingress Annotation (Automatic)

Add annotation to Ingress resource:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
  - hosts:
    - app.example.com
    secretName: my-app-tls  # cert-manager creates this automatically
  rules:
  - host: app.example.com
    http:
      # ... your routing rules
```

## Testing

### 1. Test with Staging Issuer

Always test with staging first to avoid rate limits:

```bash
# Deploy test certificate
kubectl apply -f examples/test-certificate.yaml

# Watch certificate status
kubectl describe certificate test-certificate

# Check for ready status
kubectl get certificate test-certificate

# View ACME challenge progress
kubectl get challenges
kubectl get orders
```

### 2. Verify Certificate Details

```bash
# Get certificate secret
kubectl get secret test-tls-cert -o yaml

# Decode and view certificate
kubectl get secret test-tls-cert -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -text -noout
```

### 3. Test with Real Ingress

```bash
# Deploy sample ingress with TLS
kubectl apply -f examples/ingress-with-tls.yaml

# Check certificate creation
kubectl get certificate -A

# Verify ingress has TLS
kubectl describe ingress example-ingress-tls
```

## ClusterIssuers

### Staging (letsencrypt-staging)

- **Use for**: Testing, development
- **Rate Limits**: 30,000 certs/week
- **Trust**: Not trusted by browsers (will show warning)
- **Server**: https://acme-staging-v02.api.letsencrypt.org/directory

```bash
kubectl get clusterissuer letsencrypt-staging
```

### Production (letsencrypt-prod)

- **Use for**: Production deployments
- **Rate Limits**: 50 certs/domain/week
- **Trust**: Trusted by all browsers
- **Server**: https://acme-v02.api.letsencrypt.org/directory

```bash
kubectl get clusterissuer letsencrypt-prod
```

## Integration with Knative

Knative Services can use cert-manager for automatic TLS:

```yaml
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: my-knative-app
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  template:
    metadata:
      annotations:
        # Knative annotations
    spec:
      containers:
      - image: myapp:latest
```

Or create a Certificate separately:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: knative-app-cert
spec:
  secretName: knative-app-tls
  dnsNames:
  - app.example.com
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
```

## Monitoring

### Check cert-manager Logs

```bash
# Controller logs
kubectl logs -n cert-manager -l app=cert-manager --tail=100 -f

# Webhook logs
kubectl logs -n cert-manager -l app=webhook --tail=100 -f

# CA injector logs
kubectl logs -n cert-manager -l app=cainjector --tail=100 -f
```

### Check Certificate Status

```bash
# List all certificates
kubectl get certificates -A

# Detailed certificate info
kubectl describe certificate <name>

# Check events
kubectl get events --sort-by='.lastTimestamp' | grep cert-manager
```

## Troubleshooting

### Certificate Stuck in "Issuing" State

```bash
# Check order status
kubectl get orders -A

# Check challenge status
kubectl get challenges -A

# Describe challenge for details
kubectl describe challenge <challenge-name>

# Common issue: HTTP-01 challenge can't be reached
# Verify ingress and DNS are working
kubectl get ingress -A
```

### HTTP-01 Challenge Fails

1. **Verify DNS**: Ensure domain points to LoadBalancer IP
   ```bash
   dig app.example.com
   kubectl get svc ingress-nginx-controller -n ingress-nginx
   ```

2. **Check HTTP accessibility**:
   ```bash
   curl http://app.example.com/.well-known/acme-challenge/test
   ```

3. **Verify ingress class**:
   ```bash
   kubectl get ingressclass
   # Should show 'nginx' as default
   ```

### Rate Limit Errors

If you hit Let's Encrypt rate limits:

1. Use staging issuer for testing
2. Wait for rate limit window to reset (usually 1 week)
3. Check current limits: https://letsencrypt.org/docs/rate-limits/

### Certificate Not Renewing

```bash
# Check renewal time
kubectl describe certificate <name> | grep "Renewal Time"

# Force renewal (delete secret)
kubectl delete secret <tls-secret-name>
# cert-manager will recreate it
```

## Let's Encrypt Rate Limits

**Production Server**:
- 50 certificates per registered domain per week
- 5 duplicate certificates per week
- 300 new orders per account per 3 hours

**Staging Server**:
- 30,000 certificates per week
- Higher limits for testing

**Best Practices**:
1. Always test with staging first
2. Use wildcard certificates to reduce cert count
3. Monitor certificate usage
4. Plan certificate strategy for multi-domain apps

## Security Considerations

- **Email Privacy**: Email is shared with Let's Encrypt
- **Rate Limits**: Avoid hitting production limits
- **Secret Storage**: TLS secrets stored in Kubernetes Secrets
- **RBAC**: cert-manager has cluster-wide permissions

## Cost Considerations

**Free**:
- Let's Encrypt certificates (no cost)
- Unlimited certificate issuance (within rate limits)

**Resource Cost**:
- cert-manager pods: ~$3-5/month on GKE Autopilot
- Challenge solver pods: Minimal (run only during issuance)

## Next Steps

1. **Update email addresses** in ClusterIssuer files
2. **Test with staging** issuer first
3. **Install ExternalDNS** (Task 5) for automatic DNS
4. **Deploy applications** with automatic TLS

## Resources

- [cert-manager Documentation](https://cert-manager.io/docs/)
- [Let's Encrypt Documentation](https://letsencrypt.org/docs/)
- [Rate Limits](https://letsencrypt.org/docs/rate-limits/)
- [Troubleshooting Guide](https://cert-manager.io/docs/troubleshooting/)
