#!/usr/bin/env bash
set -euo pipefail

export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:${PATH:-}"

ECHO_TYPE="echo -e"
INSTALLER_VERSION="0.1.0-rc.4"
SUPPORTED_SCHEMA_VERSION="1"
GITHUB_RAW_BASE="https://raw.githubusercontent.com/1linhao/trojanpanelnext"
DEFAULT_CONFIG_REF="v${INSTALLER_VERSION}"
CONFIG_REF="${TP_CONFIG_REF:-${DEFAULT_CONFIG_REF}}"

TP_DATA="${TP_DATA:-/tpdata}"
WEB_PATH="${WEB_PATH:-${TP_DATA}/web}"
TP_PKI_BUNDLE_DIR="${TP_PKI_BUNDLE_DIR:-${TP_DATA}/trojanpanelnext-pki}"

MARIADB_CONTAINER="${MARIADB_CONTAINER:-trojan-panel-mariadb}"
REDIS_CONTAINER="${REDIS_CONTAINER:-trojan-panel-redis}"
PANEL_CONTAINER="${PANEL_CONTAINER:-trojan-panel}"
UI_CONTAINER="${UI_CONTAINER:-trojan-panel-ui}"
CORE_CONTAINER="${CORE_CONTAINER:-trojan-panel-core}"
WEB_CADDY_CONTAINER="${WEB_CADDY_CONTAINER:-trojan-panel-web-caddy}"
NODE_CADDY_CONTAINER="${NODE_CADDY_CONTAINER:-trojan-panel-node-caddy}"

CADDY_IMAGE="${CADDY_IMAGE:-caddy:2.8.4}"
MARIADB_IMAGE="${MARIADB_IMAGE:-mariadb:10.7.3}"
REDIS_IMAGE="${REDIS_IMAGE:-redis:6.2.7}"
PANEL_IMAGE="${PANEL_IMAGE:-ghcr.io/1linhao/trojanpanelnext-api:${INSTALLER_VERSION}}"
UI_IMAGE="${UI_IMAGE:-ghcr.io/1linhao/trojanpanelnext-web:${INSTALLER_VERSION}}"
CORE_IMAGE="${CORE_IMAGE:-ghcr.io/1linhao/trojanpanelnext-node-agent:${INSTALLER_VERSION}}"
IMAGE_BUNDLE_DIR="${IMAGE_BUNDLE_DIR:-}"

MARIADB_PORT="${MARIADB_PORT:-9507}"
MARIADB_USER="${MARIADB_USER:-root}"
MARIADB_DATABASE="${MARIADB_DATABASE:-trojan_panel_db}"
ACCOUNT_TABLE="${ACCOUNT_TABLE:-account}"
REDIS_PORT="${REDIS_PORT:-6378}"
PANEL_PORT="${PANEL_PORT:-8081}"
UI_PORT="${UI_PORT:-8888}"
CORE_PORT="${CORE_PORT:-8082}"
GRPC_PORT="${GRPC_PORT:-8100}"
NODE_SERVER_ID="${NODE_SERVER_ID:-0}"
GRPC_TLS_MODE="${GRPC_TLS_MODE:-mtls}"
GRPC_TLS_SERVER_NAME="${GRPC_TLS_SERVER_NAME:-}"
GRPC_CLIENT_CA_PATH="${GRPC_CLIENT_CA_PATH:-${TP_DATA}/trojan-panel-core/pki/client-ca.crt}"
GRPC_CLIENT_CERT_PATH="${GRPC_CLIENT_CERT_PATH:-${TP_DATA}/trojan-panel/pki/client.crt}"
GRPC_CLIENT_KEY_PATH="${GRPC_CLIENT_KEY_PATH:-${TP_DATA}/trojan-panel/pki/client.key}"
GRPC_SERVER_CA_PATH="${GRPC_SERVER_CA_PATH:-}"
KERNEL_RUNTIME_PATH="${KERNEL_RUNTIME_PATH:-${TP_DATA}/trojan-panel-core/runtime}"
NODE_CADDY_HTTP_PORT="${NODE_CADDY_HTTP_PORT:-80}"
NODE_CADDY_HTTPS_PORT="${NODE_CADDY_HTTPS_PORT:-8863}"

TP_FORCE="${TP_FORCE:-0}"
TP_PURGE_DATA="${TP_PURGE_DATA:-0}"
TP_PURPOSE=""
TP_CONFIG_ROOT="${TP_CONFIG_ROOT:-}"
TP_TEMP_CONFIG_FILE=""

cleanup() {
  if [[ -n "${TP_TEMP_CONFIG_FILE}" ]]; then
    rm -f -- "${TP_TEMP_CONFIG_FILE}"
  fi
}

trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

echo_content() {
  case $1 in
  "red") ${ECHO_TYPE} "\033[31m$2\033[0m" ;;
  "green") ${ECHO_TYPE} "\033[32m$2\033[0m" ;;
  "yellow") ${ECHO_TYPE} "\033[33m$2\033[0m" ;;
  "skyBlue") ${ECHO_TYPE} "\033[36m$2\033[0m" ;;
  *) ${ECHO_TYPE} "$2" ;;
  esac
}

usage() {
  cat <<EOF
TrojanPanel Next installer ${INSTALLER_VERSION}

Usage:
  $0 config web|node [--output <file>]
  $0 validate --config <file>
  $0 install --config <file> [--force]
  $0 remove --config <file> [--purge-data]

Options:
  --config <file>    YAML deployment configuration; purpose selects web or node
  --output <file>    Destination for config (default: ./web.yaml or ./node.yaml)
  --force           Recreate API, UI, Agent and Caddy containers on install
  --purge-data      Remove the selected purpose's service data on removal
  -V, --version     Show the installer version
  -h, --help        Show this help

Configuration templates come only from GitHub Raw at ${DEFAULT_CONFIG_REF}.
Existing configuration files are never overwritten by config.
Install dependencies before use; this installer does not install software tools.
validate requires mikefarah/yq v4. install requires Docker, yq, curl,
OpenSSL, tar, coreutils, findutils and awk. install/remove require root.
EOF
}

