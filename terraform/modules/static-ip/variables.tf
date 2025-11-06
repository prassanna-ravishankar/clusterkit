variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "address_name" {
  description = "Name for the global static IP address"
  type        = string
}

variable "description" {
  description = "Description for the static IP address"
  type        = string
  default     = null
}
