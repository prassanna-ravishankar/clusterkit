terraform {
  required_version = ">= 1.9"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

# Reference shared CloudSQL instance (managed by root terraform)
data "google_sql_database_instance" "clusterkit_db" {
  name    = var.cloudsql_instance_name
  project = var.project_id
}

# Create torale-specific databases
resource "google_sql_database" "databases" {
  for_each = toset(var.databases)

  name     = each.value
  instance = data.google_sql_database_instance.clusterkit_db.name
  project  = var.project_id
}

# Create torale-specific database users
resource "google_sql_user" "users" {
  for_each = nonsensitive(var.database_users)

  name     = each.key
  instance = data.google_sql_database_instance.clusterkit_db.name
  project  = var.project_id
  password = each.value.password
}
