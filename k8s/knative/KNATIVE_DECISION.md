# Knative Serving Configuration Decisions

## Version Selection

**Selected Version: Knative Serving v1.15.x**

### Rationale:
- Latest stable release as of late 2024/early 2025
- Full support for Kubernetes 1.28+ (our minimum version)
- Improved autoscaling algorithms and scale-to-zero performance
- Better integration with GKE Autopilot
- Enhanced observability and debugging features
- Active maintenance and security updates

### Version Compatibility:
- Kubernetes: 1.28, 1.29, 1.30
- GKE Autopilot: Fully supported
- Terraform: Compatible with our infrastructure

## Networking Layer Selection

**Selected: Kourier (Knative's default networking layer)**

### Comparison: Kourier vs NGINX

| Feature | Kourier | NGINX Ingress |
|---------|---------|---------------|
| **Performance** | Optimized for Knative | General-purpose |
| **Memory Footprint** | Low (~50MB) | Higher (~200MB) |
| **Configuration** | Zero-config for Knative | Requires additional setup |
| **Scale-to-zero** | Native support | Requires workarounds |
| **Maintenance** | Part of Knative project | Separate project |
| **GKE Autopilot** | Well-tested | Works but heavier |
| **External Integration** | Clean separation | Can conflict with cluster ingress |

### Decision: Use Kourier

**Why Kourier:**
1. **Purpose-built for Knative**: Native integration, zero configuration needed
2. **Lower resource consumption**: Better for cost optimization on Autopilot
3. **Simpler architecture**: Fewer moving parts, easier debugging
4. **Clean separation**: Won't interfere with NGINX ingress for traditional workloads (Task 3)
5. **Scale-to-zero optimized**: Better cold-start performance

**Architecture:**
```
Internet → Cloudflare → Static IP → NGINX Ingress (Task 3) → Kourier → Knative Services
                                  ↓
                            Traditional Services (Task 12)
```

- **NGINX Ingress**: Cluster-level ingress (Task 3) for all external traffic
- **Kourier**: Internal networking layer specifically for Knative Services
- Both work together harmoniously

### Configuration Approach

**Method: Direct YAML Manifests**

Not using Helm because:
- Knative releases provide well-tested YAML manifests
- Simpler to version and track changes
- Easier to customize for GKE Autopilot specifics
- Recommended by Knative documentation for production

**Manifest Structure:**
```
k8s/knative/
├── 01-serving-crds.yaml          # Custom Resource Definitions
├── 02-serving-core.yaml          # Core Knative components
├── 03-kourier.yaml               # Kourier networking layer
├── 04-serving-hpa.yaml           # HPA for Knative autoscaler
├── 05-config-autoscaler.yaml     # Autoscaling configuration
└── 06-config-domain.yaml         # Domain configuration
```

## Autoscaling Parameters

**Default Configuration:**

```yaml
scale-to-zero-grace-period: "60s"    # Wait 60s before scaling to zero
scale-to-zero-pod-retention: "0s"    # No retention after grace period
target-burst-capacity: "200"          # Extra capacity for traffic spikes
stable-window: "60s"                  # Time to stabilize before scaling
container-concurrency-target: "10"    # Target 10 concurrent requests per pod
max-scale-up-rate: "10"              # Max 10x scale-up per decision
max-scale-down-rate: "2"             # Max 2x scale-down per decision
requests-per-second-target: "200"    # Alternative metric for autoscaling
```

**Per-Service Overrides (via annotations):**

```yaml
autoscaling.knative.dev/min-scale: "0"     # Scale to zero
autoscaling.knative.dev/max-scale: "100"   # Max 100 pods
autoscaling.knative.dev/target: "10"       # Requests per pod
autoscaling.knative.dev/metric: "concurrency"  # or "rps"
```

## Integration with GKE Autopilot

**Special Considerations:**

1. **Resource Requests**: Must set explicit CPU/memory requests for Autopilot billing
2. **Pod Disruption Budgets**: Not needed, Autopilot handles this
3. **Node Selectors**: Not applicable in Autopilot mode
4. **Workload Identity**: Use for accessing GCP services from Knative Services

## Next Steps

1. Install Knative Serving CRDs (Task 2.2)
2. Deploy Knative core components (Task 2.3)
3. Install Kourier networking layer (Task 2.4)
4. Configure autoscaling parameters (Task 2.5)
5. Validate with test services (Task 2.6)
