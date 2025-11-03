# Cloudflare Module Variables

variable "zone_id" {
  description = "Cloudflare Zone ID"
  type        = string
}

variable "domain" {
  description = "Domain name (e.g., example.com)"
  type        = string
}

variable "dynamic_cache_ttl" {
  description = "Edge cache TTL for dynamic content in seconds"
  type        = number
  default     = 300  # 5 minutes
}

variable "browser_cache_ttl" {
  description = "Browser cache TTL in seconds"
  type        = number
  default     = 1800  # 30 minutes
}

variable "api_rate_limit_threshold" {
  description = "Number of requests allowed per period for API endpoints"
  type        = number
  default     = 10
}

variable "api_rate_limit_period" {
  description = "Time period in seconds for rate limiting"
  type        = number
  default     = 60  # 1 minute
}

variable "security_level" {
  description = "Cloudflare security level (off, essentially_off, low, medium, high, under_attack)"
  type        = string
  default     = "medium"
  validation {
    condition     = contains(["off", "essentially_off", "low", "medium", "high", "under_attack"], var.security_level)
    error_message = "Security level must be one of: off, essentially_off, low, medium, high, under_attack"
  }
}
