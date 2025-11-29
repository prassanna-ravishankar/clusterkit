variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "GCP region for the cluster"
  type        = string
}

variable "cluster_name" {
  description = "Name of the GKE cluster"
  type        = string
}

variable "kubernetes_version" {
  description = "Minimum Kubernetes version for the cluster"
  type        = string
  default     = "1.28"
}

variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "dev"
}

variable "deletion_protection" {
  description = "Enable deletion protection for the cluster"
  type        = bool
  default     = true
}

variable "enable_workload_logging" {
  description = "Enable logging for workload containers (disable to save costs)"
  type        = bool
  default     = true
}

variable "monitoring_components" {
  description = "List of GKE monitoring components to enable"
  type        = list(string)
  default     = ["SYSTEM_COMPONENTS", "POD", "DEPLOYMENT"]
}
