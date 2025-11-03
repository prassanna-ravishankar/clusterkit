# Cloudflare Module Outputs

output "page_rule_ids" {
  description = "IDs of created page rules"
  value = {
    static_assets = cloudflare_page_rule.static_assets_cache.id
    dynamic       = cloudflare_page_rule.dynamic_cache.id
    api_bypass    = cloudflare_page_rule.api_bypass.id
  }
}

output "rate_limit_id" {
  description = "ID of the API rate limit rule"
  value       = cloudflare_rate_limit.api_rate_limit.id
}

output "firewall_rule_ids" {
  description = "IDs of created firewall rules"
  value = {
    ddos_protection  = cloudflare_firewall_rule.ddos_protection.id
    block_malicious  = cloudflare_firewall_rule.block_malicious.id
  }
}
