# Deploying Applications to ClusterKit

This guide explains how to deploy your applications to the ClusterKit GKE Autopilot cluster.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Deployment Patterns](#deployment-patterns)
- [Exposing Your Application](#exposing-your-application)
- [SSL/TLS Configuration](#ssltls-configuration)
- [Cost Optimization](#cost-optimization)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

### Required Tools

```bash
# Install gcloud CLI
curl https://sdk.cloud.google.com | bash
exec -l $SHELL

# Install kubectl
gcloud components install kubectl

# Authenticate with GCP
gcloud auth login
gcloud config set project baldmaninc
```

### Cluster Access

Get cluster credentials:

```bash
gcloud container clusters get-credentials clusterkit \
  --region us-central1 \
  --project baldmaninc
```

Verify access:

```bash
kubectl get nodes
kubectl get namespaces
```

---

## Quick Start

### 1. Deploy a Simple Application

```bash
# Create deployment
kubectl create deployment my-app \
  --image=nginx:latest \
  --replicas=2

# Expose as LoadBalancer
kubectl expose deployment my-app \
  --port=80 \
  --target-port=80 \
  --type=LoadBalancer
```

### 2. Wait for External IP

```bash
kubectl get service my-app --watch
```

### 3. Access Your App

Once you see an `EXTERNAL-IP`, visit it in your browser:

```bash
curl http://<EXTERNAL-IP>
```

### 4. Clean Up

```bash
kubectl delete service my-app
kubectl delete deployment my-app
```

---

## Deployment Patterns

### Pattern 1: Simple Deployment with Service

**Use for:** Web applications, APIs, microservices

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  labels:
    app: my-app
spec:
  replicas: 2
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      # Enable Spot Pods for 60-91% cost savings
      nodeSelector:
        cloud.google.com/gke-spot: "true"
      tolerations:
      - key: cloud.google.com/gke-spot
        operator: Equal
        value: "true"
        effect: NoSchedule
      containers:
      - name: app
        image: gcr.io/baldmaninc/my-app:latest
        ports:
        - containerPort: 8080
        # Right-size resources for cost optimization
        resources:
          requests:
            cpu: 100m      # 0.1 vCPU
            memory: 128Mi
          limits:
            cpu: 500m      # 0.5 vCPU
            memory: 512Mi
        # Health checks
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: my-app
spec:
  type: LoadBalancer
  selector:
    app: my-app
  ports:
  - port: 80
    targetPort: 8080
```

Apply:

```bash
kubectl apply -f deployment.yaml
```

### Pattern 2: Ingress with Custom Domain

**Use for:** Production apps with custom domains (e.g., app.yourdomain.com)

```yaml
# app-with-ingress.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      nodeSelector:
        cloud.google.com/gke-spot: "true"
      tolerations:
      - key: cloud.google.com/gke-spot
        operator: Equal
        value: "true"
        effect: NoSchedule
      containers:
      - name: app
        image: gcr.io/baldmaninc/my-app:latest
        ports:
        - containerPort: 8080
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 1000m
            memory: 1Gi
---
apiVersion: v1
kind: Service
metadata:
  name: my-app
  annotations:
    # DNS will be automatically managed by ExternalDNS
    external-dns.alpha.kubernetes.io/hostname: my-app.yourdomain.com
spec:
  type: ClusterIP  # Internal only, exposed via Ingress
  selector:
    app: my-app
  ports:
  - port: 80
    targetPort: 8080
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  annotations:
    # Use GKE Ingress
    kubernetes.io/ingress.class: "gce"
    # Enable automatic SSL certificate
    networking.gke.io/managed-certificates: "my-app-cert"
    # Optional: Force HTTPS redirect
    networking.gke.io/v1beta1.FrontendConfig: "ssl-redirect"
spec:
  rules:
  - host: my-app.yourdomain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: my-app
            port:
              number: 80
---
# Managed SSL Certificate (auto-provisioned by Google)
apiVersion: networking.gke.io/v1
kind: ManagedCertificate
metadata:
  name: my-app-cert
spec:
  domains:
  - my-app.yourdomain.com
---
# Optional: Force HTTPS redirect
apiVersion: networking.gke.io/v1beta1
kind: FrontendConfig
metadata:
  name: ssl-redirect
spec:
  redirectToHttps:
    enabled: true
```

Apply:

```bash
kubectl apply -f app-with-ingress.yaml
```

**What happens automatically:**

1. **GKE Ingress** creates a Google Cloud Load Balancer
2. **ExternalDNS** creates a Cloudflare DNS A record pointing to the load balancer
3. **GKE Managed Certificate** provisions a Let's Encrypt SSL certificate
4. Your app is accessible at `https://my-app.yourdomain.com`

**Check status:**

```bash
# Wait for certificate provisioning (takes ~10-15 minutes)
kubectl describe managedcertificate my-app-cert

# Check DNS record was created
dig my-app.yourdomain.com

# Get Ingress IP
kubectl get ingress my-app
```

### Pattern 3: Multiple Apps with Path-Based Routing

**Use for:** Microservices on shared domain

```yaml
# multi-app-ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: multi-app
  annotations:
    kubernetes.io/ingress.class: "gce"
    networking.gke.io/managed-certificates: "multi-app-cert"
spec:
  rules:
  - host: api.yourdomain.com
    http:
      paths:
      - path: /users
        pathType: Prefix
        backend:
          service:
            name: user-service
            port:
              number: 80
      - path: /orders
        pathType: Prefix
        backend:
          service:
            name: order-service
            port:
              number: 80
      - path: /products
        pathType: Prefix
        backend:
          service:
            name: product-service
            port:
              number: 80
---
apiVersion: networking.gke.io/v1
kind: ManagedCertificate
metadata:
  name: multi-app-cert
spec:
  domains:
  - api.yourdomain.com
```

---

## Exposing Your Application

### Option 1: LoadBalancer Service (Simple)

**Pros:**
- Simple, direct access
- Gets a dedicated external IP
- Good for non-HTTP services

**Cons:**
- Each service gets its own IP (costs ~$3/month per IP)
- No automatic SSL
- No path-based routing

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  annotations:
    # ExternalDNS will create DNS record
    external-dns.alpha.kubernetes.io/hostname: my-app.yourdomain.com
spec:
  type: LoadBalancer
  selector:
    app: my-app
  ports:
  - port: 80
    targetPort: 8080
```

### Option 2: Ingress + Managed Certificates (Recommended)

**Pros:**
- Automatic SSL certificates
- Share single load balancer across apps
- Path-based routing
- Lower cost (one IP for all apps)

**Cons:**
- HTTP/HTTPS only
- Takes 10-15 minutes to provision

**See Pattern 2 above for full example.**

---

## SSL/TLS Configuration

### Automatic SSL with GKE Managed Certificates

**ClusterKit uses GKE Managed Certificates** (automatic Let's Encrypt):

```yaml
apiVersion: networking.gke.io/v1
kind: ManagedCertificate
metadata:
  name: my-cert
spec:
  domains:
  - my-app.yourdomain.com
  - www.my-app.yourdomain.com  # Multiple domains supported
```

Reference in Ingress:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  annotations:
    networking.gke.io/managed-certificates: "my-cert"
spec:
  # ... ingress rules
```

**Check certificate status:**

```bash
kubectl describe managedcertificate my-cert
```

**Certificate provisioning status:**

- `Provisioning` - Certificate being created (10-15 mins)
- `FailedNotVisible` - DNS not pointing to ingress yet
- `Active` - Certificate ready and serving

**Common issues:**

- **DNS not propagating:** Wait 5-10 minutes for Cloudflare DNS
- **Certificate stuck:** Verify Ingress has an IP and DNS points to it

---

## Cost Optimization

### Use Spot Pods (60-91% Cheaper)

**Always add these to your deployments:**

```yaml
spec:
  template:
    spec:
      # Schedule on Spot nodes
      nodeSelector:
        cloud.google.com/gke-spot: "true"
      # Tolerate Spot node evictions
      tolerations:
      - key: cloud.google.com/gke-spot
        operator: Equal
        value: "true"
        effect: NoSchedule
```

**When to avoid Spot:**
- Critical services that can't tolerate interruptions
- Stateful workloads without proper backup

**For most apps:** Spot pods are perfect! They're preemptible but GKE gives 30 seconds warning and auto-reschedules.

### Right-Size Resources

**Use resource requests/limits:**

```yaml
resources:
  requests:
    cpu: 100m      # Minimum needed
    memory: 128Mi
  limits:
    cpu: 500m      # Maximum allowed
    memory: 512Mi
```

**Common sizes:**

| App Type | CPU Request | Memory Request |
|----------|-------------|----------------|
| Static site | 50m | 64Mi |
| Small API | 100m | 128Mi |
| Medium API | 250m | 512Mi |
| Large API | 500m | 1Gi |
| Background job | 100m | 256Mi |

**Why this matters:**
- Autopilot charges based on resource **requests**
- Over-requesting = wasted money
- Under-requesting = throttling/OOM kills

**Find the right size:**

```bash
# Check actual usage
kubectl top pod <pod-name>

# Compare to requests
kubectl describe pod <pod-name> | grep -A 5 "Requests:"
```

### Horizontal Pod Autoscaling

**Auto-scale based on load:**

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: my-app
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-app
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

**Benefits:**
- Pay only for what you need
- Handles traffic spikes automatically
- Reduces costs during low traffic

---

## Best Practices

### 1. Always Use Health Checks

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
```

### 2. Use Namespaces for Organization

```bash
# Create namespace for your team/project
kubectl create namespace my-team

# Deploy to namespace
kubectl apply -f deployment.yaml -n my-team

# Set default namespace
kubectl config set-context --current --namespace=my-team
```

### 3. Use ConfigMaps for Configuration

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-app-config
data:
  DATABASE_URL: "postgres://..."
  API_KEY: "..."
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  template:
    spec:
      containers:
      - name: app
        image: my-app:latest
        envFrom:
        - configMapRef:
            name: my-app-config
```

### 4. Use Secrets for Sensitive Data

```bash
# Create secret
kubectl create secret generic my-app-secrets \
  --from-literal=db-password='supersecret' \
  --from-literal=api-key='abc123'
```

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  template:
    spec:
      containers:
      - name: app
        image: my-app:latest
        env:
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: my-app-secrets
              key: db-password
```

### 5. Use Labels for Organization

```yaml
metadata:
  labels:
    app: my-app
    team: platform
    environment: production
    version: v1.2.3
```

**Query by labels:**

```bash
kubectl get pods -l app=my-app
kubectl get pods -l team=platform
kubectl get pods -l environment=production
```

---

## Troubleshooting

### Pod Won't Start

**Check pod status:**

```bash
kubectl get pods
kubectl describe pod <pod-name>
```

**Common issues:**

| Error | Cause | Fix |
|-------|-------|-----|
| `ImagePullBackOff` | Image doesn't exist | Check image name/tag |
| `CrashLoopBackOff` | App crashes on startup | Check logs: `kubectl logs <pod>` |
| `Pending` | No nodes available | Check resource requests are reasonable |
| `OOMKilled` | Out of memory | Increase memory limit |

### Can't Access Application

**Check service:**

```bash
kubectl get service my-app
kubectl describe service my-app
```

**Check endpoints:**

```bash
kubectl get endpoints my-app
```

If endpoints are empty, check pod labels match service selector.

**Check ingress:**

```bash
kubectl get ingress
kubectl describe ingress my-app
```

**Check DNS:**

```bash
dig my-app.yourdomain.com
```

Should point to Ingress IP.

### SSL Certificate Not Working

**Check certificate status:**

```bash
kubectl describe managedcertificate my-cert
```

**Common status meanings:**

- `Provisioning` → Wait 10-15 minutes
- `FailedNotVisible` → DNS doesn't point to ingress IP yet
- `Active` → Working!

**Debug steps:**

```bash
# 1. Verify Ingress has IP
kubectl get ingress my-app

# 2. Verify DNS points to that IP
dig my-app.yourdomain.com

# 3. Wait for DNS propagation (5-10 mins)

# 4. Check certificate again
kubectl describe managedcertificate my-cert
```

### High Costs

**Check resource usage:**

```bash
# See what's using resources
kubectl top nodes
kubectl top pods --all-namespaces

# Check pod requests
kubectl describe pods -A | grep -A 2 "Requests:"
```

**Optimization checklist:**

- [ ] Using Spot pods? (`nodeSelector: cloud.google.com/gke-spot: "true"`)
- [ ] Resource requests match actual usage?
- [ ] Using HPA to scale down during low traffic?
- [ ] Idle pods/deployments deleted?
- [ ] Using ClusterIP + Ingress instead of multiple LoadBalancers?

### Need Help?

**Useful commands:**

```bash
# View logs
kubectl logs <pod-name>
kubectl logs <pod-name> --previous  # Previous crash

# Get shell in pod
kubectl exec -it <pod-name> -- /bin/sh

# Port forward to local
kubectl port-forward <pod-name> 8080:8080

# View events
kubectl get events --sort-by='.lastTimestamp'

# Full cluster status
kubectl get all --all-namespaces
```

---

## Example: Complete Production Deployment

**Directory structure:**

```
my-app/
├── k8s/
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── ingress.yaml
│   ├── certificate.yaml
│   └── hpa.yaml
└── README.md
```

**deployment.yaml:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  labels:
    app: my-app
    version: v1.0.0
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      # Cost optimization: Use Spot pods
      nodeSelector:
        cloud.google.com/gke-spot: "true"
      tolerations:
      - key: cloud.google.com/gke-spot
        operator: Equal
        value: "true"
        effect: NoSchedule
      containers:
      - name: app
        image: gcr.io/baldmaninc/my-app:v1.0.0
        ports:
        - containerPort: 8080
        env:
        - name: PORT
          value: "8080"
        - name: DB_HOST
          valueFrom:
            configMapKeyRef:
              name: my-app-config
              key: db-host
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: my-app-secrets
              key: db-password
        resources:
          requests:
            cpu: 200m
            memory: 256Mi
          limits:
            cpu: 1000m
            memory: 1Gi
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

**service.yaml:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  annotations:
    external-dns.alpha.kubernetes.io/hostname: my-app.yourdomain.com
spec:
  type: ClusterIP
  selector:
    app: my-app
  ports:
  - port: 80
    targetPort: 8080
```

**ingress.yaml:**

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  annotations:
    kubernetes.io/ingress.class: "gce"
    networking.gke.io/managed-certificates: "my-app-cert"
    networking.gke.io/v1beta1.FrontendConfig: "ssl-redirect"
spec:
  rules:
  - host: my-app.yourdomain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: my-app
            port:
              number: 80
---
apiVersion: networking.gke.io/v1beta1
kind: FrontendConfig
metadata:
  name: ssl-redirect
spec:
  redirectToHttps:
    enabled: true
```

**certificate.yaml:**

```yaml
apiVersion: networking.gke.io/v1
kind: ManagedCertificate
metadata:
  name: my-app-cert
spec:
  domains:
  - my-app.yourdomain.com
```

**hpa.yaml:**

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: my-app
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-app
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

**Deploy everything:**

```bash
# Create config
kubectl create configmap my-app-config \
  --from-literal=db-host=postgres.example.com

# Create secrets
kubectl create secret generic my-app-secrets \
  --from-literal=db-password='supersecret'

# Deploy app
kubectl apply -f k8s/

# Watch deployment
kubectl rollout status deployment/my-app

# Get URL
kubectl get ingress my-app
```

---

## Summary

**For most applications:**

1. ✅ Use Spot pods (`nodeSelector: cloud.google.com/gke-spot: "true"`)
2. ✅ Right-size resources (start small, monitor, adjust)
3. ✅ Use Ingress + Managed Certificates for HTTPS
4. ✅ Add health checks
5. ✅ Use HPA for auto-scaling

**Your app will be:**
- ✅ Cost-optimized (~60-91% cheaper)
- ✅ Secure (automatic SSL)
- ✅ Scalable (HPA)
- ✅ Reliable (health checks + multiple replicas)

**Questions?** Check the main [README.md](./README.md) or contact the platform team.
