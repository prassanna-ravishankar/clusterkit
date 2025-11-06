terraform {
  required_version = ">= 1.9"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }
}

# Reuse the root provider configuration
provider "google" {
  project = var.project_id
  region  = var.region
}

# Cloud SQL Instance
module "cloudsql" {
  source = "../../modules/cloudsql-instance"

  project_id       = var.project_id
  instance_name    = var.cloudsql_instance_name
  region           = var.region
  database_version = "POSTGRES_16"
  tier             = "db-custom-1-3840"

  ipv4_enabled    = true
  private_network = null

  backup_enabled                 = true
  backup_start_time              = "03:00"
  point_in_time_recovery_enabled = true
  transaction_log_retention_days = 7

  maintenance_window_day          = 7
  maintenance_window_hour         = 3
  maintenance_window_update_track = "stable"

  max_connections     = "100"
  deletion_protection = true

  databases = var.databases
  users     = var.database_users
}

# Cloud SQL Proxy Service Account
module "cloudsql_proxy_sa" {
  source = "../../modules/cloudsql-proxy-sa"

  project_id         = var.project_id
  service_account_id = "cloudsql-proxy"
  display_name       = "Cloud SQL Proxy for GKE"

  enable_workload_identity = var.enable_workload_identity
  k8s_namespace            = var.k8s_namespace
  k8s_service_account      = var.k8s_service_account
}

# Static IP for Ingress
module "static_ip" {
  source = "../../modules/static-ip"

  project_id   = var.project_id
  address_name = var.static_ip_name
  description  = "Static IP for ClusterKit ingress LoadBalancer"
}
