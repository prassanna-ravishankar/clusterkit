#!/usr/bin/env bash
# ClusterKit health check — validates Gateway, HTTPRoutes, DNS, SSL, and ExternalDNS.
# Usage: ./scripts/health-check.sh [--verbose]
set -euo pipefail

GATEWAY_NAME="clusterkit-gateway"
GATEWAY_NS="clusterkit"
VERBOSE="${1:-}"
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; NC='\033[0m'
PASS=0; WARN=0; FAIL=0

pass()  { ((PASS++)); echo -e "  ${GREEN}OK${NC}   $1"; }
warn()  { ((WARN++)); echo -e "  ${YELLOW}WARN${NC} $1"; }
fail()  { ((FAIL++)); echo -e "  ${RED}FAIL${NC} $1"; }

echo "=== ClusterKit Health Check ==="
echo ""

# 1. kubectl connectivity
echo "--- Cluster Connectivity ---"
if kubectl cluster-info &>/dev/null; then
  pass "kubectl can reach the cluster"
else
  fail "kubectl cannot reach the cluster (run: gcloud container clusters get-credentials clusterkit --region us-central1 --project baldmaninc)"
  echo ""; echo "Cannot proceed without cluster access."; exit 1
fi

# 2. Gateway status
echo ""
echo "--- Gateway ---"
GW_JSON=$(kubectl get gateway "$GATEWAY_NAME" -n "$GATEWAY_NS" -o json 2>/dev/null || echo "")
if [ -z "$GW_JSON" ]; then
  fail "Gateway $GATEWAY_NAME not found in namespace $GATEWAY_NS"
else
  PROGRAMMED=$(echo "$GW_JSON" | jq -r '.status.conditions[]? | select(.type=="Programmed") | .status' 2>/dev/null || echo "Unknown")
  ACCEPTED=$(echo "$GW_JSON" | jq -r '.status.conditions[]? | select(.type=="Accepted") | .status' 2>/dev/null || echo "Unknown")
  ADDRESS=$(echo "$GW_JSON" | jq -r '.status.addresses[0].value // "none"' 2>/dev/null || echo "none")
  ATTACHED=$(echo "$GW_JSON" | jq '[.status.listeners[]?.attachedRoutes // 0] | add' 2>/dev/null || echo "0")

  if [ "$PROGRAMMED" = "True" ]; then pass "Gateway PROGRAMMED: True"; else fail "Gateway PROGRAMMED: $PROGRAMMED"; fi
  if [ "$ACCEPTED" = "True" ]; then pass "Gateway ACCEPTED: True"; else fail "Gateway ACCEPTED: $ACCEPTED"; fi
  if [ "$ADDRESS" != "none" ] && [ -n "$ADDRESS" ]; then pass "Gateway IP: $ADDRESS"; else fail "Gateway has no IP address"; fi
  if [ "$ATTACHED" -gt 0 ] 2>/dev/null; then pass "Attached routes: $ATTACHED"; else warn "No attached routes"; fi
fi

# 3. HTTPRoutes
echo ""
echo "--- HTTPRoutes ---"
ROUTES=$(kubectl get httproute -n "$GATEWAY_NS" -o json 2>/dev/null || echo '{"items":[]}')
ROUTE_COUNT=$(echo "$ROUTES" | jq '.items | length')
if [ "$ROUTE_COUNT" -eq 0 ]; then
  warn "No HTTPRoutes found in $GATEWAY_NS namespace"
else
  pass "$ROUTE_COUNT HTTPRoute(s) found"
  echo "$ROUTES" | jq -r '.items[] | .metadata.name as $name | .status.parents[]?.conditions[]? | select(.type=="Accepted") | "\($name): \(.status)"' 2>/dev/null | while read -r line; do
    route_name=$(echo "$line" | cut -d: -f1)
    status=$(echo "$line" | cut -d: -f2 | xargs)
    if [ "$status" = "True" ]; then pass "  $route_name accepted"; else fail "  $route_name NOT accepted"; fi
  done
  if [ "$VERBOSE" = "--verbose" ]; then
    echo ""
    echo "  Hostnames:"
    echo "$ROUTES" | jq -r '.items[] | "    " + .metadata.name + ": " + (.spec.hostnames | join(", "))' 2>/dev/null
  fi
fi

