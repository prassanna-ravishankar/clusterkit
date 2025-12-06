output "gateway_name" {
  description = "Name of the created Gateway"
  value       = try(kubernetes_manifest.gateway.object.metadata.name, var.gateway_name)
}

output "gateway_namespace" {
  description = "Namespace of the created Gateway"
  value       = try(kubernetes_manifest.gateway.object.metadata.namespace, var.gateway_namespace)
}

output "gateway_class" {
  description = "Gateway class name"
  value       = try(kubernetes_manifest.gateway.object.spec.gatewayClassName, "gke-l7-global-external-managed")
}
