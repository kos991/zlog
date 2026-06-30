#!/usr/bin/env bash
set -euo pipefail

SRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="${INSTALL_DIR:-/opt/sangfor-log-search}"

mkdir -p "${INSTALL_DIR}/bin" "${INSTALL_DIR}/lib" "${INSTALL_DIR}/db" "${INSTALL_DIR}/config" /usr/local/bin
cp "${SRC_DIR}/bin/log-search" "${INSTALL_DIR}/bin/log-search"
cp -r "${SRC_DIR}/lib/." "${INSTALL_DIR}/lib/"
if [ -d "${SRC_DIR}/db" ]; then
  cp -r "${SRC_DIR}/db/." "${INSTALL_DIR}/db/"
fi
cp -r "${SRC_DIR}/config/." "${INSTALL_DIR}/config/"

cat > /usr/local/bin/log-search <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [ -f /opt/sangfor-log-search/config/log-search.env ]; then
  set -a
  # shellcheck disable=SC1091
  . /opt/sangfor-log-search/config/log-search.env
  set +a
fi
export LD_LIBRARY_PATH="/opt/sangfor-log-search/lib:${LD_LIBRARY_PATH:-}"
exec /opt/sangfor-log-search/bin/log-search "$@"
SH
chmod +x /usr/local/bin/log-search "${INSTALL_DIR}/bin/log-search"

echo "安装完成：/usr/local/bin/log-search"
echo "首次使用：log-search import"

