# Knative Serving for ClusterKit

This directory contains Knative Serving v1.15.0 configuration for ClusterKit's serverless platform.

## Architecture

```
Internet → Cloudflare → GCP Static IP → NGINX Ingress → Kourier → Knative Services
```

- **Kourier**: Lightweight networking layer optimized for Knative (~50MB footprint)
- **Knative Serving**: Serverless platform with scale-to-zero capabilities
- **Autoscaling**: KPA (Knative Pod Autoscaler) with aggressive scale-to-zero

## Installation

### Prerequisites

1. GKE Autopilot cluster running (from Task 1)
2. kubectl configured and connected to cluster
3. Cluster admin permissions

### Quick Install

```bash
cd k8s/knative
./install.sh
```

The installation script will:
1. Install Knative Serving CRDs
2. Deploy Knative Serving core components
3. Install Kourier networking layer
4. Configure autoscaling parameters
5. Verify all components are ready

### Manual Installation

If you prefer manual installation:

```bash
# 1. Install CRDs
kubectl apply -f 01-serving-crds.yaml

# 2. Install Knative Serving core
kubectl apply -f https://github.com/knative/serving/releases/download/knative-v1.15.0/serving-core.yaml

# 3. Install Kourier
kubectl apply -f https://github.com/knative/net-kourier/releases/download/knative-v1.15.0/kourier.yaml

# 4. Configure Kourier as ingress
kubectl patch configmap/config-network \
  --namespace knative-serving \
  --type merge \
  --patch '{"data":{"ingress-class":"kourier.ingress.networking.knative.dev"}}'

# 5. Apply autoscaling configuration
kubectl apply -f 05-config-autoscaler.yaml
```

## Configuration Files

- `01-serving-crds.yaml` - Custom Resource Definitions for Knative
- `05-config-autoscaler.yaml` - Autoscaling configuration (scale-to-zero, concurrency targets)
- `install.sh` - Automated installation script
- `KNATIVE_DECISION.md` - Technical decisions and rationale

## Autoscaling Defaults

```yaml
Scale-to-zero grace period: 60 seconds
Target concurrency: 10 requests/pod
Max scale: 100 pods
Stable window: 60 seconds
Max scale-up rate: 10x per decision
Max scale-down rate: 2x per decision
```

### Per-Service Overrides

Override defaults using annotations:

```yaml
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: my-app
spec:
  template:
    metadata:
      annotations:
        autoscaling.knative.dev/min-scale: "0"      # Scale to zero
        autoscaling.knative.dev/max-scale: "50"     # Max 50 pods
        autoscaling.knative.dev/target: "20"        # 20 concurrent requests/pod
        autoscaling.knative.dev/metric: "concurrency"  # or "rps"
        autoscaling.knative.dev/window: "120s"      # Custom stable window
```

## Verification

### Check Installation

```bash
# Knative Serving components
kubectl get pods -n knative-serving

# Kourier networking
kubectl get pods -n kourier-system

# Kourier service (note the EXTERNAL-IP)
kubectl get svc kourier -n kourier-system
```

### Deploy Test Service

```bash
kubectl apply -f - <<EOF
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: hello-world
spec:
  template:
    metadata:
      annotations:
        autoscaling.knative.dev/min-scale: "0"
        autoscaling.knative.dev/max-scale: "10"
    spec:
      containers:
      - image: gcr.io/knative-samples/helloworld-go
        env:
        - name: TARGET
          value: "ClusterKit"
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
EOF
```

### Test Scale-to-Zero

```bash
# Watch pods
kubectl get pods -n default -w

# Send traffic
kubectl get ksvc hello-world  # Get the URL
curl <service-url>

# Wait 60 seconds without traffic
# Pod should scale to zero
```

## Cost Optimization

**Why Scale-to-Zero Matters:**

```
Traditional deployment (1 pod, 100m CPU, 128Mi RAM):
  $7-10/month per app × 10 apps = $70-100/month

Scale-to-zero (idle 22 hours/day):
  $1.50-2/month per app × 10 apps = $15-20/month

Savings: 75-80% for low-traffic apps
```

## Integration with NGINX Ingress

Kourier runs as an internal service. NGINX Ingress (Task 3) will:
1. Receive external traffic on static IP
2. Route `*.knative.clusterkit.local` to Kourier
3. Route other domains to traditional services

## Troubleshooting

### Pods Not Scaling to Zero

```bash
# Check autoscaler config
kubectl get cm config-autoscaler -n knative-serving -o yaml

# Check service annotations
kubectl describe ksvc <service-name>

# Check activator logs
kubectl logs -n knative-serving -l app=activator
```

### Cold Start Too Slow

```bash
# Set minimum scale to 1 (always-on)
kubectl annotate ksvc <service-name> \
  autoscaling.knative.dev/min-scale=1

# Or use Cloudflare edge caching to mask cold starts
```

### Service Not Accessible

```bash
# Check Kourier service
kubectl get svc kourier -n kourier-system

# Check Knative Ingress
kubectl get ingress.networking.internal.knative.dev -A

# Check route
kubectl get route -A
```

## Next Steps

After Knative installation:

1. **Task 3**: Configure NGINX Ingress to route to Kourier
2. **Task 4**: Setup cert-manager for automatic TLS
3. **Task 5**: Configure ExternalDNS for automatic DNS
4. **Task 7**: Build ClusterKit CLI to create Knative Services

## Resources

- [Knative Documentation](https://knative.dev/docs/)
- [Kourier Documentation](https://github.com/knative-extensions/net-kourier)
- [GKE Autopilot + Knative Guide](https://cloud.google.com/kubernetes-engine/docs/how-to/knative-autopilot)
- [Autoscaling Reference](https://knative.dev/docs/serving/autoscaling/)

## Version Information

- Knative Serving: v1.15.0
- Kourier: v1.15.0
- Kubernetes: 1.28+
- GKE Autopilot: Latest
