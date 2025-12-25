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
  description = "Shared Cloud SQL instance name (managed by root terraform)"
  type        = string
  default     = "clusterkit-db"
}

variable "databases" {
  description = "List of torale-specific databases to create"
  type        = list(string)
  default     = []
}

variable "database_users" {
  description = "Torale-specific database users and passwords"
  type = map(object({
    password = string
  }))
  default = {}
}
