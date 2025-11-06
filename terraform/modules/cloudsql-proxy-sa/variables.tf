variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "service_account_id" {
  description = "Service account ID (email prefix)"
  type        = string
  default     = "cloudsql-proxy"
}

variable "display_name" {
  description = "Service account display name"
  type        = string
  default     = "Cloud SQL Proxy for GKE"
}

variable "enable_workload_identity" {
  description = "Enable Workload Identity binding"
  type        = bool
  default     = true
}

variable "k8s_namespace" {
  description = "Kubernetes namespace for Workload Identity binding"
  type        = string
  default     = ""
}

variable "k8s_service_account" {
  description = "Kubernetes service account for Workload Identity binding"
  type        = string
  default     = ""
}
