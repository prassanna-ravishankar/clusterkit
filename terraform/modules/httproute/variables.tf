variable "route_name" {
  description = "Name of the HTTPRoute resource"
  type        = string
}

variable "route_namespace" {
  description = "Namespace where the HTTPRoute will be created (should match Gateway namespace for cross-namespace routing)"
  type        = string
}

variable "gateway_name" {
  description = "Name of the Gateway to attach to"
  type        = string
}

variable "gateway_namespace" {
  description = "Namespace of the Gateway"
  type        = string
}

variable "hostnames" {
  description = "List of hostnames this route should match"
  type        = list(string)
}

variable "service_name" {
  description = "Name of the backend Service"
  type        = string
}

variable "service_namespace" {
  description = "Namespace of the backend Service (can be different from route_namespace for cross-namespace routing)"
  type        = string
}

variable "service_port" {
  description = "Port of the backend Service"
  type        = number
}

variable "annotations" {
  description = "Additional annotations to add to the HTTPRoute"
  type        = map(string)
  default     = {}
}
