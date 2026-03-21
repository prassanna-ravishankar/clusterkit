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
  default   = {}
  sensitive = true
}

# App namespaces that need cross-namespace routing (ReferenceGrants from clusterkit → these)
variable "app_namespaces" {
  description = "Kubernetes namespaces that need ReferenceGrants for cross-namespace Gateway routing"
  type        = list(string)
  default     = ["torale", "torale-staging", "bananagraph", "a2aregistry", "repowire", "agentdance"]
}

# Workload Identity bindings for Cloud SQL proxy access
variable "cloudsql_workload_identity_bindings" {
  description = "K8s service accounts that need Cloud SQL proxy access via Workload Identity"
  type = map(object({
    namespace       = string
    service_account = string
  }))
  default = {
    torale                    = { namespace = "torale", service_account = "torale-sa" }
    torale-staging            = { namespace = "torale-staging", service_account = "torale-sa" }
    torale-migrations         = { namespace = "torale", service_account = "torale-sa-migrations" }
    torale-staging-migrations = { namespace = "torale-staging", service_account = "torale-sa-migrations" }
    a2aregistry               = { namespace = "a2aregistry", service_account = "a2aregistry-sa" }
    bananagraph               = { namespace = "bananagraph", service_account = "bananagraph-sa" }
  }
}

# Per-domain Cloudflare setting overrides (merged on top of defaults)
variable "cloudflare_domain_settings" {
  description = "Per-domain Cloudflare zone setting overrides"
  type        = map(map(string))
  default = {}
}

# Domains that get Cloudflare Origin CA wildcard certs on the Gateway
variable "origin_ca_domains" {
  description = "Domains to generate Cloudflare Origin CA wildcard certs for"
  type        = list(string)
  default     = ["torale.ai", "bananagraph.com", "a2aregistry.org", "repowire.io", "agentdance.ai"]
}

# All Cloudflare-managed domains (superset of origin_ca_domains — includes dns.tf-only domains)
variable "cloudflare_domains" {
  description = "All domains managed in Cloudflare (zone IDs looked up automatically)"
  type        = list(string)
  default     = ["torale.ai", "bananagraph.com", "a2aregistry.org", "repowire.io", "feedforward.space", "agentdance.ai"]
}
