# Cloudflare DNS Records — non-gateway records only
#
# Gateway A records are managed by ExternalDNS (creates proxied records from HTTPRoutes).
# This file manages: email (MX/DKIM/SPF), verification TXT, GitHub Pages, Cloudflare Pages.

# Look up zone IDs dynamically from domain names
data "cloudflare_zones" "managed" {
  filter {
    name   = ""
    status = "active"
  }
}

locals {
  cloudflare_zone_ids = {
    for zone in data.cloudflare_zones.managed.zones :
    zone.name => zone.id
    if contains(var.cloudflare_domains, zone.name)
  }
}

# torale.ai
module "dns_torale" {
  source  = "./modules/cloudflare-dns"
  zone_id = local.cloudflare_zone_ids["torale.ai"]

  records = [
    # Clerk authentication
    { name = "accounts", content = "accounts.clerk.services", type = "CNAME" },
    { name = "clerk", content = "frontend-api.clerk.services", type = "CNAME" },
    { name = "clk._domainkey", content = "dkim1.kdbjwdvfrrrp.clerk.services", type = "CNAME" },
    { name = "clk2._domainkey", content = "dkim2.kdbjwdvfrrrp.clerk.services", type = "CNAME" },
    { name = "clkmail", content = "mail.kdbjwdvfrrrp.clerk.services", type = "CNAME" },

    # Google Workspace
    { name = "app", content = "ghs.googlehosted.com", type = "CNAME" },
    # mx-google MX record managed by Cloudflare Email Routing — not Terraform.

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

# a2aregistry.org (gateway records managed by ExternalDNS, only verification TXT here)
module "dns_a2aregistry" {
  source  = "./modules/cloudflare-dns"
  zone_id = local.cloudflare_zone_ids["a2aregistry.org"]

  records = [
    # Email Routing MX/DKIM records are managed by Cloudflare Email Routing — not Terraform.
    # Workers hello AAAA record is read-only — not Terraform.
    # Gateway A records (a2aregistry.org, www.a2aregistry.org) managed by ExternalDNS via HTTPRoute.

    # Verification TXT records
    { key = "txt-google-verify", name = "a2aregistry.org", content = "google-site-verification=mEA3Y3qH_-_X9XAKFblpLkMBLS80K8k3R0ELczDZmRo", type = "TXT" },
  ]
}

# feedforward.space
module "dns_feedforward" {
  source  = "./modules/cloudflare-dns"
  zone_id = local.cloudflare_zone_ids["feedforward.space"]

  records = [
    # Cloudflare Pages
    { key = "root", name = "feedforward.space", content = "spotify-redirect.pages.dev", type = "CNAME", proxied = true },

    # Email Routing MX/DKIM/SPF records are managed by Cloudflare Email Routing — not Terraform.
  ]
}
