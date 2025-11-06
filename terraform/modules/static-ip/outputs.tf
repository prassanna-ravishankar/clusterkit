output "address" {
  description = "The static IP address"
  value       = google_compute_global_address.ingress.address
}

output "name" {
  description = "The static IP name"
  value       = google_compute_global_address.ingress.name
}

output "self_link" {
  description = "Self link of the static IP"
  value       = google_compute_global_address.ingress.self_link
}
