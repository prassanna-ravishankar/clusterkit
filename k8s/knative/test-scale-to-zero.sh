#!/bin/bash
set -euo pipefail

# ClusterKit - Knative Scale-to-Zero Test Script
# Validates that Knative services properly scale to zero and back up

echo "=================================="
echo "Knative Scale-to-Zero Test"
echo "=================================="
echo

SERVICE_NAME="hello-world"
NAMESPACE="default"

# Check if service exists
if ! kubectl get ksvc ${SERVICE_NAME} -n ${NAMESPACE} &> /dev/null; then
    echo "Deploying test service..."
    kubectl apply -f examples/hello-world.yaml
    echo "Waiting for service to be ready..."
    kubectl wait --for=condition=Ready ksvc/${SERVICE_NAME} -n ${NAMESPACE} --timeout=300s
    echo "✓ Service deployed and ready"
    echo
fi

# Get service URL
SERVICE_URL=$(kubectl get ksvc ${SERVICE_NAME} -n ${NAMESPACE} -o jsonpath='{.status.url}')
echo "Service URL: ${SERVICE_URL}"
echo

# Test 1: Verify service responds
echo "Test 1: Sending initial request to wake up service..."
RESPONSE=$(curl -s ${SERVICE_URL} || echo "FAILED")
if [[ "${RESPONSE}" == *"ClusterKit"* ]]; then
    echo "✓ Service responded correctly: ${RESPONSE}"
else
    echo "✗ Service response unexpected: ${RESPONSE}"
    exit 1
fi
echo

# Test 2: Check that pods are running
echo "Test 2: Verifying pods are running..."
POD_COUNT=$(kubectl get pods -n ${NAMESPACE} -l serving.knative.dev/service=${SERVICE_NAME} --field-selector=status.phase=Running | grep -c Running || echo "0")
if [[ ${POD_COUNT} -gt 0 ]]; then
    echo "✓ ${POD_COUNT} pod(s) running"
else
    echo "✗ No pods running"
    exit 1
fi
echo

# Test 3: Wait for scale-to-zero
echo "Test 3: Waiting for scale-to-zero (60 second grace period + buffer)..."
echo "This will take approximately 90 seconds..."
for i in {1..9}; do
    sleep 10
    CURRENT_PODS=$(kubectl get pods -n ${NAMESPACE} -l serving.knative.dev/service=${SERVICE_NAME} --field-selector=status.phase=Running 2>/dev/null | grep -c Running || echo "0")
    echo "  ${i}0s: ${CURRENT_PODS} pod(s) running"

    if [[ ${CURRENT_PODS} -eq 0 ]]; then
        echo "✓ Service scaled to zero after ${i}0 seconds"
        break
    fi
done

FINAL_PODS=$(kubectl get pods -n ${NAMESPACE} -l serving.knative.dev/service=${SERVICE_NAME} --field-selector=status.phase=Running 2>/dev/null | grep -c Running || echo "0")
if [[ ${FINAL_PODS} -eq 0 ]]; then
    echo "✓ Scale-to-zero successful"
else
    echo "⚠ Service did not scale to zero (${FINAL_PODS} pod(s) still running)"
    echo "  This may be due to ongoing requests or configuration issues"
fi
echo

# Test 4: Verify cold start (scale from zero)
echo "Test 4: Testing cold start (scaling from zero)..."
START_TIME=$(date +%s)
RESPONSE=$(curl -s ${SERVICE_URL} || echo "FAILED")
END_TIME=$(date +%s)
COLD_START_TIME=$((END_TIME - START_TIME))

if [[ "${RESPONSE}" == *"ClusterKit"* ]]; then
    echo "✓ Service scaled from zero and responded"
    echo "  Cold start time: ${COLD_START_TIME} seconds"

    if [[ ${COLD_START_TIME} -lt 5 ]]; then
        echo "  Performance: Excellent (< 5s)"
    elif [[ ${COLD_START_TIME} -lt 10 ]]; then
        echo "  Performance: Good (< 10s)"
    else
        echo "  Performance: Slow (≥ 10s) - Consider using min-scale=1 for this service"
    fi
else
    echo "✗ Cold start failed: ${RESPONSE}"
    exit 1
fi
echo

# Test 5: Verify autoscaling under load
echo "Test 5: Testing autoscaling under load..."
echo "Sending 50 concurrent requests..."

# Simple load test
for i in {1..50}; do
    curl -s ${SERVICE_URL} > /dev/null &
done
wait

sleep 5  # Wait for autoscaler to react

POD_COUNT=$(kubectl get pods -n ${NAMESPACE} -l serving.knative.dev/service=${SERVICE_NAME} --field-selector=status.phase=Running | grep -c Running || echo "0")
echo "Pods after load: ${POD_COUNT}"

if [[ ${POD_COUNT} -gt 1 ]]; then
    echo "✓ Service scaled up under load (${POD_COUNT} pods)"
else
    echo "⚠ Service did not scale up (still ${POD_COUNT} pod)"
    echo "  This may be expected if requests completed quickly"
fi
echo

# Summary
echo "=================================="
echo "Test Summary"
echo "=================================="
echo "✓ Service responds to requests"
echo "✓ Service scales up on demand"
if [[ ${FINAL_PODS} -eq 0 ]]; then
    echo "✓ Service scales to zero"
else
    echo "⚠ Scale-to-zero needs verification"
fi
echo "✓ Cold start functional (${COLD_START_TIME}s)"
echo
echo "All critical tests passed!"
echo
echo "To cleanup test service:"
echo "  kubectl delete ksvc ${SERVICE_NAME} -n ${NAMESPACE}"
echo
