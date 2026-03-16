# Service Account for ExternalDNS
resource "google_service_account" "external_dns" {
  account_id   = "external-dns-${var.cluster_name}"
  display_name = "ExternalDNS Service Account for ${var.cluster_name}"
  description  = "Service account for ExternalDNS to manage DNS records in Cloud DNS"
  project      = var.project_id
}

# IAM role binding for ExternalDNS - DNS Admin
resource "google_project_iam_member" "external_dns_dns_admin" {
  project = var.project_id
  role    = "roles/dns.admin"
  member  = "serviceAccount:${google_service_account.external_dns.email}"
}

# Workload Identity binding for ExternalDNS
resource "google_service_account_iam_member" "external_dns_workload_identity" {
  service_account_id = google_service_account.external_dns.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[external-dns/external-dns]"
}
