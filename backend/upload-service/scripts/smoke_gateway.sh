#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

json_field() {
  python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$1"
}

echo "health"
curl -fsS "$BASE_URL/api/datasets/health"
echo

echo "upload clean dataset"
clean_response="$(curl -fsS -F "file=@$ROOT_DIR/data/clean_refund_dataset.csv" "$BASE_URL/api/datasets/upload")"
echo "$clean_response"
clean_dataset_id="$(printf '%s' "$clean_response" | json_field datasetId)"

echo "upload dirty dataset"
dirty_response="$(curl -fsS -F "file=@$ROOT_DIR/data/dirty_business_refund_dataset.csv" "$BASE_URL/api/datasets/upload")"
echo "$dirty_response"
dirty_dataset_id="$(printf '%s' "$dirty_response" | json_field datasetId)"

echo "preview clean dataset"
curl -fsS "$BASE_URL/api/datasets/$clean_dataset_id/preview"
echo

echo "preview dirty dataset"
curl -fsS "$BASE_URL/api/datasets/$dirty_dataset_id/preview"
echo

echo "start analysis"
start_response="$(curl -fsS -X POST "$BASE_URL/api/analysis/$clean_dataset_id/start")"
echo "$start_response"
job_id="$(printf '%s' "$start_response" | json_field jobId)"

echo "status"
curl -fsS "$BASE_URL/api/analysis/$job_id/status"
echo

echo "demo status update"
curl -fsS -X PATCH "$BASE_URL/api/analysis/$job_id/status" \
  -H "Content-Type: application/json" \
  -d '{"status":"COMPLETED"}'
echo
