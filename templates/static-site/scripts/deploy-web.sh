#!/usr/bin/env bash
# Manual deploy for a static site on clusterkit-gateway.
#
# Adapt the four variables at the top, then run from app repo root.
# Requires: gcloud authed, docker running, helm installed, kubectl installed.

set -euo pipefail

# === ADAPT THESE ===
APP_NAME="SITE"                 # e.g. clitcoin-web, repowire-docs
NAMESPACE="APP"                 # e.g. clitcoin, repowire
DOCKERFILE="web/Dockerfile"     # path to the Dockerfile, relative to repo root
CHART_PATH="charts/${APP_NAME}" # path to the helm chart, relative to repo root
# ===================

PROJECT_ID=baldmaninc
CLUSTER=clusterkit
REGION=us-central1
REGISTRY=us-docker.pkg.dev/${PROJECT_ID}/gcr.io
SHA=$(git rev-parse --short HEAD)

echo "==> SHA=${SHA}  APP=${APP_NAME}  NS=${NAMESPACE}"

echo "==> Configuring Docker for Artifact Registry"
gcloud auth configure-docker us-docker.pkg.dev --quiet

echo "==> Getting GKE credentials"
gcloud container clusters get-credentials "${CLUSTER}" --region "${REGION}" --project "${PROJECT_ID}"

echo "==> Building + pushing ${APP_NAME}"
# --platform linux/amd64: GKE nodes are amd64; on Apple Silicon the native
# arch is arm64 which produces an unrunnable image (CrashLoopBackOff with
# `exec format error`).
docker build --platform linux/amd64 \
  -t "${REGISTRY}/${APP_NAME}:${SHA}" \
  -t "${REGISTRY}/${APP_NAME}:latest" \
  -f "${DOCKERFILE}" .
docker push "${REGISTRY}/${APP_NAME}:${SHA}"
docker push "${REGISTRY}/${APP_NAME}:latest"

echo "==> helm upgrade ${APP_NAME}"
helm upgrade --install "${APP_NAME}" "${CHART_PATH}" \
  --namespace "${NAMESPACE}" \
  --set image.tag="${SHA}" \
  --wait --timeout 5m

echo "==> Verifying"
kubectl rollout status "deployment/${APP_NAME}" -n "${NAMESPACE}" --timeout=3m

HOSTNAME=$(helm get values "${APP_NAME}" -n "${NAMESPACE}" -o json 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin).get('hostname',''))" 2>/dev/null || true)

if [ -n "${HOSTNAME}" ]; then
  echo "==> Smoke checks for https://${HOSTNAME}"
  # Cloudflare cache + GCP LB reconcile take 30-60s on first deploy.
  curl -sS -o /dev/null -w "HTTP %{http_code} in %{time_total}s\n" "https://${HOSTNAME}" || true
  echo "--- TLS issuer (Cloudflare edge cert; Origin CA validates server-side under Full Strict)"
  echo | openssl s_client -connect "${HOSTNAME}:443" -servername "${HOSTNAME}" 2>/dev/null \
    | openssl x509 -noout -issuer -subject 2>/dev/null || true
fi

echo "==> Done."
