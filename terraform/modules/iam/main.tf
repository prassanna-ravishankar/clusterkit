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

# Service Account for cert-manager
resource "google_service_account" "cert_manager" {
  account_id   = "cert-manager-${var.cluster_name}"
  display_name = "cert-manager Service Account for ${var.cluster_name}"
  description  = "Service account for cert-manager to manage DNS records for DNS-01 challenges"
  project      = var.project_id
}

# IAM role binding for cert-manager - DNS Admin (for DNS-01 challenges)
resource "google_project_iam_member" "cert_manager_dns_admin" {
  project = var.project_id
  role    = "roles/dns.admin"
  member  = "serviceAccount:${google_service_account.cert_manager.email}"
}

# Workload Identity binding for cert-manager
resource "google_service_account_iam_member" "cert_manager_workload_identity" {
  service_account_id = google_service_account.cert_manager.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[cert-manager/cert-manager]"
}

# Optional: Create service account keys for non-Workload Identity setups
# Only create if explicitly enabled via variable
resource "google_service_account_key" "external_dns_key" {
  count              = var.create_service_account_keys ? 1 : 0
  service_account_id = google_service_account.external_dns.name
}

resource "google_service_account_key" "cert_manager_key" {
  count              = var.create_service_account_keys ? 1 : 0
  service_account_id = google_service_account.cert_manager.name
}

# Save keys to local files if created (for Cloudflare provider usage)
resource "local_file" "external_dns_key" {
  count           = var.create_service_account_keys ? 1 : 0
  content         = base64decode(google_service_account_key.external_dns_key[0].private_key)
  filename        = "${path.root}/keys/external-dns-key.json"
  file_permission = "0600"
}

resource "local_file" "cert_manager_key" {
  count           = var.create_service_account_keys ? 1 : 0
  content         = base64decode(google_service_account_key.cert_manager_key[0].private_key)
  filename        = "${path.root}/keys/cert-manager-key.json"
  file_permission = "0600"
}