require_commands() {
  local name
  local missing=()
  for name in "$@"; do
    command -v "${name}" >/dev/null 2>&1 || missing+=("${name}")
  done
  if ((${#missing[@]})); then
    echo_content red "Missing dependencies: ${missing[*]}. Install them first; see the installer README." >&2
    exit 1
  fi
}

require_yq() {
  require_commands yq
  local version
  version="$(yq --version 2>/dev/null)" || {
    echo_content red "Cannot run yq. Install mikefarah/yq v4 first." >&2
    exit 1
  }
  if [[ "${version}" != *mikefarah/yq* || "${version}" != *"version v4."* ]]; then
    echo_content red "Unsupported yq. Install mikefarah/yq v4 (not the Python yq package)." >&2
    exit 1
  fi
}

require_option_value() {
  local option="$1"
  local value="${2:-}"
  if [[ -z "${value}" || "${value}" == -* ]]; then
    echo_content red "${option} requires a value" >&2
    exit 1
  fi
}

download_config() {
  local purpose="$1"
  local output="$2"
  local template="web.yaml"
  [[ "${purpose}" == node ]] && template="node-agent.yaml"
  local url="${GITHUB_RAW_BASE}/${CONFIG_REF}/deploy/installer/examples/${template}"

  require_commands curl dirname mktemp chmod ln rm
  if [[ -e "${output}" || -L "${output}" ]]; then
    echo_content red "Refusing to overwrite existing config: ${output}" >&2
    exit 1
  fi
  local output_dir
  output_dir="$(dirname -- "${output}")"
  if [[ ! -d "${output_dir}" ]]; then
    echo_content red "Output directory does not exist: ${output_dir}" >&2
    exit 1
  fi

  TP_TEMP_CONFIG_FILE="$(mktemp -- "${output}.tmp.XXXXXX")"
  if ! curl --fail --location --silent --show-error \
    --proto '=https' --proto-redir '=https' \
    --retry 2 --retry-max-time 180 --connect-timeout 10 --max-time 60 \
    --max-filesize 1048576 "${url}" -o "${TP_TEMP_CONFIG_FILE}"; then
    echo_content red "Failed to download configuration template: ${url}" >&2
    exit 1
  fi
  if [[ ! -s "${TP_TEMP_CONFIG_FILE}" ]]; then
    echo_content red "Downloaded configuration template is empty." >&2
    exit 1
  fi
  local content
  content="$(<"${TP_TEMP_CONFIG_FILE}")"
  if [[ "${content,,}" == *'<html'* || "${content,,}" == *'<!doctype html'* ]]; then
    echo_content red "Downloaded an HTML page instead of a configuration template." >&2
    exit 1
  fi

  chmod 600 -- "${TP_TEMP_CONFIG_FILE}"
  # A same-directory hard link publishes the complete file atomically, and fails
  # if the destination appeared while curl was running (including symlinks).
  if ! ln -T -- "${TP_TEMP_CONFIG_FILE}" "${output}"; then
    echo_content red "Could not create config without replacing an existing path: ${output}" >&2
    exit 1
  fi
  rm -f -- "${TP_TEMP_CONFIG_FILE}"
  TP_TEMP_CONFIG_FILE=""
  echo_content green "Configuration downloaded: ${output} (installer ${INSTALLER_VERSION}, template ref ${CONFIG_REF})"
  echo_content skyBlue "Edit this file, then run: $0 validate --config ${output}"
}

handle_config_command() {
  local purpose="${1:-}"
  local output=""
  case "${purpose}" in
  -h | --help)
    usage
    return
    ;;
  web) output="./web.yaml" ;;
  node) output="./node.yaml" ;;
  *)
    echo_content red "Usage: $0 config web|node [--output <file>]" >&2
    exit 1
    ;;
  esac
  shift
  while (($#)); do
    case "$1" in
    --output)
      require_option_value "$1" "${2:-}"
      output="$2"
      shift 2
      ;;
    -h | --help)
      usage
      return
      ;;
    *)
      echo_content red "Unknown config option: $1" >&2
      exit 1
      ;;
    esac
  done
  download_config "${purpose}" "${output}"
}

require_root() {
  if [[ "$(id -u)" != "0" ]]; then
    echo_content red "Please run as root"
    exit 1
  fi
}

require_value() {
  local name="$1"
  local value="${!name:-}"
  if [[ -z "${value}" ]]; then
    echo_content red "${name} is required"
    usage
    exit 1
  fi
}

random_password() {
  od -An -N24 -tx1 /dev/urandom | tr -d ' \n'
}

container_exists() {
  docker ps -a -q -f "name=^$1$" | grep -q .
}

container_running() {
  docker ps -q -f "name=^$1$" -f "status=running" | grep -q .
}

remove_container_if_force() {
  local name="$1"
  if container_exists "${name}" && [[ "${TP_FORCE}" == "1" ]]; then
    docker rm -fv "${name}" >/dev/null 2>&1 || true
  fi
}

yaml_read_raw() {
  local file="$1"
  local key="$2"
  yq -r "${TP_CONFIG_ROOT}.${key} // \"\"" "${file}"
}

detect_config_root() {
  local file="$1"
  if yq -e '.trojanpanelnext != null' "${file}" >/dev/null 2>&1; then
    TP_CONFIG_ROOT='.trojanpanelnext'
  else
    echo_content red "Configuration must contain a 'trojanpanelnext' root object" >&2
    exit 1
  fi
}

cfg_apply() {
  local file="$1"
  local var_name="$2"
  local key="$3"
  local value
  value="$(yaml_read_raw "${file}" "${key}")"
  if [[ -n "${value}" ]]; then
    printf -v "${var_name}" '%s' "${value}"
  fi
}

