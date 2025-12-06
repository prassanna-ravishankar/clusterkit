# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ClusterKit is a simplified Kubernetes platform for personal projects on GKE Autopilot with Cloudflare DNS integration. It automates infrastructure deployment, certificate management, and DNS configuration to minimize costs while maintaining full Kubernetes functionality.

**Key Technologies:**
- GKE Autopilot (managed Kubernetes)
- Terraform (infrastructure as code)
- Go CLI (bootstrap orchestration)
- Cloudflare (DNS via ExternalDNS)
- GKE Managed Certificates (automatic TLS)

## Architecture

### Two-Tier Infrastructure Management

**1. Root Terraform (`terraform/`)**
- GKE Autopilot cluster
- Networking (static IPs)
- IAM (service accounts with Workload Identity)
- Logging optimization module (project-level)
- Used for: Cluster infrastructure, shared resources

**2. Project-Specific Terraform (`terraform/projects/torale/`)**
- Cloud SQL instances
- Project-specific static IPs
- Application-specific service accounts
- Used for: Application infrastructure separate from cluster

Both use the same GCP project but maintain separate Terraform states.

### Terraform Module Structure

Reusable modules in `terraform/modules/`:
- `gke/` - GKE Autopilot cluster with configurable logging/monitoring
- `networking/` - Static IP addresses for load balancers
- `iam/` - Service accounts with Workload Identity bindings
- `logging/` - Cost-optimized Cloud Logging (retention, exclusions, sampling)
- `cloudsql-instance/` - PostgreSQL instances
- `cloudsql-proxy-sa/` - Service accounts for Cloud SQL Proxy
- `static-ip/` - Global static IP addresses
- `cloudflare/` - Cloudflare DNS configuration (if needed)

**Module Usage Pattern:**
```hcl
module "logging" {
  source = "./modules/logging"

  project_id            = var.project_id
  retention_days        = 7
  exclude_health_checks = true
  info_log_sample_rate  = 0.1
}
```

### Go CLI Architecture

**Bootstrap Orchestrator Pattern:**
- `pkg/bootstrap/orchestrator.go` - Main orchestration engine
- `pkg/bootstrap/components/` - Component installers (Terraform, Helm)
- `pkg/bootstrap/validation.go` - End-to-end validation
- `pkg/preflight/` - Pre-flight checks (GCP, Cloudflare)

**Execution Flow:**
1. Pre-flight validation (credentials, quotas)
2. Terraform deployment (infrastructure)
3. Component installation (ExternalDNS via Helm)
4. Health checks (cluster, components)
5. End-to-end validation

**Retry Logic:**
- Each step retries up to 3 times with exponential backoff
- Health checks run after successful execution
- Rollback capability for failed deployments

## Development Commands

### Terraform

```bash
# Root infrastructure (cluster, networking, logging)
cd terraform
terraform init
terraform plan -var="project_id=YOUR_PROJECT"
terraform apply -var="project_id=YOUR_PROJECT"

# Project-specific (Cloud SQL, app resources)
cd terraform/projects/torale
terraform init
terraform plan  # Uses project_id from variables.tf default
terraform apply
```

**Important:**
- Logging module is only in root terraform (project-level config)
- GKE cluster monitoring config is in root terraform
- Application resources go in project-specific terraform

### Go CLI

```bash
cd cli

# Build
make build

# Run tests
make test
go test ./... -v

# Format code
make fmt

# Install locally
make install

# Run directly
make run

# Build for release
make release  # Creates dist/ with multi-platform binaries
```

### Kubernetes Operations

```bash
# Connect to cluster
gcloud container clusters get-credentials clusterkit --region us-central1 --project PROJECT_ID

# Check components
kubectl get pods --all-namespaces
kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns

# Check TLS certificates
kubectl get managedcertificates
kubectl describe managedcertificate CERT_NAME

# Check DNS
kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns
```

## Cost Optimization Strategy

This project implements aggressive cost optimization for side projects:

**Logging Optimizations** (`terraform/modules/logging/`):
- Log retention: 7 days (down from 30)
- Health check exclusion (~24% of logs)
- GKE noise exclusion (gcfs-snapshotter, gcfsd, container-runtime)
- INFO log sampling at 10% (ERROR/WARNING kept at 100%)

