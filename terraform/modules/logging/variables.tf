variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "retention_days" {
  description = "Number of days to retain logs in the default bucket"
  type        = number
  default     = 7
}

variable "exclude_health_checks" {
  description = "Exclude health check logs from ingestion"
  type        = bool
  default     = true
}

variable "health_check_patterns" {
  description = "Health check endpoint patterns to exclude"
  type        = list(string)
  default     = ["/health", "/healthz", "/ready", "GET /health"]
}

variable "exclude_gke_noise" {
  description = "Exclude noisy GKE system logs (gcfs-snapshotter, gcfsd, container-runtime)"
  type        = bool
  default     = true
}

variable "info_log_sample_rate" {
  description = "Sampling rate for INFO logs (0.0-1.0). 1.0 = keep all, 0.1 = keep 10%. ERROR/WARNING always kept."
  type        = number
  default     = 0.1

  validation {
    condition     = var.info_log_sample_rate > 0.0 && var.info_log_sample_rate <= 1.0
    error_message = "Sample rate must be between 0.0 and 1.0"
  }
}

variable "custom_exclusions" {
  description = "Custom log exclusions as a map of name => filter"
  type        = map(string)
  default     = {}
}
