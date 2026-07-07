#!/usr/bin/env bash
# Smoke test: verify all services are healthy through the gateway.
# Usage: ./scripts/smoke-test.sh [BASE_URL]
#   BASE_URL defaults to http://localhost:8080 (gateway)
#   Frontend is checked on its own port (80).

set -u

GATEWAY="${1:-http://localhost:8080}"
FRONTEND="${2:-http://localhost}"

pass=0
fail=0

check() {
  local name="$1"
  local url="$2"
  if curl -sf "$url" -o /dev/null; then
    echo "  [OK]   $name"
    pass=$((pass + 1))
  else
    echo "  [FAIL] $name  ($url)"
    fail=$((fail + 1))
  fi
}

echo "Running smoke tests against $GATEWAY"
echo ""

check "Gateway health"      "$GATEWAY/health"
check "Upload service"      "$GATEWAY/api/datasets/health"
check "Scoring service"     "$GATEWAY/api/scoring/health"
check "Relations service"   "$GATEWAY/api/relations/health"
check "Frontend"            "$FRONTEND"

echo ""
echo "Passed: $pass  Failed: $fail"

[ "$fail" -eq 0 ]
