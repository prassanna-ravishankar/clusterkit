# Torale Project Infrastructure

This directory manages the Torale-specific infrastructure resources on the shared ClusterKit GKE cluster.

## Resources Managed

- **Cloud SQL Instance**: `clusterkit-db` (Postgres 16)
- **Service Account**: `cloudsql-proxy@baldmaninc.iam.gserviceaccount.com`
- **Static IP**: `clusterkit-ingress-ip` (34.149.49.202)

## Configuration

All configuration is in `terraform.tfvars`. Sensitive values like database passwords should be provided via environment variables:

```bash
export TF_VAR_database_users='{"torale":{"password":"your-secure-password"}}'
```

## Commands

```bash
# Initialize (already done)
terraform init

# Plan changes
terraform plan

# Apply changes (only if needed)
terraform apply

# View outputs
terraform output
```

## Imported Resources

These resources were created manually and imported into Terraform state:

```bash
terraform import module.cloudsql.google_sql_database_instance.main baldmaninc/clusterkit-db
terraform import module.cloudsql_proxy_sa.google_service_account.cloudsql_proxy projects/baldmaninc/serviceAccounts/cloudsql-proxy@baldmaninc.iam.gserviceaccount.com
terraform import module.static_ip.google_compute_global_address.ingress projects/baldmaninc/global/addresses/clusterkit-ingress-ip
```

## State Changes on Next Apply

The following safe changes will be applied:

1. Enable backups on Cloud SQL (currently disabled)
2. Add maintenance window configuration
3. Add max_connections database flag
4. Create IAM bindings for Cloud SQL proxy
5. Create Workload Identity binding for torale-api

## Adding New Projects

To add a new project (e.g., `future-project`):

1. Create `terraform/projects/future-project/` directory
2. Copy this directory's structure
3. Update variables for the new project
4. Initialize and import resources if they exist
5. Apply changes

All projects share the same GKE cluster but have separate Cloud SQL instances and configurations.
