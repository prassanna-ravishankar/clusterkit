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

  # Cost optimization for side project
  enable_workload_logging = true                                          # Keep app logs for debugging
  monitoring_components   = ["SYSTEM_COMPONENTS", "POD", "DEPLOYMENT"]    # Minimal monitoring
  # Note: Managed Prometheus cannot be disabled in Autopilot (auto-enabled)

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

# Logging Optimization (Side Project Mode)
module "logging" {
  source = "./modules/logging"

  project_id = var.project_id

  # Aggressive cost optimization for side project
  retention_days         = 7
  exclude_health_checks  = true
  exclude_gke_noise      = true
  info_log_sample_rate   = 0.1 # Keep only 10% of INFO logs
  health_check_patterns  = ["/health", "/healthz", "/ready", "GET /health"]
}
