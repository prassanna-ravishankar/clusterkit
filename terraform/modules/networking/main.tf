resource "google_compute_global_address" "ingress_ip" {
  name         = var.static_ip_name
  project      = var.project_id
  address_type = "EXTERNAL"
  ip_version   = "IPV4"
  description  = "Static IP for ClusterKit ingress LoadBalancer"

  labels = {
    environment = var.environment
    managed-by  = "terraform"
    purpose     = "ingress-loadbalancer"
  }
}
