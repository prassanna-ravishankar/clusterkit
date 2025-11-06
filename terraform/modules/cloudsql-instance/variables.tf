variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "instance_name" {
  description = "Cloud SQL instance name"
  type        = string
}

variable "region" {
  description = "GCP region for Cloud SQL instance"
  type        = string
  default     = "us-central1"
}

variable "database_version" {
  description = "Database version (e.g., POSTGRES_16, MYSQL_8_0)"
  type        = string
  default     = "POSTGRES_16"
}

variable "tier" {
  description = "Machine type tier (e.g., db-custom-1-3840)"
  type        = string
  default     = "db-custom-1-3840"
}

variable "availability_type" {
  description = "Availability type (ZONAL or REGIONAL)"
  type        = string
  default     = "ZONAL"
}

variable "disk_size" {
  description = "Disk size in GB"
  type        = number
  default     = 10
}

variable "disk_type" {
  description = "Disk type (PD_SSD or PD_HDD)"
  type        = string
  default     = "PD_SSD"
}

variable "ipv4_enabled" {
  description = "Enable public IPv4"
  type        = bool
  default     = true
}

variable "private_network" {
  description = "VPC network for private IP (optional)"
  type        = string
  default     = null
}


variable "backup_enabled" {
  description = "Enable automated backups"
  type        = bool
  default     = true
}

variable "backup_start_time" {
  description = "Backup start time (HH:MM)"
  type        = string
  default     = "03:00"
}

variable "point_in_time_recovery_enabled" {
  description = "Enable point-in-time recovery"
  type        = bool
  default     = true
}

variable "transaction_log_retention_days" {
  description = "Transaction log retention days"
  type        = number
  default     = 7
}

variable "maintenance_window_day" {
  description = "Maintenance window day (1-7, Sunday = 7)"
  type        = number
  default     = 7
}

variable "maintenance_window_hour" {
  description = "Maintenance window hour (0-23)"
  type        = number
  default     = 3
}

variable "maintenance_window_update_track" {
  description = "Maintenance update track (stable or canary)"
  type        = string
  default     = "stable"
}

variable "max_connections" {
  description = "Maximum number of connections"
  type        = string
  default     = "100"
}

variable "deletion_protection" {
  description = "Enable deletion protection"
  type        = bool
  default     = true
}

variable "databases" {
  description = "List of database names to create"
  type        = list(string)
  default     = []
}

variable "users" {
  description = "Map of database users and their passwords"
  type = map(object({
    password = string
  }))
  default = {}
}
