#!/usr/bin/env bash
set -euo pipefail

# End-to-end smoke test for zlog.
# Requires: docker compose up --build -d running, jq, curl.

base_url="${BASE_URL:-http://127.0.0.1:8080}"

echo "==> Health check"
curl -fsS "$base_url/health" | jq -e '.status == "ok"' >/dev/null
echo "    OK"

echo "==> Login"
login_cookie=$(mktemp)
curl -fsS -c "$login_cookie" \
  -d 'username=admin&password=change-me' \
  "$base_url/login" >/dev/null
echo "    OK"

echo "==> Trigger import"
curl -fsS -b "$login_cookie" \
  -X POST "$base_url/api/import" | jq -e '.status == "import started"' >/dev/null
echo "    OK"

echo "==> Wait for import to process"
sleep 5

echo "==> Query logs by dst IP"
curl -fsS -b "$login_cookie" \
  "$base_url/api/logs?start=2026-04-28&end=2026-04-28&ip=140.205.70.178&field=dst" \
  | jq -e '.rows | length > 0' >/dev/null
echo "    OK"

echo "==> Query logs by src IP"
curl -fsS -b "$login_cookie" \
  "$base_url/api/logs?start=2026-04-28&end=2026-04-28&ip=2.55.81.106&field=src" \
  | jq -e '.rows | length > 0' >/dev/null
echo "    OK"

echo "==> Query logs by translated IP"
curl -fsS -b "$login_cookie" \
  "$base_url/api/logs?start=2026-04-28&end=2026-04-28&ip=58.216.48.6&field=tr" \
  | jq -e '.rows | length > 0' >/dev/null
echo "    OK"

echo "==> Check geo label in result"
curl -fsS -b "$login_cookie" \
  "$base_url/api/logs?start=2026-04-28&end=2026-04-28&ip=140.205.70.178&field=dst" \
  | jq -e '.rows[0].dst_country != ""' >/dev/null
echo "    OK"

echo "==> List export jobs"
curl -fsS -b "$login_cookie" "$base_url/api/jobs" >/dev/null
echo "    OK"

echo "==> Trigger CSV export"
curl -fsS -b "$login_cookie" \
  "$base_url/api/exports?start=2026-04-28&end=2026-04-28&ip=140.205.70.178&field=dst&format=csv" \
  | jq -e '.status == "queued"' >/dev/null
echo "    OK"

echo ""
echo "All smoke tests passed."
rm -f "$login_cookie"
