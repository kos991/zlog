#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-0.1.0}"
ZVEC_VERSION="${ZVEC_VERSION:-v0.5.0}"
GOARCH="${GOARCH:-amd64}"
PACKAGE="sangfor-log-search"
MAINTAINER="${MAINTAINER:-ops@example.com}"
THIRD_PARTY_DIR="${ROOT_DIR}/third_party"
ZVEC_GO_DIR="${THIRD_PARTY_DIR}/zvec-go"
DIST_DIR="${ROOT_DIR}/dist"

case "${GOARCH}" in
  amd64)
    DEB_ARCH="amd64"
    ZVEC_LIB_ARCH="linux_amd64"
    CC_VALUE="${CC:-gcc}"
    ;;
  arm64)
    DEB_ARCH="arm64"
    ZVEC_LIB_ARCH="linux_arm64"
    CC_VALUE="${CC:-aarch64-linux-gnu-gcc}"
    ;;
  *)
    echo "错误：不支持的 GOARCH=${GOARCH}，当前仅支持 amd64 / arm64" >&2
    exit 1
    ;;
esac

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "错误：未找到命令 $1" >&2
    exit 1
  fi
}

require_cmd go
require_cmd git
require_cmd dpkg-deb
require_cmd "${CC_VALUE}"

mkdir -p "${THIRD_PARTY_DIR}" "${DIST_DIR}"

if [ ! -d "${ZVEC_GO_DIR}/.git" ]; then
  git clone --depth 1 --branch "${ZVEC_VERSION}" https://github.com/zvec-ai/zvec-go.git "${ZVEC_GO_DIR}"
fi

(cd "${ZVEC_GO_DIR}" && go run ./cmd/download-libs -version "${ZVEC_VERSION}")

ZVEC_LIB_DIR="${ZVEC_GO_DIR}/lib/${ZVEC_LIB_ARCH}"
if [ ! -d "${ZVEC_LIB_DIR}" ]; then
  echo "错误：未找到 zvec 预编译库目录：${ZVEC_LIB_DIR}" >&2
  exit 1
fi

if [ -f "${ROOT_DIR}/go.mod.bak" ]; then
  rm -f "${ROOT_DIR}/go.mod.bak"
fi
cp "${ROOT_DIR}/go.mod" "${ROOT_DIR}/go.mod.bak"
restore_go_mod() {
  mv "${ROOT_DIR}/go.mod.bak" "${ROOT_DIR}/go.mod" 2>/dev/null || true
}
trap restore_go_mod EXIT

go mod edit -replace "github.com/zvec-ai/zvec-go=${ZVEC_GO_DIR}"
go mod tidy

BIN_PATH="${DIST_DIR}/log-search-${GOARCH}"
CGO_ENABLED=1 \
GOOS=linux \
GOARCH="${GOARCH}" \
CC="${CC_VALUE}" \
CGO_CFLAGS="-I${ZVEC_GO_DIR}/lib/include" \
CGO_LDFLAGS="-L${ZVEC_LIB_DIR} -lzvec_c_api -Wl,-rpath,/opt/sangfor-log-search/lib" \
go build -trimpath -ldflags="-s -w" -o "${BIN_PATH}" "${ROOT_DIR}/cmd/log-search"

PKG_ROOT="${DIST_DIR}/${PACKAGE}_${VERSION}_${DEB_ARCH}"
rm -rf "${PKG_ROOT}"
mkdir -p \
  "${PKG_ROOT}/DEBIAN" \
  "${PKG_ROOT}/opt/sangfor-log-search/bin" \
  "${PKG_ROOT}/opt/sangfor-log-search/lib" \
  "${PKG_ROOT}/opt/sangfor-log-search/db" \
  "${PKG_ROOT}/opt/sangfor-log-search/config" \
  "${PKG_ROOT}/usr/local/bin"

cp "${BIN_PATH}" "${PKG_ROOT}/opt/sangfor-log-search/bin/log-search"
cp "${ZVEC_LIB_DIR}/"* "${PKG_ROOT}/opt/sangfor-log-search/lib/"
chmod 0755 "${PKG_ROOT}/opt/sangfor-log-search/bin/log-search"

cat > "${PKG_ROOT}/opt/sangfor-log-search/config/log-search.env" <<'ENV'
LOG_SEARCH_LOG_DIR=/data/sangfor_fw_log
LOG_SEARCH_DB=/opt/sangfor-log-search/db/sangfor_logs
ENV

cat > "${PKG_ROOT}/usr/local/bin/log-search" <<'SH'
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
chmod 0755 "${PKG_ROOT}/usr/local/bin/log-search"

cat > "${PKG_ROOT}/DEBIAN/control" <<CONTROL
Package: ${PACKAGE}
Version: ${VERSION}
Section: utils
Priority: optional
Architecture: ${DEB_ARCH}
Maintainer: ${MAINTAINER}
Depends: libc6, libstdc++6, bash
Description: Sangfor firewall log search tool powered by zvec
 A local log import and query tool for Sangfor firewall logs.
 It stores log lines in a zvec database and provides exact IP,
 date range, keyword, file-name filtering and result export.
CONTROL

cat > "${PKG_ROOT}/DEBIAN/postinst" <<'SH'
#!/usr/bin/env bash
set -e
mkdir -p /opt/sangfor-log-search/db /data
chmod 0755 /usr/local/bin/log-search /opt/sangfor-log-search/bin/log-search
echo "安装完成：请执行 log-search import 导入 /data/sangfor_fw_log 下的日志"
SH
chmod 0755 "${PKG_ROOT}/DEBIAN/postinst"

DEB_PATH="${DIST_DIR}/${PACKAGE}_${VERSION}_${DEB_ARCH}.deb"
dpkg-deb --build --root-owner-group "${PKG_ROOT}" "${DEB_PATH}"
echo "${DEB_PATH}"

