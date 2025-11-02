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

# Output the IP for use in DNS configuration
resource "null_resource" "display_ip" {
  triggers = {
    ip_address = google_compute_global_address.ingress_ip.address
  }

  provisioner "local-exec" {
    command = "echo 'Static IP Address: ${google_compute_global_address.ingress_ip.address}'"
  }
}
