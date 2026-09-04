#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
YQ_BIN="${YQ_BIN:-${ROOT_DIR}/.cache/bin/yq}"

OUTPUT_DIR="${OUTPUT_DIR:-${ROOT_DIR}/dist/test-images}"
PLATFORM="${PLATFORM:-linux/amd64}"
IMAGE_TAG="${IMAGE_TAG:-singbox}"
PANEL_DIR="${PANEL_DIR:-${ROOT_DIR}/../trojan-panel}"
UI_DIR="${UI_DIR:-${ROOT_DIR}/../trojan-panel-ui}"
CORE_DIR="${CORE_DIR:-${ROOT_DIR}/../trojan-panel-core}"
PANEL_REPO="${PANEL_REPO:-}"
UI_REPO="${UI_REPO:-}"
CORE_REPO="${CORE_REPO:-}"
PANEL_BRANCH="${PANEL_BRANCH:-}"
UI_BRANCH="${UI_BRANCH:-}"
CORE_BRANCH="${CORE_BRANCH:-}"
PANEL_IMAGE="${PANEL_IMAGE:-ghcr.io/1linhao/trojan-panel:${IMAGE_TAG}}"
UI_IMAGE="${UI_IMAGE:-ghcr.io/1linhao/trojan-panel-ui:${IMAGE_TAG}}"
CORE_IMAGE="${CORE_IMAGE:-ghcr.io/1linhao/trojan-panel-core:${IMAGE_TAG}}"
OFFICIAL_CORE_IMAGE="${OFFICIAL_CORE_IMAGE:-jonssonyan/trojan-panel-core:latest}"
BUILD_PANEL="${BUILD_PANEL:-1}"
BUILD_UI="${BUILD_UI:-1}"
BUILD_CORE="${BUILD_CORE:-1}"
BUILD_NAIVEPROXY="${BUILD_NAIVEPROXY:-1}"
SAVE_IMAGES="${SAVE_IMAGES:-1}"
ARCHIVE_NAME="${ARCHIVE_NAME:-}"
DOCKER=(docker)

usage() {
  cat <<EOF
Usage:
  $0 [examples/build-images.env.yaml]

Build local test Docker images and save them as a transferable archive.
EOF
}

install_yq() {
  if [[ -x "${YQ_BIN}" ]]; then
    return
  fi
  if command -v yq >/dev/null 2>&1; then
    YQ_BIN="$(command -v yq)"
    return
  fi

  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$(uname -m)" in
  x86_64 | amd64) arch="amd64" ;;
  aarch64 | arm64) arch="arm64" ;;
  *) echo "Unsupported yq architecture: $(uname -m)" >&2; exit 1 ;;
  esac

  mkdir -p "$(dirname "${YQ_BIN}")"
  curl -fsSL "https://github.com/mikefarah/yq/releases/download/v4.44.3/yq_${os}_${arch}" -o "${YQ_BIN}"
  chmod +x "${YQ_BIN}"
}

yaml_read_raw() {
  local file="$1"
  local key="$2"
  "${YQ_BIN}" -r ".trojan_panel.${key} // \"\"" "${file}"
}

cfg_first() {
  local file="$1"
  shift
  local key value
  for key in "$@"; do
    value="$(yaml_read_raw "${file}" "${key}")"
    if [[ -n "${value}" && "${value}" != "null" ]]; then
      printf '%s' "${value}"
      return
    fi
  done
}

cfg_apply() {
  local file="$1"
  local var_name="$2"
  shift 2
  local value
  value="$(cfg_first "${file}" "$@")"
  if [[ -n "${value}" ]]; then
    printf -v "${var_name}" '%s' "${value}"
  fi
}

