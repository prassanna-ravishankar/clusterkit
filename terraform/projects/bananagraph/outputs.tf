output "database_name" {
  description = "Name of the created database"
  value       = google_sql_database.bananagraph.name
}

output "database_user" {
  description = "Database user name"
  value       = google_sql_user.bananagraph.name
}

output "cloudsql_instance_connection_name" {
  description = "CloudSQL instance connection name for Cloud SQL Proxy"
  value       = data.google_sql_database_instance.clusterkit_db.connection_name
}

output "gcs_bucket_name" {
  description = "Name of the GCS bucket"
  value       = google_storage_bucket.assets.name
}

output "gcs_bucket_url" {
  description = "Public URL of the GCS bucket"
  value       = "https://storage.googleapis.com/${google_storage_bucket.assets.name}"
}

output "service_account_email" {
  description = "Email of the service account for Workload Identity"
  value       = google_service_account.bananagraph_sa.email
}

output "workload_identity_annotation" {
  description = "Annotation to add to Kubernetes ServiceAccount"
  value       = "iam.gke.io/gcp-service-account: ${google_service_account.bananagraph_sa.email}"
}