load_config() {
  local file="${1:-}"
  if [[ -z "${file}" ]]; then
    echo_content red "Config file is required"
    usage
    exit 1
  fi
  if [[ ! -f "${file}" ]]; then
    echo_content red "Config file not found: ${file}"
    exit 1
  fi

  require_yq
  TP_CONFIG_FILE="${file}"
  detect_config_root "${file}"
  TP_PURPOSE="$(yaml_read_raw "${file}" purpose)"
  if [[ -z "${TP_PURPOSE}" ]]; then
    echo_content red "trojanpanelnext.purpose is required" >&2
    exit 1
  fi
  case "${TP_PURPOSE}" in
  web | node) ;;
  *)
    echo_content red "Unsupported trojanpanelnext.purpose: ${TP_PURPOSE}" >&2
    exit 1
    ;;
  esac

  cfg_apply "${file}" CADDY_IMAGE caddy_image
  cfg_apply "${file}" MARIADB_IMAGE mariadb_image
  cfg_apply "${file}" REDIS_IMAGE redis_image
  cfg_apply "${file}" PANEL_IMAGE panel_image
  cfg_apply "${file}" UI_IMAGE ui_image
  cfg_apply "${file}" CORE_IMAGE core_image
  cfg_apply "${file}" IMAGE_BUNDLE_DIR image_bundle_dir

  cfg_apply "${file}" MARIADB_PORT mariadb_port
  cfg_apply "${file}" MARIADB_USER mariadb_user
  cfg_apply "${file}" MARIADB_DATABASE database
  cfg_apply "${file}" ACCOUNT_TABLE account_table
  cfg_apply "${file}" REDIS_PORT redis_port
  cfg_apply "${file}" PANEL_PORT panel_port
  cfg_apply "${file}" UI_PORT ui_port
  cfg_apply "${file}" CORE_PORT core_port
  cfg_apply "${file}" GRPC_PORT grpc_port
  cfg_apply "${file}" NODE_SERVER_ID node_server_id
  cfg_apply "${file}" GRPC_TLS_MODE grpc_tls_mode
  cfg_apply "${file}" GRPC_TLS_SERVER_NAME grpc_tls_server_name
  cfg_apply "${file}" GRPC_CLIENT_CA_PATH grpc_client_ca_path
  cfg_apply "${file}" GRPC_CLIENT_CERT_PATH grpc_client_cert_path
  cfg_apply "${file}" GRPC_CLIENT_KEY_PATH grpc_client_key_path
  cfg_apply "${file}" GRPC_SERVER_CA_PATH grpc_server_ca_path
  cfg_apply "${file}" KERNEL_RUNTIME_PATH kernel_runtime_path
  cfg_apply "${file}" TP_PKI_BUNDLE_DIR pki_bundle_dir
  cfg_apply "${file}" NODE_CADDY_HTTP_PORT node_caddy_http_port
  cfg_apply "${file}" NODE_CADDY_HTTPS_PORT node_caddy_https_port
  cfg_apply "${file}" TP_FORCE force
  cfg_apply "${file}" TP_PURGE_DATA purge_data
  case "${TP_PURPOSE}" in
  web)
    TP_WEB_DOMAIN=""
    TP_EMAIL=""
    MARIADB_PASSWORD=""
    REDIS_PASSWORD=""
    cfg_apply "${file}" TP_WEB_DOMAIN hostname
    cfg_apply "${file}" TP_EMAIL email
    cfg_apply "${file}" MARIADB_PASSWORD mariadb_password
    cfg_apply "${file}" REDIS_PASSWORD redis_password
    ;;
  node)
    TP_NODE_DOMAIN=""
    TP_EMAIL=""
    MARIADB_HOST=""
    MARIADB_PASSWORD=""
    REDIS_HOST=""
    REDIS_PASSWORD=""
    cfg_apply "${file}" TP_NODE_DOMAIN hostname
    cfg_apply "${file}" TP_EMAIL email
    cfg_apply "${file}" MARIADB_HOST mariadb_host
    cfg_apply "${file}" MARIADB_PASSWORD mariadb_password
    cfg_apply "${file}" REDIS_HOST redis_host
    cfg_apply "${file}" REDIS_PASSWORD redis_password
    ;;
  esac
}

require_one_of() {
  local name="$1"
  local value="$2"
  shift 2
  local allowed
  for allowed in "$@"; do
    if [[ "${value}" == "${allowed}" ]]; then
      return
    fi
  done
  echo_content red "${name} has unsupported value: ${value}"
  exit 1
}

require_port() {
  local name="$1"
  local value="${!name:-}"
  if [[ ! "${value}" =~ ^[0-9]+$ ]] || ((value < 1 || value > 65535)); then
    echo_content red "${name} must be an integer between 1 and 65535"
    exit 1
  fi
}

validate_config() {
  local purpose="${TP_PURPOSE}"

  local schema_version
  schema_version="$(yaml_read_raw "${TP_CONFIG_FILE}" schema_version)"
  if [[ "${schema_version}" != "${SUPPORTED_SCHEMA_VERSION}" ]]; then
    echo_content red "Unsupported schema version: ${schema_version}; expected ${SUPPORTED_SCHEMA_VERSION}" >&2
    exit 1
  fi

  require_one_of force "${TP_FORCE}" 0 1
  require_one_of purge_data "${TP_PURGE_DATA}" 0 1
  require_port MARIADB_PORT
  require_port REDIS_PORT

  case "${purpose}" in
  web)
    require_value TP_WEB_DOMAIN
    require_port PANEL_PORT
    require_port UI_PORT
    require_value PANEL_IMAGE
    require_value UI_IMAGE
    ;;
  node)
    require_value TP_NODE_DOMAIN
    require_value MARIADB_HOST
    require_value MARIADB_PASSWORD
    require_value REDIS_HOST
    require_value REDIS_PASSWORD
    require_value CORE_IMAGE
    require_port CORE_PORT
    require_port GRPC_PORT
    require_port NODE_CADDY_HTTP_PORT
    require_port NODE_CADDY_HTTPS_PORT
    require_one_of grpc_tls_mode "${GRPC_TLS_MODE}" mtls
    require_value TP_PKI_BUNDLE_DIR
    ;;
  *)
    echo_content red "Unsupported purpose: ${purpose}"
    exit 1
    ;;
  esac
}