load_config() {
  local file="${1:-}"
  if [[ -z "${file}" ]]; then
    return
  fi
  if [[ ! -f "${file}" ]]; then
    echo "Config file not found: ${file}" >&2
    exit 1
  fi

  install_yq
  cfg_apply "${file}" OUTPUT_DIR output_dir
  cfg_apply "${file}" PLATFORM platform
  cfg_apply "${file}" IMAGE_TAG image_tag tag
  cfg_apply "${file}" PANEL_DIR panel_dir
  cfg_apply "${file}" UI_DIR ui_dir
  cfg_apply "${file}" CORE_DIR core_dir
  cfg_apply "${file}" PANEL_REPO panel_repo
  cfg_apply "${file}" UI_REPO ui_repo
  cfg_apply "${file}" CORE_REPO core_repo
  cfg_apply "${file}" PANEL_BRANCH panel_branch
  cfg_apply "${file}" UI_BRANCH ui_branch
  cfg_apply "${file}" CORE_BRANCH core_branch
  cfg_apply "${file}" PANEL_IMAGE panel_image
  cfg_apply "${file}" UI_IMAGE ui_image
  cfg_apply "${file}" CORE_IMAGE core_image
  cfg_apply "${file}" OFFICIAL_CORE_IMAGE official_core_image
  cfg_apply "${file}" BUILD_PANEL build_panel
  cfg_apply "${file}" BUILD_UI build_ui
  cfg_apply "${file}" BUILD_CORE build_core
  cfg_apply "${file}" BUILD_NAIVEPROXY build_naiveproxy
  cfg_apply "${file}" SAVE_IMAGES save_images
  cfg_apply "${file}" ARCHIVE_NAME archive_name
}

