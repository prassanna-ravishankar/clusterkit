resource "google_logging_project_bucket_config" "default" {
  project        = var.project_id
  location       = "global"
  bucket_id      = "_Default"
  retention_days = var.retention_days
}

resource "google_logging_project_exclusion" "health_checks" {
  count = var.exclude_health_checks ? 1 : 0

  name   = "exclude-health-checks"
  filter = <<-EOT
    resource.type="k8s_container"
    (${join(" OR ", [for pattern in var.health_check_patterns : "textPayload=~\"${pattern}\""])})
  EOT
}

resource "google_logging_project_exclusion" "gke_noise" {
  count = var.exclude_gke_noise ? 1 : 0

  name   = "exclude-gke-system-noise"
  filter = <<-EOT
    logName=~"projects/${var.project_id}/logs/(gcfs-snapshotter|gcfsd|container-runtime)"
  EOT
}

resource "google_logging_project_exclusion" "sample_info_logs" {
  count = var.info_log_sample_rate < 1.0 ? 1 : 0

  name   = "sample-info-logs"
  filter = <<-EOT
    resource.type="k8s_container"
    severity<"ERROR"
    sample(insertId, ${var.info_log_sample_rate})
  EOT
}

resource "google_logging_project_exclusion" "custom" {
  for_each = var.custom_exclusions

  name   = each.key
  filter = each.value
}
