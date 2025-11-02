output "static_ip_address" {
  description = "The static IP address for the LoadBalancer"
  value       = google_compute_global_address.ingress_ip.address
}

output "static_ip_name" {
  description = "The name of the static IP resource"
  value       = google_compute_global_address.ingress_ip.name
}

output "static_ip_id" {
  description = "The ID of the static IP resource"
  value       = google_compute_global_address.ingress_ip.id
}
