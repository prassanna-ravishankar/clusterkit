/**
 * HTTPRoute Module
 *
 * Creates an HTTPRoute that attaches to a Gateway.
 * Supports both same-namespace and cross-namespace service references.
 */

resource "kubernetes_manifest" "httproute" {
  manifest = {
    apiVersion = "gateway.networking.k8s.io/v1"
    kind       = "HTTPRoute"
    metadata = {
      name      = var.route_name
      namespace = var.route_namespace
      annotations = merge(
        {
          "external-dns.alpha.kubernetes.io/cloudflare-proxied" = "false"
        },
        var.annotations
      )
    }
    spec = {
      parentRefs = [{
        name      = var.gateway_name
        namespace = var.gateway_namespace
      }]
      hostnames = var.hostnames
      rules = [{
        backendRefs = [{
          name      = var.service_name
          namespace = var.service_namespace
          port      = var.service_port
        }]
      }]
    }
  }
}
