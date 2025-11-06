resource "google_compute_global_address" "ingress" {
  project     = var.project_id
  name        = var.address_name
  description = var.description
}
