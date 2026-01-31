output "record_ids" {
  description = "Map of record name to Cloudflare record ID"
  value       = { for k, r in cloudflare_record.records : k => r.id }
}