container_env_value() {
  local name="$1"
  local key="$2"
  if ! container_exists "${name}"; then
    return
  fi
  docker inspect "${name}" --format '{{range .Config.Env}}{{println .}}{{end}}' 2>/dev/null |
    awk -F= -v key="${key}" '$1 == key {sub(/^[^=]*=/, ""); print; exit}'
}

write_web_generated_secrets() {
  local file="${TP_CONFIG_FILE:-}"
  if [[ -z "${file}" || ! -f "${file}" ]]; then
    return
  fi
  MARIADB_PASSWORD="${MARIADB_PASSWORD}" REDIS_PASSWORD="${REDIS_PASSWORD}" \
    yq -i '.trojanpanelnext.mariadb_password = strenv(MARIADB_PASSWORD) | .trojanpanelnext.redis_password = strenv(REDIS_PASSWORD)' "${file}"
  chmod 600 "${file}"
}

init_web_secrets() {
  if [[ -z "${MARIADB_PASSWORD:-}" ]]; then
    MARIADB_PASSWORD="$(container_env_value "${MARIADB_CONTAINER}" MYSQL_ROOT_PASSWORD || true)"
  fi
  if [[ -z "${REDIS_PASSWORD:-}" ]]; then
    REDIS_PASSWORD="$(container_env_value "${PANEL_CONTAINER}" redis_pass || true)"
  fi
  MARIADB_PASSWORD="${MARIADB_PASSWORD:-$(random_password)}"
  REDIS_PASSWORD="${REDIS_PASSWORD:-$(random_password)}"
  export MARIADB_PASSWORD REDIS_PASSWORD
  write_web_generated_secrets
}

image_exists() {
  docker image inspect "$1" >/dev/null 2>&1
}

ensure_image() {
  local image="$1"
  if image_exists "${image}" && [[ "${TP_FORCE}" != "1" ]]; then
    echo_content skyBlue "---> Image already available: ${image}"
    return
  fi
  docker pull "${image}"
}

