# GitHub Actions Workload Identity Federation
#
# Allows GitHub Actions to authenticate to GCP without service account keys.
# Uses OIDC: GitHub Actions → Workload Identity Pool → GCP Service Account.

resource "google_iam_workload_identity_pool" "github" {
  count = var.create_pool ? 1 : 0

  project                   = var.project_id
  workload_identity_pool_id = var.pool_id
  display_name              = "GitHub Actions"
  description               = "Workload Identity Pool for GitHub Actions OIDC"
}

resource "google_iam_workload_identity_pool_provider" "github" {
  count = var.create_pool ? 1 : 0

  project                            = var.project_id
  workload_identity_pool_id          = var.create_pool ? google_iam_workload_identity_pool.github[0].workload_identity_pool_id : var.pool_id
  workload_identity_pool_provider_id = "github-actions"
  display_name                       = "GitHub Actions OIDC"

  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.actor"      = "assertion.actor"
    "attribute.repository" = "assertion.repository"
  }

  attribute_condition = "assertion.repository_owner == '${var.github_org}'"

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }
}

# Per-repo service accounts and bindings
resource "google_service_account" "github_deploy" {
  for_each = var.repos

  project      = var.project_id
  account_id   = "gh-deploy-${each.key}"
  display_name = "GitHub Actions deploy for ${each.key}"
}

# Allow the GitHub repo to impersonate the service account via WIF
resource "google_service_account_iam_member" "github_wif_binding" {
  for_each = var.repos

  service_account_id = google_service_account.github_deploy[each.key].name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${local.pool_name}/attribute.repository/${var.github_org}/${each.value.repo}"
}

# Grant Artifact Registry push access
resource "google_project_iam_member" "artifact_registry_writer" {
  for_each = var.repos

  project = var.project_id
  role    = "roles/artifactregistry.writer"
  member  = "serviceAccount:${google_service_account.github_deploy[each.key].email}"
}

# Grant GKE deploy access
resource "google_project_iam_member" "gke_developer" {
  for_each = var.repos

  project = var.project_id
  role    = "roles/container.developer"
  member  = "serviceAccount:${google_service_account.github_deploy[each.key].email}"
}

locals {
  pool_name = var.create_pool ? google_iam_workload_identity_pool.github[0].name : "projects/${var.project_id}/locations/global/workloadIdentityPools/${var.pool_id}"
}
