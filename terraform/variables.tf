variable "project_id" {
  description = "GCP project ID where resources will be created"
  type        = string

  validation {
    condition     = length(var.project_id) > 0
    error_message = "Project ID must not be empty"
  }
}

variable "region" {
  description = "GCP region for the cluster and resources"
  type        = string
  default     = "us-central1"

  validation {
    condition     = can(regex("^[a-z]+-[a-z]+[0-9]$", var.region))
    error_message = "Region must be a valid GCP region format (e.g., us-central1)"
  }
}

variable "cluster_name" {
  description = "Name of the GKE Autopilot cluster"
  type        = string
  default     = "clusterkit"

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]*[a-z0-9]$", var.cluster_name))
    error_message = "Cluster name must start with a letter, contain only lowercase letters, numbers, and hyphens"
  }
}

variable "kubernetes_version" {
  description = "Minimum Kubernetes version for the cluster"
  type        = string
  default     = "1.28"

  validation {
    condition     = can(regex("^[0-9]+\\.[0-9]+$", var.kubernetes_version))
    error_message = "Kubernetes version must be in format X.Y (e.g., 1.28)"
  }
}

variable "static_ip_name" {
  description = "Name for the static IP address used by the LoadBalancer"
  type        = string
  default     = "clusterkit-ingress-ip"

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]*[a-z0-9]$", var.static_ip_name))
    error_message = "Static IP name must start with a letter, contain only lowercase letters, numbers, and hyphens"
  }
}

variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "dev"

  validation {
    condition     = contains(["dev", "staging", "prod"], var.environment)
    error_message = "Environment must be one of: dev, staging, prod"
  }
}

variable "deletion_protection" {
  description = "Enable deletion protection for the GKE cluster"
  type        = bool
  default     = true
}

variable "create_service_account_keys" {
  description = "Whether to create service account keys (not recommended when using Workload Identity)"
  type        = bool
  default     = false
}

# Cloud SQL Variables
variable "cloudsql_instance_name" {
  description = "Name of the shared Cloud SQL instance"
  type        = string
  default     = "clusterkit-db"
}

variable "cloudsql_databases" {
  description = "List of databases to create in the shared instance"
  type        = list(string)
  default     = []
}

variable "cloudsql_users" {
  description = "Database users and passwords for the shared instance"
  type = map(object({
    password = string
  }))
  default = {}
}

variable "prefect_db_password" {
  description = "Password for the Prefect database user"
  type        = string
  sensitive   = true
}

# Domains that get Cloudflare Origin CA wildcard certs on the Gateway
variable "origin_ca_domains" {
  description = "Domains to generate Cloudflare Origin CA wildcard certs for (must have zone IDs in cloudflare_zone_ids)"
  type        = list(string)
  default     = ["torale.ai", "bananagraph.com", "a2aregistry.org", "repowire.io"]
}

# Cloudflare Zone IDs
variable "cloudflare_zone_ids" {
  description = "Map of domain to Cloudflare Zone ID"
  type        = map(string)
  default = {
    "torale.ai"         = "643857070153106adc3aa071170d54fe"
    "bananagraph.com"   = "af3f2b2569cbaa24a2a2175d84b5a016"
    "a2aregistry.org"   = "9dbe037cf8e69ded608e718dd47c4d95"
    "repowire.io"       = "6e404d07f03aa873cbdeb88ad125ae51"
    "feedforward.space" = "dc8c906b65b389ce71a6f2908db94d54"
  }
}
