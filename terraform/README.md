# ClusterKit Terraform Infrastructure

Terraform configuration for ClusterKit on Google Cloud Platform with Cloudflare.

## What Gets Created

- **GKE Autopilot Cluster**: Managed Kubernetes with automatic node management
- **Gateway API**: Shared Gateway with Cloudflare Origin CA wildcard SSL certs
- **Cloudflare Zone Settings**: Full (Strict) SSL, HTTPS enforcement, TLS 1.3
- **Static IP Address**: Global static IP for the Gateway
- **Cloud SQL**: Shared PostgreSQL instance (db-f1-micro)
- **Cloud SQL Proxy SA**: Workload Identity bindings for database access
- **Artifact Registry**: Shared container image repository with cleanup policies
- **Logging Optimization**: 7-day retention, health check exclusions, INFO sampling
- **DNS Records** (`dns.tf`): Email, verification, GitHub Pages (not gateway A records)

## Prerequisites

1. **Google Cloud SDK** installed and authenticated
2. **Terraform** >= 1.6.0
3. **Cloudflare API token** with Zone DNS Edit, Zone Zone Read, SSL and Certificates Edit, Zone Settings Edit
4. **GCP Project** with billing enabled

## State Backend

Terraform state is stored in GCS with versioning and locking:

| State | GCS prefix |
|-------|------------|
| Root | `gs://tf-state-baldmaninc/clusterkit/root/` |
| Torale | `gs://tf-state-baldmaninc/clusterkit/projects/torale/` |
| Bananagraph | `gs://tf-state-baldmaninc/clusterkit/projects/bananagraph/` |

To set up the backend from scratch: `PROJECT_ID=baldmaninc ./scripts/bootstrap-backend.sh`

## Quick Start

```bash
terraform init
export CLOUDFLARE_API_TOKEN="your-token"
terraform apply
```

## Module Structure

```
terraform/
├── main.tf              # Root config (cluster, Gateway, Origin CA certs, Cloud SQL)
├── dns.tf               # Cloudflare DNS records (email, verification, Pages)
├── artifact-registry.tf # Container image repository
├── variables.tf         # Input variables
├── outputs.tf           # Output values
├── versions.tf          # Terraform and provider versions
├── modules/
│   ├── gke/             # GKE Autopilot cluster
│   ├── gateway-api/     # Gateway + ReferenceGrants
│   ├── networking/      # Static IP
│   ├── logging/         # Cost-optimized Cloud Logging
│   ├── cloudflare-dns/  # Cloudflare DNS record management
│   ├── httproute/       # HTTPRoute template (for app use)
│   ├── cloudsql-instance/ # PostgreSQL instances
│   └── cloudsql-proxy-sa/ # Cloud SQL proxy service account
└── projects/            # Project-specific Terraform (torale, bananagraph)
```

## Key Outputs

- `static_ip_address`: Gateway IP (configure in Cloudflare)
- `kubectl_connection_command`: Command to connect kubectl
- `cloudsql_connection_name`: Cloud SQL connection string for proxy
