output "route_name" {
  description = "Name of the created HTTPRoute"
  value       = kubernetes_manifest.httproute.manifest.metadata.name
}

output "route_namespace" {
  description = "Namespace of the created HTTPRoute"
  value       = kubernetes_manifest.httproute.manifest.metadata.namespace
}

output "hostnames" {
  description = "Hostnames configured for this route"
  value       = kubernetes_manifest.httproute.manifest.spec.hostnames
}
