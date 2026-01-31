# ClusterKit Terraform Infrastructure

Terraform configuration for ClusterKit on Google Cloud Platform with Cloudflare.

## What Gets Created

- **GKE Autopilot Cluster**: Managed Kubernetes with automatic node management
- **Gateway API**: Shared Gateway with Cloudflare Origin CA wildcard SSL certs
- **Cloudflare Zone Settings**: Full (Strict) SSL per zone
- **Static IP Address**: Global static IP for the Gateway
- **Cloud SQL**: Shared PostgreSQL instance (db-f1-micro)
- **IAM Service Accounts**: ExternalDNS (with Workload Identity)
- **Logging Optimization**: 7-day retention, health check exclusions, INFO sampling
- **DNS Records** (`dns.tf`): Email, verification, GitHub Pages (not gateway A records)

## Prerequisites

1. **Google Cloud SDK** installed and authenticated
2. **Terraform** >= 1.6.0
3. **Cloudflare API token** with Zone DNS Edit, Zone Zone Read, SSL and Certificates Edit, Zone Settings Edit
4. **GCP Project** with billing enabled

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
├── variables.tf         # Input variables
├── outputs.tf           # Output values
├── versions.tf          # Terraform and provider versions
├── modules/
│   ├── gke/             # GKE Autopilot cluster
│   ├── gateway-api/     # Gateway + ReferenceGrants
│   ├── networking/      # Static IP
│   ├── iam/             # Service accounts + Workload Identity
│   ├── logging/         # Cost-optimized Cloud Logging
│   ├── cloudflare-dns/  # Cloudflare DNS record management
│   ├── httproute/       # HTTPRoute template (for app use)
│   ├── cloudsql-instance/ # PostgreSQL instances
│   └── cloudsql-proxy-sa/ # Cloud SQL proxy service account
└── projects/            # Project-specific Terraform (torale, bananagraph)
```

## Key Outputs

- `static_ip_address`: Gateway IP (configure in Cloudflare)
- `external_dns_service_account_email`: For ExternalDNS Helm chart
- `kubectl_connection_command`: Command to connect kubectl
- `cloudsql_connection_name`: Cloud SQL connection string for proxy
