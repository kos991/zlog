#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-0.1.0}"
ZVEC_VERSION="${ZVEC_VERSION:-v0.5.0}"
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-$(go env GOARCH)}"
PACKAGE_NAME="log-search-kylin-${GOARCH}-${VERSION}"
THIRD_PARTY_DIR="${ROOT_DIR}/third_party"
ZVEC_GO_DIR="${THIRD_PARTY_DIR}/zvec-go"
DIST_DIR="${ROOT_DIR}/dist"
STAGE_DIR="${DIST_DIR}/${PACKAGE_NAME}"

if ! command -v go >/dev/null 2>&1; then
  echo "错误：未找到 go，请在构建机安装 Go 1.21 或更高版本" >&2
  exit 1
fi

if ! command -v git >/dev/null 2>&1; then
  echo "错误：未找到 git" >&2
  exit 1
fi

mkdir -p "${THIRD_PARTY_DIR}" "${DIST_DIR}"

if [ ! -d "${ZVEC_GO_DIR}/.git" ]; then
  git clone --depth 1 --branch "${ZVEC_VERSION}" https://github.com/zvec-ai/zvec-go.git "${ZVEC_GO_DIR}"
fi

(cd "${ZVEC_GO_DIR}" && go run ./cmd/download-libs -version "${ZVEC_VERSION}")

case "${GOARCH}" in
  amd64) ZVEC_LIB_ARCH="linux_amd64" ;;
  arm64) ZVEC_LIB_ARCH="linux_arm64" ;;
  *) echo "错误：zvec-go 官方预编译库暂不支持 GOARCH=${GOARCH}" >&2; exit 1 ;;
esac

ZVEC_LIB_DIR="${ZVEC_GO_DIR}/lib/${ZVEC_LIB_ARCH}"
if [ ! -d "${ZVEC_LIB_DIR}" ]; then
  echo "错误：未找到 zvec 库目录：${ZVEC_LIB_DIR}" >&2
  exit 1
fi

cp "${ROOT_DIR}/go.mod" "${ROOT_DIR}/go.mod.bak"
trap 'mv "${ROOT_DIR}/go.mod.bak" "${ROOT_DIR}/go.mod" 2>/dev/null || true' EXIT
go mod edit -replace "github.com/zvec-ai/zvec-go=${ZVEC_GO_DIR}"

CGO_ENABLED=1 \
GOOS="${GOOS}" \
GOARCH="${GOARCH}" \
CGO_CFLAGS="-I${ZVEC_GO_DIR}/lib/include" \
CGO_LDFLAGS="-L${ZVEC_LIB_DIR} -lzvec_c_api -Wl,-rpath,\$ORIGIN/../lib" \
go build -trimpath -ldflags="-s -w" -o "${DIST_DIR}/log-search" "${ROOT_DIR}/cmd/log-search"

rm -rf "${STAGE_DIR}"
mkdir -p "${STAGE_DIR}/bin" "${STAGE_DIR}/lib" "${STAGE_DIR}/db" "${STAGE_DIR}/config"
cp "${DIST_DIR}/log-search" "${STAGE_DIR}/bin/log-search"
cp "${ZVEC_LIB_DIR}/"* "${STAGE_DIR}/lib/"
cp "${ROOT_DIR}/scripts/install.sh" "${STAGE_DIR}/install.sh"
cp "${ROOT_DIR}/scripts/uninstall.sh" "${STAGE_DIR}/uninstall.sh"
chmod +x "${STAGE_DIR}/bin/log-search" "${STAGE_DIR}/install.sh" "${STAGE_DIR}/uninstall.sh"

cat > "${STAGE_DIR}/config/log-search.env" <<'ENV'
LOG_SEARCH_LOG_DIR=/data/sangfor_fw_log
LOG_SEARCH_DB=/opt/sangfor-log-search/db/sangfor_logs
LD_LIBRARY_PATH=/opt/sangfor-log-search/lib:${LD_LIBRARY_PATH:-}
ENV

(cd "${DIST_DIR}" && tar -czf "${PACKAGE_NAME}.tar.gz" "${PACKAGE_NAME}")
echo "安装包已生成：${DIST_DIR}/${PACKAGE_NAME}.tar.gz"

