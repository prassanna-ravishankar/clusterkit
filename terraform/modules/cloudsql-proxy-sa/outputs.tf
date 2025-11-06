output "service_account_email" {
  description = "Cloud SQL proxy service account email"
  value       = google_service_account.cloudsql_proxy.email
}

output "service_account_name" {
  description = "Cloud SQL proxy service account name"
  value       = google_service_account.cloudsql_proxy.name
}
