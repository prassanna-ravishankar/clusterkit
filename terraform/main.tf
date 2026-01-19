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
  domains          = ["bananagraph.com", "www.bananagraph.com", "api.bananagraph.com"]

  depends_on = [google_project_service.required_apis]
}

module "ssl_cert_a2aregistry_beta" {
  source = "./modules/ssl-certificate"

  project_id       = var.project_id
  certificate_name = "a2aregistry-beta-cert"
  domains          = ["beta.a2aregistry.org"]

  depends_on = [google_project_service.required_apis]
}

module "ssl_cert_repowire_prod" {
  source = "./modules/ssl-certificate"

  project_id       = var.project_id
  certificate_name = "repowire-prod-cert"
  domains          = ["repowire.io"]

  depends_on = [google_project_service.required_apis]
}

# Gateway API - Shared Gateway for all applications
module "gateway" {
  source = "./modules/gateway-api"

  gateway_name      = "clusterkit-gateway"
  gateway_namespace = "clusterkit"
  static_ip_name    = var.static_ip_name

  ssl_certificate_names = [
    module.ssl_cert_torale_prod.certificate_name,
    module.ssl_cert_torale_staging.certificate_name,
    module.ssl_cert_bananagraph_prod.certificate_name,
    module.ssl_cert_a2aregistry_beta.certificate_name,
    module.ssl_cert_repowire_prod.certificate_name,
  ]

  # Allow HTTPRoutes in clusterkit namespace to reference services in app namespaces
  allowed_route_namespaces = ["torale", "torale-staging", "bananagraph", "a2aregistry", "repowire"]

  depends_on = [
    module.gke,
    module.networking,
    module.ssl_cert_torale_prod,
    module.ssl_cert_torale_staging,
    module.ssl_cert_bananagraph_prod,
    module.ssl_cert_a2aregistry_beta,
    module.ssl_cert_repowire_prod,
  ]
}

# Shared Cloud SQL Instance
module "cloudsql" {
  source = "./modules/cloudsql-instance"

  project_id       = var.project_id
  instance_name    = var.cloudsql_instance_name
  region           = var.region
  database_version = "POSTGRES_16"
  tier             = "db-f1-micro"

  ipv4_enabled    = true
  private_network = null

  backup_enabled                 = true
  backup_start_time              = "03:00"
  point_in_time_recovery_enabled = false
  transaction_log_retention_days = 7

  maintenance_window_day          = 7
  maintenance_window_hour         = 3
  maintenance_window_update_track = "stable"

  max_connections     = "100"
  deletion_protection = true

  databases = var.cloudsql_databases
  users     = var.cloudsql_users

  depends_on = [google_project_service.required_apis]
}

# Shared Cloud SQL Proxy Service Account
module "cloudsql_proxy_sa" {
  source = "./modules/cloudsql-proxy-sa"

  project_id         = var.project_id
  service_account_id = "cloudsql-proxy"
  display_name       = "Cloud SQL Proxy for GKE"

  enable_workload_identity = true
  k8s_namespace            = "torale"
  k8s_service_account      = "torale-sa"
}

# Workload Identity bindings for other namespaces
resource "google_service_account_iam_member" "workload_identity_torale_staging" {
  service_account_id = module.cloudsql_proxy_sa.service_account_name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[torale-staging/torale-sa]"
}

resource "google_service_account_iam_member" "workload_identity_torale_migrations" {
  service_account_id = module.cloudsql_proxy_sa.service_account_name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[torale/torale-sa-migrations]"
}

resource "google_service_account_iam_member" "workload_identity_torale_staging_migrations" {
  service_account_id = module.cloudsql_proxy_sa.service_account_name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[torale-staging/torale-sa-migrations]"
}

resource "google_service_account_iam_member" "workload_identity_a2aregistry" {
  service_account_id = module.cloudsql_proxy_sa.service_account_name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[a2aregistry/a2aregistry-sa]"
}

resource "google_service_account_iam_member" "workload_identity_bananagraph" {
  service_account_id = module.cloudsql_proxy_sa.service_account_name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[bananagraph/bananagraph-sa]"
}
