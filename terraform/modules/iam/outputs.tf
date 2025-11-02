output "external_dns_service_account_email" {
  description = "Email of the ExternalDNS service account"
  value       = google_service_account.external_dns.email
}

output "external_dns_service_account_name" {
  description = "Name of the ExternalDNS service account"
  value       = google_service_account.external_dns.name
}

output "cert_manager_service_account_email" {
  description = "Email of the cert-manager service account"
  value       = google_service_account.cert_manager.email
}

output "cert_manager_service_account_name" {
  description = "Name of the cert-manager service account"
  value       = google_service_account.cert_manager.name
}

output "external_dns_key_path" {
  description = "Path to the ExternalDNS service account key file (if created)"
  value       = var.create_service_account_keys ? "${path.root}/keys/external-dns-key.json" : null
}

output "cert_manager_key_path" {
  description = "Path to the cert-manager service account key file (if created)"
  value       = var.create_service_account_keys ? "${path.root}/keys/cert-manager-key.json" : null
}
