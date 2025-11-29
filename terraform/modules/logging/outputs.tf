output "bucket_id" {
  description = "The ID of the configured log bucket"
  value       = google_logging_project_bucket_config.default.id
}

output "bucket_retention_days" {
  description = "Number of days logs are retained"
  value       = google_logging_project_bucket_config.default.retention_days
}

output "exclusion_ids" {
  description = "List of created log exclusion IDs"
  value = concat(
    google_logging_project_exclusion.health_checks[*].id,
    google_logging_project_exclusion.gke_noise[*].id,
    google_logging_project_exclusion.sample_info_logs[*].id,
    [for exclusion in google_logging_project_exclusion.custom : exclusion.id]
  )
}

output "enabled_optimizations" {
  description = "Summary of enabled logging optimizations"
  value = {
    retention_days         = var.retention_days
    health_checks_excluded = var.exclude_health_checks
    gke_noise_excluded     = var.exclude_gke_noise
    info_sample_rate       = var.info_log_sample_rate
    custom_exclusions      = length(var.custom_exclusions)
  }
}
