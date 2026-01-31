variable "zone_id" {
  description = "Cloudflare Zone ID"
  type        = string
}

variable "records" {
  description = "List of DNS records to create"
  type = list(object({
    key      = optional(string) # unique key for for_each, defaults to name
    name     = string
    content  = string
    type     = optional(string, "A")
    proxied  = optional(bool, false)
    ttl      = optional(number, 1) # 1 = automatic
    priority = optional(number)
  }))
}
