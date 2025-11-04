resource "google_container_cluster" "primary" {
  name     = var.cluster_name
  location = var.region

  # Enable Autopilot mode - Google manages all infrastructure automatically
  enable_autopilot = true

  # Release channel for automatic updates
  release_channel {
    channel = "REGULAR"
  }

  # Network configuration (use default VPC for simplicity)
  network    = "default"
  subnetwork = "default"

  # IP allocation for pods and services (empty = auto-assign)
  ip_allocation_policy {
    cluster_ipv4_cidr_block  = ""
    services_ipv4_cidr_block = ""
  }

  # Deletion protection
  deletion_protection = var.deletion_protection

  # Labels for organization
  resource_labels = {
    environment = var.environment
    managed-by  = "terraform"
    project     = "clusterkit"
  }

  # Note: The following are automatically configured in Autopilot mode:
  # - Shielded Nodes (enabled by default)
  # - Workload Identity (auto-configured)
  # - Logging and Monitoring (SYSTEM_COMPONENTS enabled)
  # - Binary Authorization (configurable via GCP console if needed)
  # - Node auto-provisioning, scaling, and repair
  # - Security patches and upgrades
  # Note: Spot Pods can be used for cost savings (~60-91% off) by adding
  # nodeSelector: cloud.google.com/gke-spot: "true" to workload manifests
}
