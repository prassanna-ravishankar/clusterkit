# ClusterKit

**Simplified Kubernetes platform for personal projects on GKE Autopilot with Cloudflare.**

Automates setup of:
- GKE Autopilot cluster (managed Kubernetes)
- GKE Ingress (built-in HTTP(S) load balancer)
- GKE Managed Certificates (automatic TLS)
- ExternalDNS (automatic Cloudflare DNS updates)
- Spot Pods support (60-91% cost savings on workloads)

## Quick Start

### Prerequisites

- Google Cloud account with billing enabled
- Cloudflare account with a domain
- Tools installed:
  - `gcloud` (Google Cloud CLI)
  - `kubectl` (Kubernetes CLI)
  - `terraform` (Infrastructure as Code)
  - `helm` (Kubernetes package manager - required for ExternalDNS)

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

This takes ~10-15 minutes and sets up:
- GKE Autopilot cluster (Terraform)
- ExternalDNS (Helm: bitnami/external-dns)
- GKE Ingress and Managed Certificates are built-in

### 5. Deploy Your First App

```bash
# Use example manifests
kubectl apply -f examples/manifests/static-site.yaml

# Or create your own
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  replicas: 1
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      # Use Spot Pods for cost savings
      nodeSelector:
        cloud.google.com/gke-spot: "true"
      containers:
      - name: app
        image: your-image:latest
        ports:
        - containerPort: 8080
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
---
apiVersion: v1
kind: Service
metadata:
  name: myapp
spec:
  selector:
    app: myapp
  ports:
  - port: 80
    targetPort: 8080
---
apiVersion: networking.gke.io/v1
kind: ManagedCertificate
metadata:
  name: myapp-cert
spec:
  domains:
    - myapp.yourdomain.com
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: myapp
  annotations:
    kubernetes.io/ingress.class: "gce"
    networking.gke.io/managed-certificates: "myapp-cert"
    external-dns.alpha.kubernetes.io/hostname: "myapp.yourdomain.com"
spec:
  defaultBackend:
    service:
      name: myapp
      port:
        number: 80
EOF
```

Your app will be available at `https://myapp.yourdomain.com` with automatic TLS and DNS!

## How It Works

### Architecture

```
┌─────────────────┐
│   Cloudflare    │  DNS: myapp.yourdomain.com → LoadBalancer IP
│   (via External │  (automatically updated)
│       DNS)      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  GKE Ingress    │  Routes traffic, terminates TLS
│  (HTTP(S) LB)   │  (Google-managed certificates)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Your Deployment │  Standard K8s Deployment
│  (Spot Pods)    │  60-91% cost savings
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   Your App      │  Container running on GKE Autopilot
└─────────────────┘
```

### Cost Optimization

**GKE Autopilot:**
- Pay only for running pods (not idle nodes)
- $74.40/month free tier credit (covers cluster management fee)
- Automatically provisions resources on-demand

**Spot Pods:**
- 60-91% discount on pod costs
- Works for fault-tolerant workloads
- Automatic rescheduling on preemption

**GKE Managed Certificates & Ingress:**
- No additional pod overhead (built-in to GKE)
- HTTP(S) Load Balancer: ~$18-20/month (fixed cost)

**Example Monthly Costs:**
- Infrastructure (ExternalDNS): ~$6/month
- Load Balancer: ~$18-20/month
- Applications (Spot Pods): ~$1-5/month
- **Total: ~$25-30/month** for personal projects

Compare to:
- Cloud Run: $0-10/month (but less control)
- Traditional GKE: $75+/month minimum

## Example Applications

See `examples/` directory:

### Static Site
```bash
kubectl apply -f examples/manifests/static-site.yaml
```
- Nginx serving static files
- Runs on Spot Pods (60-91% savings)
- Automatic TLS at demo.yourdomain.com
- Automatic DNS via ExternalDNS

### API
```bash
kubectl apply -f examples/manifests/api.yaml
```
- REST API with health checks
- Runs on Spot Pods
- Automatic TLS at api.yourdomain.com
- Automatic DNS via ExternalDNS

### Custom App
Use examples as templates:
1. Copy manifest (Deployment + Service + ManagedCertificate + Ingress)
2. Change image, domain, resources
3. Add `cloud.google.com/gke-spot: "true"` nodeSelector for cost savings
4. `kubectl apply -f your-app.yaml`

## Operations

### Add Additional Domains

After initial setup, you can add more domains to your cluster:

**Option 1: Add Subdomain of Existing Domain**

