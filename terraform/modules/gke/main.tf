resource "google_container_cluster" "primary" {
  name     = var.cluster_name
  location = var.region

  # Enable Autopilot mode
  enable_autopilot = true

  # Autopilot clusters are configured automatically, but we can set some preferences
  release_channel {
    channel = "REGULAR"
  }

  # Minimum version constraint
  min_master_version = var.kubernetes_version

  # Network configuration
  network    = "default"
  subnetwork = "default"

  # IP allocation for pods and services
  ip_allocation_policy {
    cluster_ipv4_cidr_block  = ""
    services_ipv4_cidr_block = ""
  }

  # Workload Identity for secure pod-to-GCP service authentication
  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }

  # Enable Binary Authorization for enhanced security
  binary_authorization {
    evaluation_mode = "PROJECT_SINGLETON_POLICY_ENFORCE"
  }

  # Enable Shielded Nodes for security
  enable_shielded_nodes = true

  # Maintenance window
  maintenance_policy {
    daily_maintenance_window {
      start_time = "03:00" # 3 AM UTC
    }
  }

  # Deletion protection
  deletion_protection = var.deletion_protection

  # Labels for organization
  resource_labels = {
    environment = var.environment
    managed-by  = "terraform"
    project     = "clusterkit"
  }

  # Logging and monitoring configuration
  logging_config {
    enable_components = ["SYSTEM_COMPONENTS", "WORKLOADS"]
  }

  monitoring_config {
    enable_components = ["SYSTEM_COMPONENTS"]
    managed_prometheus {
      enabled = true
    }
  }
}
