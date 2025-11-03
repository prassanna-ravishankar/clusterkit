# ClusterKit CLI

Command-line tool for managing serverless applications on ClusterKit - a GKE Autopilot platform with scale-to-zero capabilities.

## Features

- **Bootstrap Infrastructure**: Deploy complete ClusterKit stack with one command
- **App Management**: Create, deploy, and manage Knative applications
- **Automatic DNS & TLS**: Integrated ExternalDNS and cert-manager
- **Database Support**: Deploy and attach PostgreSQL databases
- **Multi-Domain**: Support for multiple domains per app
- **Status Monitoring**: Real-time cluster and application status

## Prerequisites

- Go 1.21 or higher
- kubectl configured with GKE cluster access
- Cloudflare account (for DNS)

## Installation

### From Source

```bash
cd cli
go build -o clusterkit ./cmd/clusterkit
sudo mv clusterkit /usr/local/bin/
```

### Verify Installation

```bash
clusterkit version
```

## Quick Start

### 1. Bootstrap Infrastructure

Deploy Knative, NGINX Ingress, cert-manager, and ExternalDNS:

```bash
clusterkit bootstrap \
  --project-id=my-gcp-project \
  --region=us-central1 \
  --domain=example.com \
  --cloudflare-token=$CLOUDFLARE_API_TOKEN
```

### 2. Deploy Your First App

```bash
clusterkit create my-app \
  --domain=myapp.example.com \
  --image=gcr.io/my-project/my-app:latest
```

This automatically:
- Creates Knative Service with scale-to-zero
- Configures DNS in Cloudflare
- Issues TLS certificate via Let's Encrypt
- Sets up NGINX Ingress routing

### 3. Check Status

```bash
clusterkit status my-app
```

## Commands

### bootstrap

Deploy ClusterKit infrastructure to a GKE cluster:

```bash
clusterkit bootstrap [flags]

Flags:
  --project-id string         GCP project ID
  --region string            GCP region (default: us-central1)
  --cluster-name string      GKE cluster name (default: clusterkit)
  --domain string            Primary domain for apps
  --cloudflare-token string  Cloudflare API token
  --skip-terraform          Skip Terraform infrastructure (cluster exists)
  --skip-knative            Skip Knative installation
  --skip-ingress            Skip NGINX Ingress installation
  --skip-cert-manager       Skip cert-manager installation
  --skip-external-dns       Skip ExternalDNS installation
```

### create

Create a new Knative application:

```bash
clusterkit create NAME [flags]

Flags:
  --domain string            App domain (e.g., app.example.com)
  --image string             Container image
  --env stringArray          Environment variables (key=value)
  --min-scale int           Minimum replicas (default: 0)
  --max-scale int           Maximum replicas (default: 10)
  --concurrency int         Target concurrent requests (default: 10)
  --memory string           Memory limit (default: 256Mi)
  --cpu string              CPU limit (default: 1000m)
  --traditional             Use Deployment instead of Knative
```

### status

Show application or cluster status:

```bash
clusterkit status [NAME]

# Cluster-wide status
clusterkit status

# Specific app status
clusterkit status my-app
```

### delete

Delete an application:

```bash
clusterkit delete NAME [flags]

Flags:
  --keep-dns               Keep DNS records
  --keep-cert              Keep TLS certificate
  --force                  Skip confirmation
```

### db

Database management commands:

```bash
# Create PostgreSQL database
clusterkit db create NAME [flags]

Flags:
  --size string            Storage size (default: 10Gi)
  --version string         PostgreSQL version (default: 15)

# Attach database to app
clusterkit db attach DB_NAME --app=APP_NAME

# List databases
clusterkit db list

# Delete database
clusterkit db delete NAME
```

## Configuration

ClusterKit CLI looks for configuration in the following locations (in order):

1. `.clusterkit.yaml` in current directory
2. `~/.clusterkit/config.yaml`
3. Environment variables prefixed with `CLUSTERKIT_`

