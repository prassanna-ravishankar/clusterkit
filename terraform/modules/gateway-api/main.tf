/**
 * Gateway API Module
 *
 * Creates a shared GKE Gateway with Google-managed SSL certificates.
 * Application teams create HTTPRoutes in their namespaces that attach to this Gateway.
 */

resource "kubernetes_manifest" "gateway" {
  manifest = {
    apiVersion = "gateway.networking.k8s.io/v1"
    kind       = "Gateway"
    metadata = {
      name      = var.gateway_name
      namespace = var.gateway_namespace
    }
    spec = {
      gatewayClassName = "gke-l7-global-external-managed"
      listeners = [{
        name     = "https"
        protocol = "HTTPS"
        port     = 443
        tls = {
          mode = "Terminate"
          options = {
            "networking.gke.io/pre-shared-certs" = join(",", var.ssl_certificate_names)
          }
        }
        allowedRoutes = {
          namespaces = {
            from = "All"
          }
        }
      }]
      addresses = [{
        type  = "NamedAddress"
        value = var.static_ip_name
      }]
    }
  }
}

resource "kubernetes_manifest" "reference_grants" {
  for_each = toset(var.allowed_route_namespaces)

  manifest = {
    apiVersion = "gateway.networking.k8s.io/v1beta1"
    kind       = "ReferenceGrant"
    metadata = {
      name      = "allow-${var.gateway_namespace}-to-${each.key}-services"
      namespace = each.key
    }
    spec = {
      from = [{
        group     = "gateway.networking.k8s.io"
        kind      = "HTTPRoute"
        namespace = var.gateway_namespace
      }]
      to = [{
        group = ""
        kind  = "Service"
      }]
    }
  }
}
