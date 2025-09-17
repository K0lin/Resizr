#!/bin/sh

# healthcheck.sh - Smart health check script for Docker
# This script respects the S3_HEALTHCHECKS_DISABLE and HEALTHCHECK_INTERVAL environment variables

set -e

# Configuration with defaults (matching the Go config defaults)
HEALTHCHECK_INTERVAL=${HEALTHCHECK_INTERVAL:-30}
S3_HEALTHCHECKS_DISABLE=${S3_HEALTHCHECKS_DISABLE:-false}
PORT=${PORT:-8080}
HEALTH_URL="http://localhost:${PORT}/health"

# Enforce minimum interval of 10 seconds
if [ "$HEALTHCHECK_INTERVAL" -lt 10 ]; then
    echo "Warning: HEALTHCHECK_INTERVAL ($HEALTHCHECK_INTERVAL) is less than minimum 10s, using 10s"
    HEALTHCHECK_INTERVAL=10
fi

# Log configuration for debugging
echo "Health check configuration:"
echo "  - Interval: ${HEALTHCHECK_INTERVAL}s"
echo "  - S3 checks disabled: $S3_HEALTHCHECKS_DISABLE"
echo "  - Health URL: $HEALTH_URL"

# Perform the health check
echo "Performing health check..."
if curl -f --max-time 5 --silent --show-error "$HEALTH_URL" > /dev/null 2>&1; then
    echo "Health check successful"
    exit 0
else
    echo "Health check failed"
    exit 1
fi