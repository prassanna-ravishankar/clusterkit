# Cloudflare Edge Configuration Module

Configures Cloudflare for optimal performance, caching, and security for ClusterKit applications.

## Features

- **Static Asset Caching**: 30-day cache for CSS, JS, images, fonts
- **Dynamic Content Caching**: 5-minute edge cache for HTML
- **API Bypass**: No caching for API endpoints
- **Rate Limiting**: 10 requests/minute per IP for API endpoints
- **DDoS Protection**: Automatic challenge for suspicious traffic
- **WAF Integration**: Blocks known malicious IPs
- **SSL/TLS**: Strict mode with automatic HTTPS rewrites
- **Performance**: Brotli compression, HTTP/2, HTTP/3

## Usage

```hcl
module "cloudflare" {
  source = "./modules/cloudflare"

  zone_id = "your-zone-id-here"
  domain  = "example.com"

  # Optional: Customize caching
  dynamic_cache_ttl  = 300   # 5 minutes
  browser_cache_ttl  = 1800  # 30 minutes

  # Optional: Customize rate limiting
  api_rate_limit_threshold = 10  # requests
  api_rate_limit_period    = 60  # seconds

  # Optional: Security level
  security_level = "medium"  # off, low, medium, high, under_attack
}
```

## DNS Proxy Configuration (Orange Cloud)

To enable Cloudflare's CDN and WAF:

1. Go to Cloudflare Dashboard → DNS
2. Click the cloud icon next to your DNS record to turn it "orange"
3. This enables:
   - CDN caching
   - DDoS protection
   - WAF (Web Application Firewall)
   - SSL/TLS termination

**Note**: ExternalDNS can automatically enable proxy mode with the annotation:
```yaml
external-dns.alpha.kubernetes.io/cloudflare-proxied: "true"
```

## SSL/TLS Configuration

### Recommended Settings

1. **SSL/TLS Mode**: Full (Strict)
   - Cloudflare → Origin Server uses valid SSL certificate
   - cert-manager provides the origin certificate

2. **Minimum TLS Version**: 1.2
   - Configured automatically by this module

3. **Always Use HTTPS**: Enabled
   - HTTP requests automatically redirect to HTTPS

4. **HSTS**: Enable in Cloudflare Dashboard
   - Max Age: 6 months (15768000 seconds)
   - Include Subdomains: Yes
   - Preload: Optional

### SSL/TLS Certificate Chain

```
Client
  ↓ (TLS 1.3)
Cloudflare Edge
  ↓ (TLS 1.2+)
NGINX Ingress Controller
  ↓ (TLS 1.2+)
Application Pod
```

## Caching Rules

### Static Assets
- **Pattern**: `*.css`, `*.js`, `*.jpg`, `*.png`, `*.woff2`, etc.
- **Edge Cache**: 30 days
- **Browser Cache**: 30 days
- **Behavior**: Cache everything, serve stale if origin is down

### Dynamic Content
- **Pattern**: All other URLs
- **Edge Cache**: 5 minutes (configurable)
- **Browser Cache**: 30 minutes (configurable)
- **Behavior**: Respect cache headers from origin

### API Endpoints
- **Pattern**: `/api/*`
- **Cache**: Bypass (no caching)
- **Reason**: API responses should always be fresh

## Rate Limiting

### API Protection
- **Path**: `/api/*`
- **Limit**: 10 requests per minute per IP
- **Action**: Challenge (CAPTCHA)
- **Timeout**: 60 seconds

### Customization

To adjust rate limits:

```hcl
module "cloudflare" {
  # ...
  api_rate_limit_threshold = 20  # Increase to 20 requests/minute
  api_rate_limit_period    = 60
}
```

## Security Rules

### DDoS Protection
- **Trigger**: Threat score > 10
- **Action**: Challenge (CAPTCHA)
- **Effect**: Stops basic bot attacks

### Malicious IP Blocking
- **Trigger**: Threat score > 50
- **Action**: Block
- **Effect**: Blocks known malicious IPs

### Security Levels

- **off**: No security features
- **essentially_off**: Minimal security
- **low**: Challenge very threatening visitors
- **medium**: Challenge threatening visitors (default)
- **high**: Challenge all visitors that show moderate threat
- **under_attack**: I'm Under Attack mode (use temporarily)

## Performance Optimizations

### Enabled Features
- **Brotli Compression**: Better than gzip
- **Minification**: CSS, JS, HTML
- **HTTP/2**: Multiplexing, server push
- **HTTP/3**: QUIC protocol for faster connections
- **IPv6**: Full IPv6 support

### Disabled Features
- **Rocket Loader**: Can break some JavaScript apps
- **Mirage**: Image optimization (enable if needed)
- **Polish**: Image compression (requires paid plan)

## Cost

All features in this module are available on Cloudflare's **Free Plan**:
- Unlimited DNS queries
- Unlimited DDoS protection
- Basic WAF
- Page rules (3 on free plan, this module uses 3)
- Rate limiting (1 rule on free plan)

## Monitoring

Check Cloudflare Analytics for:
- Traffic volume
- Cache hit ratio
- Threat mitigation
- Performance metrics

## Troubleshooting

### Cache Not Working
1. Check page rule priority (lower number = higher priority)
2. Verify cache headers from origin
3. Use Cloudflare's "Purge Cache" if needed

### Rate Limiting Too Aggressive
1. Increase `api_rate_limit_threshold`
2. Or increase `api_rate_limit_period`
3. Or disable rate limiting for specific IPs

### SSL/TLS Errors
1. Verify origin certificate is valid (from cert-manager)
2. Check SSL/TLS mode is "Full (Strict)"
3. Ensure NGINX Ingress is serving HTTPS

## Resources

- [Cloudflare Page Rules](https://developers.cloudflare.com/rules/page-rules/)
- [Cloudflare Rate Limiting](https://developers.cloudflare.com/waf/rate-limiting-rules/)
- [Cloudflare SSL/TLS](https://developers.cloudflare.com/ssl/)
