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

output "static_ip_address" {
  description = "Static IP address for ingress"
  value       = module.static_ip.address
}