load_image_archives() {
  if [[ -z "${IMAGE_BUNDLE_DIR:-}" ]]; then
    return
  fi
  if [[ ! -d "${IMAGE_BUNDLE_DIR}" ]]; then
    echo_content red "Image bundle directory not found: ${IMAGE_BUNDLE_DIR}"
    exit 1
  fi

  local archive found=0
  shopt -s nullglob
  for archive in "${IMAGE_BUNDLE_DIR}"/*.tar "${IMAGE_BUNDLE_DIR}"/*.tar.gz "${IMAGE_BUNDLE_DIR}"/*.tgz; do
    found=1
    echo_content green "---> Load Docker image archive: ${archive}"
    docker load -i "${archive}"
  done
  shopt -u nullglob

  if [[ "${found}" == "0" ]]; then
    echo_content yellow "---> No Docker image archives found in ${IMAGE_BUNDLE_DIR}"
  fi
}

prepare_dirs() {
  mkdir -p \
    "${TP_DATA}" \
    "${WEB_PATH}" \
    "${TP_DATA}/mariadb/data" \
    "${TP_DATA}/redis/data" \
    "${TP_DATA}/trojan-panel/webfile" \
    "${TP_DATA}/trojan-panel/logs" \
    "${TP_DATA}/trojan-panel/config" \
    "${TP_DATA}/trojan-panel/pki" \
    "${TP_DATA}/trojan-panel-ui/nginx" \
    "${TP_DATA}/trojan-panel-core/bin/xray/config" \
    "${TP_DATA}/trojan-panel-core/bin/naiveproxy/config" \
    "${TP_DATA}/trojan-panel-core/bin/hysteria2/config" \
    "${TP_DATA}/trojan-panel-core/logs" \
    "${TP_DATA}/trojan-panel-core/config" \
    "${TP_DATA}/trojan-panel-core/pki" \
    "${KERNEL_RUNTIME_PATH}" \
    "${TP_DATA}/custom/web-caddy" \
    "${TP_DATA}/custom/node-caddy"
}

generate_web_client_pki() {
  if [[ -f "${TP_PKI_BUNDLE_DIR}/client-ca.crt" && \
    -f "${TP_PKI_BUNDLE_DIR}/client-ca.key" && \
    -f "${TP_PKI_BUNDLE_DIR}/client.crt" && \
    -f "${TP_PKI_BUNDLE_DIR}/client.key" ]]; then
    return
  fi

  if find "${TP_PKI_BUNDLE_DIR}" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null | grep -q .; then
    echo_content red "PKI directory is incomplete: ${TP_PKI_BUNDLE_DIR}"
    exit 1
  fi

  echo_content green "---> Generate control-plane mTLS identity"
  local temporary_pki
  temporary_pki="$(mktemp -d)"
  openssl req -x509 -newkey rsa:3072 -sha256 -days 3650 -nodes \
    -subj "/CN=TrojanPanel Next Control CA" \
    -keyout "${temporary_pki}/client-ca.key" \
    -out "${temporary_pki}/client-ca.crt" >/dev/null 2>&1
  openssl req -newkey rsa:3072 -sha256 -nodes \
    -subj "/CN=trojanpanelnext-control-plane" \
    -keyout "${temporary_pki}/client.key" \
    -out "${temporary_pki}/client.csr" >/dev/null 2>&1
  cat >"${temporary_pki}/client.ext" <<'EOF'
basicConstraints=CA:FALSE
keyUsage=digitalSignature,keyEncipherment
extendedKeyUsage=clientAuth
EOF
  openssl x509 -req -sha256 -days 825 \
    -in "${temporary_pki}/client.csr" \
    -CA "${temporary_pki}/client-ca.crt" \
    -CAkey "${temporary_pki}/client-ca.key" \
    -CAcreateserial \
    -extfile "${temporary_pki}/client.ext" \
    -out "${temporary_pki}/client.crt" >/dev/null 2>&1

  mkdir -p "${TP_PKI_BUNDLE_DIR}"
  chmod 700 "${TP_PKI_BUNDLE_DIR}"
  install -m 0600 "${temporary_pki}/client-ca.key" "${TP_PKI_BUNDLE_DIR}/client-ca.key"
  install -m 0644 "${temporary_pki}/client-ca.crt" "${TP_PKI_BUNDLE_DIR}/client-ca.crt"
  install -m 0600 "${temporary_pki}/client.key" "${TP_PKI_BUNDLE_DIR}/client.key"
  install -m 0644 "${temporary_pki}/client.crt" "${TP_PKI_BUNDLE_DIR}/client.crt"
  rm -rf -- "${temporary_pki}"
}

install_pki_material() {
  local purpose="$1"
  case "${purpose}" in
  web)
    generate_web_client_pki
    mkdir -p "$(dirname "${GRPC_CLIENT_CERT_PATH}")" "$(dirname "${GRPC_CLIENT_KEY_PATH}")"
    install -m 0644 "${TP_PKI_BUNDLE_DIR}/client.crt" "${GRPC_CLIENT_CERT_PATH}"
    install -m 0600 "${TP_PKI_BUNDLE_DIR}/client.key" "${GRPC_CLIENT_KEY_PATH}"
    ;;
  node)
    if [[ ! -f "${TP_PKI_BUNDLE_DIR}/client-ca.crt" ]]; then
      echo_content red "Missing control-plane CA: ${TP_PKI_BUNDLE_DIR}/client-ca.crt"
      exit 1
    fi
    mkdir -p "$(dirname "${GRPC_CLIENT_CA_PATH}")"
    install -m 0644 "${TP_PKI_BUNDLE_DIR}/client-ca.crt" "${GRPC_CLIENT_CA_PATH}"
    ;;
  esac
}

persist_container_path() {
  local name="$1"
  local src="$2"
  local dst="$3"

  mkdir -p "${dst}"
  if ! container_exists "${name}"; then
    return
  fi
  if find "${dst}" -mindepth 1 -print -quit 2>/dev/null | grep -q .; then
    return
  fi

  echo_content green "---> Persist ${name} data: ${src} -> ${dst}"
  local tmp_dir
  tmp_dir="$(mktemp -d)"
  docker cp "${name}:${src}/." "${tmp_dir}/"
  cp -a "${tmp_dir}/." "${dst}/"
  rm -rf "${tmp_dir}"
}

write_panel_runtime_config() {
  cat >"${TP_DATA}/trojan-panel/config/config.ini" <<EOF
[mysql]
host=127.0.0.1
user=${MARIADB_USER}
password=${MARIADB_PASSWORD}
port=${MARIADB_PORT}
[log]
filename=logs/trojan-panel.log
max_size=1
max_backups=5
max_age=30
compress=true
[redis]
host=127.0.0.1
port=${REDIS_PORT}
password=${REDIS_PASSWORD}
db=0
max_idle=2
max_active=4
wait=true
[server]
port=${PANEL_PORT}
[grpc]
client_cert_path=${GRPC_CLIENT_CERT_PATH}
client_key_path=${GRPC_CLIENT_KEY_PATH}
server_ca_path=${GRPC_SERVER_CA_PATH}
EOF
  chmod 600 "${TP_DATA}/trojan-panel/config/config.ini"
}

write_core_runtime_config() {
  local crt_path="$1"
  local key_path="$2"

  cat >"${TP_DATA}/trojan-panel-core/config/config.ini" <<EOF
[mysql]
host=${MARIADB_HOST}
user=${MARIADB_USER}
password=${MARIADB_PASSWORD}
port=${MARIADB_PORT}
database=${MARIADB_DATABASE}
account_table=${ACCOUNT_TABLE}
[redis]
host=${REDIS_HOST}
port=${REDIS_PORT}
password=${REDIS_PASSWORD}
db=0
max_idle=2
max_active=4
wait=true
[cert]
crt_path=${crt_path}
key_path=${key_path}
[log]
filename=logs/trojan-panel-core.log
max_size=1
max_backups=5
max_age=30
compress=true
[grpc]
port=${GRPC_PORT}
tls_mode=${GRPC_TLS_MODE}
client_ca_path=${GRPC_CLIENT_CA_PATH}
[server]
port=${CORE_PORT}
[node]
server_id=${NODE_SERVER_ID}
EOF
  chmod 600 "${TP_DATA}/trojan-panel-core/config/config.ini"
}

prepare_static_web() {
  if [[ -f "${WEB_PATH}/index.html" ]]; then
    return
  fi
  echo_content green "---> Prepare camouflage web files"
  cat >"${WEB_PATH}/index.html" <<'EOF'
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Welcome</title>
  <style>
    body { align-items: center; background: #f5f7fa; color: #243447; display: flex;
      font: 16px/1.6 system-ui, sans-serif; justify-content: center; margin: 0; min-height: 100vh; }
    main { background: #fff; border-radius: 16px; box-shadow: 0 12px 40px #1d2d3d1a;
      max-width: 36rem; padding: 3rem; text-align: center; }
  </style>
</head>
<body><main><h1>Welcome</h1><p>The service is online.</p></main></body>
</html>
EOF
}

write_web_caddyfile() {
  local domain="$1"
  local caddyfile="${TP_DATA}/custom/web-caddy/Caddyfile"
  if [[ -n "${TP_EMAIL:-}" ]]; then
    cat >"${caddyfile}" <<EOF
{
    email ${TP_EMAIL}
}

${domain} {
    reverse_proxy 127.0.0.1:${UI_PORT}
}
EOF
  else
    cat >"${caddyfile}" <<EOF
${domain} {
    reverse_proxy 127.0.0.1:${UI_PORT}
}
EOF
  fi
}

write_node_caddyfile() {
  local domain="$1"
  local caddyfile="${TP_DATA}/custom/node-caddy/Caddyfile"
  if [[ -n "${TP_EMAIL:-}" ]]; then
    cat >"${caddyfile}" <<EOF
{
    email ${TP_EMAIL}
    http_port ${NODE_CADDY_HTTP_PORT}
    https_port ${NODE_CADDY_HTTPS_PORT}
}

${domain} {
    root * /srv
    file_server
}
EOF
  else
    cat >"${caddyfile}" <<EOF
{
    http_port ${NODE_CADDY_HTTP_PORT}
    https_port ${NODE_CADDY_HTTPS_PORT}
}

${domain} {
    root * /srv
    file_server
}
EOF
  fi
}

start_caddy() {
  local name="$1"
  local config_dir="$2"
  local data_dir="$3"
  local web_dir="$4"

  remove_container_if_force "${name}"
  if container_running "${name}"; then
    echo_content skyBlue "---> ${name} already running"
    return
  fi
  if container_exists "${name}"; then
    docker start "${name}" >/dev/null
    return
  fi

  ensure_image "${CADDY_IMAGE}"
  docker run -d --name "${name}" --restart always \
    --network=host \
    -v "${config_dir}/Caddyfile:/etc/caddy/Caddyfile" \
    -v "${data_dir}:/data" \
    -v "${config_dir}:/config" \
    -v "${web_dir}:/srv" \
    "${CADDY_IMAGE}"
}

wait_for_cert() {
  local domain="$1"
  local data_dir="$2"
  local cert_file
  local key_file

  echo_content green "---> Wait for certificate: ${domain}"
  for _ in $(seq 1 60); do
    cert_file="$(find "${data_dir}/caddy/certificates" -path "*/${domain}/${domain}.crt" -type f -size +0c 2>/dev/null | head -n 1 || true)"
    key_file="$(find "${data_dir}/caddy/certificates" -path "*/${domain}/${domain}.key" -type f -size +0c 2>/dev/null | head -n 1 || true)"
    if [[ -n "${cert_file}" && -n "${key_file}" ]]; then
      echo_content skyBlue "---> Certificate ready: ${cert_file}"
      return
    fi
    sleep 3
  done

  echo_content red "---> Certificate is not ready. Check DNS, firewall, and Caddy logs."
  exit 1
}