# 4. ReferenceGrants
echo ""
echo "--- ReferenceGrants ---"
RG_COUNT=$(kubectl get referencegrant --all-namespaces --no-headers 2>/dev/null | wc -l | xargs)
if [ "$RG_COUNT" -gt 0 ]; then
  pass "$RG_COUNT ReferenceGrant(s) found"
  if [ "$VERBOSE" = "--verbose" ]; then
    kubectl get referencegrant --all-namespaces --no-headers 2>/dev/null | while read -r ns name _rest; do
      echo "    $ns/$name"
    done
  fi
else
  warn "No ReferenceGrants found (cross-namespace routing won't work)"
fi

# 5. SSL certificates
echo ""
echo "--- SSL Certificates ---"
CERTS=$(gcloud compute ssl-certificates list --format='value(name,type,expireTime)' 2>/dev/null || echo "")
if [ -n "$CERTS" ]; then
  CERT_COUNT=$(echo "$CERTS" | wc -l | xargs)
  pass "$CERT_COUNT SSL certificate(s)"
  if [ "$VERBOSE" = "--verbose" ]; then
    echo "$CERTS" | while read -r name type expire; do
      echo "    $name ($type, expires: ${expire:-N/A})"
    done
  fi
else
  fail "No SSL certificates found"
fi

# 6. ExternalDNS
echo ""
echo "--- ExternalDNS ---"
EDNS_PODS=$(kubectl get pods -n external-dns -l app.kubernetes.io/name=external-dns --no-headers 2>/dev/null || echo "")
if [ -n "$EDNS_PODS" ]; then
  RUNNING=$(echo "$EDNS_PODS" | grep -c "Running" || true)
  TOTAL=$(echo "$EDNS_PODS" | wc -l | xargs)
  if [ "$RUNNING" -eq "$TOTAL" ]; then
    pass "ExternalDNS: $RUNNING/$TOTAL pods running"
  else
    fail "ExternalDNS: $RUNNING/$TOTAL pods running"
  fi
  # Check for recent errors
  RECENT_ERRORS=$(kubectl logs -n external-dns -l app.kubernetes.io/name=external-dns --since=5m 2>/dev/null | grep -ci "error" || true)
  if [ "$RECENT_ERRORS" -gt 0 ]; then
    warn "ExternalDNS has $RECENT_ERRORS error(s) in last 5 minutes"
  else
    pass "No errors in last 5 minutes"
  fi
else
  fail "ExternalDNS pods not found"
fi

# 7. Cloud SQL
echo ""
echo "--- Cloud SQL ---"
INSTANCE_STATUS=$(gcloud sql instances describe clusterkit-db --format='value(state)' 2>/dev/null || echo "")
if [ "$INSTANCE_STATUS" = "RUNNABLE" ]; then
  pass "Cloud SQL instance: RUNNABLE"
  if [ "$VERBOSE" = "--verbose" ]; then
    DB_VERSION=$(gcloud sql instances describe clusterkit-db --format='value(databaseVersion)' 2>/dev/null)
    DB_TIER=$(gcloud sql instances describe clusterkit-db --format='value(settings.tier)' 2>/dev/null)
    echo "    Version: $DB_VERSION, Tier: $DB_TIER"
  fi
elif [ -z "$INSTANCE_STATUS" ]; then
  fail "Cloud SQL instance clusterkit-db not found"
else
  fail "Cloud SQL instance state: $INSTANCE_STATUS"
fi

# 8. DNS spot checks (pick hostnames from HTTPRoutes)
echo ""
echo "--- DNS Resolution ---"
HOSTNAMES=$(echo "$ROUTES" | jq -r '.items[].spec.hostnames[]?' 2>/dev/null | head -5)
if [ -n "$HOSTNAMES" ]; then
  for host in $HOSTNAMES; do
    RESOLVED=$(dig +short "$host" @1.1.1.1 2>/dev/null | head -1)
    if [ -n "$RESOLVED" ]; then
      pass "$host -> $RESOLVED"
    else
      fail "$host does not resolve"
    fi
  done
else
  warn "No hostnames to check (no HTTPRoutes)"
fi

# Summary
echo ""
echo "=== Summary ==="
echo -e "  ${GREEN}$PASS passed${NC}, ${YELLOW}$WARN warnings${NC}, ${RED}$FAIL failed${NC}"
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
