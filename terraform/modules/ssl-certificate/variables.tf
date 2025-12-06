variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "certificate_name" {
  description = "Name of the SSL certificate resource"
  type        = string
}

variable "domains" {
  description = "List of domains to include in the certificate"
  type        = list(string)
}

variable "description" {
  description = "Description of the SSL certificate"
  type        = string
  default     = "Google-managed SSL certificate for GKE Gateway"
}
