#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-/opt/sangfor-log-search}"
rm -f /usr/local/bin/log-search
rm -rf "${INSTALL_DIR}"
echo "卸载完成"

