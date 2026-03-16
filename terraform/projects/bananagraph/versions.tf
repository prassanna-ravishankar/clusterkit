terraform {
  required_version = ">= 1.9"

  backend "gcs" {
    bucket = "tf-state-baldmaninc"
    prefix = "clusterkit/projects/bananagraph"
  }

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}
