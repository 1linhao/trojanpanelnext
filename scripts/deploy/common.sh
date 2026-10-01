#!/usr/bin/env bash
# Configuration values are consumed by scripts that source this library.
# shellcheck disable=SC2034
set -euo pipefail

export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:${PATH:-}"

ECHO_TYPE="echo -e"
INSTALLER_VERSION="1.0.2-rc.3"
SUPPORTED_SCHEMA_VERSION="1"
GITHUB_RAW_BASE="https://raw.githubusercontent.com/1linhao/trojanpanelnext"
DEFAULT_CONFIG_REF="v${INSTALLER_VERSION}"
CONFIG_REF="${TP_RELEASE_REF:-${DEFAULT_CONFIG_REF}}"
if [[ "${CONFIG_REF}" != "${DEFAULT_CONFIG_REF}" ]]; then
  printf 'Release ref must be %s for this script library.\n' "${DEFAULT_CONFIG_REF}" >&2
  exit 1
fi

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
NODE_CERTIFICATE_MODE="${NODE_CERTIFICATE_MODE:-caddy}"
NODE_CERTIFICATE_PATH="${NODE_CERTIFICATE_PATH:-}"
NODE_PRIVATE_KEY_PATH="${NODE_PRIVATE_KEY_PATH:-}"

TP_FORCE="${TP_FORCE:-0}"
TP_PURGE_DATA="${TP_PURGE_DATA:-0}"
TP_PURPOSE=""
TP_CONFIG_ROOT="${TP_CONFIG_ROOT:-}"

require_matching_version() {
  if [[ "$1" != "${INSTALLER_VERSION}" ]]; then
    printf 'Script version %s does not match common.sh version %s. Use tp.sh to fetch matching files.\n' "$1" "${INSTALLER_VERSION}" >&2
    exit 1
  fi
}

handle_metadata() {
  case "${1:-}" in
  -h | --help | help) usage; return 0 ;;
  -V | --version | version) printf '%s\n' "${INSTALLER_VERSION}"; return 0 ;;
  *) return 1 ;;
  esac
}

# Parse only the options allowed by this command. The caller performs any
# validation or deployment work after configuration has been loaded.
parse_config_options() {
  local action="$1"
  shift
  local config_file="" force_override="" purge_override=""
  while (($#)); do
    case "$1" in
    --config)
      require_option_value "$1" "${2:-}"
      config_file="$2"
      shift 2
      ;;
    --force)
      if [[ "${action}" != install ]]; then
        echo_content red "--force is only valid with install" >&2
        exit 1
      fi
      force_override=1
      shift
      ;;
    --purge-data | --purge)
      if [[ "${action}" != remove ]]; then
        echo_content red "--purge-data is only valid with remove" >&2
        exit 1
      fi
      if [[ "${purge_override}" == 0 ]]; then
        echo_content red "--keep-data and --purge-data cannot be combined" >&2
        exit 1
      fi
      purge_override=1
      shift
      ;;
    --keep-data)
      if [[ "${action}" != remove ]]; then
        echo_content red "--keep-data is only valid with remove" >&2
        exit 1
      fi
      if [[ "${purge_override}" == 1 ]]; then
        echo_content red "--keep-data and --purge-data cannot be combined" >&2
        exit 1
      fi
      purge_override=0
      shift
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      echo_content red "Unknown option: $1" >&2
      exit 1
      ;;
    esac
  done
  if [[ -z "${config_file}" ]]; then
    echo_content red "--config is required" >&2
    exit 1
  fi
  [[ "${action}" == validate ]] || require_root
  load_config "${config_file}"
  [[ -z "${force_override}" ]] || TP_FORCE="${force_override}"
  [[ -z "${purge_override}" ]] || TP_PURGE_DATA="${purge_override}"
}

echo_content() {
  case $1 in
  "red") ${ECHO_TYPE} "\033[31m$2\033[0m" ;;
  "green") ${ECHO_TYPE} "\033[32m$2\033[0m" ;;
  "yellow") ${ECHO_TYPE} "\033[33m$2\033[0m" ;;
  "skyBlue") ${ECHO_TYPE} "\033[36m$2\033[0m" ;;
  *) ${ECHO_TYPE} "$2" ;;
  esac
}

