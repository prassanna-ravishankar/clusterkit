# Cloudflare DNS Records - Single source of truth for all domains
#
# Provider v4: root records use full domain name, subdomains use short name.
# ExternalDNS TXT ownership records (heritage=external-dns) are NOT managed here.

locals {
  gateway_ip = module.networking.static_ip_address
}

# torale.ai
module "dns_torale" {
  source  = "./modules/cloudflare-dns"
  zone_id = var.cloudflare_zone_ids["torale.ai"]

  records = [
    # Gateway A records
    { key = "root", name = "torale.ai", content = local.gateway_ip },
    { name = "api", content = local.gateway_ip },
    { name = "docs", content = local.gateway_ip },
    { name = "staging", content = local.gateway_ip },
    { name = "api.staging", content = local.gateway_ip },

    # Clerk authentication
    { name = "accounts", content = "accounts.clerk.services", type = "CNAME" },
    { name = "clerk", content = "frontend-api.clerk.services", type = "CNAME" },
    { name = "clk._domainkey", content = "dkim1.kdbjwdvfrrrp.clerk.services", type = "CNAME" },
    { name = "clk2._domainkey", content = "dkim2.kdbjwdvfrrrp.clerk.services", type = "CNAME" },
    { name = "clkmail", content = "mail.kdbjwdvfrrrp.clerk.services", type = "CNAME" },

    # Google Workspace
    { name = "app", content = "ghs.googlehosted.com", type = "CNAME" },
    { key = "mx-google", name = "torale.ai", content = "smtp.google.com", type = "MX", priority = 1 },

    # Resend (transactional email via SES)
    { key = "mx-ses", name = "send", content = "feedback-smtp.eu-west-1.amazonses.com", type = "MX", priority = 10 },
    { key = "txt-send-spf", name = "send", content = "v=spf1 include:amazonses.com ~all", type = "TXT" },
    { key = "txt-resend-dkim", name = "resend._domainkey", content = "p=MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDUGDelSMuhI3U6xe9D8l7Y7w19TZQz9qwikptfdwPNILwlPBG2W8Bwj1EjMT9U7KmuiLdWshVKjPb0b9VU+TL0BVfye+zytg4JNWbIDXSBs4bfin4xBihl/UspAnIs2w4V74NEaZ6c9nxTlkQhAKWSmr7eHsg7ylDHk3pwDbuA+QIDAQAB", type = "TXT" },

    # Email authentication
    { key = "txt-dmarc", name = "_dmarc", content = "v=DMARC1; p=none; rua=mailto:me@prassanna.io", type = "TXT" },
    { key = "txt-google-dkim", name = "google._domainkey", content = "v=DKIM1; k=rsa; p=MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAjhJ0mrKtg15mrSUCXK8kp7+20IJOMFc3TSS+F4p0DuGklsJ/CCOt3siFxjwbVYOc22auf8HtALUqh80p2gEIFzbkbKcYGWWCgG+grecNZZBZvG3igAwRukmmlNY0eImxr4iWZsxP2/LLYxzCj295lQUJ0DXV51QlldccTUwvvAXx+qkVnRUb5o0Ejx5ZCM7SWSut/a/wvuiPjEHZ48MwTSax7Qi5MQurLadRjAWS/LBhvv6i/RbhApidiAGbFQEf+9s9QgodXTQQ0hoDpXlhrZ5pfJb4oqhN+DhnxiJSbdu4dya9mNkvs+8a2WtIqQvdtPLu7G9VwxWQKaIeFqERyQIDAQAB", type = "TXT" },

    # Google site verification
    { key = "txt-google-verify", name = "torale.ai", content = "google-site-verification=cJcMetAHB1YEXVfvXUkgt_WvknMimyDOs9L5lew61P4", type = "TXT" },
  ]
}

# bananagraph.com
module "dns_bananagraph" {
  source  = "./modules/cloudflare-dns"
  zone_id = var.cloudflare_zone_ids["bananagraph.com"]

  records = [
    # Gateway A records
    { key = "root", name = "bananagraph.com", content = local.gateway_ip },
    { name = "www", content = local.gateway_ip },
    { name = "api", content = local.gateway_ip },
  ]
}

# a2aregistry.org
module "dns_a2aregistry" {
  source  = "./modules/cloudflare-dns"
  zone_id = var.cloudflare_zone_ids["a2aregistry.org"]

  records = [
    # Gateway A record
    { name = "beta", content = local.gateway_ip },

    # GitHub Pages (root domain - 4 A records)
    { key = "gh-pages-1", name = "a2aregistry.org", content = "185.199.108.153", proxied = true },
    { key = "gh-pages-2", name = "a2aregistry.org", content = "185.199.109.153", proxied = true },
    { key = "gh-pages-3", name = "a2aregistry.org", content = "185.199.110.153", proxied = true },
    { key = "gh-pages-4", name = "a2aregistry.org", content = "185.199.111.153", proxied = true },
    { name = "www", content = "prassanna-ravishankar.github.io", type = "CNAME", proxied = true },

    # Email Routing MX/DKIM records are managed by Cloudflare Email Routing — not Terraform.
    # Workers hello AAAA record is read-only — not Terraform.

    # Verification TXT records
    { key = "txt-google-verify", name = "a2aregistry.org", content = "google-site-verification=mEA3Y3qH_-_X9XAKFblpLkMBLS80K8k3R0ELczDZmRo", type = "TXT" },
    { key = "txt-gh-pages", name = "_github-pages-challenge-prassanna-ravishankar", content = "7f19b004f93b6b454fd825b436f33b", type = "TXT" },
  ]
}

# repowire.io
module "dns_repowire" {
  source  = "./modules/cloudflare-dns"
  zone_id = var.cloudflare_zone_ids["repowire.io"]

  records = [
    # Gateway A records
    { key = "root", name = "repowire.io", content = local.gateway_ip },
    { name = "relay", content = local.gateway_ip },
  ]
}

# feedforward.space
module "dns_feedforward" {
  source  = "./modules/cloudflare-dns"
  zone_id = var.cloudflare_zone_ids["feedforward.space"]

  records = [
    # Cloudflare Pages
    { key = "root", name = "feedforward.space", content = "spotify-redirect.pages.dev", type = "CNAME", proxied = true },

    # Email Routing MX/DKIM/SPF records are managed by Cloudflare Email Routing — not Terraform.
  ]
}
