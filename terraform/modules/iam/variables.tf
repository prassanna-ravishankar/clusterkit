variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "cluster_name" {
  description = "Name of the GKE cluster"
  type        = string
}

variable "create_service_account_keys" {
  description = "Whether to create service account keys (not recommended for Workload Identity)"
  type        = bool
  default     = false
}
