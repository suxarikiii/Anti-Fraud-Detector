#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
RABBIT_API_URL="${RABBIT_API_URL:-}"
RABBIT_USER="${RABBIT_USER:-guest}"
RABBIT_PASSWORD="${RABBIT_PASSWORD:-guest}"
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

rabbit_publish() {
  local routing_key="$1"
  local payload="$2"
  local request
  request="$(python3 -c 'import json,sys; print(json.dumps({"properties":{"content_type":"application/json","delivery_mode":2},"routing_key":sys.argv[1],"payload":sys.argv[2],"payload_encoding":"string"}))' "$routing_key" "$payload")"
  curl -fsS -u "$RABBIT_USER:$RABBIT_PASSWORD" -H 'Content-Type: application/json' \
    -d "$request" "$RABBIT_API_URL/api/exchanges/%2F/pipeline.exchange/publish"
  echo
}

wait_for_status() {
  local id="$1"
  local expected="$2"
  for _ in $(seq 1 50); do
    local body
    body="$(curl -fsS "$BASE_URL/api/analysis/$id/status")"
    if [[ "$(printf '%s' "$body" | json_field status)" == "$expected" ]]; then
      printf '%s\n' "$body"
      return 0
    fi
    sleep 0.2
  done
  echo "job $id did not reach $expected" >&2
  return 1
}

echo "health"
curl -fsS "$BASE_URL/api/datasets/health"
echo

echo "upload clean dataset"
clean_response="$(curl -fsS -F "file=@$ROOT_DIR/data/clean_refund_dataset.csv" "$BASE_URL/api/datasets/upload")"
echo "$clean_response"
clean_dataset_id="$(printf '%s' "$clean_response" | json_field datasetId)"

echo "expected validation error: duplicate return IDs"
duplicate_csv="$(mktemp)"
printf '%s\n' \
  'order_id,customer_id,return_id,support_agent_id,order_amount,refund_amount,decision,timestamp' \
  'order_1,customer_1,return_1,agent_1,10,5,APPROVED,2026-06-01T09:06:00Z' \
  'order_2,customer_2,return_1,agent_2,20,7,APPROVED,2026-06-01T09:07:00Z' > "$duplicate_csv"
expect_status 400 -F "file=@$duplicate_csv;filename=duplicates.csv" "$BASE_URL/api/datasets/upload"
rm -f "$duplicate_csv"

echo "preview clean dataset"
curl -fsS "$BASE_URL/api/datasets/$clean_dataset_id/preview"
echo

echo "start analysis"
start_response="$(curl -fsS -X POST "$BASE_URL/api/analysis/$clean_dataset_id/start")"
echo "$start_response"
job_id="$(printf '%s' "$start_response" | json_field jobId)"

echo "status"
curl -fsS "$BASE_URL/api/analysis/$job_id/status"
echo

echo "dataset list"
curl -fsS "$BASE_URL/api/datasets?page=1&pageSize=10"
echo

echo "dataset history and audit"
curl -fsS "$BASE_URL/api/datasets/$clean_dataset_id"
echo

if [[ -n "$RABBIT_API_URL" ]]; then
  echo "automatic lifecycle events"
  ids="\"datasetId\":\"$clean_dataset_id\",\"jobId\":\"$job_id\""
  rabbit_publish dataset.normalized "{$ids,\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}"
  rabbit_publish refund.relations.built "{$ids,\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}"
  rabbit_publish refund.scoring.completed "{$ids,\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}"
  wait_for_status "$job_id" COMPLETED

  echo "retry completed job"
  retry_response="$(curl -fsS -X POST "$BASE_URL/api/analysis/$job_id/retry")"
  echo "$retry_response"
  retry_job_id="$(printf '%s' "$retry_response" | json_field jobId)"
  retry_ids="\"datasetId\":\"$clean_dataset_id\",\"jobId\":\"$retry_job_id\""
  rabbit_publish dataset.normalized "{$retry_ids}"
  rabbit_publish refund.relations.built "{$retry_ids}"
  rabbit_publish refund.scoring.completed "{$retry_ids}"
  wait_for_status "$retry_job_id" COMPLETED

  echo "archive terminal dataset"
  expect_status 204 -X POST "$BASE_URL/api/datasets/$clean_dataset_id/archive"
fi

echo "expected error: dataset not found"
expect_status 404 "$BASE_URL/api/datasets/00000000-0000-0000-0000-000000000000/preview"

echo "expected error: job not found"
expect_status 404 "$BASE_URL/api/analysis/00000000-0000-0000-0000-000000000000/status"

echo "expected error: empty CSV"
empty_csv="$(mktemp)"
printf ' \n' > "$empty_csv"
expect_status 400 -F "file=@$empty_csv;filename=empty.csv" "$BASE_URL/api/datasets/upload"
rm -f "$empty_csv"