wait_for_container() {
  local name="$1"
  for _ in $(seq 1 60); do
    if container_running "${name}"; then
      return
    fi
    sleep 2
  done
  echo_content red "---> ${name} is not running"
  docker logs "${name}" 2>/dev/null || true
  exit 1
}

wait_for_mariadb() {
  for _ in $(seq 1 60); do
    if docker exec "${MARIADB_CONTAINER}" sh -c "mariadb -uroot -p\"${MARIADB_PASSWORD}\" -e 'select 1' >/dev/null 2>&1 || mysql -uroot -p\"${MARIADB_PASSWORD}\" -e 'select 1' >/dev/null 2>&1"; then
      return
    fi
    sleep 2
  done
  echo_content red "---> MariaDB is not ready"
  docker logs "${MARIADB_CONTAINER}" 2>/dev/null || true
  exit 1
}

create_database() {
  docker exec "${MARIADB_CONTAINER}" sh -c "mariadb -uroot -p\"${MARIADB_PASSWORD}\" -e 'create database if not exists ${MARIADB_DATABASE} default character set utf8mb4;' >/dev/null 2>&1 || mysql -uroot -p\"${MARIADB_PASSWORD}\" -e 'create database if not exists ${MARIADB_DATABASE} default character set utf8mb4;' >/dev/null 2>&1"
}

write_ui_nginx_config() {
  cat >"${TP_DATA}/trojan-panel-ui/nginx/default.conf" <<EOF
server {
    listen       ${UI_PORT};
    server_name  localhost;

    location / {
        root   /tpdata/trojan-panel-ui;
        index  index.html index.htm;
    }

    location /api {
        proxy_pass http://127.0.0.1:${PANEL_PORT};
    }
}
EOF
}

deploy_mariadb() {
  persist_container_path "${MARIADB_CONTAINER}" "/var/lib/mysql" "${TP_DATA}/mariadb/data"
  if container_running "${MARIADB_CONTAINER}"; then
    echo_content skyBlue "---> MariaDB already running"
    return
  fi
  if container_exists "${MARIADB_CONTAINER}"; then
    docker start "${MARIADB_CONTAINER}" >/dev/null
    wait_for_mariadb
    return
  fi

  ensure_image "${MARIADB_IMAGE}"
  docker run -d --name "${MARIADB_CONTAINER}" --restart always \
    --network=host \
    -e MYSQL_DATABASE="${MARIADB_DATABASE}" \
    -e MYSQL_ROOT_PASSWORD="${MARIADB_PASSWORD}" \
    -e TZ=Asia/Shanghai \
    -v "${TP_DATA}/mariadb/data:/var/lib/mysql" \
    "${MARIADB_IMAGE}" \
    --port "${MARIADB_PORT}" \
    --character-set-server=utf8mb4 \
    --collation-server=utf8mb4_unicode_ci
  wait_for_mariadb
  create_database
}

deploy_redis() {
  persist_container_path "${REDIS_CONTAINER}" "/data" "${TP_DATA}/redis/data"
  if container_running "${REDIS_CONTAINER}"; then
    echo_content skyBlue "---> Redis already running"
    return
  fi
  if container_exists "${REDIS_CONTAINER}"; then
    docker start "${REDIS_CONTAINER}" >/dev/null
    return
  fi

  ensure_image "${REDIS_IMAGE}"
  docker run -d --name "${REDIS_CONTAINER}" --restart always \
    --network=host \
    -v "${TP_DATA}/redis/data:/data" \
    "${REDIS_IMAGE}" redis-server --requirepass "${REDIS_PASSWORD}" --port "${REDIS_PORT}"
  wait_for_container "${REDIS_CONTAINER}"
}

