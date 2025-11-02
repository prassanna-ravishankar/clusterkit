# Enable required Google Cloud APIs
resource "google_project_service" "required_apis" {
  for_each = toset([
    "container.googleapis.com",      # GKE
    "compute.googleapis.com",        # Compute Engine (for static IPs)
    "iam.googleapis.com",            # IAM
    "cloudresourcemanager.googleapis.com", # Resource Manager
    "dns.googleapis.com",            # Cloud DNS (for ExternalDNS)
  ])

  service            = each.value
  disable_on_destroy = false
}

# GKE Autopilot Cluster Module
module "gke" {
  source = "./modules/gke"

  project_id         = var.project_id
  region             = var.region
  cluster_name       = var.cluster_name
  kubernetes_version = var.kubernetes_version

  depends_on = [google_project_service.required_apis]
}

# Static IP for LoadBalancer
module "networking" {
  source = "./modules/networking"

  project_id     = var.project_id
  region         = var.region
  static_ip_name = var.static_ip_name

  depends_on = [google_project_service.required_apis]
}

# IAM Service Accounts for ExternalDNS and cert-manager
module "iam" {
  source = "./modules/iam"

  project_id   = var.project_id
  cluster_name = var.cluster_name

  depends_on = [module.gke]
}
