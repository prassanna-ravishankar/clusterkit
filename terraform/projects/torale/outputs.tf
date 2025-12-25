output "cloudsql_connection_name" {
  description = "Cloud SQL connection name for proxy (from shared instance)"
  value       = data.google_sql_database_instance.clusterkit_db.connection_name
}

output "databases" {
  description = "List of torale databases created"
  value       = [for db in google_sql_database.databases : db.name]
}
