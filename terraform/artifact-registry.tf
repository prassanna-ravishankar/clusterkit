# Artifact Registry - Shared container image repository
# Note: This repository already exists and is being imported into Terraform management

resource "google_artifact_registry_repository" "gcr" {
  repository_id = "gcr.io"
  location      = "us"
  format        = "DOCKER"
  project       = var.project_id

  # Cleanup policy: Keep recent images, delete old ones to save storage costs
  cleanup_policies {
    id     = "keep-recent-50"
    action = "KEEP"

    most_recent_versions {
      keep_count = 50  # Keep 50 most recent images across all packages
    }
  }

  cleanup_policies {
    id     = "delete-old"
    action = "DELETE"

    condition {
      older_than = "7776000s" # 90 days
    }
  }

  lifecycle {
    prevent_destroy = true
  }
}
