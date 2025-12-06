variable "gateway_name" {
  description = "Name of the Gateway resource"
  type        = string
  default     = "clusterkit-gateway"
}

variable "gateway_namespace" {
  description = "Namespace where the Gateway will be created"
  type        = string
}

variable "static_ip_name" {
  description = "Name of the GCP static IP address resource (must already exist)"
  type        = string
}

variable "ssl_certificate_names" {
  description = "List of Google-managed SSL certificate names (must already exist)"
  type        = list(string)
}

variable "allowed_route_namespaces" {
  description = "List of namespaces that can attach HTTPRoutes to this Gateway. ReferenceGrants will be created for cross-namespace service access."
  type        = list(string)
  default     = []
}
