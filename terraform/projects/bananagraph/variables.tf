variable "project_id" {
  description = "GCP project ID"
  type        = string
  default     = "baldmaninc"
}

variable "region" {
  description = "GCP region"
  type        = string
  default     = "us-central1"
}

variable "cloudsql_instance_name" {
  description = "Existing Cloud SQL instance name (shared)"
  type        = string
  default     = "clusterkit-db"
}

variable "database_name" {
  description = "Database name to create"
  type        = string
  default     = "bananagraph"
}

variable "database_user" {
  description = "Database user"
  type        = string
  default     = "bananagraph"
}

variable "database_password" {
  description = "Database password"
  type        = string
  sensitive   = true
}

variable "gcs_bucket_name" {
  description = "GCS bucket name for assets"
  type        = string
  default     = "bananagraph-assets-baldmaninc"
}

variable "k8s_namespace" {
  description = "Kubernetes namespace for Workload Identity"
  type        = string
  default     = "bananagraph"
}

variable "k8s_service_account" {
  description = "Kubernetes service account for Workload Identity"
  type        = string
  default     = "bananagraph-sa"
}
