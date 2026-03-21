variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "pool_id" {
  description = "Workload Identity Pool ID"
  type        = string
  default     = "github-actions"
}

variable "create_pool" {
  description = "Whether to create the pool (false if it already exists)"
  type        = bool
  default     = true
}

variable "github_org" {
  description = "GitHub organization or user that owns the repos"
  type        = string
}

variable "repos" {
  description = "Map of app name to GitHub repo config"
  type = map(object({
    repo = string # GitHub repo name (without org prefix)
  }))
}