### Example Configuration

Create `~/.clusterkit/config.yaml`:

```yaml
# GCP Settings
project_id: my-gcp-project
region: us-central1
cluster_name: clusterkit

# Default Domain
domain: example.com

# Cloudflare
cloudflare_token: your-token-here

# Defaults for New Apps
defaults:
  min_scale: 0
  max_scale: 10
  concurrency: 10
  memory: 256Mi
  cpu: 1000m

# Logging
log_level: info  # debug, info, warn, error
log_format: text # text, json
```

### Environment Variables

```bash
export CLUSTERKIT_PROJECT_ID=my-gcp-project
export CLUSTERKIT_REGION=us-central1
export CLUSTERKIT_DOMAIN=example.com
export CLUSTERKIT_CLOUDFLARE_TOKEN=your-token-here
export CLUSTERKIT_LOG_LEVEL=debug
```

## Architecture

ClusterKit CLI integrates with:

```
ClusterKit CLI
  ↓
kubectl (via client-go)
  ↓
GKE Autopilot Cluster
  ├─ Knative Serving (scale-to-zero apps)
  ├─ NGINX Ingress (routing)
  ├─ cert-manager (TLS certificates)
  ├─ ExternalDNS (Cloudflare DNS)
  └─ PostgreSQL StatefulSets (databases)
```

## Development

### Building

```bash
cd cli
go build -o clusterkit ./cmd/clusterkit
```

### Running Tests

```bash
go test ./...
```

### Running with Debug Logging

```bash
clusterkit --log-level=debug status
```

## Examples

### Deploy Node.js App

```bash
clusterkit create api \
  --domain=api.example.com \
  --image=node:18 \
  --env="NODE_ENV=production" \
  --env="PORT=8080" \
  --memory=512Mi \
  --cpu=500m
```

### Deploy with Database

```bash
# Create database
clusterkit db create myapp-db --size=20Gi

# Create app with database
clusterkit create myapp \
  --domain=myapp.example.com \
  --image=gcr.io/my-project/myapp:v1.0.0

# Attach database
clusterkit db attach myapp-db --app=myapp
```

### Multi-Domain App

```bash
clusterkit create blog \
  --domain=blog.example.com \
  --domain=www.blog.example.com \
  --domain=blog.example.org \
  --image=wordpress:latest
```

### Traditional Deployment (No Scale-to-Zero)

For apps that need constant availability:

```bash
clusterkit create cache \
  --domain=cache.example.com \
  --image=redis:7 \
  --traditional \
  --min-scale=2 \
  --max-scale=5
```

## Troubleshooting

### "cluster unreachable" Error

Check kubectl context:

```bash
kubectl config current-context
kubectl cluster-info
```

### "Cloudflare authentication failed"

Verify API token:

```bash
# Test token
curl -X GET "https://api.cloudflare.com/client/v4/zones" \
  -H "Authorization: Bearer ${CLOUDFLARE_API_TOKEN}" \
  -H "Content-Type: application/json"
```

### Debug Mode

Enable verbose logging:

```bash
clusterkit --log-level=debug create myapp --domain=myapp.example.com --image=nginx
```

## Cost Optimization

ClusterKit is designed for cost efficiency:

- **Scale-to-zero**: Apps with no traffic use $0
- **GKE Autopilot**: Pay only for running pods
- **Shared Infrastructure**: All apps share NGINX/cert-manager/ExternalDNS
- **Estimated Cost**: ~$30-50/month for infrastructure + $0.50-2/app/month (active)

## Resources

- [ClusterKit Infrastructure Repo](https://github.com/clusterkit/infrastructure)
- [Knative Documentation](https://knative.dev/docs/)
- [GKE Autopilot Pricing](https://cloud.google.com/kubernetes-engine/pricing#autopilot_mode)

## License

MIT
