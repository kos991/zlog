#!/usr/bin/env bash
set -euo pipefail

# Seed a sample NAT log file for smoke testing.
# Usage: bash scripts/seed-sample.sh [data_dir]

DATA_DIR="${1:-data}"
mkdir -p "$DATA_DIR"

cat > "$DATA_DIR/10.10.10.1_2026-04-28.log-20260429.gz" <<'SAMPLE'
Apr 28 00:00:23 localhost nat: 日志类型:NAT日志, NAT类型:snat, 源IP:2.55.81.106, 源端口:1799, 目的IP:140.205.70.178, 目的端口:443, 协议:6, 转换后的IP:58.216.48.6, 转换后的端口:1799
Apr 28 00:00:24 localhost nat: 日志类型:NAT日志, NAT类型:snat, 源IP:1.1.1.1, 源端口:80, 目的IP:2.2.2.2, 目的端口:443, 协议:6, 转换后的IP:3.3.3.3, 转换后的端口:80
Apr 28 00:00:25 localhost nat: 日志类型:NAT日志, NAT类型:dnat, 源IP:4.4.4.4, 源端口:443, 目的IP:5.5.5.5, 目的端口:80, 协议:17, 转换后的IP:6.6.6.6, 转换后的端口:443
SAMPLE

# Re-compress as gzip
tmp=$(mktemp)
gzip -c "$DATA_DIR/10.10.10.1_2026-04-28.log-20260429.gz" > "$tmp"
mv "$tmp" "$DATA_DIR/10.10.10.1_2026-04-28.log-20260429.gz"

echo "Seeded: $DATA_DIR/10.10.10.1_2026-04-28.log-20260429.gz"