Just create a new ManagedCertificate and Ingress with the subdomain:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: networking.gke.io/v1
kind: ManagedCertificate
metadata:
  name: another-app-cert
spec:
  domains:
    - another.yourdomain.com
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: another-app
  annotations:
    kubernetes.io/ingress.class: "gce"
    networking.gke.io/managed-certificates: "another-app-cert"
    external-dns.alpha.kubernetes.io/hostname: "another.yourdomain.com"
spec:
  defaultBackend:
    service:
      name: another-app
      port:
        number: 80
EOF
```

ExternalDNS will automatically create the DNS record in Cloudflare!
GKE will automatically provision the TLS certificate!

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
   apiVersion: networking.gke.io/v1
   kind: ManagedCertificate
   metadata:
     name: app-newdomain-cert
   spec:
     domains:
       - app.newdomain.com
   ---
   apiVersion: networking.k8s.io/v1
   kind: Ingress
   metadata:
     name: app-on-new-domain
     annotations:
       kubernetes.io/ingress.class: "gce"
       networking.gke.io/managed-certificates: "app-newdomain-cert"
       external-dns.alpha.kubernetes.io/hostname: "app.newdomain.com"
   spec:
     defaultBackend:
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

**Note**: The GKE Ingress LoadBalancer IP is shared across all domains. ExternalDNS will point all domains to the same IP, and GKE Ingress routes based on hostname.

### View Services
```bash
kubectl get deployments
kubectl get services
kubectl get ingress
kubectl get managedcertificates
```

### View Logs
```bash
kubectl logs -l app=myapp -f
```

### Check DNS
```bash
kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns
```

### Check TLS Certificates
```bash
kubectl get managedcertificates
kubectl describe managedcertificate myapp-cert
```

### Scale Configuration

Edit your Deployment:
```yaml
spec:
  replicas: 3  # Number of pods
  template:
    spec:
      # For cost savings, use Spot Pods
      nodeSelector:
        cloud.google.com/gke-spot: "true"
```

Or use HorizontalPodAutoscaler:
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: myapp-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: myapp
  minReplicas: 1
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 50
```

### Troubleshooting

```bash
# Check cluster
kubectl get nodes
kubectl get pods --all-namespaces

# Check components
clusterkit troubleshoot

# View component logs
kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns

# Check Ingress status
kubectl describe ingress myapp

# Check ManagedCertificate status
kubectl describe managedcertificate myapp-cert
```

## Cost Management

### Monitor Costs
```bash
gcloud billing accounts list
gcloud billing projects describe YOUR_PROJECT_ID
```

### Reduce Costs
1. Use Spot Pods for all workloads (60-91% savings)
2. Right-size resource requests (only request what you need)
3. Use HPA to scale down during low traffic
4. Delete unused apps and Ingresses

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
│           └── components/ # Component installers
└── examples/           # Example applications
    └── manifests/      # K8s manifests (Deployment + Ingress + ManagedCertificate)
```

**Kubernetes components:**
- GKE Ingress: Built-in to GKE
- GKE Managed Certificates: Built-in to GKE
- ExternalDNS: Helm (bitnami/external-dns)

## FAQ

**Q: Why GKE Autopilot vs Cloud Run?**
A: More flexibility (full Kubernetes API, persistent storage, custom networking) with managed infrastructure.

**Q: Why not use Knative for scale-to-zero?**
A: Knative's overhead (~850m CPU, 875MB RAM) makes it expensive on Autopilot. For personal projects, the cost savings from scale-to-zero don't offset the infrastructure cost. Spot Pods provide better savings (60-91% off).

**Q: How much does this cost?**
A: ~$25-30/month for infrastructure + apps. Compare to Cloud Run ($0-10/month) or traditional GKE ($75+/month).

**Q: Can I use wildcard certificates?**
A: No, GKE Managed Certificates don't support wildcards. You need one ManagedCertificate per subdomain. For wildcard support, use cert-manager with DNS challenges (adds overhead).

**Q: Can I use multiple domains?**
A: Yes! ExternalDNS supports unlimited domains. Create one ManagedCertificate and Ingress per subdomain.

**Q: What are Spot Pods?**
A: Discounted pods (60-91% off) that can be preempted. Great for web apps where brief downtime is acceptable. Kubernetes automatically reschedules preempted pods.

**Q: Can I add databases?**
A: Yes, deploy StatefulSets or use Helm charts (e.g., `helm install postgresql bitnami/postgresql`). Consider using regular pods (not Spot) for databases.

## License

MIT
