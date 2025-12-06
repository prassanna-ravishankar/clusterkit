output "certificate_name" {
  description = "Name of the created SSL certificate"
  value       = google_compute_managed_ssl_certificate.cert.name
}

output "certificate_id" {
  description = "ID of the SSL certificate"
  value       = google_compute_managed_ssl_certificate.cert.id
}

output "domains" {
  description = "Domains covered by this certificate"
  value       = google_compute_managed_ssl_certificate.cert.managed[0].domains
}
