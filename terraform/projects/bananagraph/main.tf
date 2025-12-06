# Reference existing CloudSQL instance
data "google_sql_database_instance" "clusterkit_db" {
  name    = var.cloudsql_instance_name
  project = var.project_id
}

# Create database in existing instance
resource "google_sql_database" "bananagraph" {
  name     = var.database_name
  instance = data.google_sql_database_instance.clusterkit_db.name
  project  = var.project_id
}

# Create database user
resource "google_sql_user" "bananagraph" {
  name     = var.database_user
  instance = data.google_sql_database_instance.clusterkit_db.name
  project  = var.project_id
  password = var.database_password
}

# GCS bucket for assets
resource "google_storage_bucket" "assets" {
  name          = var.gcs_bucket_name
  project       = var.project_id
  location      = "US"
  storage_class = "STANDARD"
  force_destroy = false

  uniform_bucket_level_access = true

  cors {
    origin          = ["*"]
    method          = ["GET", "HEAD"]
    response_header = ["Content-Type"]
    max_age_seconds = 3600
  }
}

# Public read access to bucket
resource "google_storage_bucket_iam_member" "public_read" {
  bucket = google_storage_bucket.assets.name
  role   = "roles/storage.objectViewer"
  member = "allUsers"
}

# Service account for Workload Identity
resource "google_service_account" "bananagraph_sa" {
  account_id   = "bananagraph-sa"
  display_name = "Bananagraph Service Account"
  project      = var.project_id
}

# Grant CloudSQL client role to service account
resource "google_project_iam_member" "cloudsql_client" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.bananagraph_sa.email}"
}

# Grant GCS objectAdmin to service account
resource "google_storage_bucket_iam_member" "workload_identity_object_admin" {
  bucket = google_storage_bucket.assets.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.bananagraph_sa.email}"
}

# Workload Identity binding
resource "google_service_account_iam_member" "workload_identity" {
  service_account_id = google_service_account.bananagraph_sa.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[${var.k8s_namespace}/${var.k8s_service_account}]"
}
