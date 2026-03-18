#!/bin/bash
# Example script to test the metrics endpoint
# Usage: ./test_metrics.sh

set -e

GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"

echo "Testing metrics endpoint at ${GATEWAY_URL}/metrics"
echo ""

# Fetch metrics
curl -s "${GATEWAY_URL}/metrics"

echo ""
echo ""
echo "=== Sample Queries ==="
echo ""

# Show request count metrics
echo "Request counts:"
curl -s "${GATEWAY_URL}/metrics" | grep "llm_gateway_requests_total"

echo ""
echo ""

# Show token metrics
echo "Token metrics:"
curl -s "${GATEWAY_URL}/metrics" | grep "llm_gateway.*tokens_total"

echo ""
echo ""

# Show cost metrics
echo "Cost metrics:"
curl -s "${GATEWAY_URL}/metrics" | grep "llm_gateway_cost_usd_total"

echo ""
echo ""

# Show latency histogram
echo "Latency histogram:"
curl -s "${GATEWAY_URL}/metrics" | grep "llm_gateway_request_duration_seconds"

echo ""
echo "Metrics endpoint test complete!"
