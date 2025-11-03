# Cloudflare Configuration Module for ClusterKit
# Manages page rules, caching, security rules, and rate limiting

terraform {
  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.0"
    }
  }
}

# Page Rule: Static Asset Caching
resource "cloudflare_page_rule" "static_assets_cache" {
  zone_id  = var.zone_id
  target   = "${var.domain}/*.(css|js|jpg|jpeg|png|gif|ico|svg|woff|woff2|ttf|eot|otf)"
  priority = 1

  actions {
    cache_level         = "cache_everything"
    edge_cache_ttl      = 2592000  # 30 days
    browser_cache_ttl   = 2592000  # 30 days
  }
}

# Page Rule: Dynamic Content Caching
resource "cloudflare_page_rule" "dynamic_cache" {
  zone_id  = var.zone_id
  target   = "${var.domain}/*"
  priority = 2

  actions {
    cache_level       = "standard"
    edge_cache_ttl    = var.dynamic_cache_ttl
    browser_cache_ttl = var.browser_cache_ttl
  }
}

# Page Rule: API Bypass Caching
resource "cloudflare_page_rule" "api_bypass" {
  zone_id  = var.zone_id
  target   = "${var.domain}/api/*"
  priority = 3

  actions {
    cache_level = "bypass"
  }
}

# Rate Limiting: API Protection
resource "cloudflare_rate_limit" "api_rate_limit" {
  zone_id   = var.zone_id
  threshold = var.api_rate_limit_threshold
  period    = var.api_rate_limit_period
  match {
    request {
      url_pattern = "${var.domain}/api/*"
    }
  }
  action {
    mode    = "challenge"
    timeout = 60
  }
  description = "Rate limit for API endpoints"
}

# Firewall Rule: Basic DDoS Protection
resource "cloudflare_filter" "ddos_protection" {
  zone_id     = var.zone_id
  description = "Basic DDoS protection - block known bad bots"
  expression  = "(cf.threat_score > 10)"
}

resource "cloudflare_firewall_rule" "ddos_protection" {
  zone_id     = var.zone_id
  description = "Block requests with threat score > 10"
  filter_id   = cloudflare_filter.ddos_protection.id
  action      = "challenge"
  priority    = 1
}

# Firewall Rule: Block Known Malicious IPs
resource "cloudflare_filter" "block_malicious" {
  zone_id     = var.zone_id
  description = "Block known malicious IP addresses"
  expression  = "(cf.threat_score > 50)"
}

resource "cloudflare_firewall_rule" "block_malicious" {
  zone_id     = var.zone_id
  description = "Block highly threatening requests"
  filter_id   = cloudflare_filter.block_malicious.id
  action      = "block"
  priority    = 2
}

# WAF Managed Rules (if on paid plan)
# resource "cloudflare_waf_group" "owasp" {
#   zone_id  = var.zone_id
#   group_id = "de677e5818985db1285d0e80225f06e5"  # OWASP ModSecurity Core Rule Set
#   mode     = "on"
# }

# Zone Settings for Optimal Performance
resource "cloudflare_zone_settings_override" "settings" {
  zone_id = var.zone_id

  settings {
    # SSL/TLS
    ssl                      = "strict"  # Requires valid cert on origin
    always_use_https         = "on"
    min_tls_version          = "1.2"
    tls_1_3                  = "on"
    automatic_https_rewrites = "on"

    # Security
    security_level           = var.security_level
    challenge_ttl            = 1800
    browser_check            = "on"
    privacy_pass             = "on"

    # Performance
    brotli                   = "on"
    minify {
      css  = "on"
      js   = "on"
      html = "on"
    }
    rocket_loader            = "off"  # Can break some apps
    http2                    = "on"
    http3                    = "on"

    # Caching
    browser_cache_ttl        = var.browser_cache_ttl
    cache_level              = "aggressive"

    # IPv6
    ipv6                     = "on"

    # Bot Management
    bot_management {
      enable_js = true
    }
  }
}