deploy_panel_backend() {
  remove_container_if_force "${PANEL_CONTAINER}"
  if container_running "${PANEL_CONTAINER}"; then
    echo_content skyBlue "---> Trojan Panel backend already running"
    return
  fi
  if container_exists "${PANEL_CONTAINER}"; then
    docker start "${PANEL_CONTAINER}" >/dev/null
    return
  fi

  ensure_image "${PANEL_IMAGE}"
  docker run -d --name "${PANEL_CONTAINER}" --restart always \
    --network=host \
    -v "${WEB_PATH}:${TP_DATA}/trojan-panel/webfile/" \
    -v "${TP_DATA}/trojan-panel/logs/:${TP_DATA}/trojan-panel/logs/" \
    -v "${TP_DATA}/trojan-panel/config/:${TP_DATA}/trojan-panel/config/" \
    -v "${TP_DATA}/trojan-panel/pki/:${TP_DATA}/trojan-panel/pki/:ro" \
    -v /etc/localtime:/etc/localtime \
    -e GIN_MODE=release \
    -e "mariadb_ip=127.0.0.1" \
    -e "mariadb_port=${MARIADB_PORT}" \
    -e "mariadb_user=${MARIADB_USER}" \
    -e "mariadb_pas=${MARIADB_PASSWORD}" \
    -e "redis_host=127.0.0.1" \
    -e "redis_port=${REDIS_PORT}" \
    -e "redis_pass=${REDIS_PASSWORD}" \
    -e "server_port=${PANEL_PORT}" \
    -e "GRPC_CLIENT_CERT_PATH=${GRPC_CLIENT_CERT_PATH}" \
    -e "GRPC_CLIENT_KEY_PATH=${GRPC_CLIENT_KEY_PATH}" \
    -e "GRPC_SERVER_CA_PATH=${GRPC_SERVER_CA_PATH}" \
    "${PANEL_IMAGE}"
  wait_for_container "${PANEL_CONTAINER}"
}

deploy_panel_ui() {
  remove_container_if_force "${UI_CONTAINER}"
  write_ui_nginx_config
  if container_running "${UI_CONTAINER}"; then
    echo_content skyBlue "---> Trojan Panel UI already running"
    return
  fi
  if container_exists "${UI_CONTAINER}"; then
    docker start "${UI_CONTAINER}" >/dev/null
    return
  fi

  ensure_image "${UI_IMAGE}"
  docker run -d --name "${UI_CONTAINER}" --restart always \
    --network=host \
    -v "${TP_DATA}/trojan-panel-ui/nginx/default.conf:/etc/nginx/conf.d/default.conf" \
    "${UI_IMAGE}"
  wait_for_container "${UI_CONTAINER}"
}

deploy_core() {
  local domain="$1"
  local cert_data="${TP_DATA}/custom/node-caddy/data"
  local crt_path="${cert_data}/caddy/certificates/acme-v02.api.letsencrypt.org-directory/${domain}/${domain}.crt"
  local key_path="${cert_data}/caddy/certificates/acme-v02.api.letsencrypt.org-directory/${domain}/${domain}.key"

  write_core_runtime_config "${crt_path}" "${key_path}"
  remove_container_if_force "${CORE_CONTAINER}"
  if container_running "${CORE_CONTAINER}"; then
    echo_content skyBlue "---> Trojan Panel Core already running"
    return
  fi
  if container_exists "${CORE_CONTAINER}"; then
    docker start "${CORE_CONTAINER}" >/dev/null
    return
  fi

  ensure_image "${CORE_IMAGE}"
  docker run -d --name "${CORE_CONTAINER}" --restart always \
    --network=host \
    -v "${TP_DATA}/trojan-panel-core/bin/xray/config/:${TP_DATA}/trojan-panel-core/bin/xray/config/" \
    -v "${TP_DATA}/trojan-panel-core/bin/naiveproxy/config/:${TP_DATA}/trojan-panel-core/bin/naiveproxy/config/" \
    -v "${TP_DATA}/trojan-panel-core/bin/hysteria2/config/:${TP_DATA}/trojan-panel-core/bin/hysteria2/config/" \
    -v "${TP_DATA}/trojan-panel-core/logs/:${TP_DATA}/trojan-panel-core/logs/" \
    -v "${TP_DATA}/trojan-panel-core/config/:${TP_DATA}/trojan-panel-core/config/" \
    -v "${TP_DATA}/trojan-panel-core/pki/:${TP_DATA}/trojan-panel-core/pki/:ro" \
    -v "${KERNEL_RUNTIME_PATH}:${TP_DATA}/trojan-panel-core/runtime/" \
    -v "${cert_data}:${cert_data}" \
    -v "${WEB_PATH}:${WEB_PATH}" \
    -v /etc/localtime:/etc/localtime \
    -e GIN_MODE=release \
    -e "mariadb_ip=${MARIADB_HOST}" \
    -e "mariadb_port=${MARIADB_PORT}" \
    -e "mariadb_user=${MARIADB_USER}" \
    -e "mariadb_pas=${MARIADB_PASSWORD}" \
    -e "database=${MARIADB_DATABASE}" \
    -e "account_table=${ACCOUNT_TABLE}" \
    -e "redis_host=${REDIS_HOST}" \
    -e "redis_port=${REDIS_PORT}" \
    -e "redis_pass=${REDIS_PASSWORD}" \
    -e "crt_path=${crt_path}" \
    -e "key_path=${key_path}" \
    -e "grpc_port=${GRPC_PORT}" \
	-e "NODE_SERVER_ID=${NODE_SERVER_ID}" \
    -e "grpc_tls_mode=${GRPC_TLS_MODE}" \
    -e "grpc_client_ca_path=${GRPC_CLIENT_CA_PATH}" \
    -e "TP_KERNEL_RUNTIME=${TP_DATA}/trojan-panel-core/runtime" \
    -e "server_port=${CORE_PORT}" \
    "${CORE_IMAGE}"
  wait_for_container "${CORE_CONTAINER}"
}

