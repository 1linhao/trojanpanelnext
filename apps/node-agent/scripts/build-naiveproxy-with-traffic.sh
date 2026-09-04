#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK_DIR="${NAIVEPROXY_BUILD_WORKDIR:-${ROOT_DIR}/.cache/naiveproxy-build}"
FORWARDPROXY_REPO="${FORWARDPROXY_REPO:-https://github.com/klzgrad/forwardproxy.git}"
FORWARDPROXY_REF="${FORWARDPROXY_REF:-d62c80d3dd2c706b6b87579844d2397bddd18317}"
CADDY_VERSION="${CADDY_VERSION:-v2.8.4}"
GOOS_VALUE="${GOOS:-$(go env GOOS)}"
GOARCH_VALUE="${GOARCH:-$(go env GOARCH)}"
GOARM_VALUE="${GOARM:-}"
OUTPUT="${OUTPUT:-${ROOT_DIR}/build/naiveproxy-${GOOS_VALUE}-${GOARCH_VALUE}${GOARM_VALUE:+/v${GOARM_VALUE}}}"
XCADDY_BIN="${XCADDY_BIN:-$(go env GOPATH)/bin/xcaddy}"

mkdir -p "${WORK_DIR}" "$(dirname "${OUTPUT}")"

if [[ ! -x "${XCADDY_BIN}" ]]; then
  mkdir -p "$(dirname "${XCADDY_BIN}")"
  env -u GOOS -u GOARCH -u GOARM \
    GOBIN="$(dirname "${XCADDY_BIN}")" \
    go install github.com/caddyserver/xcaddy/cmd/xcaddy@v0.4.4
fi

if [[ ! -d "${WORK_DIR}/forwardproxy/.git" ]]; then
  rm -rf "${WORK_DIR}/forwardproxy"
  git clone "${FORWARDPROXY_REPO}" "${WORK_DIR}/forwardproxy"
fi

git -C "${WORK_DIR}/forwardproxy" fetch --depth 1 origin "${FORWARDPROXY_REF}"
git -C "${WORK_DIR}/forwardproxy" checkout --force FETCH_HEAD
git -C "${WORK_DIR}/forwardproxy" clean -fd
git -C "${WORK_DIR}/forwardproxy" apply "${ROOT_DIR}/scripts/naiveproxy/forwardproxy-traffic.patch"
gofmt -w "${WORK_DIR}/forwardproxy/forwardproxy.go" "${WORK_DIR}/forwardproxy/trojan_panel_traffic.go"

GOOS="${GOOS_VALUE}" GOARCH="${GOARCH_VALUE}" GOARM="${GOARM_VALUE}" \
  "${XCADDY_BIN}" build "${CADDY_VERSION}" \
  --with "github.com/caddyserver/forwardproxy=${WORK_DIR}/forwardproxy" \
  --output "${OUTPUT}"

chmod 755 "${OUTPUT}"
echo "built ${OUTPUT}"
