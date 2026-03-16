# GKE Cluster Outputs
output "cluster_name" {
  description = "Name of the GKE cluster"
  value       = module.gke.cluster_name
}

output "cluster_endpoint" {
  description = "Endpoint for the GKE cluster API server"
  value       = module.gke.cluster_endpoint
  sensitive   = true
}

output "cluster_ca_certificate" {
  description = "CA certificate for the GKE cluster"
  value       = module.gke.cluster_ca_certificate
  sensitive   = true
}

output "cluster_location" {
  description = "Location of the GKE cluster"
  value       = module.gke.cluster_location
}

output "workload_identity_pool" {
  description = "Workload Identity pool for the cluster"
  value       = module.gke.workload_identity_pool
}

# Networking Outputs
output "static_ip_address" {
  description = "The static IP address for the LoadBalancer ingress"
  value       = module.networking.static_ip_address
}

output "static_ip_name" {
  description = "The name of the static IP resource"
  value       = module.networking.static_ip_name
}

# IAM Outputs
output "external_dns_service_account_email" {
  description = "Email of the ExternalDNS service account"
  value       = module.iam.external_dns_service_account_email
}

# Connection Instructions
output "kubectl_connection_command" {
  description = "Command to configure kubectl for cluster access"
  value       = "gcloud container clusters get-credentials ${module.gke.cluster_name} --region ${var.region} --project ${var.project_id}"
}

output "next_steps" {
  description = "Next steps after infrastructure deployment"
  value       = <<-EOT

  Infrastructure deployment complete!

  Next steps:
  1. Configure kubectl: ${format("gcloud container clusters get-credentials %s --region %s --project %s", module.gke.cluster_name, var.region, var.project_id)}
  2. Verify cluster access: kubectl get nodes
  3. Note the static IP: ${module.networking.static_ip_address}
  4. Deploy ExternalDNS with Helm (see docs/external-dns-values.yaml)
  5. Deploy your app (see docs/app-integration.md)

  EOT
}

# Cloud SQL Outputs
output "cloudsql_instance_name" {
  description = "Name of the shared Cloud SQL instance"
  value       = module.cloudsql.instance_name
}

output "cloudsql_connection_name" {
  description = "Cloud SQL connection name for proxy"
  value       = module.cloudsql.connection_name
}

output "cloudsql_public_ip" {
  description = "Cloud SQL public IP address"
  value       = module.cloudsql.public_ip_address
}

output "cloudsql_proxy_service_account" {
  description = "Cloud SQL proxy service account email"
  value       = module.cloudsql_proxy_sa.service_account_email
}