deploy_web() {
  require_value TP_WEB_DOMAIN

  init_web_secrets
  load_image_archives
  prepare_dirs
  install_pki_material web
  deploy_mariadb
  deploy_redis
  write_panel_runtime_config
  deploy_panel_backend
  deploy_panel_ui
  write_web_caddyfile "${TP_WEB_DOMAIN}"
  start_caddy "${WEB_CADDY_CONTAINER}" "${TP_DATA}/custom/web-caddy" "${TP_DATA}/custom/web-caddy/data" "${WEB_PATH}"

  echo_content red "\n=============================================================="
  echo_content skyBlue "Trojan Panel web side deployed"
  echo_content yellow "URL: https://${TP_WEB_DOMAIN}"
  echo_content yellow "Default username: sysadmin"
  echo_content yellow "Credentials are stored in the restricted deployment configuration and are not printed."
  echo_content red "==============================================================\n"
}

deploy_node() {
  require_value TP_NODE_DOMAIN
  require_value MARIADB_HOST
  require_value MARIADB_PASSWORD
  require_value REDIS_HOST
  require_value REDIS_PASSWORD

  load_image_archives
  prepare_dirs
  install_pki_material node
  prepare_static_web
  write_node_caddyfile "${TP_NODE_DOMAIN}"
  start_caddy "${NODE_CADDY_CONTAINER}" "${TP_DATA}/custom/node-caddy" "${TP_DATA}/custom/node-caddy/data" "${WEB_PATH}"
  wait_for_cert "${TP_NODE_DOMAIN}" "${TP_DATA}/custom/node-caddy/data"
  deploy_core "${TP_NODE_DOMAIN}"

  echo_content red "\n=============================================================="
  echo_content skyBlue "Trojan Panel node side deployed"
  echo_content yellow "Node domain: ${TP_NODE_DOMAIN}"
  echo_content yellow "Core gRPC port: ${GRPC_PORT}"
  echo_content yellow "Core API port: ${CORE_PORT}"
  echo_content red "==============================================================\n"
}

remove_web() {
  docker rm -f "${WEB_CADDY_CONTAINER}" "${UI_CONTAINER}" "${PANEL_CONTAINER}" "${REDIS_CONTAINER}" "${MARIADB_CONTAINER}" >/dev/null 2>&1 || true
  if [[ "${TP_PURGE_DATA}" == "1" ]]; then
    rm -rf "${TP_DATA}/custom/web-caddy" "${TP_DATA}/trojan-panel" "${TP_DATA}/trojan-panel-ui" "${TP_DATA}/mariadb" "${TP_DATA}/redis"
  fi
  echo_content skyBlue "---> Trojan Panel web side removed"
}

remove_node() {
  docker rm -f "${CORE_CONTAINER}" "${NODE_CADDY_CONTAINER}" >/dev/null 2>&1 || true
  if [[ "${TP_PURGE_DATA}" == "1" ]]; then
    rm -rf "${TP_DATA}/custom/node-caddy" "${TP_DATA}/trojan-panel-core"
  fi
  echo_content skyBlue "---> Trojan Panel node side removed"
}

main() {
  local command="${1:-}"
  local config_file=""
  local force_override=""
  local purge_override=""

  case "${command}" in
  -h | --help | help | "")
    usage
    return
    ;;
  -V | --version | version)
    printf '%s\n' "${INSTALLER_VERSION}"
    return
    ;;
  config)
    shift
    handle_config_command "$@"
    return
    ;;
  install | remove | validate)
    shift
    ;;
  *)
    echo_content red "Unknown command: ${command}"
    usage
    exit 1
    ;;
  esac

  while (($# > 0)); do
    case "$1" in
    --config)
      require_option_value "$1" "${2:-}"
      config_file="$2"
      shift 2
      ;;
    --force)
      force_override=1
      shift
      ;;
    --purge-data)
      purge_override=1
      shift
      ;;
    -h | --help)
      usage
      return
      ;;
    *)
      echo_content red "Unknown option: $1"
      usage
      exit 1
      ;;
    esac
  done

  if [[ -z "${config_file}" ]]; then
    echo_content red "--config is required"
    usage
    exit 1
  fi
  if [[ -n "${force_override}" && "${command}" != install ]]; then
    echo_content red "--force is only valid with install"
    exit 1
  fi
  if [[ -n "${purge_override}" && "${command}" != remove ]]; then
    echo_content red "--purge-data is only valid with remove"
    exit 1
  fi
  if [[ "${command}" != validate ]]; then
    require_root
  fi
  load_config "${config_file}"
  [[ -n "${force_override}" ]] && TP_FORCE="${force_override}"
  [[ -n "${purge_override}" ]] && TP_PURGE_DATA="${purge_override}"
  validate_config
  echo_content skyBlue "Operation: ${command}; purpose: ${TP_PURPOSE}; config: ${config_file}; installer: ${INSTALLER_VERSION}"

  case "${command}:${TP_PURPOSE}" in
  validate:web | validate:node)
    echo_content green "Configuration is valid for ${TP_PURPOSE} purpose: ${config_file}"
    ;;
  install:web)
    require_commands docker curl openssl tar od sha256sum find seq awk
    docker info >/dev/null
    deploy_web
    ;;
  install:node)
    require_commands docker curl openssl tar od sha256sum find seq awk
    docker info >/dev/null
    deploy_node
    ;;
  remove:web)
    require_commands docker
    docker info >/dev/null
    remove_web
    ;;
  remove:node)
    require_commands docker
    docker info >/dev/null
    remove_node
    ;;
  esac
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
