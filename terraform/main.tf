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

# SSL Certificates for Gateway (Google-managed)
module "ssl_cert_torale_prod" {
  source = "./modules/ssl-certificate"

  project_id       = var.project_id
  certificate_name = "torale-prod-cert"
  domains          = ["torale.ai", "api.torale.ai", "docs.torale.ai"]

  depends_on = [google_project_service.required_apis]
}

module "ssl_cert_torale_staging" {
  source = "./modules/ssl-certificate"

  project_id       = var.project_id
  certificate_name = "torale-staging-cert"
  domains          = ["staging.torale.ai", "api.staging.torale.ai"]

  depends_on = [google_project_service.required_apis]
}

module "ssl_cert_bananagraph_prod" {
  source = "./modules/ssl-certificate"

  project_id       = var.project_id
  certificate_name = "bananagraph-prod-cert"
  domains          = ["bananagraph.com", "api.bananagraph.com"]

  depends_on = [google_project_service.required_apis]
}

# Gateway API - Shared Gateway for all applications
module "gateway" {
  source = "./modules/gateway-api"

  gateway_name      = "clusterkit-gateway"
  gateway_namespace = "torale"
  static_ip_name    = var.static_ip_name

  ssl_certificate_names = [
    module.ssl_cert_torale_prod.certificate_name,
    module.ssl_cert_torale_staging.certificate_name,
    module.ssl_cert_bananagraph_prod.certificate_name,
  ]

  # Allow HTTPRoutes in torale namespace to reference services in torale-staging and bananagraph
  allowed_route_namespaces = ["torale-staging", "bananagraph"]

  depends_on = [
    module.gke,
    module.networking,
    module.ssl_cert_torale_prod,
    module.ssl_cert_torale_staging,
    module.ssl_cert_bananagraph_prod,
  ]
}