abs_path() {
  case "$1" in
  /*) printf '%s' "$1" ;;
  *) printf '%s/%s' "${ROOT_DIR}" "$1" ;;
  esac
}

parse_platform() {
  case "${PLATFORM}" in
  linux/386)
    GOOS_VALUE="linux"; GOARCH_VALUE="386"; GOARM_VALUE=""; TARGETVARIANT_VALUE=""; TARGET_SUFFIX="linux-386" ;;
  linux/amd64)
    GOOS_VALUE="linux"; GOARCH_VALUE="amd64"; GOARM_VALUE=""; TARGETVARIANT_VALUE=""; TARGET_SUFFIX="linux-amd64" ;;
  linux/arm/v6)
    GOOS_VALUE="linux"; GOARCH_VALUE="arm"; GOARM_VALUE="6"; TARGETVARIANT_VALUE="v6"; TARGET_SUFFIX="linux-armv6" ;;
  linux/arm/v7)
    GOOS_VALUE="linux"; GOARCH_VALUE="arm"; GOARM_VALUE="7"; TARGETVARIANT_VALUE="v7"; TARGET_SUFFIX="linux-armv7" ;;
  linux/arm64)
    GOOS_VALUE="linux"; GOARCH_VALUE="arm64"; GOARM_VALUE=""; TARGETVARIANT_VALUE=""; TARGET_SUFFIX="linux-arm64" ;;
  linux/ppc64le)
    GOOS_VALUE="linux"; GOARCH_VALUE="ppc64le"; GOARM_VALUE=""; TARGETVARIANT_VALUE=""; TARGET_SUFFIX="linux-ppc64le" ;;
  linux/s390x)
    GOOS_VALUE="linux"; GOARCH_VALUE="s390x"; GOARM_VALUE=""; TARGETVARIANT_VALUE=""; TARGET_SUFFIX="linux-s390x" ;;
  *)
    echo "Unsupported platform: ${PLATFORM}" >&2
    exit 1
    ;;
  esac
}

docker_build_with_target_args() {
  local image="$1"
  "${DOCKER[@]}" build --platform "${PLATFORM}" \
    --build-arg TARGETOS="${GOOS_VALUE}" \
    --build-arg TARGETARCH="${GOARCH_VALUE}" \
    --build-arg TARGETVARIANT="${TARGETVARIANT_VALUE}" \
    -t "${image}" .
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

detect_docker() {
  if docker ps >/dev/null 2>&1; then
    DOCKER=(docker)
    return
  fi
  if command -v sudo >/dev/null 2>&1 && sudo -n docker ps >/dev/null 2>&1; then
    DOCKER=(sudo -n docker)
    return
  fi
  echo "Cannot connect to Docker daemon. Run as a Docker-enabled user or configure passwordless sudo for docker." >&2
  exit 1
}

sync_repo() {
  local dir="$1"
  local repo="$2"
  local branch="$3"

  if [[ ! -d "${dir}/.git" ]]; then
    if [[ -z "${repo}" ]]; then
      echo "Repository directory not found: ${dir}" >&2
      exit 1
    fi
    mkdir -p "$(dirname "${dir}")"
    if [[ -n "${branch}" ]]; then
      git clone --branch "${branch}" "${repo}" "${dir}"
    else
      git clone "${repo}" "${dir}"
    fi
    return
  fi

  if [[ -n "${branch}" ]]; then
    git -C "${dir}" fetch origin "${branch}"
    git -C "${dir}" checkout "${branch}"
    git -C "${dir}" pull --ff-only origin "${branch}"
  fi
}

build_panel() {
  sync_repo "${PANEL_DIR}" "${PANEL_REPO}" "${PANEL_BRANCH}"
  mkdir -p "${PANEL_DIR}/build"
  echo "---> Build backend binary: ${TARGET_SUFFIX}"
  (
    cd "${PANEL_DIR}"
    CGO_ENABLED=0 GOOS="${GOOS_VALUE}" GOARCH="${GOARCH_VALUE}" GOARM="${GOARM_VALUE}" \
      go build -o "build/trojan-panel-${TARGET_SUFFIX}" -trimpath -ldflags "-s -w -buildid=" .
    docker_build_with_target_args "${PANEL_IMAGE}"
  )
}

build_ui() {
  sync_repo "${UI_DIR}" "${UI_REPO}" "${UI_BRANCH}"
  echo "---> Build UI dist"
  (
    cd "${UI_DIR}"
    if command -v yarn >/dev/null 2>&1; then
      yarn install --frozen-lockfile
      yarn build
    elif command -v npm >/dev/null 2>&1; then
      npm install
      npm run build
    else
      echo "Missing yarn or npm" >&2
      exit 1
    fi
    "${DOCKER[@]}" build --platform "${PLATFORM}" -t "${UI_IMAGE}" .
  )
}

copy_core_binary_from_official_image() {
  local container_id
  "${DOCKER[@]}" pull --platform "${PLATFORM}" "${OFFICIAL_CORE_IMAGE}"
  container_id="$("${DOCKER[@]}" create --platform "${PLATFORM}" "${OFFICIAL_CORE_IMAGE}")"
  trap '"${DOCKER[@]}" rm -f "${container_id}" >/dev/null 2>&1 || true' RETURN

  "${DOCKER[@]}" cp "${container_id}:/tpdata/trojan-panel-core/bin/xray/xray" "${CORE_DIR}/build/xray-${TARGET_SUFFIX}"
  "${DOCKER[@]}" cp "${container_id}:/tpdata/trojan-panel-core/bin/trojango/trojan-go" "${CORE_DIR}/build/trojan-go-${TARGET_SUFFIX}"
  "${DOCKER[@]}" cp "${container_id}:/tpdata/trojan-panel-core/bin/hysteria/hysteria" "${CORE_DIR}/build/hysteria-${TARGET_SUFFIX}"
  "${DOCKER[@]}" cp "${container_id}:/tpdata/trojan-panel-core/bin/naiveproxy/naiveproxy" "${CORE_DIR}/build/naiveproxy-${TARGET_SUFFIX}"
  "${DOCKER[@]}" cp "${container_id}:/tpdata/trojan-panel-core/bin/hysteria2/hysteria2" "${CORE_DIR}/build/hysteria2-${TARGET_SUFFIX}"
  "${DOCKER[@]}" rm -f "${container_id}" >/dev/null
  trap - RETURN
}

build_core() {
  sync_repo "${CORE_DIR}" "${CORE_REPO}" "${CORE_BRANCH}"
  mkdir -p "${CORE_DIR}/build"
  echo "---> Build core binary: ${TARGET_SUFFIX}"
  (
    cd "${CORE_DIR}"
    CGO_ENABLED=0 GOOS="${GOOS_VALUE}" GOARCH="${GOARCH_VALUE}" GOARM="${GOARM_VALUE}" \
      go build -o "build/trojan-panel-core-${TARGET_SUFFIX}" -trimpath -ldflags "-s -w -buildid=" .
  )

  echo "---> Copy protocol binaries from ${OFFICIAL_CORE_IMAGE}"
  copy_core_binary_from_official_image

  if [[ "${BUILD_NAIVEPROXY}" == "1" ]]; then
    echo "---> Build custom naiveproxy with traffic API"
    (
      cd "${CORE_DIR}"
      OUTPUT="${CORE_DIR}/build/naiveproxy-${TARGET_SUFFIX}" \
        CGO_ENABLED=0 \
        GOOS="${GOOS_VALUE}" GOARCH="${GOARCH_VALUE}" GOARM="${GOARM_VALUE}" \
        bash scripts/build-naiveproxy-with-traffic.sh
    )
  fi

  (
    cd "${CORE_DIR}"
    docker_build_with_target_args "${CORE_IMAGE}"
  )
}

write_generated_configs() {
  local generated_dir="$1"
  mkdir -p "${generated_dir}"
  cat >"${generated_dir}/web-local-image.env.yaml" <<EOF
trojan_panel:
  mode: "deploy"
  web_hostname: "panel-test.example.com"
  web_mail: "admin@example.com"
  image_bundle_dir: "/root/trojan-panel-test-images"

  caddy_image: "caddy:2.8.4"
  mariadb_image: "mariadb:10.7.3"
  redis_image: "redis:6.2.7"
  panel_image: "${PANEL_IMAGE}"
  ui_image: "${UI_IMAGE}"

  mariadb_port: "9507"
  redis_port: "6378"
  panel_port: "8081"
  ui_port: "8888"
  mariadb_password: ""
  redis_password: ""
  force: "1"
  purge_data: "0"
EOF

  cat >"${generated_dir}/node-local-image.env.yaml" <<EOF
trojan_panel:
  mode: "deploy"
  node_hostname: "node-test.example.com"
  node_mail: "admin@example.com"
  image_bundle_dir: "/root/trojan-panel-test-images"

  node_caddy_http_port: "80"
  node_caddy_https_port: "8863"
  caddy_image: "caddy:2.8.4"
  core_image: "${CORE_IMAGE}"

  mariadb_host: "panel-test.example.com"
  mariadb_port: "9507"
  mariadb_user: "root"
  mariadb_password: "your-mariadb-password"
  database: "trojan_panel_db"
  account_table: "account"

  redis_host: "panel-test.example.com"
  redis_port: "6378"
  redis_password: "your-redis-password"

  grpc_port: "8100"
  core_port: "8082"
  force: "1"
  purge_data: "0"
EOF
}

save_images() {
  if [[ "${SAVE_IMAGES}" != "1" ]]; then
    return
  fi

  mkdir -p "${OUTPUT_DIR}"
  local archive="${ARCHIVE_NAME}"
  if [[ -z "${archive}" ]]; then
    archive="trojan-panel-test-images-${PLATFORM//\//-}-${IMAGE_TAG}.tar.gz"
  fi
  case "${archive}" in
  /*) ;;
  *) archive="${OUTPUT_DIR}/${archive}" ;;
  esac
  mkdir -p "$(dirname "${archive}")"

  echo "---> Save Docker images: ${archive}"
  "${DOCKER[@]}" save "${PANEL_IMAGE}" "${UI_IMAGE}" "${CORE_IMAGE}" | gzip -c >"${archive}"
  write_generated_configs "${OUTPUT_DIR}"

  cat >"${OUTPUT_DIR}/README.txt" <<EOF
Transfer this directory to the VPS, for example:
  scp -r ${OUTPUT_DIR} root@your-vps:/root/trojan-panel-test-images

On the VPS, edit web-local-image.env.yaml or node-local-image.env.yaml, then run:
  bash /tmp/tp-custom.sh web /root/trojan-panel-test-images/web-local-image.env.yaml
  bash /tmp/tp-custom.sh node /root/trojan-panel-test-images/node-local-image.env.yaml
EOF
}

main() {
  case "${1:-}" in
  -h | --help | help)
    usage
    return
    ;;
  esac

  load_config "${1:-}"
  OUTPUT_DIR="$(abs_path "${OUTPUT_DIR}")"
  PANEL_DIR="$(abs_path "${PANEL_DIR}")"
  UI_DIR="$(abs_path "${UI_DIR}")"
  CORE_DIR="$(abs_path "${CORE_DIR}")"
  parse_platform

  require_cmd docker
  require_cmd git
  require_cmd go
  detect_docker

  [[ "${BUILD_PANEL}" == "1" ]] && build_panel
  [[ "${BUILD_UI}" == "1" ]] && build_ui
  [[ "${BUILD_CORE}" == "1" ]] && build_core
  save_images

  echo "---> Done"
  echo "Archive/config output: ${OUTPUT_DIR}"
}

main "$@"
