#!/usr/bin/env bash
set -euo pipefail

# Verify ClickHouse is reachable and tables exist.

CLICKHOUSE_URL="${CLICKHOUSE_URL:-http://127.0.0.1:8123}"

echo "==> Ping ClickHouse"
curl -fsS "$CLICKHOUSE_URL/ping"
echo ""

echo "==> Check nat_logs table"
curl -fsS "$CLICKHOUSE_URL/?query=SELECT+count()+FROM+zlog.nat_logs"
echo ""

echo "==> Check import_state table"
curl -fsS "$CLICKHOUSE_URL/?query=SELECT+count()+FROM+zlog.import_state"
echo ""

echo "==> Check jobs table"
curl -fsS "$CLICKHOUSE_URL/?query=SELECT+count()+FROM+zlog.jobs"
echo ""

echo "ClickHouse verification passed."
