terraform {
  required_version = ">= 1.6.0"

  # Remote state in GCS — create bucket first with scripts/bootstrap-backend.sh
  # Then run: terraform init -migrate-state
  backend "gcs" {
    bucket = "tf-state-baldmaninc"
    prefix = "clusterkit/root"
  }

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 3.0"
    }
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.0"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
  }
}

# Configure Kubernetes provider to use GKE cluster credentials
provider "kubernetes" {
  host  = "https://${module.gke.cluster_endpoint}"
  token = data.google_client_config.default.access_token
  cluster_ca_certificate = base64decode(
    module.gke.cluster_ca_certificate,
  )
}

data "google_client_config" "default" {}

provider "cloudflare" {
  # Reads CLOUDFLARE_API_TOKEN from environment (via direnv + .env)
}
