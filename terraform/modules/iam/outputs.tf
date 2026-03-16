output "external_dns_service_account_email" {
  description = "Email of the ExternalDNS service account"
  value       = google_service_account.external_dns.email
}

output "external_dns_service_account_name" {
  description = "Name of the ExternalDNS service account"
  value       = google_service_account.external_dns.name
}
