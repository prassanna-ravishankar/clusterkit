# ClusterKit Terraform Infrastructure

This Terraform configuration creates the foundational infrastructure for ClusterKit on Google Cloud Platform.

## What Gets Created

- **GKE Autopilot Cluster**: Fully managed Kubernetes cluster with automatic node management
- **Static IP Address**: Global static IP for the ingress LoadBalancer
- **IAM Service Accounts**:
  - ExternalDNS service account (with DNS Admin role)
  - cert-manager service account (with DNS Admin role for DNS-01 challenges)
- **Workload Identity**: Configured for secure pod-to-GCP authentication
- **Required APIs**: Enables necessary GCP APIs automatically

## Prerequisites

1. **Google Cloud SDK** installed and authenticated
2. **Terraform** >= 1.6.0 installed
3. **GCP Project** with billing enabled
4. **Permissions**: You need the following IAM roles in your GCP project:
   - `roles/container.admin` (for GKE)
   - `roles/compute.admin` (for networking)
   - `roles/iam.serviceAccountAdmin` (for service accounts)
   - `roles/iam.serviceAccountKeyAdmin` (if creating keys)
   - `roles/resourcemanager.projectIamAdmin` (for IAM bindings)

## Quick Start

1. **Copy the example tfvars file**:
   ```bash
   cp terraform.tfvars.example terraform.tfvars
   ```

2. **Edit `terraform.tfvars`** with your project details:
   ```hcl
   project_id = "your-gcp-project-id"
   region     = "us-central1"
   cluster_name = "clusterkit"
   ```

3. **Initialize Terraform**:
   ```bash
   terraform init
   ```

4. **Review the plan**:
   ```bash
   terraform plan
   ```

5. **Apply the configuration**:
   ```bash
   terraform apply
   ```

6. **Configure kubectl**:
   ```bash
   gcloud container clusters get-credentials clusterkit --region us-central1 --project your-gcp-project-id
   ```

## Module Structure

```
terraform/
├── main.tf              # Main configuration
├── variables.tf         # Input variables
├── outputs.tf          # Output values
├── providers.tf        # Provider configuration
├── versions.tf         # Terraform and provider versions
├── modules/
│   ├── gke/            # GKE Autopilot cluster module
│   ├── networking/     # Static IP configuration
│   └── iam/            # Service account management
└── environments/
    └── dev/            # Environment-specific overrides
```

## Configuration Options

### Required Variables

- `project_id`: Your GCP project ID

### Optional Variables

- `region`: GCP region (default: `us-central1`)
- `cluster_name`: Name of the cluster (default: `clusterkit`)
- `kubernetes_version`: Minimum K8s version (default: `1.28`)
- `static_ip_name`: Name for the static IP (default: `clusterkit-ingress-ip`)
- `environment`: Environment tag (default: `dev`)
- `deletion_protection`: Protect cluster from deletion (default: `true`)
- `create_service_account_keys`: Create SA keys instead of using Workload Identity (default: `false`)

## Outputs

After successful deployment, Terraform provides:

- `cluster_endpoint`: API server endpoint
- `static_ip_address`: LoadBalancer IP (configure in Cloudflare)
- `external_dns_service_account_email`: For ExternalDNS configuration
- `cert_manager_service_account_email`: For cert-manager configuration
- `kubectl_connection_command`: Command to connect kubectl

## Cost Estimation

Approximate monthly costs for a minimal setup (us-central1):

- GKE Autopilot cluster: ~$72/month (cluster management fee)
- Static IP (in use): ~$5/month
- Pods and compute: Pay-per-use based on resource requests
- **Estimated baseline**: $80-100/month

With scale-to-zero apps, costs can drop to ~$80-90/month for idle workloads.

## Next Steps

After infrastructure deployment:

1. Install Knative Serving (Task 2)
2. Configure cert-manager (Task 4)
3. Setup ExternalDNS with Cloudflare (Task 5)
4. Build the ClusterKit CLI (Task 6)

## Cleanup

To destroy all resources:

```bash
# Disable deletion protection first
terraform apply -var="deletion_protection=false"

# Then destroy
terraform destroy
```

**Warning**: This will delete the cluster and all workloads!

## Security Notes

- Workload Identity is enabled by default for secure pod authentication
- Service account keys are NOT created by default (set `create_service_account_keys=true` only if needed)
- Deletion protection is enabled by default to prevent accidental cluster deletion
- Binary Authorization is configured for container image verification
- Shielded GKE nodes are enabled for enhanced security

## Troubleshooting

### API Not Enabled Error

If you see errors about APIs not being enabled, ensure you have billing enabled on your project and wait a few minutes after applying for APIs to propagate.

### Insufficient Permissions

Ensure your account has the required IAM roles listed in Prerequisites.

### Quota Exceeded

Check your GCP quotas in the Cloud Console if you encounter quota errors.
