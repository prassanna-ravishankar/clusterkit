output "instance_name" {
  description = "Cloud SQL instance name"
  value       = google_sql_database_instance.main.name
}

output "connection_name" {
  description = "Cloud SQL connection name for proxy"
  value       = google_sql_database_instance.main.connection_name
}

output "public_ip_address" {
  description = "Public IP address"
  value       = google_sql_database_instance.main.public_ip_address
}

output "private_ip_address" {
  description = "Private IP address"
  value       = google_sql_database_instance.main.private_ip_address
}

output "self_link" {
  description = "Self link of the Cloud SQL instance"
  value       = google_sql_database_instance.main.self_link
}
