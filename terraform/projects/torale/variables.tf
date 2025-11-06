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
  description = "Cloud SQL instance name"
  type        = string
  default     = "clusterkit-db"
}

variable "static_ip_name" {
  description = "Static IP name for ingress"
  type        = string
  default     = "clusterkit-ingress-ip"
}

variable "databases" {
  description = "List of databases to create"
  type        = list(string)
  default     = []
}

variable "database_users" {
  description = "Database users and passwords"
  type = map(object({
    password = string
  }))
  default = {}
  sensitive = true
}

variable "enable_workload_identity" {
  description = "Enable Workload Identity for Cloud SQL proxy"
  type        = bool
  default     = true
}

variable "k8s_namespace" {
  description = "Kubernetes namespace for Workload Identity"
  type        = string
  default     = "torale"
}

variable "k8s_service_account" {
  description = "Kubernetes service account for Workload Identity"
  type        = string
  default     = "torale-api"
}
