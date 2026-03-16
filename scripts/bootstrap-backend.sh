#!/usr/bin/env bash
# Creates the GCS bucket for Terraform remote state.
# Run once before `terraform init` when setting up from scratch.
#
# Usage: PROJECT_ID=baldmaninc ./scripts/bootstrap-backend.sh
set -euo pipefail

PROJECT_ID="${PROJECT_ID:?Set PROJECT_ID}"
BUCKET="tf-state-${PROJECT_ID}"
REGION="${REGION:-us-central1}"

echo "Creating GCS bucket gs://${BUCKET} for Terraform state..."

if gsutil ls "gs://${BUCKET}" &>/dev/null; then
  echo "Bucket already exists."
else
  gsutil mb -p "${PROJECT_ID}" -l "${REGION}" -b on "gs://${BUCKET}"
  gsutil versioning set on "gs://${BUCKET}"
  echo "Bucket created with versioning enabled."
fi

echo ""
echo "Next steps:"
echo "  1. cd terraform"
echo "  2. terraform init -migrate-state   # migrates local state to GCS"
echo ""
echo "For project-specific states:"
echo "  cd terraform/projects/<project>"
echo "  terraform init -migrate-state"
