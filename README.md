# ClusterKit

**Cost-effective serverless Kubernetes platform for personal projects on GKE Autopilot with Cloudflare.**

Automates setup of:
- GKE Autopilot cluster (pay only for running pods)
- Knative Serving (scale-to-zero serverless containers)
- Automatic TLS certificates (Let's Encrypt)
- Automatic DNS (Cloudflare)
- NGINX Ingress Controller

## Quick Start

### Prerequisites

- Google Cloud account with billing enabled
- Cloudflare account with a domain
- Tools installed:
  - `gcloud` (Google Cloud CLI)
  - `kubectl` (Kubernetes CLI)
  - `terraform` (Infrastructure as Code)
  - `helm` (Kubernetes package manager - **required for all components**)

### 1. Install ClusterKit CLI

```bash
cd cli
go build -o clusterkit ./cmd/clusterkit
sudo mv clusterkit /usr/local/bin/
```

### 2. Create GCP Project

```bash
# Login to Google Cloud
gcloud auth login

# Create a new project (or use existing)
gcloud projects create YOUR_PROJECT_ID --name="ClusterKit"

# Set as current project
gcloud config set project YOUR_PROJECT_ID

# Enable billing for the project
# Go to: https://console.cloud.google.com/billing
# Link your billing account to the project

# Enable required APIs
gcloud services enable container.googleapis.com
gcloud services enable compute.googleapis.com
gcloud services enable servicenetworking.googleapis.com
```

### 3. Get Cloudflare API Token

**Detailed Steps:**

1. Login to Cloudflare: https://dash.cloudflare.com
2. Navigate to API Tokens:
   - Click your profile icon (top right)
   - Select "My Profile"
   - Click "API Tokens" in left sidebar
   - OR go directly to: https://dash.cloudflare.com/profile/api-tokens
3. Create Token:
   - Click "Create Token" button
   - Find "Edit zone DNS" template
   - Click "Use template"
4. Configure Permissions:
   - **Permissions**: Zone → DNS → Edit (should be pre-filled)
   - **Zone Resources**: Select "Include → Specific zone → [your-domain.com]"
   - **TTL**: Leave default or set custom expiration
5. Create and Copy:
   - Click "Continue to summary"
   - Click "Create Token"
   - **Copy the token immediately** (won't be shown again)
   - Save it securely (password manager recommended)

**Test Your Token:**

```bash
curl -X GET "https://api.cloudflare.com/client/v4/zones" \
  -H "Authorization: Bearer YOUR_TOKEN_HERE" \
  -H "Content-Type: application/json"
```

You should see JSON response with your zones.

### 4. Bootstrap Cluster

```bash
clusterkit bootstrap \
  --project-id=YOUR_PROJECT_ID \
  --region=us-central1 \
  --cluster-name=clusterkit \
  --domain=yourdomain.com \
  --cloudflare-token=YOUR_TOKEN
```

This takes ~15-20 minutes and uses **Helm** to install:
- GKE Autopilot cluster (Terraform)
- Knative Serving (Helm: knative/serving)
- NGINX Ingress Controller (Helm: ingress-nginx/ingress-nginx)
- cert-manager (Helm: jetstack/cert-manager)
- ExternalDNS (Helm: bitnami/external-dns)
- Configures automatic TLS and DNS

### 5. Deploy Your First App

```bash
# Use example manifests
kubectl apply -f examples/manifests/static-site.yaml

# Or create your own
cat <<EOF | kubectl apply -f -
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: myapp
  namespace: default
spec:
  template:
    spec:
      containers:
      - image: gcr.io/YOUR_PROJECT/myapp:latest
        env:
        - name: PORT
          value: "8080"
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: myapp
  namespace: default
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    external-dns.alpha.kubernetes.io/hostname: "myapp.yourdomain.com"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - myapp.yourdomain.com
    secretName: myapp-tls
  rules:
  - host: myapp.yourdomain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: myapp
            port:
              number: 80
EOF
```

Your app will be available at `https://myapp.yourdomain.com` with automatic TLS!

## How It Works

### Architecture

```
┌─────────────────┐
│   Cloudflare    │  DNS: myapp.yourdomain.com → LoadBalancer IP
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  NGINX Ingress  │  Routes traffic, terminates TLS
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Knative Service │  Scales pods 0→N based on traffic
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   Your App      │  Container running on GKE Autopilot
└─────────────────┘
```

### Cost Optimization

**GKE Autopilot:**
- Pay only for running pods (not idle cluster)
- No node management fees
- Automatically rightsizes resources

**Knative Scale-to-Zero:**
- Apps scale to 0 when not in use
- Save money on idle services
- Scale up on first request (~2s cold start)

**Example Monthly Costs:**
- Small app (rarely used): $5-10/month
- Medium app (moderate traffic): $20-40/month
- Multiple small apps: ~$15-30/month total

Compare to: Cloud Run ($0-5/month per service but limited) or traditional GKE ($75+/month minimum)

## Example Applications

See `examples/` directory:

### Static Site
```bash
kubectl apply -f examples/manifests/static-site.yaml
```
- Nginx serving static files
- Scale-to-zero capable
- Automatic TLS at demo.yourdomain.com

### Go API
```bash
kubectl apply -f examples/manifests/api.yaml
```
- REST API with health checks
- CORS enabled
- Automatic TLS at api.yourdomain.com

### Custom App
Use examples as templates:
1. Copy manifest
2. Change image, domain, environment variables
3. `kubectl apply -f your-app.yaml`

## Operations

### Add Additional Domains

After initial setup, you can add more domains to your cluster:

**Option 1: Add Subdomain of Existing Domain**

Just create a new Ingress with the subdomain:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: another-app
  namespace: default
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    external-dns.alpha.kubernetes.io/hostname: "another.yourdomain.com"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - another.yourdomain.com
    secretName: another-app-tls
  rules:
  - host: another.yourdomain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: another-app
            port:
              number: 80
EOF
```

ExternalDNS will automatically create the DNS record in Cloudflare!

**Option 2: Add Completely Different Domain**

1. **Update Cloudflare API Token**:
   - Go to https://dash.cloudflare.com/profile/api-tokens
   - Edit your existing token
   - Under "Zone Resources", add the new domain
   - Or create a new token with both domains

2. **Update ExternalDNS** (if using new token):
   ```bash
   kubectl create secret generic cloudflare-api-token \
     --from-literal=apiToken=YOUR_NEW_TOKEN \
     -n external-dns \
     --dry-run=client -o yaml | kubectl apply -f -

   # Restart ExternalDNS to pick up new token
   kubectl rollout restart deployment external-dns -n external-dns
   ```

3. **Deploy app with new domain**:
   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: networking.k8s.io/v1
   kind: Ingress
   metadata:
     name: app-on-new-domain
     namespace: default
     annotations:
       cert-manager.io/cluster-issuer: "letsencrypt-prod"
       external-dns.alpha.kubernetes.io/hostname: "app.newdomain.com"
   spec:
     ingressClassName: nginx
     tls:
     - hosts:
       - app.newdomain.com
       secretName: app-newdomain-tls
     rules:
     - host: app.newdomain.com
       http:
         paths:
         - path: /
           pathType: Prefix
           backend:
             service:
               name: my-app
               port:
                 number: 80
   EOF
   ```

4. **Verify DNS and TLS**:
   ```bash
   # Check DNS record was created
   kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns

   # Check TLS certificate was issued
   kubectl get certificate
   kubectl describe certificate app-newdomain-tls
   ```

**Note**: The NGINX Ingress LoadBalancer IP is shared across all domains. ExternalDNS will point all domains to the same IP, and NGINX routes based on hostname.

### View Services
```bash
kubectl get ksvc
kubectl get ingress
kubectl get certificate
```

### View Logs
```bash
kubectl logs -l serving.knative.dev/service=myapp -f
```

### Check DNS
```bash
kubectl logs -n external-dns -l app=external-dns
```

### Check TLS Certificates
```bash
kubectl get certificate
kubectl describe certificate myapp-tls
```

### Scale Configuration

Edit your Knative Service:
```yaml
metadata:
  annotations:
    autoscaling.knative.dev/minScale: "0"    # Minimum pods (0 = scale to zero)
    autoscaling.knative.dev/maxScale: "10"   # Maximum pods
    autoscaling.knative.dev/target: "10"     # Concurrent requests per pod
```

### Troubleshooting

```bash
# Check cluster
kubectl get nodes
kubectl get pods --all-namespaces

# Check components
clusterkit troubleshoot

# View component logs
kubectl logs -n knative-serving -l app=controller
kubectl logs -n ingress-nginx -l app.kubernetes.io/name=ingress-nginx
kubectl logs -n cert-manager -l app=cert-manager
kubectl logs -n external-dns -l app=external-dns
```

## Cost Management

### Monitor Costs
```bash
gcloud billing accounts list
gcloud billing projects describe YOUR_PROJECT_ID
```

### Reduce Costs
1. Use scale-to-zero (minScale: 0) for all apps
2. Set appropriate maxScale limits
3. Use small resource requests
4. Delete unused apps

### Destroy Cluster
```bash
cd terraform
terraform destroy \
  -var="project_id=YOUR_PROJECT_ID" \
  -var="region=us-central1" \
  -var="cluster_name=clusterkit"
```

## Project Structure

```
.
├── terraform/           # GKE infrastructure (Terraform)
├── cli/                # ClusterKit CLI (Go)
│   ├── cmd/            # CLI commands
│   └── pkg/            # Core packages
│       └── bootstrap/  # Bootstrap orchestration
│           └── components/ # Helm-based component installers
└── examples/           # Example applications
```

**All Kubernetes components installed via Helm:**
- Knative: `helm repo add knative https://knative.github.io/helm-charts`
- NGINX Ingress: `helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx`
- cert-manager: `helm repo add jetstack https://charts.jetstack.io`
- ExternalDNS: `helm repo add bitnami https://charts.bitnami.com/bitnami`

## FAQ

**Q: Why Helm for everything?**
A: Helm provides consistent package management, easy upgrades, and declarative configuration. All components can be managed with `helm list` and upgraded with `helm upgrade`.

**Q: Why GKE Autopilot vs Cloud Run?**
A: More flexibility (bring your own Dockerfile, persistent storage, full k8s API) with similar serverless experience.

**Q: Why Knative vs regular Deployments?**
A: Scale-to-zero saves money. Apps you don't use don't cost anything.

**Q: How much does this cost?**
A: $5-50/month depending on usage. Much cheaper than traditional GKE cluster.

**Q: Can I use multiple domains?**
A: Yes! ExternalDNS (via Helm) supports unlimited domains. Just add Ingress resources with different hostnames.

**Q: Can I add databases?**
A: Yes, deploy StatefulSets or use Helm charts (e.g., `helm install postgresql bitnami/postgresql`).

## License

MIT
