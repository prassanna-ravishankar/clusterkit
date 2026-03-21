output "pool_name" {
  description = "Full resource name of the Workload Identity Pool"
  value       = local.pool_name
}

output "provider_name" {
  description = "Full resource name of the WIF provider (use in GitHub Actions auth)"
  value       = var.create_pool ? google_iam_workload_identity_pool_provider.github[0].name : "${local.pool_name}/providers/github-actions"
}

output "service_account_emails" {
  description = "Map of app name to deploy service account email"
  value       = { for k, sa in google_service_account.github_deploy : k => sa.email }
}
