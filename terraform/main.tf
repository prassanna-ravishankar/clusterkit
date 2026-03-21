# Enable required Google Cloud APIs
resource "google_project_service" "required_apis" {
  for_each = toset([
    "container.googleapis.com",            # GKE
    "compute.googleapis.com",              # Compute Engine (for static IPs)
    "iam.googleapis.com",                  # IAM
    "cloudresourcemanager.googleapis.com", # Resource Manager
    "sqladmin.googleapis.com",             # Cloud SQL Admin
    "artifactregistry.googleapis.com",     # Artifact Registry
    "iamcredentials.googleapis.com",       # Workload Identity Federation
    "sts.googleapis.com",                  # Security Token Service (WIF)
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
  enable_workload_logging = true # Keep app logs for debugging
  monitoring_components   = ["SYSTEM_COMPONENTS", "POD"]
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

# Logging Optimization (Side Project Mode)
module "logging" {
  source = "./modules/logging"

  project_id = var.project_id

  # Aggressive cost optimization for side project
  retention_days        = 7
  exclude_health_checks = true
  exclude_gke_noise     = true
  info_log_sample_rate  = 0.1 # Keep only 10% of INFO logs
  health_check_patterns = ["/health", "/healthz", "/ready", "GET /health"]

  custom_exclusions = {
    "exclude-external-dns-noise" = <<-EOT
      resource.type="k8s_container"
      resource.labels.namespace_name="external-dns"
      severity="INFO"
    EOT
  }
}

# SSL Certificates for Gateway (Cloudflare Origin CA — wildcard per domain)
# Keys and certs are generated entirely in Terraform, no manual steps needed.

resource "tls_private_key" "origin_ca" {
  for_each  = toset(var.origin_ca_domains)
  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_cert_request" "origin_ca" {
  for_each        = toset(var.origin_ca_domains)
  private_key_pem = tls_private_key.origin_ca[each.key].private_key_pem

  subject {
    common_name = each.key
  }
}

resource "cloudflare_origin_ca_certificate" "origin_ca" {
  for_each           = toset(var.origin_ca_domains)
  csr                = tls_cert_request.origin_ca[each.key].cert_request_pem
  hostnames          = [each.key, "*.${each.key}"]
  request_type       = "origin-rsa"
  requested_validity = 5475 # 15 years
}

resource "google_compute_ssl_certificate" "origin_ca" {
  for_each = toset(var.origin_ca_domains)

  name        = "${replace(each.key, ".", "-")}-origin-cert"
  project     = var.project_id
  certificate = cloudflare_origin_ca_certificate.origin_ca[each.key].certificate
  private_key = tls_private_key.origin_ca[each.key].private_key_pem

  lifecycle {
    create_before_destroy = true
  }
}

# Cloudflare zone security settings — HTTPS enforcement and TLS hardening.
# Origin CA domains get Full (Strict) to validate the cert on Gateway.
# Other domains (e.g. Cloudflare Pages) get Full (sufficient for Pages-hosted sites).
# Per-domain overrides (e.g. HTTP/3 off for SSE) via var.cloudflare_domain_settings.
resource "cloudflare_zone_settings_override" "zone_settings" {
  for_each = {
    for domain in var.cloudflare_domains :
    domain => local.cloudflare_zone_ids[domain]
    if contains(keys(local.cloudflare_zone_ids), domain)
  }
  zone_id = each.value
  settings {
    ssl              = contains(var.origin_ca_domains, each.key) ? "strict" : "full"
    always_use_https = "on"
    min_tls_version  = "1.2"
    tls_1_3          = "on"
    http3            = lookup(lookup(var.cloudflare_domain_settings, each.key, {}), "http3", "on")
  }
}

# Gateway API - Shared Gateway for all applications
module "gateway" {
  source = "./modules/gateway-api"

  gateway_name      = "clusterkit-gateway"
  gateway_namespace = "clusterkit"
  static_ip_name    = var.static_ip_name

  ssl_certificate_names = [for cert in google_compute_ssl_certificate.origin_ca : cert.name]

  allowed_route_namespaces = var.app_namespaces

  depends_on = [
    module.gke,
    module.networking,
    google_compute_ssl_certificate.origin_ca,
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
  transaction_log_retention_days = 1 # Minimum — PITR is disabled, no need to retain

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

  enable_workload_identity = false # Bindings managed below via for_each
}

# GitHub Actions Workload Identity Federation (keyless CI/CD)
module "github_wif" {
  source = "./modules/github-wif"

  project_id = var.project_id
  github_org = var.github_org
  repos      = var.github_deploy_repos

  depends_on = [google_project_service.required_apis]
}

# Workload Identity bindings for Cloud SQL proxy access
resource "google_service_account_iam_member" "cloudsql_workload_identity" {
  for_each = var.cloudsql_workload_identity_bindings

  service_account_id = module.cloudsql_proxy_sa.service_account_name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[${each.value.namespace}/${each.value.service_account}]"
}

