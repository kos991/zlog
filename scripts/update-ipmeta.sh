#!/usr/bin/env bash
set -euo pipefail

# Download metowolf/iplist CIDR snapshots and convert to zlog builtin format.
# Usage: bash scripts/update-ipmeta.sh [output_dir]

OUTPUT="${1:-internal/ipmeta/data}"
mkdir -p "$OUTPUT"

echo "Downloading ipmeta snapshots..."
curl -fsSL "https://raw.githubusercontent.com/metowolf/iplist/master/data/country/cn_ipv4.json" -o "$OUTPUT/cn_ipv4.json"

echo "Converting to zlog builtin format..."
jq '[.[] | {cidr: .cidr, country: .country, province: .province, city: .city}]' "$OUTPUT/cn_ipv4.json" > "$OUTPUT/ipmeta-builtin.json"

echo "Done: $OUTPUT/ipmeta-builtin.json"
