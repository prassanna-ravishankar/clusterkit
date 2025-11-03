# ClusterKit Examples

This directory contains sample applications demonstrating ClusterKit's serverless capabilities on GKE Autopilot with Knative.

## Applications

### Static Site (`static-site/`)

A simple static website served by nginx that demonstrates:
- Scale-to-zero capabilities
- Automatic TLS certificates
- DNS management via ExternalDNS
- Health check endpoints

**Features:**
- Lightweight nginx-alpine base image
- Gzip compression enabled
- Security headers configured
- Health check endpoint at `/health`

### API (`api/`)

A Go-based REST API demonstrating:
- Serverless backend with auto-scaling
- CORS-enabled endpoints
- Health monitoring
- Load balancing across pods

**Endpoints:**
- `GET /` - API documentation page
- `GET /health` or `GET /api/health` - Health check
- `GET /api/message` - Demo message endpoint

## Quick Start

### Prerequisites

- ClusterKit CLI installed
- Kubernetes cluster with ClusterKit bootstrapped
- kubectl configured
- Domain configured in Cloudflare

### Deploy Using ClusterKit CLI

```bash
# Deploy the static site
clusterkit create static-site \
  --image=ghcr.io/clusterkit/clusterkit/static-site:latest \
  --port=80 \
  --min-scale=0 \
  --max-scale=10

# Deploy the API
clusterkit create api \
  --image=ghcr.io/clusterkit/clusterkit/api:latest \
  --port=8080 \
  --min-scale=0 \
  --max-scale=20
```

### Deploy Using Kubectl

```bash
# Apply Knative service manifests
kubectl apply -f manifests/static-site.yaml
kubectl apply -f manifests/api.yaml

# Check deployment status
kubectl get ksvc

# Get service URL
kubectl get ksvc static-site -o jsonpath='{.status.url}'
kubectl get ksvc api -o jsonpath='{.status.url}'
```

### Update Domain in Manifests

Before deploying via kubectl, update the domain in the manifest files:

```bash
# Replace example.com with your domain
sed -i 's/example.com/yourdomain.com/g' manifests/*.yaml
```

## Building Locally

### Build Static Site

```bash
cd static-site
docker build -t static-site:local .
docker run -p 8080:80 static-site:local

# Test
open http://localhost:8080
```

### Build API

```bash
cd api
docker build -t api:local .
docker run -p 8080:8080 api:local

# Test
curl http://localhost:8080/health
```

## CI/CD

Images are automatically built and published to GitHub Container Registry on pushes to main that modify the `examples/` directory.

**Image URLs:**
- `ghcr.io/clusterkit/clusterkit/static-site:latest`
- `ghcr.io/clusterkit/clusterkit/api:latest`

### Building Your Own Images

```bash
# Build and tag
docker build -t ghcr.io/YOUR_USERNAME/static-site:v1.0.0 ./static-site
docker build -t ghcr.io/YOUR_USERNAME/api:v1.0.0 ./api

# Push to registry
docker push ghcr.io/YOUR_USERNAME/static-site:v1.0.0
docker push ghcr.io/YOUR_USERNAME/api:v1.0.0
```

## Testing Scale-to-Zero

Watch your deployments scale to zero after period of inactivity:

```bash
# Monitor pods
watch kubectl get pods

# Generate traffic
for i in {1..100}; do curl https://api.yourdomain.com/health; done

# Wait 5 minutes and check - pods should scale to zero
kubectl get pods
```

## Monitoring

### View Logs

```bash
# ClusterKit CLI
clusterkit logs static-site --follow
clusterkit logs api --follow

# kubectl
kubectl logs -l serving.knative.dev/service=static-site -f
kubectl logs -l serving.knative.dev/service=api -f
```

### Check Service Status

```bash
# ClusterKit CLI
clusterkit status static-site
clusterkit status api

# kubectl
kubectl describe ksvc static-site
kubectl describe ksvc api
```

### View Metrics

```bash
# Pod resource usage
kubectl top pods

# Service details
kubectl get ksvc -o wide
```

## Customization

### Modify Scaling Behavior

Edit the autoscaling annotations in `manifests/*.yaml`:

```yaml
annotations:
  # Time before scaling to zero (default: 5m)
  autoscaling.knative.dev/scale-to-zero-pod-retention-period: "10m"

  # Minimum pods (0 = scale to zero)
  autoscaling.knative.dev/minScale: "1"

  # Maximum pods
  autoscaling.knative.dev/maxScale: "50"

  # Target concurrent requests per pod
  autoscaling.knative.dev/target: "100"
```

### Add Environment Variables

Add to the container spec:

```yaml
spec:
  containers:
  - image: ...
    env:
    - name: API_KEY
      value: "your-secret-key"
    - name: LOG_LEVEL
      value: "debug"
```

### Configure Resource Limits

Adjust resources in the manifest:

```yaml
resources:
  requests:
    memory: "256Mi"
    cpu: "200m"
  limits:
    memory: "512Mi"
    cpu: "2000m"
```

## Troubleshooting

### Service Won't Start

```bash
# Check pod status
kubectl get pods -l serving.knative.dev/service=static-site

# View pod logs
kubectl logs POD_NAME

# Describe pod for events
kubectl describe pod POD_NAME
```

### DNS Not Working

```bash
# Check ExternalDNS logs
kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns

# Verify ingress
kubectl get ingress

# Check DNS records in Cloudflare dashboard
```

### TLS Certificate Issues

```bash
# Check certificate status
kubectl get certificate

# View cert-manager logs
kubectl logs -n cert-manager -l app=cert-manager

# Describe certificate for details
kubectl describe certificate static-site-tls
```

### Service Not Scaling

```bash
# Check Knative autoscaler logs
kubectl logs -n knative-serving -l app=autoscaler

# View service configuration
kubectl get ksvc static-site -o yaml

# Check metrics
kubectl top pods
```

## Clean Up

```bash
# Using ClusterKit CLI
clusterkit delete static-site
clusterkit delete api

# Using kubectl
kubectl delete -f manifests/

# Verify deletion
kubectl get ksvc
```

## Next Steps

1. **Customize Applications**: Modify the source code to fit your needs
2. **Add Databases**: Use `clusterkit db create` to add PostgreSQL
3. **Configure Monitoring**: Set up Prometheus and Grafana
4. **Add Custom Domains**: Use `clusterkit domain add`
5. **Set Up CI/CD**: Connect your own GitHub repository

## Learn More

- [ClusterKit Documentation](../README.md)
- [Knative Serving Docs](https://knative.dev/docs/serving/)
- [GKE Autopilot Guide](https://cloud.google.com/kubernetes-engine/docs/concepts/autopilot-overview)
- [cert-manager Documentation](https://cert-manager.io/docs/)
- [ExternalDNS Guide](https://github.com/kubernetes-sigs/external-dns)

## Support

For issues or questions:
- Check [Troubleshooting Guide](../docs/TROUBLESHOOTING.md)
- Open an issue on [GitHub](https://github.com/clusterkit/clusterkit/issues)
- Review [ClusterKit CLI Reference](../docs/CLI.md)
