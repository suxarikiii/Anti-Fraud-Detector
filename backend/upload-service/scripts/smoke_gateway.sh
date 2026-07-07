#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

json_field() {
  python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$1"
}

expect_status() {
  local expected_status="$1"
  shift
  local body_file
  body_file="$(mktemp)"
  local actual_status
  actual_status="$(curl -sS -o "$body_file" -w "%{http_code}" "$@")"
  cat "$body_file"
  echo
  rm -f "$body_file"
  if [[ "$actual_status" != "$expected_status" ]]; then
    echo "expected HTTP $expected_status, got HTTP $actual_status" >&2
    exit 1
  fi
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

echo "demo status update: NORMALIZED"
curl -fsS -X PATCH "$BASE_URL/api/analysis/$job_id/status" \
  -H "Content-Type: application/json" \
  -d '{"status":"NORMALIZED"}'
echo

echo "demo status update: BUILDING_RELATIONS"
curl -fsS -X PATCH "$BASE_URL/api/analysis/$job_id/status" \
  -H "Content-Type: application/json" \
  -d '{"status":"BUILDING_RELATIONS"}'
echo

echo "demo status update: SCORING"
curl -fsS -X PATCH "$BASE_URL/api/analysis/$job_id/status" \
  -H "Content-Type: application/json" \
  -d '{"status":"SCORING"}'
echo

echo "demo status update: COMPLETED"
curl -fsS -X PATCH "$BASE_URL/api/analysis/$job_id/status" \
  -H "Content-Type: application/json" \
  -d '{"status":"COMPLETED"}'
echo

echo "expected error: dataset not found"
expect_status 404 "$BASE_URL/api/datasets/00000000-0000-0000-0000-000000000000/preview"

echo "expected error: job not found"
expect_status 404 "$BASE_URL/api/analysis/00000000-0000-0000-0000-000000000000/status"

echo "expected error: empty CSV"
empty_csv="$(mktemp)"
printf ' \n' > "$empty_csv"
expect_status 400 -F "file=@$empty_csv;filename=empty.csv" "$BASE_URL/api/datasets/upload"
rm -f "$empty_csv"
