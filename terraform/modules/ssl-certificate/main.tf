/**
 * Google-managed SSL Certificate Module
 *
 * Creates a Google-managed SSL certificate for GKE Gateway.
 * Certificate provisioning is automatic and handles renewal.
 */

resource "google_compute_managed_ssl_certificate" "cert" {
  name    = var.certificate_name
  project = var.project_id

  managed {
    domains = var.domains
  }

  lifecycle {
    create_before_destroy = true
  }
}