require_commands() {
  local name
  local missing=()
  for name in "$@"; do
    command -v "${name}" >/dev/null 2>&1 || missing+=("${name}")
  done
  if ((${#missing[@]})); then
    echo_content red "Missing dependencies: ${missing[*]}. Install them first; see docs/deployment.md." >&2
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
  local release
  release="$(yaml_read_raw "${file}" release)"
  if [[ "${release}" != "${INSTALLER_VERSION}" ]]; then
    echo_content red "Configuration release ${release:-<missing>} does not match script release ${INSTALLER_VERSION}. Fetch a configuration template for the selected release." >&2
    exit 1
  fi
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
  cfg_apply "${file}" NODE_CERTIFICATE_MODE node_certificate_mode
  cfg_apply "${file}" NODE_CERTIFICATE_PATH node_certificate_path
  cfg_apply "${file}" NODE_PRIVATE_KEY_PATH node_private_key_path
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

require_product_image() {
  local variable="$1" repository="$2"
  local expected="ghcr.io/1linhao/${repository}:${INSTALLER_VERSION}"
  if [[ "${!variable}" != "${expected}" ]]; then
    echo_content red "${variable} must be ${expected} for release ${INSTALLER_VERSION}" >&2
    exit 1
  fi
}

resolve_node_certificate_paths() {
  if [[ "${NODE_CERTIFICATE_MODE}" == caddy ]]; then
    local base="${TP_DATA}/custom/node-caddy/data/caddy/certificates/acme-v02.api.letsencrypt.org-directory/${TP_NODE_DOMAIN}/${TP_NODE_DOMAIN}"
    NODE_CERTIFICATE_PATH="${base}.crt"
    NODE_PRIVATE_KEY_PATH="${base}.key"
  fi
}

# Keep both the configured paths and each visible symlink target. Certbot's
# live/name/*.pem links point into archive/name; mounting a PEM inode would
# freeze the old certificate when Certbot replaces that link during renewal.
external_certificate_references() {
  local path current target count
  for path in "${NODE_CERTIFICATE_PATH}" "${NODE_PRIVATE_KEY_PATH}"; do
    current="$(realpath -ms -- "${path}")"
    printf '%s\n' "${current}"
    count=0
    while [[ -L "${current}" ]]; do
      count=$((count + 1))
      if ((count > 40)); then
        echo_content red "Too many certificate symlinks: ${path}" >&2
        return 1
      fi
      target="$(readlink -- "${current}")"
      if [[ "${target}" == *,* || "${target}" == *$'\n'* || "${target}" == *$'\r'* ]]; then
        echo_content red "Certificate symlink targets must contain no commas or line breaks" >&2
        return 1
      fi
      if [[ "${target}" != /* ]]; then target="$(dirname -- "${current}")/${target}"; fi
      current="$(realpath -ms -- "${target}")"
      printf '%s\n' "${current}"
    done
    realpath -m -- "${path}"
  done
}

protect_external_certificates() {
  [[ "${TP_PURPOSE}" == node && "${NODE_CERTIFICATE_MODE}" == external ]] || return 0
  local references reference path resolved
  references="$(external_certificate_references)" || return
  for path in "$@"; do
    resolved="$(realpath -m -- "${path}")"
    while IFS= read -r reference; do
      if [[ "${reference}" == "${resolved}" || "${reference}" == "${resolved}"/* ]]; then
        echo_content red "External certificate overlaps a project removal path: ${path}. Store externally managed certificates outside project data and maintenance directories." >&2
        return 1
      fi
    done <<<"${references}"
  done
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
  if [[ "${TP_PKI_BUNDLE_DIR}" != /* ]]; then
    echo_content red "pki_bundle_dir must be an absolute path" >&2
    exit 1
  fi

  case "${purpose}" in
  web)
    require_value TP_WEB_DOMAIN
    require_port PANEL_PORT
    require_port UI_PORT
    require_value PANEL_IMAGE
    require_value UI_IMAGE
    require_product_image PANEL_IMAGE trojanpanelnext-api
    require_product_image UI_IMAGE trojanpanelnext-web
    ;;
  node)
    require_value TP_NODE_DOMAIN
    require_value MARIADB_HOST
    require_value MARIADB_PASSWORD
    require_value REDIS_HOST
    require_value REDIS_PASSWORD
    require_value CORE_IMAGE
    require_product_image CORE_IMAGE trojanpanelnext-node-agent
    require_port CORE_PORT
    require_port GRPC_PORT
    if ((GRPC_PORT >= 65535)) || [[ ! "${NODE_SERVER_ID}" =~ ^[1-9][0-9]*$ ]]; then
      echo_content red "Node requires node_server_id >= 1 and grpc_port <= 65534 (host removal uses grpc_port + 1)" >&2
      exit 1
    fi
    require_one_of node_certificate_mode "${NODE_CERTIFICATE_MODE}" caddy external
    if [[ "${NODE_CERTIFICATE_MODE}" == caddy ]]; then
      require_port NODE_CADDY_HTTP_PORT
      require_port NODE_CADDY_HTTPS_PORT
    else
      require_value NODE_CERTIFICATE_PATH
      require_value NODE_PRIVATE_KEY_PATH
      local path
      for path in "${NODE_CERTIFICATE_PATH}" "${NODE_PRIVATE_KEY_PATH}"; do
        if [[ "${path}" != /* || "${path}" == *$'\n'* || "${path}" == *$'\r'* || "${path}" == *,* ]]; then
          echo_content red "External certificate paths must be absolute and contain no commas or line breaks" >&2
          exit 1
        fi
      done
      protect_external_certificates \
        "${TP_DATA}/custom/node-caddy" "${TP_DATA}/custom/web-caddy" \
        "${TP_DATA}/trojan-panel" "${TP_DATA}/trojan-panel-ui" "${TP_DATA}/trojan-panel-core" \
        "${TP_DATA}/mariadb" "${TP_DATA}/redis" "${WEB_PATH}" "${KERNEL_RUNTIME_PATH}" "${TP_PKI_BUNDLE_DIR}" \
        /etc/trojanpanelnext-host /usr/local/lib/trojanpanelnext-host
    fi
    require_one_of grpc_tls_mode "${GRPC_TLS_MODE}" mtls
    require_value TP_PKI_BUNDLE_DIR
    if [[ "${GRPC_CLIENT_CA_PATH}" != /* ]]; then
      echo_content red "grpc_client_ca_path must be an absolute path" >&2
      exit 1
    fi
    ;;
  *)
    echo_content red "Unsupported purpose: ${purpose}"
    exit 1
    ;;
  esac
}