**GKE Monitoring Optimizations** (`terraform/modules/gke/`):
- Monitoring components reduced to essential: SYSTEM_COMPONENTS, POD, DEPLOYMENT
- Managed Prometheus: Cannot be disabled in Autopilot (GKE 1.25+, enforced by Google)
- Workload logging: Configurable (kept enabled for debugging)

**Target Costs:**
- Cloud Monitoring: ~£3-5/month (down from £14/month)
- Infrastructure: ~£25-30/month total for side projects

**Spot Pods:**
- 60-91% cost savings for fault-tolerant workloads
- Add nodeSelector in manifests: `cloud.google.com/gke-spot: "true"`

## Key Patterns and Conventions

### Terraform Best Practices

1. **Module Variables:** Always provide defaults for optional parameters
2. **Outputs:** Return useful information (IDs, emails, connection commands)
3. **Deletion Protection:** Enabled by default on critical resources
4. **Workload Identity:** Preferred over service account keys
5. **Lock Files:** Always commit `.terraform.lock.hcl`

### Go Code Patterns

1. **Component Interface:** All bootstrap components implement Install/HealthCheck/Uninstall
2. **Structured Logging:** Use logrus with structured fields
3. **Error Wrapping:** Always wrap errors with context (`fmt.Errorf("context: %w", err)`)
4. **Dry-Run Support:** All components support dry-run mode
5. **Progress Callbacks:** Orchestrator supports progress reporting via callbacks

### Kubernetes Manifest Pattern

Standard app deployment requires 4 resources:
1. Deployment (with Spot Pod nodeSelector for cost savings)
2. Service (ClusterIP exposing pod ports)
3. ManagedCertificate (GKE-managed TLS, one per domain)
4. Ingress (with GKE annotations, ExternalDNS hostname)

See `examples/manifests/` for templates.

## Important Caveats

### GKE Autopilot Limitations

- **Managed Prometheus:** Cannot be disabled (enforced in GKE 1.25+)
- **Node Management:** Fully automated, no node pool configuration
- **Resource Limits:** Automatic resource provisioning based on pod requests
- **Monitoring Components:** Some components cannot be fully disabled

### Multi-Project Setup

- Same GCP project (`baldmaninc`) used for both cluster and applications
- Separate Terraform states (root vs projects/torale)
- Logging config is project-level (only configure once in root terraform)
- Static IPs can be created in either location

### Cloudflare Integration

- ExternalDNS automatically creates/updates DNS records
- Requires Cloudflare API token with DNS edit permissions
- One token can manage multiple domains (configure zone resources)
- DNS records point to GKE Ingress LoadBalancer IP (shared across apps)

## Common Workflows

### Adding New Terraform Module

1. Create `terraform/modules/MODULE_NAME/`
2. Add `main.tf`, `variables.tf`, `outputs.tf`
3. Use in root or project-specific terraform
4. Document module purpose and variables

### Deploying New Application

1. Create manifests (Deployment, Service, ManagedCertificate, Ingress)
2. Add Spot Pod nodeSelector for cost savings
3. Apply: `kubectl apply -f app.yaml`
4. Verify: Check Ingress, ManagedCertificate, and ExternalDNS logs

### Optimizing Costs

1. **Logging:** Adjust `terraform/modules/logging/` variables
2. **Monitoring:** Modify `monitoring_components` in GKE module
3. **Resources:** Right-size pod requests in manifests
4. **Spot Pods:** Add to all fault-tolerant workloads

### Debugging Deployments

1. Check pod status: `kubectl get pods`
2. Check events: `kubectl get events --sort-by='.lastTimestamp'`
3. Check Ingress: `kubectl describe ingress APP_NAME`
4. Check cert: `kubectl describe managedcertificate CERT_NAME`
5. Check DNS: `kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns`

## Project-Specific Notes

### Current Setup

- **Cluster:** GKE Autopilot in us-central1
- **Project:** baldmaninc
- **Domains:** Managed via Cloudflare
- **Production:** Shares cluster with staging (separate namespaces)
- **Database:** Cloud SQL PostgreSQL (db-f1-micro, PITR disabled)
