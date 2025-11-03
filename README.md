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
- Tools installed: `gcloud`, `kubectl`, `terraform`, `helm`

### 1. Install ClusterKit CLI

```bash
cd cli
go build -o clusterkit ./cmd/clusterkit
sudo mv clusterkit /usr/local/bin/
```

### 2. Configure GCP

```bash
gcloud auth login
gcloud config set project YOUR_PROJECT_ID
gcloud services enable container.googleapis.com
gcloud services enable compute.googleapis.com
```

### 3. Get Cloudflare API Token

1. Go to Cloudflare Dashboard → My Profile → API Tokens
2. Create Token with permissions:
   - Zone → DNS → Edit
   - Zone → Zone → Read
3. Copy the token

### 4. Bootstrap Cluster

```bash
clusterkit bootstrap \
  --project-id=YOUR_PROJECT_ID \
  --region=us-central1 \
  --cluster-name=clusterkit \
  --domain=yourdomain.com \
  --cloudflare-token=YOUR_TOKEN
```

This takes ~15-20 minutes and:
- Creates GKE Autopilot cluster with Terraform
- Installs Knative, Ingress, cert-manager, ExternalDNS
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
├── terraform/           # GKE infrastructure
├── k8s/                # Kubernetes manifests
│   ├── knative/        # Knative Serving
│   ├── nginx-ingress/  # NGINX Ingress
│   ├── cert-manager/   # TLS certificates
│   └── external-dns/   # Cloudflare DNS
├── cli/                # ClusterKit CLI
└── examples/           # Example applications
```

## FAQ

**Q: Why GKE Autopilot vs Cloud Run?**
A: More flexibility (bring your own Dockerfile, persistent storage, full k8s API) with similar serverless experience.

**Q: Why Knative vs regular Deployments?**
A: Scale-to-zero saves money. Apps you don't use don't cost anything.

**Q: How much does this cost?**
A: $5-50/month depending on usage. Much cheaper than traditional GKE cluster.

**Q: Can I use multiple domains?**
A: Yes, just add more Ingress resources with different hostnames.

**Q: Can I add databases?**
A: Yes, deploy StatefulSets. See examples/manifests for reference.

## License

MIT
