#!/usr/bin/env bash
set -euo pipefail

export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:${PATH:-}"

ECHO_TYPE="echo -e"
YQ_VERSION="v4.53.6"

TP_DATA="${TP_DATA:-/tpdata}"
WEB_PATH="${WEB_PATH:-${TP_DATA}/web}"
INITIAL_SYSADMIN_PASSWORD_FILE="${INITIAL_SYSADMIN_PASSWORD_FILE:-${TP_DATA}/trojan-panel/config/initial-admin-password}"
TP_PKI_BUNDLE_DIR="${TP_PKI_BUNDLE_DIR:-${TP_DATA}/trojanpanelnext-pki}"
INSTALLER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SECURE_FILE_HELPER_OVERRIDE="${SECURE_FILE_HELPER:-}"
SECURE_FILE_HELPER="${SECURE_FILE_HELPER_OVERRIDE:-${INSTALLER_DIR}/secure-file}"
NODE_BUNDLE_HELPER_OVERRIDE="${NODE_BUNDLE_HELPER:-}"
NODE_BUNDLE_HELPER="${NODE_BUNDLE_HELPER_OVERRIDE:-${INSTALLER_DIR}/node-bundle}"
ENTRYCTL_PATH_OVERRIDE="${ENTRYCTL_PATH:-}"
ENTRYCTL_PATH="${ENTRYCTL_PATH_OVERRIDE:-${INSTALLER_DIR}/entry/entryctl.sh}"
ENTRY_RUNTIME_DIR="${ENTRY_RUNTIME_DIR:-/usr/local/lib/trojanpanelnext/entry}"
ENTRY_SPEC_FILE="${ENTRY_SPEC_FILE:-}"
EXTERNAL_MANAGED_DIR="${EXTERNAL_MANAGED_DIR:-${TP_DATA}/trojanpanelnext-external}"
EXTERNAL_ROUTES_DIR="${EXTERNAL_ROUTES_DIR:-${TP_DATA}/trojan-panel-core/external}"
MANAGED_CERT_DIR="${MANAGED_CERT_DIR:-${TP_DATA}/trojan-panel-core/cert}"
NETWORK_PLAN_DIR="${NETWORK_PLAN_DIR:-${TP_DATA}/trojanpanelnext-network}"
INSTALLER_STATE_DIR="${INSTALLER_STATE_DIR:-${TP_DATA}/trojanpanelnext-installer}"
INSTALLER_STATE_FILE=""

MARIADB_CONTAINER="${MARIADB_CONTAINER:-trojan-panel-mariadb}"
REDIS_CONTAINER="${REDIS_CONTAINER:-trojan-panel-redis}"
PANEL_CONTAINER="${PANEL_CONTAINER:-trojan-panel}"
UI_CONTAINER="${UI_CONTAINER:-trojan-panel-ui}"
CORE_CONTAINER="${CORE_CONTAINER:-trojan-panel-core}"
WEB_CADDY_CONTAINER="${WEB_CADDY_CONTAINER:-trojan-panel-web-caddy}"
NODE_CADDY_CONTAINER="${NODE_CADDY_CONTAINER:-trojan-panel-node-caddy}"
COMBINED_ENTRY_DEPLOYMENT_ID="${COMBINED_ENTRY_DEPLOYMENT_ID:-trojanpanelnext-combined-entry}"
COMBINED_ENTRY_ROOT="${COMBINED_ENTRY_ROOT:-${TP_DATA}/custom/web-caddy}"
COMBINED_ENTRY_STATE_ROOT="${COMBINED_ENTRY_STATE_ROOT:-${TP_DATA}/trojanpanelnext-entry/state}"
COMBINED_ENTRY_SPEC="${COMBINED_ENTRY_SPEC:-${TP_DATA}/trojanpanelnext-entry/combined-spec.json}"
COMBINED_ENTRY_CONTAINER="${COMBINED_ENTRY_CONTAINER:-${WEB_CADDY_CONTAINER}}"
COMBINED_OWNER_TOKEN="${COMBINED_OWNER_TOKEN:-}"
COMBINED_OWNER_MARKER="${COMBINED_OWNER_MARKER:-${COMBINED_ENTRY_ROOT}/.trojanpanelnext-owner}"
COMBINED_RESTORE_ROLE="${COMBINED_RESTORE_ROLE:-}"

CADDY_IMAGE="${CADDY_IMAGE:-caddy:2.8.4}"
MARIADB_IMAGE="${MARIADB_IMAGE:-mariadb:10.7.3}"
REDIS_IMAGE="${REDIS_IMAGE:-redis:6.2.7}"
PANEL_IMAGE="${PANEL_IMAGE:-ghcr.io/1linhao/trojanpanelnext-api:latest}"
UI_IMAGE="${UI_IMAGE:-ghcr.io/1linhao/trojanpanelnext-web:latest}"
CORE_IMAGE="${CORE_IMAGE:-ghcr.io/1linhao/trojanpanelnext-node-agent:latest}"
IMAGE_BUNDLE_DIR="${IMAGE_BUNDLE_DIR:-}"

MARIADB_PORT="${MARIADB_PORT:-9507}"
MARIADB_USER="${MARIADB_USER:-root}"
MARIADB_DATABASE="${MARIADB_DATABASE:-trojan_panel_db}"
ACCOUNT_TABLE="${ACCOUNT_TABLE:-account}"
REDIS_PORT="${REDIS_PORT:-6378}"
REDIS_USERNAME="${REDIS_USERNAME:-}"
REDIS_AUTH_USERNAME="${REDIS_AUTH_USERNAME:-}"
PANEL_PORT="${PANEL_PORT:-8081}"
UI_PORT="${UI_PORT:-8888}"
CORE_PORT="${CORE_PORT:-8082}"
GRPC_PORT="${GRPC_PORT:-8100}"
NODE_SERVER_ID="${NODE_SERVER_ID:-0}"
NODE_IDENTITY_ID="${NODE_IDENTITY_ID:-}"
NODE_IDENTITY_GENERATION="${NODE_IDENTITY_GENERATION:-0}"
NODE_BOOTSTRAP_CHALLENGE=""
GRPC_TLS_MODE="${GRPC_TLS_MODE:-mtls}"
GRPC_TLS_SERVER_NAME="${GRPC_TLS_SERVER_NAME:-}"
GRPC_CLIENT_CA_PATH="${GRPC_CLIENT_CA_PATH:-${TP_DATA}/trojan-panel-core/pki/client-ca.crt}"
GRPC_CLIENT_CERT_PATH="${GRPC_CLIENT_CERT_PATH:-${TP_DATA}/trojan-panel/pki/client.crt}"
GRPC_CLIENT_KEY_PATH="${GRPC_CLIENT_KEY_PATH:-${TP_DATA}/trojan-panel/pki/client.key}"
GRPC_SERVER_CA_PATH="${GRPC_SERVER_CA_PATH:-}"
KERNEL_RUNTIME_PATH="${KERNEL_RUNTIME_PATH:-${TP_DATA}/trojan-panel-core/runtime}"
NODE_CADDY_HTTP_PORT="${NODE_CADDY_HTTP_PORT:-80}"
NODE_CADDY_HTTPS_PORT="${NODE_CADDY_HTTPS_PORT:-8863}"
TP_NODE_NAME="${TP_NODE_NAME:-}"
TP_NODE_PUBLIC_IP="${TP_NODE_PUBLIC_IP:-}"
CONTROL_PLANE_PUBLIC_IP="${CONTROL_PLANE_PUBLIC_IP:-}"
NODE_IDENTITY_CREDENTIAL_FILE="${NODE_IDENTITY_CREDENTIAL_FILE:-}"
WEB_MARIADB_USER="${WEB_MARIADB_USER:-}"
WEB_MARIADB_PASSWORD="${WEB_MARIADB_PASSWORD:-}"
WEB_REDIS_PASSWORD="${WEB_REDIS_PASSWORD:-}"
TLS_MODE="${TLS_MODE:-acme}"
TLS_CERT_DIR="${TLS_CERT_DIR:-}"
TLS_CERT_FILE="${TLS_CERT_FILE:-}"
TLS_KEY_FILE="${TLS_KEY_FILE:-}"
BIND_ADDRESS="${BIND_ADDRESS:-0.0.0.0}"
UI_LISTEN=""
TLS_CERT_PAIR=""
EXTERNAL_ROUTES_NOTE=""

TP_FORCE="${TP_FORCE:-0}"
TP_PURGE_DATA="${TP_PURGE_DATA:-0}"
TP_INSTALL_DEPS="${TP_INSTALL_DEPS:-1}"
TP_HEALTH_ATTEMPTS="${TP_HEALTH_ATTEMPTS:-30}"
TP_HEALTH_DELAY_SECONDS="${TP_HEALTH_DELAY_SECONDS:-2}"
TP_CONTAINER_ATTEMPTS="${TP_CONTAINER_ATTEMPTS:-60}"
TP_CONTAINER_DELAY_SECONDS="${TP_CONTAINER_DELAY_SECONDS:-2}"
TP_OS_RELEASE_FILE="${TP_OS_RELEASE_FILE:-/etc/os-release}"
TP_DEPLOYMENT_MODE=""
TP_CONFIG_ROOT="${TP_CONFIG_ROOT:-}"
TP_CONFIG_FILE=""
TP_CONFIG_READ_FILE=""
TP_CONFIG_IDENTITY=""
TP_REQUEST_COMMAND=""
INSTALLER_ASSET_VERSION="development"
TP_ASSET_VERSION=""
TP_TEMP_TOOLS_DIR=""
TP_SECURE_CONFIG_DIR=""
TP_NODE_BUNDLE_DIR=""
TP_NODE_BUNDLE_TMP_ROOT="${TP_NODE_BUNDLE_TMP_ROOT:-/dev/shm}"
TP_NODE_BUNDLE_ACTIVE=0
TP_DEPENDENCY_PLAN=(
  'docker|docker|install|docker|-|download and run the Docker installer from https://get.docker.com'
  'age|age|install|package|age|apt-get install -y age'
  'curl|curl|base|package|curl|apt-get install -y curl'
  'tar|tar|base|package|tar|apt-get install -y tar'
  'coreutils|od sha256sum install realpath|base|package|coreutils|apt-get install -y coreutils'
  'openssl|openssl|base|package|openssl|apt-get install -y openssl'
  "yq|yq|install|yq|-|install the pinned ${YQ_VERSION} linux_amd64 binary from github.com/mikefarah/yq to /usr/local/bin/yq"
  'jq|jq|entry|package|jq|apt-get install -y jq'
)

cleanup() {
  if [[ -n "${TP_TEMP_TOOLS_DIR}" && -d "${TP_TEMP_TOOLS_DIR}" ]]; then
    rm -rf -- "${TP_TEMP_TOOLS_DIR}"
  fi
  if [[ -n "${TP_SECURE_CONFIG_DIR}" && -d "${TP_SECURE_CONFIG_DIR}" ]]; then
    rm -rf -- "${TP_SECURE_CONFIG_DIR}"
  fi
  if [[ -n "${TP_NODE_BUNDLE_DIR}" && -d "${TP_NODE_BUNDLE_DIR}" ]]; then
    rm -rf -- "${TP_NODE_BUNDLE_DIR}"
  fi
}

trap cleanup EXIT

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
  local mode="${1:-web|node|combined}"
  cat <<EOF
Usage:
  $0 install  --mode $mode (--config <file>|--bundle <encrypted.age>) [--entry-spec <0600-file>]
  $0 remove   --mode $mode --config <file> [--entry-spec <0600-file>] [--purge-data|--keep-data]
  $0 validate --mode $mode (--config <file>|--bundle <encrypted.age>) [--entry-spec <file>]
  $0 refresh-cert --mode node|combined --config <file>

Options:
  --mode <mode>      Deployment mode: Web control plane, Node Agent, or combined
  --entry-spec <file>  Versioned EntrySpec consumed by EntryController
  --restore-role <role> Explicitly restore a removed combined role (web|node)
  --config <file>    YAML configuration file
  --bundle <file>    Encrypted Node bootstrap bundle (node install/validate only)
  --force            Recreate existing containers during installation
  --purge-data       Delete generated data during removal
  --keep-data        Preserve generated data during removal, overriding the config
  -h, --help         Show this help

Examples:
  $0 validate --mode web --config ./examples/web.yaml
  $0 install --mode web --config ./examples/web.yaml
  $0 install --mode node --config ./examples/node-agent.yaml
  $0 install --mode node --config ./examples/external-node.yaml

The command is non-interactive. The value of --mode must match
trojanpanelnext.deployment_mode in the configuration file. Legacy
trojanpanelnext.purpose remains accepted as compatibility input.

TLS ownership:
  tls_mode: acme      (default) the installer runs a Caddy container that
                      listens on 80/443 and obtains its own ACME certificate.
                      In combined mode one shared Caddy deployment owns both
                      domains and both certificates.
  tls_mode: external  no Caddy container is created. The external entry
                      point owns Web 80/443, ACME and required plain-HTTP
                      fallback listeners. Node protocol ports stay direct.
                      The installer only copies the
                      certificates in tls_cert_dir to ${MANAGED_CERT_DIR}
                      for the proxy kernels to read.
  See deploy/installer/EXTERNAL.md for the external entry point contract.
EOF
}

require_root() {
  if [[ "$(id -u)" != "0" ]]; then
    echo_content red "Please run as root"
    exit 1
  fi
}

require_supported_install_platform() {
  local os_id=""
  local os_version=""
  local architecture
  architecture="$(uname -m)"

  if [[ ! -r "${TP_OS_RELEASE_FILE}" ]]; then
    echo_content red "Cannot identify the operating system: ${TP_OS_RELEASE_FILE} is not readable"
    exit 1
  fi

  os_id="$(sed -nE 's/^ID="?([^"[:space:]]+)"?$/\1/p' "${TP_OS_RELEASE_FILE}" | head -n 1)"
  os_version="$(sed -nE 's/^VERSION_ID="?([^"[:space:]]+)"?$/\1/p' "${TP_OS_RELEASE_FILE}" | head -n 1)"
  if [[ "${os_id}" != debian || "${os_version}" != 12 || "${architecture}" != x86_64 ]]; then
    echo_content red "Unsupported platform: ${os_id:-unknown} ${os_version:-unknown} ${architecture}"
    echo_content yellow "Bare VPS installation currently supports Debian 12 x86_64 only"
    exit 1
  fi
}

append_unique_word() {
  local array_name="$1"
  local value="$2"
  local known
  local -n words="${array_name}"
  for known in "${words[@]}"; do
    [[ "${known}" == "${value}" ]] && return
  done
  words+=("${value}")
}

required_dependency_plan() {
  local record name commands scope method target advice
  for record in "${TP_DEPENDENCY_PLAN[@]}"; do
    IFS='|' read -r name commands scope method target advice <<<"${record}"
    if [[ "${scope}" == entry && -z "${ENTRY_SPEC_FILE:-}" ]]; then
      continue
    fi
    printf '%s\n' "${record}"
  done
}

dependency_record_is_missing() {
  local record="$1"
  local name commands scope method target advice required_command
  IFS='|' read -r name commands scope method target advice <<<"${record}"
  for required_command in ${commands}; do
    command -v "${required_command}" >/dev/null 2>&1 || return 0
  done
  return 1
}

missing_install_dependencies() {
  TP_MISSING_DEPENDENCY_PLAN=()
  local method_filter="${1:-}"
  local record name commands scope method target advice
  while IFS= read -r record; do
    IFS='|' read -r name commands scope method target advice <<<"${record}"
    if [[ -n "${method_filter}" && "${method}" != "${method_filter}" ]]; then
      continue
    fi
    if dependency_record_is_missing "${record}"; then
      TP_MISSING_DEPENDENCY_PLAN+=("${record}")
    fi
  done < <(required_dependency_plan)
}

print_dependency_installation_advice() {
  local record name commands scope method target advice
  echo_content red "Missing required Debian 12 dependencies:"
  for record in "${TP_MISSING_DEPENDENCY_PLAN[@]}"; do
    IFS='|' read -r name commands scope method target advice <<<"${record}"
    echo_content yellow "- ${name}: ${advice}"
  done
}

install_missing_package_dependencies() {
  local record name commands scope method target advice
  local -a packages=()

  for record in "${TP_MISSING_DEPENDENCY_PLAN[@]}"; do
    IFS='|' read -r name commands scope method target advice <<<"${record}"
    [[ "${method}" == package ]] && append_unique_word packages "${target}"
  done

  install_packages "${packages[@]}"
}

install_missing_special_dependencies() {
  local record name commands scope method target advice
  local docker_missing=0
  local yq_missing=0

  for record in "${TP_MISSING_DEPENDENCY_PLAN[@]}"; do
    IFS='|' read -r name commands scope method target advice <<<"${record}"
    case "${method}" in
    docker) docker_missing=1 ;;
    yq) yq_missing=1 ;;
    esac
  done

  if [[ "${docker_missing}" == 1 ]]; then
    install_docker
  fi
  if [[ "${yq_missing}" == 1 ]]; then
    install_yq
  fi
}

preflight_install_dependencies() {
  require_one_of TP_INSTALL_DEPS "${TP_INSTALL_DEPS}" 0 1
  missing_install_dependencies
  if [[ ${#TP_MISSING_DEPENDENCY_PLAN[@]} -eq 0 ]]; then
    return
  fi

  if [[ "${TP_INSTALL_DEPS}" == 0 ]]; then
    print_dependency_installation_advice
    echo_content yellow "Set TP_INSTALL_DEPS=1 to let the installer add only these dependencies"
    exit 1
  fi

  missing_install_dependencies package
  install_missing_package_dependencies
  missing_install_dependencies package
  if [[ ${#TP_MISSING_DEPENDENCY_PLAN[@]} -ne 0 ]]; then
    print_dependency_installation_advice
    echo_content yellow "Package installation completed, but the commands above are still unavailable"
    exit 1
  fi

  missing_install_dependencies
  install_missing_special_dependencies
  missing_install_dependencies
  if [[ ${#TP_MISSING_DEPENDENCY_PLAN[@]} -ne 0 ]]; then
    print_dependency_installation_advice
    echo_content yellow "Dependency installation completed, but the commands above are still unavailable"
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

random_sysadmin_password() {
  od -An -N20 -tu1 /dev/urandom | awk '
    BEGIN { alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789" }
    { for (i = 1; i <= NF; i++) printf "%s", substr(alphabet, ($i % 62) + 1, 1) }
    END { print "" }
  '
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

install_packages() {
  local packages=("$@")
  if [[ ${#packages[@]} -eq 0 ]]; then
    return
  fi

  if command -v apt-get >/dev/null 2>&1; then
    DEBIAN_FRONTEND=noninteractive apt-get update
    DEBIAN_FRONTEND=noninteractive apt-get install -y "${packages[@]}"
  elif command -v apt >/dev/null 2>&1; then
    DEBIAN_FRONTEND=noninteractive apt update
    DEBIAN_FRONTEND=noninteractive apt install -y "${packages[@]}"
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y "${packages[@]}"
  elif command -v yum >/dev/null 2>&1; then
    yum install -y "${packages[@]}"
  else
    echo_content red "No supported package manager found"
    exit 1
  fi
}

install_base_tools() {
  local record name commands scope method target advice
  local -a packages=()
  while IFS= read -r record; do
    IFS='|' read -r name commands scope method target advice <<<"${record}"
    if [[ "${scope}" != install ]] && dependency_record_is_missing "${record}"; then
      append_unique_word packages "${target}"
    fi
  done < <(required_dependency_plan)
  install_packages "${packages[@]}"
}

validate_entry_spec_binding() {
  local mode="$1"
  local spec="${ENTRY_SPEC_FILE:-}"
  [[ -n "${spec}" ]] || return 0
  [[ "${TLS_MODE}" == external ]] || {
    echo_content red "--entry-spec is only valid with tls_mode: external"
    return 1
  }
  [[ "${spec}" == /* && -f "${spec}" && ! -L "${spec}" ]] || {
    echo_content red "--entry-spec must be an absolute regular non-symlink file"
    return 1
  }
  command -v jq >/dev/null 2>&1 || {
    echo_content red "jq is required to validate --entry-spec"
    return 1
  }
  local domain
  domain="${TP_WEB_DOMAIN:-}"
  [[ "${mode}" == node ]] && domain="${TP_NODE_DOMAIN:-}"
  jq -e --arg mode "${mode}" --arg domain "${domain}" '
    .schema_version == 1 and
    .provider == "external" and
    .purpose == $mode and
    .domain == $domain and
    (.deployment_id | type == "string" and test("^[a-z][a-z0-9-]{0,62}$")) and
    .external_driver.protocol_version == 1 and
    (.external_driver.path | type == "string" and startswith("/"))
  ' "${spec}" >/dev/null || {
    echo_content red "EntrySpec does not match installer purpose/domain or protocol v1"
    return 1
  }
}

entry_controller() {
  local action="$1"
  [[ -n "${ENTRY_SPEC_FILE:-}" ]] || return 0
  [[ -x "${ENTRYCTL_PATH}" && ! -L "${ENTRYCTL_PATH}" ]] || {
    echo_content red "EntryController is unavailable or unsafe: ${ENTRYCTL_PATH}"
    return 1
  }
  case "${action}" in
  reconcile)
    CADDY_ADAPTER_IMAGE="${CADDY_IMAGE}" CADDY_ADAPTER_ENTRYCTL_PATH="${ENTRYCTL_PATH}" \
      "${ENTRYCTL_PATH}" reconcile --spec "${ENTRY_SPEC_FILE}"
    ;;
  remove)
    if [[ "${TP_PURGE_DATA}" == 1 ]]; then
      CADDY_ADAPTER_IMAGE="${CADDY_IMAGE}" CADDY_ADAPTER_ENTRYCTL_PATH="${ENTRYCTL_PATH}" \
        "${ENTRYCTL_PATH}" remove --spec "${ENTRY_SPEC_FILE}" --purge
    else
      CADDY_ADAPTER_IMAGE="${CADDY_IMAGE}" CADDY_ADAPTER_ENTRYCTL_PATH="${ENTRYCTL_PATH}" \
        "${ENTRYCTL_PATH}" remove --spec "${ENTRY_SPEC_FILE}"
    fi
    ;;
  *)
    echo_content red "Unsupported EntryController action: ${action}"
    return 1
    ;;
  esac
}

install_entry_runtime_assets() {
  local target="${ENTRY_RUNTIME_DIR}" source
  [[ "${target}" == /usr/local/lib/trojanpanelnext/entry ]] || return 1
  for source in \
    "${INSTALLER_DIR}/entry/entryctl.sh" \
    "${INSTALLER_DIR}/entry/controller.sh" \
    "${INSTALLER_DIR}/entry/v2.sh" \
    "${INSTALLER_DIR}/entry/adapters/external.sh" \
    "${INSTALLER_DIR}/entry/adapters/nginx_certbot.sh" \
    "${INSTALLER_DIR}/entry/adapters/caddy.sh"; do
    [[ -f "${source}" && ! -L "${source}" ]] || return 1
  done
  install -d -m 0755 "${target}" "${target}/adapters" || return 1
  install -m 0755 "${INSTALLER_DIR}/entry/entryctl.sh" "${target}/entryctl.sh" || return 1
  install -m 0644 "${INSTALLER_DIR}/entry/controller.sh" "${target}/controller.sh" || return 1
  install -m 0644 "${INSTALLER_DIR}/entry/v2.sh" "${target}/v2.sh" || return 1
  install -m 0644 "${INSTALLER_DIR}/entry/adapters/external.sh" "${target}/adapters/external.sh" || return 1
  install -m 0755 "${INSTALLER_DIR}/entry/adapters/nginx_certbot.sh" "${target}/adapters/nginx_certbot.sh" || return 1
  install -m 0755 "${INSTALLER_DIR}/entry/adapters/caddy.sh" "${target}/adapters/caddy.sh" || return 1
}

install_yq() {
  local destination="${1:-/usr/local/bin/yq}"
  if command -v yq >/dev/null 2>&1; then
    return
  fi

  install_base_tools
  local arch checksum
  case "$(uname -m)" in
  x86_64 | amd64)
    arch="amd64"
    checksum="c5f056448f973ae7d39b5401949648a78f2dc1947d6a8eb65be60d5c504b9385"
    ;;
  aarch64 | arm64)
    arch="arm64"
    checksum="88a1016bc1d657375a35864e4f44b6f333df8ff97b559f51bba0adcb2169df09"
    ;;
  *)
    echo_content red "Unsupported architecture for yq: $(uname -m)"
    exit 1
    ;;
  esac

  echo_content green "---> Install yq ${YQ_VERSION}"
  local download
  download="$(mktemp)"
  curl -fsSL "https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/yq_linux_${arch}" -o "${download}"
  printf '%s  %s\n' "${checksum}" "${download}" | sha256sum -c -
  install -m 0755 "${download}" "${destination}"
  rm -f "${download}"
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
    echo_content red "Configuration must contain a 'trojanpanelnext' root object"
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

cfg_apply_compat() {
  local file="$1"
  local var_name="$2"
  local canonical_key="$3"
  local legacy_key="$4"
  local canonical_value legacy_value
  canonical_value="$(yaml_read_raw "${file}" "${canonical_key}")"
  legacy_value="$(yaml_read_raw "${file}" "${legacy_key}")"
  if [[ -n "${canonical_value}" && -n "${legacy_value}" && "${canonical_value}" != "${legacy_value}" ]]; then
    echo_content red "Configuration keys '${canonical_key}' and legacy '${legacy_key}' disagree"
    exit 1
  fi
  if [[ -n "${canonical_value}" ]]; then
    printf -v "${var_name}" '%s' "${canonical_value}"
  elif [[ -n "${legacy_value}" ]]; then
    printf -v "${var_name}" '%s' "${legacy_value}"
  fi
}

verify_release_assets_before_host_change() {
  local config_file="${1:-}"
  local assets_only="${2:-0}"
  local verifier="${INSTALLER_DIR}/verify-assets.sh"

  [[ "${INSTALLER_ASSET_VERSION}" != development ]] || return 0
  if [[ ! -x "${verifier}" || -L "${verifier}" ]]; then
    echo_content red "Released installer requires its bundled verify-assets.sh"
    exit 1
  fi
  if [[ "${assets_only}" == 1 ]]; then
    "${verifier}" --assets-dir "${INSTALLER_DIR}" --assets-only
  else
    "${verifier}" --assets-dir "${INSTALLER_DIR}" --config "${config_file}"
  fi
}

apply_executable_asset_policy() {
  if [[ "${INSTALLER_ASSET_VERSION}" != development ]]; then
    if [[ -n "${SECURE_FILE_HELPER_OVERRIDE}" ]]; then
      echo_content red "Released installer rejects SECURE_FILE_HELPER overrides"
      exit 1
    fi
    if [[ -n "${NODE_BUNDLE_HELPER_OVERRIDE}" ]]; then
      echo_content red "Released installer rejects NODE_BUNDLE_HELPER overrides"
      exit 1
    fi
    if [[ -n "${ENTRYCTL_PATH_OVERRIDE}" ]]; then
      echo_content red "Released installer rejects ENTRYCTL_PATH overrides"
      exit 1
    fi
    SECURE_FILE_HELPER="${INSTALLER_DIR}/secure-file"
    NODE_BUNDLE_HELPER="${INSTALLER_DIR}/node-bundle"
    ENTRYCTL_PATH="${INSTALLER_DIR}/entry/entryctl.sh"
    return
  fi

  [[ -n "${SECURE_FILE_HELPER_OVERRIDE}" ]] && SECURE_FILE_HELPER="${SECURE_FILE_HELPER_OVERRIDE}"
  [[ -n "${NODE_BUNDLE_HELPER_OVERRIDE}" ]] && NODE_BUNDLE_HELPER="${NODE_BUNDLE_HELPER_OVERRIDE}"
  [[ -n "${ENTRYCTL_PATH_OVERRIDE}" ]] && ENTRYCTL_PATH="${ENTRYCTL_PATH_OVERRIDE}"
  return 0
}

ensure_node_bundle_helper() {
  if [[ -x "${NODE_BUNDLE_HELPER}" && ! -L "${NODE_BUNDLE_HELPER}" ]]; then
    return
  fi
  if [[ "${INSTALLER_ASSET_VERSION}" == development ]] && command -v go >/dev/null 2>&1; then
    [[ -n "${TP_TEMP_TOOLS_DIR}" ]] || {
      TP_TEMP_TOOLS_DIR="$(mktemp -d /tmp/trojanpanelnext-tools.XXXXXX)"
      chmod 0700 "${TP_TEMP_TOOLS_DIR}"
    }
    NODE_BUNDLE_HELPER="${TP_TEMP_TOOLS_DIR}/node-bundle"
    (cd "${INSTALLER_DIR}/nodebundle" && CGO_ENABLED=0 go build -trimpath -o "${NODE_BUNDLE_HELPER}" .)
    chmod 0700 "${NODE_BUNDLE_HELPER}"
    return
  fi
  echo_content red "Installer node-bundle helper is missing or unsafe"
  exit 1
}

prepare_node_bundle() {
  local bundle="$1"
  [[ -f "${bundle}" && ! -L "${bundle}" ]] || {
    echo_content red "Encrypted Node bootstrap bundle is missing or unsafe: ${bundle}"
    exit 1
  }
  [[ -d "${TP_NODE_BUNDLE_TMP_ROOT}" && ! -L "${TP_NODE_BUNDLE_TMP_ROOT}" ]] || {
    echo_content red "Node bootstrap temporary root is missing or unsafe: ${TP_NODE_BUNDLE_TMP_ROOT}"
    exit 1
  }
  if [[ "$(stat -f -c %T "${TP_NODE_BUNDLE_TMP_ROOT}")" != tmpfs ]]; then
    echo_content red "Node bootstrap plaintext may only be opened on a tmpfs temporary root"
    exit 1
  fi
  ensure_node_bundle_helper
  TP_NODE_BUNDLE_DIR="$(mktemp -d "${TP_NODE_BUNDLE_TMP_ROOT%/}/trojanpanelnext-node-bundle.XXXXXX")"
  chmod 0700 "${TP_NODE_BUNDLE_DIR}"
  if ! "${NODE_BUNDLE_HELPER}" extract --bundle "${bundle}" --directory "${TP_NODE_BUNDLE_DIR}"; then
    echo_content red "Could not decrypt or validate the Node bootstrap bundle"
    exit 1
  fi
  unset TP_NODE_BUNDLE_PASSWORD
  mkdir -m 0700 "${TP_NODE_BUNDLE_DIR}/secure"
  TP_SECURE_CONFIG_DIR="${TP_NODE_BUNDLE_DIR}/secure"
  TP_NODE_BUNDLE_ACTIVE=1
  prepare_secure_config "${TP_NODE_BUNDLE_DIR}/config-node.yaml"
}

initialize_node_bootstrap_challenge() {
  local runtime_config="${TP_DATA}/trojan-panel-core/config/config.ini" previous
  if [[ "${TP_DEPLOYMENT_MODE}" == combined && -f "${runtime_config}" && ! -L "${runtime_config}" &&
    "$(stat -c %a "${runtime_config}")" == 600 ]]; then
    previous="$(sed -n 's/^bootstrap_challenge=//p' "${runtime_config}" | head -n 1)"
    if [[ "${previous}" =~ ^[0-9a-f]{64}$ ]]; then
      NODE_BOOTSTRAP_CHALLENGE="${previous}"
      return
    fi
  fi
  NODE_BOOTSTRAP_CHALLENGE="$(openssl rand -hex 32)"
  if [[ ! "${NODE_BOOTSTRAP_CHALLENGE}" =~ ^[0-9a-f]{64}$ ]]; then
    echo_content red "Could not create a fresh Node installation challenge"
    exit 1
  fi
}

ensure_secure_file_helper() {
  if [[ -x "${SECURE_FILE_HELPER}" && ! -L "${SECURE_FILE_HELPER}" ]]; then
    return
  fi
  if [[ "${INSTALLER_ASSET_VERSION}" == development ]] && command -v go >/dev/null 2>&1; then
    [[ -n "${TP_SECURE_CONFIG_DIR}" ]] || {
      TP_SECURE_CONFIG_DIR="$(mktemp -d /tmp/trojanpanelnext-secure.XXXXXX)"
      chmod 0700 "${TP_SECURE_CONFIG_DIR}"
    }
    SECURE_FILE_HELPER="${TP_SECURE_CONFIG_DIR}/secure-file"
    (cd "${INSTALLER_DIR}/securefile" && CGO_ENABLED=0 go build -trimpath -o "${SECURE_FILE_HELPER}" .)
    chmod 0700 "${SECURE_FILE_HELPER}"
    return
  fi
  echo_content red "Installer secure-file helper is missing or unsafe"
  exit 1
}

prepare_secure_config() {
  local path="$1"
  ensure_secure_file_helper
  [[ -n "${TP_SECURE_CONFIG_DIR}" ]] || {
    TP_SECURE_CONFIG_DIR="$(mktemp -d /tmp/trojanpanelnext-secure.XXXXXX)"
    chmod 0700 "${TP_SECURE_CONFIG_DIR}"
  }
  TP_CONFIG_FILE="${path}"
  TP_CONFIG_READ_FILE="${TP_SECURE_CONFIG_DIR}/config.yaml"
  if ! TP_CONFIG_IDENTITY="$("${SECURE_FILE_HELPER}" snapshot --path "${path}" --output "${TP_CONFIG_READ_FILE}")"; then
    echo_content red "Sensitive configuration path must not contain symbolic links or .. components"
    exit 1
  fi
}

load_config() {
  local action="$1"
  local file="${2:-}"
  local allow_yq_install="${3:-1}"
  if [[ -z "${file}" ]]; then
    echo_content red "Config file is required"
    usage
    exit 1
  fi
  if [[ ! -f "${file}" ]]; then
    echo_content red "Config file not found: ${file}"
    exit 1
  fi

  if ! command -v yq >/dev/null 2>&1; then
    if [[ "${allow_yq_install}" == "1" ]]; then
      install_yq
    else
      TP_TEMP_TOOLS_DIR="$(mktemp -d /tmp/trojanpanelnext-tools.XXXXXX)"
      install_yq "${TP_TEMP_TOOLS_DIR}/yq"
      export PATH="${TP_TEMP_TOOLS_DIR}:${PATH}"
    fi
  fi
  detect_config_root "${file}"

  cfg_apply_compat "${file}" TP_DEPLOYMENT_MODE deployment_mode purpose
  cfg_apply "${file}" TP_ASSET_VERSION asset_version

  cfg_apply "${file}" CADDY_IMAGE caddy_image
  cfg_apply "${file}" MARIADB_IMAGE mariadb_image
  cfg_apply "${file}" REDIS_IMAGE redis_image
  cfg_apply_compat "${file}" PANEL_IMAGE api_image panel_image
  cfg_apply_compat "${file}" UI_IMAGE web_image ui_image
  cfg_apply_compat "${file}" CORE_IMAGE node_agent_image core_image
  cfg_apply "${file}" IMAGE_BUNDLE_DIR image_bundle_dir

  cfg_apply "${file}" MARIADB_PORT mariadb_port
  cfg_apply "${file}" MARIADB_DATABASE database
  cfg_apply "${file}" ACCOUNT_TABLE account_table
  cfg_apply "${file}" REDIS_PORT redis_port
  cfg_apply "${file}" PANEL_PORT panel_port
  cfg_apply "${file}" UI_PORT ui_port
  cfg_apply "${file}" CORE_PORT core_port
  cfg_apply "${file}" GRPC_PORT grpc_port
  cfg_apply "${file}" NODE_SERVER_ID node_server_id
  cfg_apply "${file}" NODE_IDENTITY_ID node_identity_id
  cfg_apply "${file}" NODE_IDENTITY_GENERATION node_identity_generation
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
  cfg_apply "${file}" TLS_MODE tls_mode
  cfg_apply "${file}" TLS_CERT_DIR tls_cert_dir
  cfg_apply "${file}" TLS_CERT_FILE tls_cert_file
  cfg_apply "${file}" TLS_KEY_FILE tls_key_file
  cfg_apply "${file}" BIND_ADDRESS bind_address
  cfg_apply "${file}" MANAGED_CERT_DIR managed_cert_dir
  cfg_apply "${file}" EXTERNAL_MANAGED_DIR external_managed_dir
  cfg_apply "${file}" EXTERNAL_ROUTES_DIR external_routes_dir
  cfg_apply "${file}" NODE_IDENTITY_CREDENTIAL_FILE node_identity_credential_file
  cfg_apply "${file}" TP_FORCE force
  cfg_apply "${file}" TP_PURGE_DATA purge_data
  # A combined configuration is also accepted by role-scoped removal. Load
  # the complete shared configuration before deciding which role to remove.
  if [[ "${TP_DEPLOYMENT_MODE}" == combined ]]; then
    action=combined
  fi
  case "${action}" in
  web)
    TP_WEB_DOMAIN=""
    TP_EMAIL=""
    MARIADB_PASSWORD=""
    REDIS_PASSWORD=""
    SYSADMIN_PASSWORD=""
    CONTROL_PLANE_PUBLIC_IP=""
    cfg_apply "${file}" MARIADB_USER mariadb_user
    cfg_apply "${file}" TP_WEB_DOMAIN hostname
    cfg_apply "${file}" TP_EMAIL email
    cfg_apply "${file}" MARIADB_PASSWORD mariadb_password
    cfg_apply "${file}" REDIS_PASSWORD redis_password
    cfg_apply "${file}" SYSADMIN_PASSWORD sysadmin_password
    ;;
  node)
    TP_NODE_DOMAIN=""
    TP_EMAIL=""
    MARIADB_HOST=""
    MARIADB_USER=""
    MARIADB_PASSWORD=""
    REDIS_HOST=""
    REDIS_USERNAME=""
    REDIS_AUTH_USERNAME=""
    REDIS_AUTH_PASSWORD=""
    REDIS_PASSWORD=""
    cfg_apply "${file}" TP_NODE_DOMAIN hostname
    cfg_apply "${file}" TP_EMAIL email
    cfg_apply "${file}" MARIADB_HOST mariadb_host
    cfg_apply "${file}" MARIADB_USER mariadb_user
    cfg_apply "${file}" MARIADB_PASSWORD mariadb_password
    cfg_apply "${file}" REDIS_HOST redis_host
    cfg_apply "${file}" REDIS_USERNAME redis_username
    cfg_apply "${file}" REDIS_PASSWORD redis_password
    cfg_apply "${file}" REDIS_AUTH_USERNAME redis_auth_username
    cfg_apply "${file}" REDIS_AUTH_PASSWORD redis_auth_password
    cfg_apply "${file}" CONTROL_PLANE_PUBLIC_IP control_plane_public_ip
    ;;
  combined)
    TP_WEB_DOMAIN=""
    TP_NODE_DOMAIN=""
    TP_EMAIL=""
    TP_NODE_NAME=""
    TP_NODE_PUBLIC_IP=""
    MARIADB_PASSWORD=""
    REDIS_PASSWORD=""
    SYSADMIN_PASSWORD=""
    CONTROL_PLANE_PUBLIC_IP=""
    cfg_apply "${file}" TP_WEB_DOMAIN web_hostname
    cfg_apply "${file}" TP_NODE_DOMAIN node_hostname
    cfg_apply "${file}" TP_EMAIL email
    cfg_apply "${file}" TP_NODE_NAME node_name
    cfg_apply "${file}" TP_NODE_PUBLIC_IP node_public_ip
    cfg_apply "${file}" MARIADB_USER mariadb_user
    cfg_apply "${file}" MARIADB_PASSWORD mariadb_password
    cfg_apply "${file}" REDIS_PASSWORD redis_password
    cfg_apply "${file}" SYSADMIN_PASSWORD sysadmin_password
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

require_bind_address() {
  local name="$1"
  local value="${!name:-}"
  local -a parts=()
  local part nonempty=0

  if [[ "${value}" == *.* ]]; then
    IFS=. read -r -a parts <<<"${value}"
    if [[ "${#parts[@]}" -eq 4 ]]; then
      for part in "${parts[@]}"; do
        if [[ ! "${part}" =~ ^[0-9]{1,3}$ ]] || ((10#${part} > 255)); then
          nonempty=-1
          break
        fi
      done
      [[ "${nonempty}" != "-1" ]] && return
    fi
  elif [[ "${value}" == *:* && "${value}" =~ ^[0-9a-fA-F:]+$ && "${value}" != *:::* ]]; then
    local remainder="${value#*::}"
    local compressed=0
    if [[ "${remainder}" != "${value}" ]]; then
      compressed=1
      [[ "${remainder}" != *::* ]] || compressed=-1
    fi
    IFS=: read -r -a parts <<<"${value}"
    for part in "${parts[@]}"; do
      [[ -z "${part}" ]] && continue
      if [[ ! "${part}" =~ ^[0-9a-fA-F]{1,4}$ ]]; then
        nonempty=-1
        break
      fi
      nonempty=$((nonempty + 1))
    done
    if [[ "${compressed}" == "1" && "${nonempty}" -ge 0 && "${nonempty}" -lt 8 ]] ||
      [[ "${compressed}" == "0" && "${nonempty}" == "8" ]]; then
      return
    fi
  fi

  echo_content red "${name} must be an IPv4 or IPv6 address, for example 127.0.0.1"
  exit 1
}

require_sysadmin_password() {
  local value="${SYSADMIN_PASSWORD:-}"
  [[ -z "${value}" ]] && return
  if [[ ! "${value}" =~ ^[A-Za-z0-9]{16,20}$ ]]; then
    echo_content red "sysadmin_password must contain 16 to 20 ASCII letters or digits"
    exit 1
  fi
}

require_domain() {
  local name="$1"
  local value="${!name:-}"
  if [[ ! "${value}" =~ ^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$ ]]; then
    echo_content red "${name} must be a lowercase DNS hostname with valid labels"
    exit 1
  fi
}

require_public_ip() {
  local name="$1"
  local value="${!name:-}"
  [[ -n "${value}" ]] || {
    echo_content red "${name} is required for a combined Node identity"
    exit 1
  }
  local valid=0
  local -a parts=()
  if [[ "${value}" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then
    local part first second
    IFS=. read -r -a parts <<<"${value}"
    valid=1
    for part in "${parts[@]}"; do
      if [[ ! "${part}" =~ ^[0-9]+$ ]] || ((10#${part} > 255)); then
        valid=0
        break
      fi
    done
    first=$((10#${parts[0]})); second=$((10#${parts[1]}))
    if [[ "${valid}" == 1 ]] && {
      ((first == 0 || first == 10 || first == 127 || first >= 224)) ||
      ((first == 169 && second == 254)) ||
      ((first == 172 && second >= 16 && second <= 31)) ||
      ((first == 192 && second == 168));
    }; then
      valid=0
    fi
  elif [[ "${value}" == *:* && "${value}" =~ ^[0-9a-fA-F:]+$ ]]; then
    valid=1
    [[ "${value}" != *:::* && "${value}" != "::" ]] || valid=0
    if [[ "${valid}" == 1 ]]; then
      local compressed=0 count=0 part left right left_count right_count gap
      local -a left_parts=() right_parts=() full_parts=()
      [[ "${value}" == *::* ]] && compressed=1
      if [[ "${compressed}" == 0 && ( "${value}" == :* || "${value}" == *: ) ]]; then valid=0; fi
      if [[ "${compressed}" == 1 ]]; then
        left="${value%%::*}"
        right="${value#*::}"
        [[ -z "${left}" ]] || IFS=: read -r -a left_parts <<<"${left}"
        [[ -z "${right}" ]] || IFS=: read -r -a right_parts <<<"${right}"
        left_count="${#left_parts[@]}"
        right_count="${#right_parts[@]}"
        gap=$((8 - left_count - right_count))
        ((gap > 0)) || valid=0
        full_parts+=("${left_parts[@]}")
        for ((count = 0; count < gap; count++)); do full_parts+=(0000); done
        full_parts+=("${right_parts[@]}")
      else
        IFS=: read -r -a full_parts <<<"${value}"
      fi
      count=0
      for part in "${full_parts[@]}"; do
        [[ "${part}" =~ ^[0-9a-fA-F]{1,4}$ ]] || { valid=0; break; }
        count=$((count + 1))
      done
      ((count == 8)) || valid=0
      if [[ "${valid}" == 1 ]]; then
        local group_value all_zero=1 all_but_last_zero=1 first_group last_group mapped_group
        for part in "${full_parts[@]}"; do
          group_value=$((16#${part}))
          ((group_value == 0)) || all_zero=0
        done
        count=0
        while ((count < 7)); do
          group_value=$((16#${full_parts[count]}))
          ((group_value == 0)) || all_but_last_zero=0
          count=$((count + 1))
        done
        first_group=$((16#${full_parts[0]}))
        last_group=$((16#${full_parts[7]}))
        mapped_group=$((16#${full_parts[5]}))
        # Reject ULA, link-local, multicast, unspecified, loopback and
        # IPv4-mapped forms anywhere in the normalized 8-group address.
        ((first_group != 0)) || valid=0
        ((first_group < 0xfc00 || first_group > 0xfdff)) || valid=0
        ((first_group < 0xfe80 || first_group > 0xfebf)) || valid=0
        ((first_group < 0xff00)) || valid=0
        ((all_zero == 0 && (all_but_last_zero == 0 || last_group != 1))) || valid=0
        if ((all_but_last_zero == 1 && mapped_group == 65535)); then valid=0; fi
      fi
    fi
  fi
  if [[ "${valid}" != 1 ]]; then
    echo_content red "${name} must be an IP address"
    exit 1
  fi
}

require_node_name() {
  local value="${TP_NODE_NAME:-}"
  [[ "${value}" =~ ^[A-Za-z0-9._\ -]{1,64}$ ]] || {
    echo_content red "node_name must be 1 to 64 ASCII letters, digits, spaces, dots, underscores, or hyphens"
    exit 1
  }
}

validate_combined_entry_preconditions() {
  require_domain TP_WEB_DOMAIN
  require_domain TP_NODE_DOMAIN
  if [[ "${TP_WEB_DOMAIN,,}" == "${TP_NODE_DOMAIN,,}" ]]; then
    echo_content red "combined deployment requires two different domains (web_hostname and node_hostname)"
    exit 1
  fi
  require_value TP_NODE_NAME
  require_node_name
  require_public_ip TP_NODE_PUBLIC_IP
  require_one_of tls_mode "${TLS_MODE}" acme
  if [[ "${NODE_CADDY_HTTP_PORT}" != 80 || "${NODE_CADDY_HTTPS_PORT}" != 443 ]]; then
    echo_content red "combined shared Entry must own ports 80 and 443"
    exit 1
  fi
  local name port
  for name in UI_PORT PANEL_PORT MARIADB_PORT REDIS_PORT CORE_PORT GRPC_PORT; do
    port="${!name}"
    if [[ "${port}" == 80 || "${port}" == 443 ]]; then
      echo_content red "${name} must not compete with the shared Entry ports 80/443"
      exit 1
    fi
  done
  if [[ -z "${NODE_IDENTITY_CREDENTIAL_FILE}" ]]; then
    NODE_IDENTITY_CREDENTIAL_FILE="${TP_DATA}/trojan-panel/config/node-identities/combined-node.json"
  fi
  local identity_root="${TP_DATA%/}/trojan-panel/config/node-identities"
  [[ "${NODE_IDENTITY_CREDENTIAL_FILE}" == "${identity_root}/"* &&
    "${NODE_IDENTITY_CREDENTIAL_FILE}" != *$'\n'* &&
    "${NODE_IDENTITY_CREDENTIAL_FILE}" != *'/../'* ]] || {
    echo_content red "node_identity_credential_file must be below ${identity_root}"
    exit 1
  }
}

check_combined_host_preconditions() {
  if container_exists "${NODE_CADDY_CONTAINER}"; then
    echo_content red "A standalone Node Entry already exists: ${NODE_CADDY_CONTAINER}; remove it explicitly before combined installation"
    exit 1
  fi
  if container_exists "${WEB_CADDY_CONTAINER}"; then
    local entry_owner
    entry_owner="$(docker inspect -f '{{ index .Config.Labels "io.trojanpanelnext.deployment" }}' "${WEB_CADDY_CONTAINER}" 2>/dev/null || true)"
    if [[ "${entry_owner}" != "${COMBINED_ENTRY_DEPLOYMENT_ID}" ]]; then
      echo_content red "Shared Entry ownership conflict: ${WEB_CADDY_CONTAINER} is not owned by ${COMBINED_ENTRY_DEPLOYMENT_ID}"
      exit 1
    fi
  fi
  combined_owner_prepare || exit 1
  combined_owner_check_data_paths || exit 1
  combined_resource_check || exit 1
  combined_entry_target_guard
  [[ "${TP_SKIP_NETWORK_PRECHECK:-0}" != 1 ]] || return 0
  command -v getent >/dev/null 2>&1 || {
    echo_content red "getent is required to verify combined DNS prerequisites"
    exit 1
  }
  local domain
  for domain in "${TP_WEB_DOMAIN}" "${TP_NODE_DOMAIN}"; do
    if ! getent ahosts "${domain}" 2>/dev/null |
      awk -v expected="${TP_NODE_PUBLIC_IP}" '$1 == expected {found=1} END {exit !found}'; then
      echo_content red "combined DNS prerequisite failed: ${domain} does not resolve to ${TP_NODE_PUBLIC_IP}"
      exit 1
    fi
  done

  # Replays may encounter the installer-owned shared Entry. A running Caddy
  # must account for exactly one listener per shared port; any additional
  # listener is still a hard ownership conflict before containers change.
  command -v ss >/dev/null 2>&1 || {
    echo_content red "ss is required to verify combined port ownership"
    exit 1
  }
  local expected_listeners=0 count port owned_pid=0 listeners line pid
  if container_running "${COMBINED_ENTRY_CONTAINER}"; then
    expected_listeners=1
    owned_pid="$(docker inspect -f '{{.State.Pid}}' "${COMBINED_ENTRY_CONTAINER}" 2>/dev/null || true)"
    [[ "${owned_pid}" =~ ^[1-9][0-9]*$ ]] || {
      echo_content red "combined shared Entry has no verifiable process owner"
      exit 1
    }
  fi
  for port in 80 443; do
    listeners="$(ss -Hlnpt "( sport = :${port} )" 2>/dev/null)" || return 1
    if [[ "${expected_listeners}" == 0 ]]; then
      if [[ -n "${listeners}" ]]; then
        echo_content red "combined shared Entry cannot own 80/443 because another listener is active"
        exit 1
      fi
      continue
    fi
    [[ -n "${listeners}" ]] || { echo_content red "combined shared Entry has no listener on port ${port}"; exit 1; }
    while IFS= read -r line; do
      [[ "${line}" == *"pid=${owned_pid},"* ]] || {
        echo_content red "combined shared Entry has a foreign listener on port ${port}"
        exit 1
      }
      while [[ "${line}" =~ pid=([0-9]+), ]]; do
        pid="${BASH_REMATCH[1]}"
        [[ "${pid}" == "${owned_pid}" ]] || { echo_content red "combined shared Entry has a foreign listener on port ${port}"; exit 1; }
        line="${line#*pid=${pid},}"
      done
    done <<<"${listeners}"
  done
}

combined_entry_target_guard() {
  local state="${COMBINED_ENTRY_STATE_ROOT}/${COMBINED_ENTRY_DEPLOYMENT_ID}.json" old_web old_node roles missing
  [[ -f "${state}" && ! -L "${state}" ]] || return 0
  old_web="$(jq -r '.domains.web // empty' "${state}")" || return 1
  old_node="$(jq -r '.domains.node // empty' "${state}")" || return 1
  if [[ "${old_web}" != "${TP_WEB_DOMAIN}" || "${old_node}" != "${TP_NODE_DOMAIN}" ]]; then
    echo_content red "Same-version combined domain changes require an explicit migration"
    return 1
  fi
  roles="$(jq -r '.committed_target.spec.active_roles // [] | sort | join(",")' "${state}")" || return 1
  if [[ -n "${roles}" && "${roles}" != node,web ]]; then
    [[ "${TP_REQUEST_COMMAND}" == install && -n "${COMBINED_RESTORE_ROLE}" ]] || {
      echo_content red "Restoring a removed combined role requires explicit --restore-role web|node"
      return 1
    }
    [[ "${roles}" == node && "${COMBINED_RESTORE_ROLE}" == web ||
      "${roles}" == web && "${COMBINED_RESTORE_ROLE}" == node ]] || {
      echo_content red "--restore-role must name the inactive combined role"
      return 1
    }
    missing="$(jq -r '.committed_target.digest // empty' "${state}")"
    [[ "${missing}" =~ ^[a-f0-9]{64}$ ]] || {
      echo_content red "Combined journal has no valid committed digest for explicit restore"
      return 1
    }
  elif [[ -n "${COMBINED_RESTORE_ROLE}" ]]; then
    echo_content red "--restore-role is only valid when restoring an inactive combined role"
    return 1
  fi
}

combined_owner_token_generate() {
  COMBINED_OWNER_TOKEN="$(openssl rand -hex 32)"
  [[ "${COMBINED_OWNER_TOKEN}" =~ ^[0-9a-f]{64}$ ]] || {
    echo_content red "Could not create a combined deployment owner token"
    return 1
  }
}

combined_owner_prepare() {
  local root="${COMBINED_ENTRY_ROOT}" marker="${COMBINED_OWNER_MARKER}" token
  [[ "$(realpath -m -- "${root}")" == "${root}" ]] || {
    echo_content red "Combined Entry root contains a symlink or non-canonical component"
    return 1
  }
  if [[ -e "${root}" || -L "${root}" ]]; then
    [[ -d "${root}" && ! -L "${root}" ]] || {
      echo_content red "Combined Entry root is not a safe directory"
      return 1
    }
    if [[ ! -f "${marker}" || -L "${marker}" ]]; then
      # A legacy interrupted first install may have created the reserved root
      # before EntryController wrote its marker. Only an empty, owner-owned
      # root can be recovered without adopting foreign Caddy state.
      [[ "$(find "${root}" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null)" == "" &&
        "$(stat -c %u "${root}")" == "${EUID}" ]] || {
        echo_content red "Combined Entry root has no ownership marker; refusing adoption"
        return 1
      }
      combined_owner_token_generate || return 1
    else
      [[ ! -L "${marker}" ]] || return 1
    fi
    if [[ -f "${marker}" ]]; then
      [[ "$(stat -c %u "${marker}")" == "$(stat -c %u "${TP_DATA}")" && "$(stat -c %a "${marker}")" == 600 ]] || {
      echo_content red "Combined Entry ownership marker must be root-only"
      return 1
      }
      token="$(sed -n 's/^owner_token=//p' "${marker}" | head -n 1)"
      [[ "$(sed -n 's/^deployment=//p' "${marker}" | head -n 1)" == "${COMBINED_ENTRY_DEPLOYMENT_ID}" &&
        "${token}" =~ ^[0-9a-f]{64}$ ]] || {
      echo_content red "Combined Entry ownership marker is invalid"
      return 1
      }
      COMBINED_OWNER_TOKEN="${token}"
    fi
  else
    combined_owner_token_generate || return 1
  fi
}

combined_owner_write_marker() {
  local marker="${COMBINED_OWNER_MARKER}" temporary
  [[ "${COMBINED_OWNER_TOKEN}" =~ ^[0-9a-f]{64}$ ]] || return 1
  [[ "$(realpath -m -- "${COMBINED_ENTRY_ROOT}")" == "${COMBINED_ENTRY_ROOT}" ]] || return 1
  mkdir -p "${COMBINED_ENTRY_ROOT}" || return 1
  temporary="$(mktemp)" || return 1
  printf 'deployment=%s\nowner_token=%s\n' "${COMBINED_ENTRY_DEPLOYMENT_ID}" "${COMBINED_OWNER_TOKEN}" >"${temporary}"
  chmod 0600 "${temporary}" || { rm -f "${temporary}"; return 1; }
  mv -f -- "${temporary}" "${marker}" || { rm -f "${temporary}"; return 1; }
  chmod 0600 "${marker}"
}

combined_owned_paths() {
  printf '%s\n' \
    "${TP_DATA}/custom/web-caddy" \
    "${TP_DATA}/custom/node-caddy" \
    "${TP_DATA}/trojan-panel" \
    "${TP_DATA}/trojan-panel-ui" \
    "${TP_DATA}/trojan-panel-core" \
    "${MANAGED_CERT_DIR}" \
    "${TP_DATA}/mariadb" \
    "${TP_DATA}/redis" \
    "${TP_DATA}/trojanpanelnext-entry" \
    "${INSTALLER_STATE_DIR}" \
    "${NETWORK_PLAN_DIR}" \
    "$(dirname "${NODE_IDENTITY_CREDENTIAL_FILE:-${TP_DATA}/trojan-panel/config/node-identities/combined-node.json}")"
}

COMBINED_NEW_DATA_PATHS=()

combined_owner_check_data_path() {
  local path="$1" marker deployment token
  [[ "$(realpath -m -- "$path")" == "$path" ]] || {
    echo_content red "Combined data path contains a symlink or non-canonical component: ${path}"
    return 1
  }
  [[ -d "$path" && ! -L "$path" ]] || {
    echo_content red "Combined data path is not a safe directory: ${path}"
    return 1
  }
  [[ "$(stat -c %u "$path")" == "${EUID}" ]] || {
    echo_content red "Combined data path is not root-owned: ${path}"
    return 1
  }
  marker="${path}/.trojanpanelnext-owner"
  [[ -f "$marker" && ! -L "$marker" && "$(stat -c %u "$marker")" == "${EUID}" && "$(stat -c %a "$marker")" == 600 ]] || {
    echo_content red "Combined data path has no valid ownership marker: ${path}"
    return 1
  }
  deployment="$(sed -n 's/^deployment=//p' "$marker" | head -n 1)"
  token="$(sed -n 's/^owner_token=//p' "$marker" | head -n 1)"
  [[ "$deployment" == "$COMBINED_ENTRY_DEPLOYMENT_ID" && "$token" == "$COMBINED_OWNER_TOKEN" ]] || {
    echo_content red "Combined data ownership conflict: ${path}"
    return 1
  }
}

combined_owner_check_data_paths() {
  local path
  COMBINED_NEW_DATA_PATHS=()
  while IFS= read -r path; do
    [[ -n "$path" && "$path" != "$COMBINED_ENTRY_ROOT" ]] || continue
    if [[ ! -e "$path" && ! -L "$path" ]]; then
      [[ "$(realpath -m -- "$path")" == "$path" ]] || return 1
      COMBINED_NEW_DATA_PATHS+=("$path")
    else
      combined_owner_check_data_path "$path" || return 1
    fi
  done < <(combined_owned_paths)
}

combined_owner_write_data_markers() {
  local path marker temporary expected is_new
  while IFS= read -r path; do
    [[ -n "${path}" ]] || continue
    [[ "${path}" == "${COMBINED_ENTRY_ROOT}" ]] && continue
    mkdir -p "${path}" || return 1
    marker="${path}/.trojanpanelnext-owner"
    if [[ -e "$marker" || -L "$marker" ]]; then
      combined_owner_check_data_path "$path" || return 1
      continue
    fi
    is_new=0
    for expected in "${COMBINED_NEW_DATA_PATHS[@]}"; do
      [[ "$expected" == "$path" ]] && is_new=1
    done
    [[ "$is_new" == 1 ]] || {
      echo_content red "Combined data path appeared without a preflight ownership decision: ${path}"
      return 1
    }
    temporary="$(mktemp)" || return 1
    printf 'deployment=%s\nowner_token=%s\n' "${COMBINED_ENTRY_DEPLOYMENT_ID}" "${COMBINED_OWNER_TOKEN}" >"${temporary}"
    chmod 0600 "${temporary}" || { rm -f "${temporary}"; return 1; }
    mv -f -- "${temporary}" "${marker}" || { rm -f "${temporary}"; return 1; }
  done < <(combined_owned_paths)
}

combined_owner_purge_preflight() {
  local path marker token deployment
  while IFS= read -r path; do
    [[ -n "${path}" ]] || continue
    [[ ! -e "${path}" && ! -L "${path}" ]] && continue
    [[ -d "${path}" && ! -L "${path}" ]] || {
      echo_content red "Combined purge target is not a safe directory: ${path}"
      return 1
    }
    marker="${path}/.trojanpanelnext-owner"
    [[ -f "${marker}" && ! -L "${marker}" && "$(stat -c %u "${marker}")" == "$(stat -c %u "${TP_DATA}")" && "$(stat -c %a "${marker}")" == 600 ]] || {
      echo_content red "Combined purge target has no root-only ownership marker: ${path}"
      return 1
    }
    deployment="$(sed -n 's/^deployment=//p' "${marker}" | head -n 1)"
    token="$(sed -n 's/^owner_token=//p' "${marker}" | head -n 1)"
    [[ "${deployment}" == "${COMBINED_ENTRY_DEPLOYMENT_ID}" && "${token}" == "${COMBINED_OWNER_TOKEN}" ]] || {
      echo_content red "Combined purge ownership conflict: ${path}"
      return 1
    }
  done < <(combined_owned_paths)
}

combined_container_owner_token() {
  local name="$1"
  docker inspect -f '{{ index .Config.Labels "io.trojanpanelnext.owner-token" }}' "${name}" 2>/dev/null || true
}

require_combined_entry_ownership() {
  container_exists "${COMBINED_ENTRY_CONTAINER}" || return 0
  local owner
  owner="$(docker inspect -f '{{ index .Config.Labels "io.trojanpanelnext.deployment" }}' "${COMBINED_ENTRY_CONTAINER}" 2>/dev/null || true)"
  if [[ "${owner}" != "${COMBINED_ENTRY_DEPLOYMENT_ID}" ]]; then
    echo_content red "Shared Entry ownership conflict: ${COMBINED_ENTRY_CONTAINER} is not owned by ${COMBINED_ENTRY_DEPLOYMENT_ID}"
    return 1
  fi
}

combined_resource_check() {
  local name owner token
  for name in "${COMBINED_ENTRY_CONTAINER}" "${MARIADB_CONTAINER}" "${REDIS_CONTAINER}" "${PANEL_CONTAINER}" "${UI_CONTAINER}" "${CORE_CONTAINER}"; do
    if container_exists "${name}"; then
      owner="$(docker inspect -f '{{ index .Config.Labels "io.trojanpanelnext.deployment" }}' "${name}" 2>/dev/null || true)"
      [[ "${owner}" == "${COMBINED_ENTRY_DEPLOYMENT_ID}" ]] || {
        echo_content red "Combined resource ownership conflict: ${name}"
        return 1
      }
      token="$(combined_container_owner_token "${name}")"
      [[ "${token}" == "${COMBINED_OWNER_TOKEN}" ]] || {
        echo_content red "Combined resource owner token conflict: ${name}"
        return 1
      }
    fi
  done
}

combined_entry_write_spec() {
  local roles="$1" temporary route spec_dir restore_digest=""
  route="${EXTERNAL_ROUTES_DIR}/routes.json"
  spec_dir="$(dirname "${COMBINED_ENTRY_SPEC}")"
  [[ ! -L "${spec_dir}" && ! -L "${COMBINED_ENTRY_SPEC}" ]] || return 1
  install -d -m 0700 "${spec_dir}" || return 1
  chmod 0700 "${spec_dir}" || return 1
  mkdir -p "$(dirname "${route}")"
  if [[ ! -e "${route}" ]]; then printf '{"routes":[]}\n' >"${route}"; chmod 0600 "${route}"; fi
  if [[ -n "${COMBINED_RESTORE_ROLE}" ]]; then
    restore_digest="$(jq -r '.committed_target.digest // empty' "${COMBINED_ENTRY_STATE_ROOT}/${COMBINED_ENTRY_DEPLOYMENT_ID}.json")"
  fi
  temporary="$(mktemp)"
  jq -cn --argjson roles "${roles}" --arg deployment "${COMBINED_ENTRY_DEPLOYMENT_ID}" --arg token "${COMBINED_OWNER_TOKEN}" \
    --arg web "${TP_WEB_DOMAIN}" --arg node "${TP_NODE_DOMAIN}" \
    --arg root "${COMBINED_ENTRY_ROOT}" --arg cert_root "${TP_DATA}/trojanpanelnext-entry/cert" --arg consumer "${MANAGED_CERT_DIR}" \
    --arg route "${route}" --arg upstream "127.0.0.1:${UI_PORT}" --arg restore_role "${COMBINED_RESTORE_ROLE}" --arg restore_digest "${restore_digest}" --argjson revision "${COMBINED_ENTRY_REVISION:-1}" '
    {schema_version:2,topology:"combined",revision:$revision,deployment_id:$deployment,owner_token:$token,provider:"caddy-legacy",
     domains:{web:$web,node:$node},active_roles:($roles|sort),
     roles: ({} + (if ($roles|index("web")) then {web:{web_upstream:$upstream}} else {} end) +
       (if ($roles|index("node")) then {node:{node_exposure:"direct",route_manifest:$route,certificate_consumer:$consumer}} else {} end)),
     certificate_targets: ({} + (if ($roles|index("web")) then {web:{managed_dir:($root+"/data"),cert_path:($cert_root+"/web/fullchain.pem"),key_path:($cert_root+"/web/privkey.pem"),renewal_owner:"caddy-legacy"}} else {} end) +
       (if ($roles|index("node")) then {node:{managed_dir:($root+"/data"),cert_path:($cert_root+"/node/fullchain.pem"),key_path:($cert_root+"/node/privkey.pem"),renewal_owner:"caddy-legacy"}} else {} end))} |
     if ($restore_role | length) > 0 then . + {restore_intent:{roles:[$restore_role],expected_committed_digest:$restore_digest}} else . end' >"${temporary}"
  chmod 0600 "${temporary}"
  mv -f "${temporary}" "${COMBINED_ENTRY_SPEC}"
}

combined_entry_reconcile() {
  local bootstrap="${1:-0}"
  local image="${CADDY_IMAGE}"
  if [[ "${CADDY_ADAPTER_FAKE:-0}" != 1 && "${image}" != *@sha256:* ]]; then
    image="$(docker image inspect -f '{{index .RepoDigests 0}}' "${image}")" || return 1
  fi
    ENTRY_SPEC_OWNER_UID="$(stat -c %u "${COMBINED_ENTRY_SPEC}")" CADDY_ADAPTER_INSTALLER_OWNERSHIP=1 CADDY_ADAPTER_IMAGE="${image}" CADDY_ADAPTER_ROOT="${COMBINED_ENTRY_ROOT}" \
    CADDY_ADAPTER_CONTAINER="${COMBINED_ENTRY_CONTAINER}" CADDY_ADAPTER_WEB_ROOT="${WEB_PATH}" \
    CADDY_ADAPTER_OWNER_TOKEN="${COMBINED_OWNER_TOKEN}" \
    CADDY_ADAPTER_NODE_CONTAINER="${CORE_CONTAINER}" CADDY_ADAPTER_BOOTSTRAP="${bootstrap}" \
    CADDY_ADAPTER_PURGE_RETIRED_ROLES="${COMBINED_PURGE_RETIRED_ROLES:-0}" \
    "${ENTRYCTL_PATH}" reconcile --spec "${COMBINED_ENTRY_SPEC}" --state-root "${COMBINED_ENTRY_STATE_ROOT}"
}

combined_entry_remove() {
  local purge="${1:-0}"
  local image="${CADDY_IMAGE}"
  if [[ "${CADDY_ADAPTER_FAKE:-0}" != 1 && "${image}" != *@sha256:* ]]; then
    image="$(docker image inspect -f '{{index .RepoDigests 0}}' "${image}")" || return 1
  fi
  local -a args=(remove --spec "${COMBINED_ENTRY_SPEC}" --state-root "${COMBINED_ENTRY_STATE_ROOT}")
  [[ "${purge}" != 1 ]] || args+=(--purge)
  ENTRY_SPEC_OWNER_UID="$(stat -c %u "${COMBINED_ENTRY_SPEC}")" CADDY_ADAPTER_INSTALLER_OWNERSHIP=1 CADDY_ADAPTER_IMAGE="${image}" CADDY_ADAPTER_ROOT="${COMBINED_ENTRY_ROOT}" \
    CADDY_ADAPTER_CONTAINER="${COMBINED_ENTRY_CONTAINER}" CADDY_ADAPTER_WEB_ROOT="${WEB_PATH}" \
    CADDY_ADAPTER_OWNER_TOKEN="${COMBINED_OWNER_TOKEN}" \
    CADDY_ADAPTER_NODE_CONTAINER="${CORE_CONTAINER}" \
    "${ENTRYCTL_PATH}" "${args[@]}"
}

validate_config() {
  local mode="$1"

  if [[ -n "${TP_DEPLOYMENT_MODE}" && "${TP_DEPLOYMENT_MODE}" != "${mode}" ]]; then
    local combined_role_remove=0
    if [[ "${TP_REQUEST_COMMAND}" == remove && "${TP_DEPLOYMENT_MODE}" == combined &&
      ( "${mode}" == web || "${mode}" == node ) ]]; then
      combined_role_remove=1
    fi
    if [[ "${combined_role_remove}" != 1 ]]; then
      echo_content red "Configuration deployment mode '${TP_DEPLOYMENT_MODE}' does not match --mode '${mode}'"
      exit 1
    fi
  fi
  require_value TP_DEPLOYMENT_MODE
  local schema_version
  schema_version="$(yaml_read_raw "${TP_CONFIG_READ_FILE}" schema_version)"
  if [[ "${schema_version}" != "1" ]]; then
    echo_content red "trojanpanelnext.schema_version must be 1"
    exit 1
  fi

  if [[ "${INSTALLER_ASSET_VERSION}" != "development" && "${TP_ASSET_VERSION}" != "${INSTALLER_ASSET_VERSION}" ]]; then
    echo_content red "Configuration asset_version '${TP_ASSET_VERSION:-<missing>}' does not match installer assets '${INSTALLER_ASSET_VERSION}'"
    exit 1
  fi

  require_one_of force "${TP_FORCE}" 0 1
  require_one_of purge_data "${TP_PURGE_DATA}" 0 1
  require_one_of tls_mode "${TLS_MODE}" acme external
  if [[ ! "${TP_HEALTH_ATTEMPTS}" =~ ^[1-9][0-9]*$ ]]; then
    echo_content red "TP_HEALTH_ATTEMPTS must be a positive integer"
    exit 1
  fi
  if [[ ! "${TP_HEALTH_DELAY_SECONDS}" =~ ^[0-9]+$ ]]; then
    echo_content red "TP_HEALTH_DELAY_SECONDS must be a non-negative integer"
    exit 1
  fi
  require_port MARIADB_PORT
  require_port REDIS_PORT
  require_bind_address BIND_ADDRESS

  case "${mode}" in
  web)
    require_value TP_WEB_DOMAIN
    require_sysadmin_password
    require_port PANEL_PORT
    require_port UI_PORT
    require_value PANEL_IMAGE
    require_value UI_IMAGE
    ;;
  node)
    require_value TP_NODE_DOMAIN
    require_value MARIADB_HOST
    require_value MARIADB_USER
    if [[ "${MARIADB_USER,,}" == "root" ]]; then
      echo_content red "mariadb_user must be the dedicated user from the Node credential file"
      exit 1
    fi
    require_value MARIADB_PASSWORD
    require_value REDIS_HOST
    require_value REDIS_USERNAME
    if [[ "${REDIS_USERNAME,,}" == "default" ]]; then
      echo_content red "redis_username must be the dedicated cache user from the Node credential file"
      exit 1
    fi
    require_value REDIS_PASSWORD
    require_value REDIS_AUTH_USERNAME
    if [[ "${REDIS_AUTH_USERNAME,,}" == "default" || "${REDIS_AUTH_USERNAME}" == "${REDIS_USERNAME}" ]]; then
      echo_content red "redis_auth_username must be a distinct dedicated read-only user"
      exit 1
    fi
    require_value REDIS_AUTH_PASSWORD
    if [[ ! "${NODE_SERVER_ID}" =~ ^[1-9][0-9]*$ ]]; then
      echo_content red "node_server_id must be a positive integer"
      exit 1
    fi
    if [[ ! "${NODE_IDENTITY_ID}" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$ ]]; then
      echo_content red "node_identity_id must be a UUID from the Node bootstrap bundle"
      exit 1
    fi
    if [[ ! "${NODE_IDENTITY_GENERATION}" =~ ^[1-9][0-9]*$ ]]; then
      echo_content red "node_identity_generation must be a positive integer"
      exit 1
    fi
    if [[ -n "${CONTROL_PLANE_PUBLIC_IP}" ]]; then
      require_public_ip CONTROL_PLANE_PUBLIC_IP
    fi
    require_value CORE_IMAGE
    require_port CORE_PORT
    require_port GRPC_PORT
    require_port NODE_CADDY_HTTP_PORT
    require_port NODE_CADDY_HTTPS_PORT
    require_one_of grpc_tls_mode "${GRPC_TLS_MODE}" mtls
    require_value TP_PKI_BUNDLE_DIR
    if [[ "${TLS_MODE}" == "external" && -z "${TLS_CERT_DIR}" ]]; then
      echo_content red "tls_cert_dir is required when tls_mode is external"
      echo_content yellow "Point it at the directory that holds this node's certificate pair,"
      echo_content yellow "for example /etc/vps-factory/certs/<domain>, and see deploy/installer/EXTERNAL.md"
      exit 1
    fi
    ;;
  combined)
    validate_combined_entry_preconditions
    require_sysadmin_password
    require_port PANEL_PORT
    require_port UI_PORT
    require_port CORE_PORT
    require_port GRPC_PORT
    require_one_of grpc_tls_mode "${GRPC_TLS_MODE}" mtls
    require_value PANEL_IMAGE
    require_value UI_IMAGE
    require_value CORE_IMAGE
    require_value TP_PKI_BUNDLE_DIR
    ;;
  *)
    echo_content red "Unsupported deployment mode: ${mode}"
    exit 1
    ;;
  esac

  if [[ "${BIND_ADDRESS}" == *:* ]]; then
    UI_LISTEN="[${BIND_ADDRESS}]:${UI_PORT}"
  else
    UI_LISTEN="${BIND_ADDRESS}:${UI_PORT}"
  fi
}

install_docker() {
  if command -v docker >/dev/null 2>&1; then
    echo_content skyBlue "---> Docker already installed"
    return
  fi

  echo_content green "---> Install Docker"
  if [[ "${DOCKER_INSTALL_MIRROR:-}" == "aliyun" ]]; then
    sh <(curl -fsSL https://get.docker.com) --mirror Aliyun
  else
    sh <(curl -fsSL https://get.docker.com)
  fi

  if command -v systemctl >/dev/null 2>&1; then
    systemctl enable docker >/dev/null 2>&1 || true
    systemctl restart docker >/dev/null 2>&1 || true
  fi
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

# Recreate only when an installer-owned setting changes. Missing values are
# treated as the legacy default so existing acme deployments keep their
# original no-op upgrade behaviour.
recreate_container_if_env_changed() {
  local name="$1"
  local key="$2"
  local desired="$3"
  local legacy_default="$4"
  if ! container_exists "${name}"; then
    return
  fi

  local current
  current="$(container_env_value "${name}" "${key}" || true)"
  current="${current:-${legacy_default}}"
  if [[ "${current}" == "${desired}" ]]; then
    return
  fi

  echo_content yellow "---> ${key} changed for ${name}: ${current} -> ${desired}; recreate container"
  docker rm -f "${name}" >/dev/null 2>&1
}

write_web_generated_secrets() {
  local file="${TP_CONFIG_FILE:-}"
  if [[ -z "${file}" ]]; then
    return
  fi
  local snapshot="${TP_SECURE_CONFIG_DIR}/config-write.yaml"
  install -m 0600 "${TP_CONFIG_READ_FILE}" "${snapshot}"
  MARIADB_PASSWORD="${MARIADB_PASSWORD}" REDIS_PASSWORD="${REDIS_PASSWORD}" SYSADMIN_PASSWORD="${SYSADMIN_PASSWORD}" \
    yq -i '.trojanpanelnext.mariadb_password = strenv(MARIADB_PASSWORD) | .trojanpanelnext.redis_password = strenv(REDIS_PASSWORD) | .trojanpanelnext.sysadmin_password = strenv(SYSADMIN_PASSWORD)' "${snapshot}"
  "${SECURE_FILE_HELPER}" atomic-write --path "${file}" --input "${snapshot}" \
    --expected "${TP_CONFIG_IDENTITY}" --mode 0600 || {
    echo_content red "Sensitive configuration changed during credential persistence"
    exit 1
  }
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
  SYSADMIN_PASSWORD="${SYSADMIN_PASSWORD:-$(random_sysadmin_password)}"
  export MARIADB_PASSWORD REDIS_PASSWORD SYSADMIN_PASSWORD
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
    "${EXTERNAL_ROUTES_DIR}" \
    "${KERNEL_RUNTIME_PATH}" \
    "${TP_DATA}/custom/web-caddy" \
    "${TP_DATA}/custom/node-caddy" \
    "${MANAGED_CERT_DIR}" \
    "${EXTERNAL_MANAGED_DIR}"
}

# Print every readable certificate file under TLS_CERT_DIR exactly once, walking
# the directory to a bounded depth because ACME tooling often nests certificates
# one directory per hostname. The second argument narrows the match to a file name
# (for example fullchain.pem) so an explicit tls_cert_file wins over the generic
# fallback names. Private-key file names are never treated as certificates.
cert_files_for() {
  local dir="$1"
  local name="${2:-}"
  if [[ ! -d "${dir}" ]]; then
    return
  fi
  if [[ -n "${name}" ]]; then
    if [[ -f "${dir}/${name}" ]]; then
      printf '%s\n' "${dir}/${name}"
    fi
    return
  fi

  local entry
  while IFS= read -r entry; do
    [[ -n "${entry}" ]] && printf '%s\n' "${entry}"
  done < <(cert_files_collect "${dir}" 0)
}

# Print certificate candidates found in one directory and its subdirectories up
# to depth three. Conventional names found directly in the requested directory
# come first; private-key names are never treated as certificates.
cert_files_collect() {
  local dir="$1"
  local depth="$2"
  local entry file base
  local -a current=()

  if [[ "${depth}" == "0" ]]; then
    for file in fullchain.pem cert.pem server.crt server.pem; do
      [[ -f "${dir}/${file}" ]] && current+=("${dir}/${file}")
    done
  fi
  for file in "${dir}"/*.crt; do
    [[ -f "${file}" ]] && current+=("${file}")
  done
  for file in "${dir}"/*.pem; do
    [[ -f "${file}" ]] || continue
    base="$(basename "${file}")"
    case "${base}" in
    privkey.pem | key.pem | *-key.pem | *_key.pem | key-*.pem) continue ;;
    esac
    case "${base}" in
    fullchain.pem | cert.pem) [[ "${depth}" == "0" ]] && continue ;;
    esac
    current+=("${file}")
  done

  # Print each candidate once, in the order collected above.
  local printed=$'\n'
  for entry in "${current[@]}"; do
    if [[ "${printed}" == *$'\n'"${entry}"$'\n'* ]]; then
      continue
    fi
    printed+="${entry}"$'\n'
    printf '%s\n' "${entry}"
  done

  if [[ "${depth}" -ge 3 ]]; then
    return
  fi
  for entry in "${dir}"/*/; do
    [[ -d "${entry}" ]] || continue
    cert_files_collect "${entry%/}" "$((depth + 1))"
  done
}

# Pick the private key for one certificate. A same-stem key wins, so a directory
# holding several hostnames picks the right pair; otherwise the conventional
# privkey.pem/key.pem of the certd layout is used. Prints nothing when several
# ambiguous candidates remain, which makes the caller report the directory as
# ambiguous instead of guessing.
key_for_cert() {
  local certificate="$1"
  local dir stem
  dir="$(dirname "${certificate}")"
  stem="$(basename "${certificate}")"
  stem="${stem%.*}"
  if [[ -f "${dir}/${stem}.key" ]]; then
    printf '%s\n' "${dir}/${stem}.key"
    return
  fi
  local direct=""
  local fallback
  for fallback in privkey.pem key.pem; do
    [[ -f "${dir}/${fallback}" ]] && direct+="${dir}/${fallback}"$'\n'
  done
  local count
  count="$(printf '%s' "${direct}" | grep -c .)"
  if [[ "${count}" == "1" ]]; then
    printf '%s' "${direct}" | grep .
  fi
}

# Print "<cert>|<key>" for the certificate a directory unambiguously names.
# Conventional names win over scanning, because an ACME export directory such as
# /etc/vps-factory/certs/<domain>/ also holds chain.pem, which is an intermediate
# certificate and never the pair to hand to a kernel.
discover_conventional_pair() {
  local dir="$1"
  local cert key
  for cert in "${dir}/fullchain.pem" "${dir}/cert.pem" "${dir}/server.crt" "${dir}/server.pem"; do
    if [[ -f "${cert}" ]]; then
      key="$(key_for_cert "${cert}")"
      if [[ -n "${key}" ]]; then
        printf '%s|%s\n' "${cert}" "${key}"
        return
      fi
    fi
  done
}

# Discover the external TLS certificate and key pair. Prints "<cert>|<key>".
# An explicit tls_cert_file/tls_key_file always wins; after that the directory is
# scanned and every candidate pair is reported, with more than one pair refused.
# Discovery only pairs file names; require_external_cert validates the selected
# X.509 material before validate or install reports success.
discover_external_cert() {
  local dir="${TLS_CERT_DIR}"
  if [[ -z "${dir}" ]]; then
    echo_content red "tls_cert_dir is required when tls_mode is external"
    exit 1
  fi
  if [[ ! -d "${dir}" ]]; then
    echo_content red "tls_cert_dir not found: ${dir}"
    exit 1
  fi

  if [[ -z "${TLS_CERT_FILE}" && -z "${TLS_KEY_FILE}" ]]; then
    local conventional
    conventional="$(discover_conventional_pair "${dir}")"
    if [[ -n "${conventional}" ]]; then
      printf '%s\n' "${conventional}"
      return
    fi
  fi

  local cert key hits=""
  local -a certificates=()
  while IFS= read -r cert; do
    [[ -n "${cert}" ]] && certificates+=("${cert}")
  done < <(cert_files_for "${dir}" "${TLS_CERT_FILE}")
  if [[ "${#certificates[@]}" -eq 0 ]]; then
    echo_content red "No certificate found in ${dir} (tls_cert_file may name a file that does not exist)"
    exit 1
  fi

  local certificate
  for certificate in "${certificates[@]}"; do
    if [[ -n "${TLS_KEY_FILE}" ]]; then
      if [[ "${TLS_KEY_FILE}" == /* ]]; then
        key="${TLS_KEY_FILE}"
      else
        key="$(dirname "${certificate}")/${TLS_KEY_FILE}"
        [[ -f "${key}" ]] || key="${dir}/${TLS_KEY_FILE}"
      fi
    else
      key="$(key_for_cert "${certificate}")"
    fi
    if [[ -n "${key}" && -f "${key}" ]]; then
      hits+="${certificate}|${key}"$'\n'
    fi
  done

  if [[ -z "${hits}" ]]; then
    echo_content red "No certificate/key pair found in ${dir}"
    echo_content yellow "Candidates considered: ${certificates[*]}"
    exit 1
  fi
  if [[ "$(printf '%s' "${hits}" | grep -c .)" -gt 1 ]]; then
    echo_content red "More than one certificate/key pair found in ${dir}"
    printf '%s' "${hits}" | while IFS= read -r line; do
      [[ -n "${line}" ]] && echo_content yellow "  ${line}"
    done
    echo_content yellow "Set tls_cert_file and tls_key_file to select one pair"
    exit 1
  fi
  printf '%s' "${hits}" | grep .
}

# Copy the external certificate into MANAGED_CERT_DIR, which is the only TLS
# material the core container mounts. Each file is prepared under a temporary
# name and renamed into place so readers never observe a partially written PEM.
install_external_cert() {
  local pair="$1"
  local cert="${pair%%|*}"
  local key="${pair##*|}"
  validate_external_cert_pair "${pair}" "${TP_NODE_DOMAIN:-}"

  mkdir -p "${MANAGED_CERT_DIR}"
  chmod 700 "${MANAGED_CERT_DIR}"

  local target_cert="${MANAGED_CERT_DIR}/fullchain.pem"
  local target_key="${MANAGED_CERT_DIR}/privkey.pem"
  local current_cert current_key want_cert want_key
  current_cert="$(realpath -m "${cert}")"
  current_key="$(realpath -m "${key}")"
  want_cert="$(realpath -m "${target_cert}")"
  want_key="$(realpath -m "${target_key}")"
  if [[ "${current_cert}" == "${want_cert}" && "${current_key}" == "${want_key}" ]]; then
    echo_content skyBlue "---> TLS material already managed: ${MANAGED_CERT_DIR}"
    chmod 0600 "${target_key}"
    chmod 0644 "${target_cert}"
    CERT_REFRESH_RESULT=unchanged
    return
  fi
  if [[ -f "${target_cert}" && -f "${target_key}" ]] &&
    cmp -s "${cert}" "${target_cert}" && cmp -s "${key}" "${target_key}"; then
    echo_content skyBlue "---> TLS material unchanged: ${MANAGED_CERT_DIR}"
    chmod 0600 "${target_key}"
    chmod 0644 "${target_cert}"
    CERT_REFRESH_RESULT=unchanged
    return
  fi

  echo_content green "---> Install external TLS material: ${cert} -> ${target_cert}"
  local temporary_cert temporary_key
  temporary_cert="$(mktemp "${MANAGED_CERT_DIR}/.fullchain.pem.XXXXXX")"
  temporary_key="$(mktemp "${MANAGED_CERT_DIR}/.privkey.pem.XXXXXX")"
  if ! install -m 0644 "${cert}" "${temporary_cert}" ||
    ! install -m 0600 "${key}" "${temporary_key}"; then
    rm -f "${temporary_cert}" "${temporary_key}"
    echo_content red "Failed to prepare managed TLS material"
    exit 1
  fi
  mv -f "${temporary_cert}" "${target_cert}"
  mv -f "${temporary_key}" "${target_key}"
  sync -f "${target_cert}" 2>/dev/null || sync 2>/dev/null || true
  sync -f "${target_key}" 2>/dev/null || sync 2>/dev/null || true
  CERT_REFRESH_RESULT=changed
  echo_content skyBlue "---> Kernels read TLS from ${MANAGED_CERT_DIR} (certificate ${target_cert}, key ${target_key})"
}

refresh_node_certificate() {
  if [[ "${TLS_MODE}" != "external" ]]; then
    echo_content red "refresh-cert is only valid for tls_mode: external"
    return 1
  fi
  command -v flock >/dev/null 2>&1 || {
    echo_content red "flock is required for certificate refresh"
    return 1
  }
  mkdir -p "${EXTERNAL_MANAGED_DIR}"
  chmod 0700 "${EXTERNAL_MANAGED_DIR}"
  local lock_file="${EXTERNAL_MANAGED_DIR}/refresh-cert.lock"
  exec 9>"${lock_file}"
  chmod 0600 "${lock_file}"
  if ! flock -n 9; then
    echo_content red "Another certificate refresh is already running"
    return 1
  fi

  local pair
  pair="$(discover_external_cert)"
  install_external_cert "${pair}"
  if [[ "${CERT_REFRESH_RESULT:-unchanged}" == "changed" ]]; then
    if ! container_exists "${CORE_CONTAINER}"; then
      echo_content red "Certificate changed but ${CORE_CONTAINER} does not exist"
      return 1
    fi
    docker restart "${CORE_CONTAINER}" >/dev/null
    wait_for_container "${CORE_CONTAINER}"
    echo_content green "---> Certificate refreshed and ${CORE_CONTAINER} restarted"
  fi
  printf '%s\n' "${CERT_REFRESH_RESULT:-unchanged}"
}

# Certificates written by the Caddy container, used by the default acme mode.
caddy_cert_files() {
  local domain="$1"
  local data_dir="$2"
  local root="${data_dir}/caddy/certificates"
  local certificate key
  for certificate in "${root}"/*/"${domain}"/"${domain}".crt; do
    if [[ -f "${certificate}" ]]; then
      key="${certificate%.crt}.key"
      if [[ -f "${key}" ]]; then
        printf '%s|%s\n' "${certificate}" "${key}"
      fi
    fi
  done
}

remove_caddy_container() {
  local name="$1"
  if ! container_exists "${name}"; then
    return
  fi
  echo_content yellow "---> tls_mode is external: remove the installer-managed reverse proxy container ${name}"
  if ! docker rm -f "${name}" >/dev/null 2>&1; then
    echo_content red "Failed to remove ${name}; external mode cannot continue while the old entry point may still own public ports"
    exit 1
  fi
}

validate_external_cert_pair() {
  local pair="$1"
  local domain="${2:-}"
  local cert="${pair%%|*}"
  local key="${pair##*|}"
  local candidate
  for candidate in "${cert}" "${key}"; do
    if [[ -z "${candidate}" || ! -r "${candidate}" || ! -s "${candidate}" ]]; then
      echo_content red "TLS file is not readable or is empty: ${candidate:-<empty>}"
      exit 1
    fi
  done
  if ! command -v openssl >/dev/null 2>&1; then
    echo_content red "openssl is required to validate external TLS material"
    exit 1
  fi
  if ! openssl x509 -in "${cert}" -noout -checkend 0 >/dev/null 2>&1; then
    echo_content red "TLS certificate is invalid or expired: ${cert}"
    exit 1
  fi
  if ! openssl pkey -in "${key}" -passin pass: -noout </dev/null >/dev/null 2>&1; then
    echo_content red "TLS private key is invalid or encrypted: ${key}"
    exit 1
  fi

  local cert_public key_public
  cert_public="$(openssl x509 -in "${cert}" -pubkey -noout 2>/dev/null |
    openssl pkey -pubin -outform DER 2>/dev/null |
    openssl dgst -sha256 2>/dev/null)"
  key_public="$(openssl pkey -in "${key}" -passin pass: -pubout -outform DER </dev/null 2>/dev/null |
    openssl dgst -sha256 2>/dev/null)"
  if [[ -z "${cert_public}" || "${cert_public}" != "${key_public}" ]]; then
    echo_content red "TLS certificate and private key do not match"
    exit 1
  fi
  if [[ -n "${domain}" ]]; then
    # Some OpenSSL releases print a hostname mismatch but still exit zero, so
    # require the positive result text as well as invoking the public checker.
    local hostname_check
    hostname_check="$(openssl x509 -in "${cert}" -noout -checkhost "${domain}" 2>&1 || true)"
    if [[ "${hostname_check}" != *" does match certificate"* ]]; then
      echo_content red "TLS certificate does not cover node hostname: ${domain}"
      exit 1
    fi
  fi
}

require_external_cert() {
  validate_external_cert_pair "${TLS_CERT_PAIR}" "${TP_NODE_DOMAIN:-${TP_WEB_DOMAIN:-}}"
  echo_content skyBlue "---> External TLS material: ${TLS_CERT_PAIR%%|*} + ${TLS_CERT_PAIR##*|}"
}

warn_if_port_exposed() {
  local label="$1"
  local port="$2"
  command -v ss >/dev/null 2>&1 || return 0
  local exposed
  exposed="$(ss -H -ltn 2>/dev/null | awk -v port=":${port}" '
    $4 ~ port "$" && $4 !~ /^(127\.|\[::1\]:)/ { found = 1 }
    END { print found + 0 }
  ')"
  if [[ "${exposed}" == "1" ]]; then
    echo_content yellow "---> Note: ${label} port ${port} listens on every interface (the application"
    echo_content yellow "    does not support a bind address), so the firewall must keep it closed"
    echo_content yellow "    except to explicitly authorised internal callers"
  fi
}

# The panel and core internal services bind all interfaces in the current
# release, so the machine owner must restrict them at the firewall. Kernel
# protocol listeners are intentionally direct and follow the node configuration.
warn_external_ports() {
  local mode="$1"
  case "${mode}" in
  web)
    warn_if_port_exposed "panel API" "${PANEL_PORT}"
    ;;
  node)
    warn_if_port_exposed "core API" "${CORE_PORT}"
    warn_if_port_exposed "core gRPC" "${GRPC_PORT}"
    ;;
  esac
  if [[ "${BIND_ADDRESS}" == "0.0.0.0" ]]; then
    echo_content yellow "---> Warning: bind_address is 0.0.0.0; set bind_address: 127.0.0.1"
    echo_content yellow "    for a Web deployment unless its ingress runs on another host"
  fi
}

# The installer writes a machine-readable plan for the operator to apply to
# nftables, ufw, or a cloud security group. It never invokes those tools. Keep
# this output deterministic so a same-version replay only changes it when the
# topology or configured addresses change.
json_quote() {
  local value="${1:-}"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  printf '"%s"' "${value}"
}

network_sources_for_host() {
  local host="${1:-}" address
  if [[ -n "${CONTROL_PLANE_PUBLIC_IP:-}" ]]; then
    printf '%s\n' "${CONTROL_PLANE_PUBLIC_IP}"
    return
  fi
  if [[ -n "${host}" ]] && command -v getent >/dev/null 2>&1; then
    getent ahosts "${host}" 2>/dev/null | awk '{print $1}' | awk '!seen[$0]++'
  fi
}

network_sources_for_web_nodes() {
  local identity_dir="${TP_DATA}/trojan-panel/config/node-identities" file address found=0
  if [[ -d "${identity_dir}" ]]; then
    while IFS= read -r file; do
      address="$(sed -n 's/.*"public_ip"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "${file}" | head -n 1)"
      if [[ -n "${address}" ]]; then
        printf '%s\n' "${address}"
        found=1
      fi
    done < <(find "${identity_dir}" -maxdepth 1 -type f -name '*.json' -print 2>/dev/null | sort)
  fi
  if [[ "${found}" == 0 ]]; then
    printf '%s\n' 'operator-must-supply-node-public-ip'
  fi
}

network_json_sources() {
  local source first=1
  printf '['
  while IFS= read -r source; do
    [[ -n "${source}" ]] || continue
    [[ "${first}" == 1 ]] || printf ','
    json_quote "${source}"
    first=0
  done
  printf ']'
}

network_plan_write() {
  local mode="$1" json_file="${NETWORK_PLAN_DIR}/allowlist.json" md_file="${NETWORK_PLAN_DIR}/allowlist.md"
  local public_sources='["0.0.0.0/0","::/0"]' local_sources='["127.0.0.1/32","::1/128"]'
  local peer_sources='[]' peer_label='the Web control plane'
  local tmp_json tmp_md
  [[ ! -L "${NETWORK_PLAN_DIR}" ]] || {
    echo_content red "Network plan directory must not be a symbolic link"
    return 1
  }
  install -d -m 0700 "${NETWORK_PLAN_DIR}" || return 1

  if [[ "${mode}" == web ]]; then
    peer_sources="$(network_json_sources < <(network_sources_for_web_nodes))"
    peer_label='registered Node public IPs'
  elif [[ "${mode}" == node ]]; then
    peer_sources="$(network_json_sources < <(network_sources_for_host "${MARIADB_HOST:-}"))"
    [[ "${peer_sources}" != '[]' ]] || peer_sources='["operator-must-supply-control-plane-public-ip"]'
  else
    peer_sources="${local_sources}"
    peer_label='local combined Web role'
  fi

  tmp_json="$(mktemp "${NETWORK_PLAN_DIR}/.allowlist.json.XXXXXX")"
  tmp_md="$(mktemp "${NETWORK_PLAN_DIR}/.allowlist.md.XXXXXX")"
  chmod 0600 "${tmp_json}" "${tmp_md}"
  {
    printf '{"schema_version":1,"deployment_mode":'
    json_quote "${mode}"
    printf ',"firewall_mutation_by_installer":false,"rules":['
    local comma=''
    if [[ "${mode}" == web || "${mode}" == combined || "${TLS_MODE}" == acme ]]; then
      printf '%s{"name":"web-http","direction":"inbound","protocol":"tcp","port":80,"sources":%s,"purpose":"ACME HTTP-01 and plain HTTP entry"}' "${comma}" "${public_sources}"; comma=,
    fi
    if [[ "${mode}" == web || "${mode}" == combined ]]; then
      printf '%s{"name":"web-https","direction":"inbound","protocol":"tcp","port":443,"sources":%s,"purpose":"HTTPS entry"}' "${comma}" "${public_sources}"; comma=,
    elif [[ "${mode}" == node && "${TLS_MODE}" == acme ]]; then
      printf '%s{"name":"node-https","direction":"inbound","protocol":"tcp","port":%s,"sources":%s,"purpose":"Node Entry HTTPS"}' "${comma}" "${NODE_CADDY_HTTPS_PORT}" "${public_sources}"; comma=,
    fi
    if [[ "${mode}" == node && "${TLS_MODE}" == acme ]]; then
      printf '%s{"name":"node-http","direction":"inbound","protocol":"tcp","port":%s,"sources":%s,"purpose":"Node Entry HTTP and ACME HTTP-01"}' "${comma}" "${NODE_CADDY_HTTP_PORT}" "${public_sources}"; comma=,
    fi
    printf '%s{"name":"mariadb","direction":"inbound","protocol":"tcp","port":%s,"sources":%s,"purpose":"database; allow only %s"}' "${comma}" "${MARIADB_PORT}" "${peer_sources}" "${peer_label}"; comma=,
    printf '%s{"name":"redis","direction":"inbound","protocol":"tcp","port":%s,"sources":%s,"purpose":"Redis ACL; allow only %s"}' "${comma}" "${REDIS_PORT}" "${peer_sources}" "${peer_label}"; comma=,
    if [[ "${mode}" == web ]]; then
      printf '%s{"name":"panel-api","direction":"inbound","protocol":"tcp","port":%s,"sources":%s,"purpose":"panel API; keep private behind the Entry"}' "${comma}" "${PANEL_PORT}" "${local_sources}"; comma=,
      printf '%s{"name":"panel-ui","direction":"inbound","protocol":"tcp","port":%s,"sources":%s,"purpose":"panel UI; keep private behind the Entry"}' "${comma}" "${UI_PORT}" "${local_sources}"; comma=,
    elif [[ "${mode}" == node ]]; then
      printf '%s{"name":"core-api","direction":"inbound","protocol":"tcp","port":%s,"sources":%s,"purpose":"Core API; allow only %s"}' "${comma}" "${CORE_PORT}" "${peer_sources}" "${peer_label}"; comma=,
      printf '%s{"name":"core-grpc","direction":"inbound","protocol":"tcp","port":%s,"sources":%s,"purpose":"control-plane gRPC; allow only %s"}' "${comma}" "${GRPC_PORT}" "${peer_sources}" "${peer_label}"; comma=,
    else
      printf '%s{"name":"panel-api","direction":"inbound","protocol":"tcp","port":%s,"sources":%s,"purpose":"combined local panel API"}' "${comma}" "${PANEL_PORT}" "${local_sources}"; comma=,
      printf '%s{"name":"panel-ui","direction":"inbound","protocol":"tcp","port":%s,"sources":%s,"purpose":"combined local panel UI"}' "${comma}" "${UI_PORT}" "${local_sources}"; comma=,
      printf '%s{"name":"core-api","direction":"inbound","protocol":"tcp","port":%s,"sources":%s,"purpose":"combined local Core API"}' "${comma}" "${CORE_PORT}" "${local_sources}"; comma=,
      printf '%s{"name":"core-grpc","direction":"inbound","protocol":"tcp","port":%s,"sources":%s,"purpose":"combined local control-plane gRPC"}' "${comma}" "${GRPC_PORT}" "${local_sources}"; comma=,
    fi
    printf '],"notes":["All listed rules are inbound host rules; apply them outside the installer.","Do not expose MariaDB, Redis, panel API, panel UI, Core API, or gRPC to 0.0.0.0/0.","Node protocol ports are direct kernel listeners; inspect the generated routes.json and add only declared ports."],"egress":[{"name":"dns","protocol":"udp/tcp","port":53,"destinations":["configured DNS resolvers"],"purpose":"name resolution"},{"name":"https","protocol":"tcp","port":443,"destinations":["configured registries and ACME endpoints"],"purpose":"image, release, and certificate retrieval"}]}\n'
  } >"${tmp_json}"
  if [[ "${mode}" == node || "${mode}" == combined ]] && [[ -s "${EXTERNAL_ROUTES_DIR}/routes.json" ]] && jq -e '.routes | type == "array"' "${EXTERNAL_ROUTES_DIR}/routes.json" >/dev/null 2>&1; then
    local route_tmp
    route_tmp="$(mktemp "${NETWORK_PLAN_DIR}/.allowlist-route.XXXXXX")"
    jq --slurpfile routes "${EXTERNAL_ROUTES_DIR}/routes.json" '
      .rules += ([($routes[0].routes[]? // empty) as $route |
        {name:("node-protocol-" + ($route.kernel // "kernel") + "-" + (($route.port // 0)|tostring)),
         direction:"inbound", protocol:($route.network // "tcp"), port:($route.port // 0),
         sources:["0.0.0.0/0","::/0"], purpose:"Node direct kernel listener"}])
    ' "${tmp_json}" >"${route_tmp}" && mv -f -- "${route_tmp}" "${tmp_json}"
  fi
  mv -f -- "${tmp_json}" "${json_file}"

  {
    printf '# Network allowlist plan (%s)\n\n' "${mode}"
    printf 'Generated by TrojanPanel Next. The installer does not modify nftables, ufw, or cloud security groups. Apply and audit these rules with the host or cloud operator.\n\n'
    printf '| Service | Port | Allowed sources | Purpose |\n| --- | ---: | --- | --- |\n'
    tr -d '\n' <"${json_file}" | sed 's/},{"name"/}\n{"name"/g' |
      sed -n 's/.*"name":"\([^\"]*\)".*"port":\([0-9]*\).*"sources":\(\[[^]]*\]\).*"purpose":"\([^\"]*\)".*/| \1 | \2 | \3 | \4 |/p'
    printf '\nThe Node direct listener list is read from `%s/routes.json`; only routes marked for public exposure should be opened.\n' "${EXTERNAL_ROUTES_DIR}"
  } >"${tmp_md}"
  mv -f -- "${tmp_md}" "${md_file}"
  chmod 0600 "${json_file}" "${md_file}"
  echo_content skyBlue "---> Network allowlist written to ${json_file}"
  echo_content yellow "    The installer does not change host firewalls or cloud security groups; apply this plan manually."
}

installer_state_path_for() {
  printf '%s/%s.state\n' "${INSTALLER_STATE_DIR}" "$1"
}

installer_state_value() {
  local key="$1" file="$2"
  sed -n "s/^${key}=//p" "${file}" | head -n 1
}

check_same_version_replay_preconditions() {
  local mode="$1" state expected actual key
  state="$(installer_state_path_for "${mode}")"
  INSTALLER_STATE_FILE="${state}"
  [[ ! -e "${state}" && ! -L "${state}" ]] && return 0
  [[ -f "${state}" && ! -L "${state}" ]] || {
    echo_content red "Installer state is not a safe regular file: ${state}"
    return 1
  }
  [[ "$(stat -c %u "${state}")" == "${EUID}" && "$(stat -c %a "${state}")" == 600 ]] || {
    echo_content red "Installer state must be owned by root and mode 0600: ${state}"
    return 1
  }
  [[ "$(installer_state_value schema_version "${state}")" == 1 ]] || {
    echo_content red "Unsupported installer state schema; explicit migration is required"
    return 1
  }
  [[ "$(installer_state_value mode "${state}")" == "${mode}" ]] || {
    echo_content red "Installer state deployment mode changed; explicit migration is required"
    return 1
  }
  for key in asset_version tp_data; do
    case "${key}" in
    asset_version) expected="${TP_ASSET_VERSION:-development}" ;;
    tp_data) expected="${TP_DATA}" ;;
    esac
    actual="$(installer_state_value "${key}" "${state}")"
    [[ "${actual}" == "${expected}" ]] || {
      echo_content red "Same-version ${mode} replay rejected immutable ${key} change; explicit migration is required"
      return 1
    }
  done
  case "${mode}" in
  web)
    expected="${TP_WEB_DOMAIN}"; key=domain ;;
  node)
    expected="${TP_NODE_DOMAIN}"; key=domain ;;
  combined)
    expected="${TP_WEB_DOMAIN}"
    actual="$(installer_state_value web_domain "${state}")"
    [[ "${actual}" == "${expected}" ]] || {
      echo_content red "Same-version combined Web domain changes require an explicit migration"
      return 1
    }
    expected="${TP_NODE_DOMAIN}"
    actual="$(installer_state_value node_domain "${state}")"
    [[ "${actual}" == "${expected}" ]] || {
      echo_content red "Same-version combined Node domain changes require an explicit migration"
      return 1
    }
    ;;
  esac
  if [[ "${mode}" != combined ]]; then
    actual="$(installer_state_value "${key}" "${state}")"
    [[ "${actual}" == "${expected}" ]] || {
      echo_content red "Same-version ${mode} domain changes require an explicit migration"
      return 1
    }
  fi
  local path_key path_expected
  for path_key in web_path pki_bundle_dir managed_cert_dir external_routes_dir kernel_runtime_path node_identity_credential_file; do
    case "${path_key}" in
    web_path) path_expected="${WEB_PATH}" ;;
    pki_bundle_dir) path_expected="${TP_PKI_BUNDLE_DIR}" ;;
    managed_cert_dir) path_expected="${MANAGED_CERT_DIR}" ;;
    external_routes_dir) path_expected="${EXTERNAL_ROUTES_DIR}" ;;
    kernel_runtime_path) path_expected="${KERNEL_RUNTIME_PATH}" ;;
    node_identity_credential_file) path_expected="${NODE_IDENTITY_CREDENTIAL_FILE}" ;;
    esac
    [[ -z "${path_expected}" ]] && continue
    actual="$(installer_state_value "${path_key}" "${state}")"
    [[ "${actual}" == "${path_expected}" ]] || {
      echo_content red "Same-version ${mode} data path changes require an explicit migration"
      return 1
    }
  done
  if [[ "${mode}" == node ]]; then
    for key in node_identity_id node_server_id; do
      expected="${!key}"
      actual="$(installer_state_value "${key}" "${state}")"
      [[ "${actual}" == "${expected}" ]] || {
        echo_content red "Same-version Node identity changes require explicit revoke and registration"
        return 1
      }
    done
  fi
}

write_installer_state() {
  local mode="$1" state temporary
  state="${INSTALLER_STATE_FILE:-$(installer_state_path_for "${mode}")}"
  install -d -m 0700 "${INSTALLER_STATE_DIR}" || return 1
  [[ ! -L "${INSTALLER_STATE_DIR}" && ! -L "${state}" ]] || {
    echo_content red "Installer state path must not be a symbolic link"
    return 1
  }
  temporary="$(mktemp "${INSTALLER_STATE_DIR}/.${mode}.state.XXXXXX")"
  chmod 0600 "${temporary}"
  {
    printf 'schema_version=1\nmode=%s\nasset_version=%s\ntp_data=%s\n' \
      "${mode}" "${TP_ASSET_VERSION:-development}" "${TP_DATA}"
    printf 'web_path=%s\npki_bundle_dir=%s\nmanaged_cert_dir=%s\nexternal_routes_dir=%s\nkernel_runtime_path=%s\nnode_identity_credential_file=%s\n' \
      "${WEB_PATH}" "${TP_PKI_BUNDLE_DIR}" "${MANAGED_CERT_DIR}" "${EXTERNAL_ROUTES_DIR}" \
      "${KERNEL_RUNTIME_PATH}" "${NODE_IDENTITY_CREDENTIAL_FILE}"
    case "${mode}" in
    web) printf 'domain=%s\n' "${TP_WEB_DOMAIN}" ;;
    node) printf 'domain=%s\nnode_identity_id=%s\nnode_server_id=%s\n' "${TP_NODE_DOMAIN}" "${NODE_IDENTITY_ID}" "${NODE_SERVER_ID}" ;;
    combined) printf 'web_domain=%s\nnode_domain=%s\nnode_identity_id=%s\n' "${TP_WEB_DOMAIN}" "${TP_NODE_DOMAIN}" "${NODE_IDENTITY_ID}" ;;
    esac
  } >"${temporary}"
  mv -f -- "${temporary}" "${state}"
  chmod 0600 "${state}"
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
  local mode="$1"
  case "${mode}" in
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
  combined)
    generate_web_client_pki
    mkdir -p "$(dirname "${GRPC_CLIENT_CERT_PATH}")" "$(dirname "${GRPC_CLIENT_KEY_PATH}")"
    install -m 0644 "${TP_PKI_BUNDLE_DIR}/client.crt" "${GRPC_CLIENT_CERT_PATH}"
    install -m 0600 "${TP_PKI_BUNDLE_DIR}/client.key" "${GRPC_CLIENT_KEY_PATH}"
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
  local temporary="${TP_SECURE_CONFIG_DIR}/panel-config.ini"
  cat >"${temporary}" <<EOF
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
# host is reserved: the control plane binds every interface even when this key
# is present, so external mode relies on an explicit firewall policy.
host=${BIND_ADDRESS}
[grpc]
client_cert_path=${GRPC_CLIENT_CERT_PATH}
client_key_path=${GRPC_CLIENT_KEY_PATH}
server_ca_path=${GRPC_SERVER_CA_PATH}
EOF
  chmod 0600 "${temporary}"
  "${SECURE_FILE_HELPER}" atomic-write \
    --path "${TP_DATA}/trojan-panel/config/config.ini" --input "${temporary}" \
    --mode 0600 --create-parents
}

write_initial_sysadmin_password_file() {
  local temporary="${TP_SECURE_CONFIG_DIR}/initial-admin-password"
  : >"${temporary}"
  chmod 0600 "${temporary}"
  printf '%s\n' "${SYSADMIN_PASSWORD}" >"${temporary}"
  "${SECURE_FILE_HELPER}" atomic-write \
    --path "${INITIAL_SYSADMIN_PASSWORD_FILE}" --input "${temporary}" \
    --mode 0600 --create-parents
}

write_core_runtime_config() {
  local crt_path="$1"
  local key_path="$2"
  ensure_secure_file_helper
  local temporary="${TP_SECURE_CONFIG_DIR}/core-config.ini"

  cat >"${temporary}" <<EOF
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
username=${REDIS_USERNAME}
password=${REDIS_PASSWORD}
auth_username=${REDIS_AUTH_USERNAME}
auth_password=${REDIS_AUTH_PASSWORD}
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
# host is reserved: the core binds every interface even when this key is
# present, so external mode relies on an explicit firewall policy.
host=${BIND_ADDRESS}
[node]
server_id=${NODE_SERVER_ID}
domain=${TP_NODE_DOMAIN}
identity_id=${NODE_IDENTITY_ID}
identity_generation=${NODE_IDENTITY_GENERATION}
bootstrap_challenge=${NODE_BOOTSTRAP_CHALLENGE}
EOF
  chmod 0600 "${temporary}"
  "${SECURE_FILE_HELPER}" atomic-write \
    --path "${TP_DATA}/trojan-panel-core/config/config.ini" --input "${temporary}" \
    --mode 0600 --create-parents
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

write_combined_caddyfile() {
  local caddyfile="${TP_DATA}/custom/web-caddy/Caddyfile"
  if [[ -n "${TP_EMAIL:-}" ]]; then
    cat >"${caddyfile}" <<EOF
{
    email ${TP_EMAIL}
}

${TP_WEB_DOMAIN} {
    reverse_proxy 127.0.0.1:${UI_PORT}
}

${TP_NODE_DOMAIN} {
    root * /srv
    file_server
}
EOF
  else
    cat >"${caddyfile}" <<EOF
${TP_WEB_DOMAIN} {
    reverse_proxy 127.0.0.1:${UI_PORT}
}

${TP_NODE_DOMAIN} {
    root * /srv
    file_server
}
EOF
  fi
}

write_combined_node_only_caddyfile() {
  local caddyfile="${TP_DATA}/custom/web-caddy/Caddyfile"
  cat >"${caddyfile}" <<EOF
${TP_NODE_DOMAIN} {
    root * /srv
    file_server
}
EOF
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
  local deployment_id="${5:-}"

  if [[ -n "${deployment_id}" ]] && container_exists "${name}"; then
    local current_owner
    current_owner="$(container_env_value "${name}" TP_ENTRY_DEPLOYMENT_ID || true)"
    [[ "${current_owner}" == "${deployment_id}" ]] || {
      echo_content red "Entry container ownership conflict: ${name}"
      return 1
    }
  fi
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
  local -a ownership_env=()
  [[ -z "${deployment_id}" ]] || ownership_env=(-e "TP_ENTRY_DEPLOYMENT_ID=${deployment_id}")
  docker run -d --name "${name}" --restart always \
    --network=host \
    "${ownership_env[@]}" \
    -v "${config_dir}/Caddyfile:/etc/caddy/Caddyfile" \
    -v "${data_dir}:/data" \
    -v "${config_dir}:/config" \
    -v "${web_dir}:/srv" \
    "${CADDY_IMAGE}"
}

wait_for_cert() {
  local domain="$1"
  local data_dir="$2"
  local pair
  local cert_file
  local key_file

  echo_content green "---> Wait for certificate: ${domain}"
  for _ in $(seq 1 60); do
    pair="$(caddy_cert_files "${domain}" "${data_dir}" | head -n 1)"
    if [[ -n "${pair}" ]]; then
      cert_file="${pair%%|*}"
      key_file="${pair##*|}"
      if [[ -s "${cert_file}" && -s "${key_file}" ]]; then
        if openssl x509 -in "${cert_file}" -noout -checkend 0 >/dev/null 2>&1; then
          echo_content skyBlue "---> Certificate ready: ${cert_file}"
          return
        fi
        echo_content yellow "---> Certificate for ${domain} is already expired, waiting for renewal"
      fi
    fi
    sleep 3
  done

  echo_content red "---> Certificate for ${domain} is not ready."
  echo_content red "    Check DNS, firewall, and Caddy logs. If another process owns port 80,"
  echo_content red "    switch the configuration to tls_mode: external instead."
  exit 1
}

wait_for_combined_certs() {
  local data_dir="$1"
  wait_for_cert "${TP_WEB_DOMAIN}" "${data_dir}"
  wait_for_cert "${TP_NODE_DOMAIN}" "${data_dir}"
  validate_combined_certificate_domains "${data_dir}"
}

validate_combined_certificate_domains() {
  local role domain certificate hostname_check
  [[ -f "${COMBINED_ENTRY_SPEC}" ]] || return 1
  while IFS= read -r role; do
    domain="$(jq -r --arg role "${role}" '.domains[$role]' "${COMBINED_ENTRY_SPEC}")"
    certificate="$(jq -r --arg role "${role}" '.certificate_targets[$role].cert_path' "${COMBINED_ENTRY_SPEC}")"
    [[ -s "${certificate}" ]] || {
      echo_content red "Shared Entry certificate is missing for ${domain}"
      return 1
    }
    hostname_check="$(openssl x509 -in "${certificate}" -noout -checkhost "${domain}" 2>&1 || true)"
    if [[ "${hostname_check}" != *" does match certificate"* ]]; then
      echo_content red "Shared Entry certificate does not cover ${domain}"
      return 1
    fi
  done < <(jq -r '.active_roles[]' "${COMBINED_ENTRY_SPEC}")
}

combined_node_cert_sha256() {
  [[ -s "${MANAGED_CERT_DIR}/fullchain.pem" && -s "${MANAGED_CERT_DIR}/privkey.pem" ]] || return 1
  sha256sum "${MANAGED_CERT_DIR}/fullchain.pem" "${MANAGED_CERT_DIR}/privkey.pem" | sha256sum | awk '{print $1}'
}

record_combined_node_cert_generation() {
  local state_dir="${TP_DATA}/trojanpanelnext-entry"
  local fingerprint="$1"
  mkdir -p "${state_dir}"
  chmod 0700 "${state_dir}"
  printf '%s\n' "${fingerprint}" >"${state_dir}/combined-node-cert.sha256"
  chmod 0600 "${state_dir}/combined-node-cert.sha256"
}

reconcile_combined_node_cert_consumer() {
  local fingerprint previous=""
  fingerprint="$(combined_node_cert_sha256)" || return 1
  local state_file="${TP_DATA}/trojanpanelnext-entry/combined-node-cert.sha256"
  [[ ! -r "${state_file}" ]] || previous="$(<"${state_file}")"
  if [[ "${fingerprint}" != "${previous}" ]]; then
    docker restart "${CORE_CONTAINER}" >/dev/null
    wait_for_container "${CORE_CONTAINER}"
    echo_content green "---> Reconciled renewed combined Node certificate generation"
  fi
  record_combined_node_cert_generation "${fingerprint}"
}

refresh_combined_certificate() {
  local data_dir="${TP_DATA}/custom/web-caddy/data"
  validate_combined_certificate_domains "${data_dir}"
  local fingerprint previous=""
  fingerprint="$(combined_node_cert_sha256)" || {
    echo_content red "Combined Node certificate material is unavailable"
    return 1
  }
  local state_file="${TP_DATA}/trojanpanelnext-entry/combined-node-cert.sha256"
  [[ ! -r "${state_file}" ]] || previous="$(<"${state_file}")"
  if [[ "${fingerprint}" == "${previous}" ]]; then
    printf 'unchanged\n'
    return 0
  fi
  container_exists "${CORE_CONTAINER}" || {
    echo_content red "Certificate changed but ${CORE_CONTAINER} does not exist"
    return 1
  }
  docker restart "${CORE_CONTAINER}" >/dev/null
  wait_for_container "${CORE_CONTAINER}"
  record_combined_node_cert_generation "${fingerprint}"
  echo_content green "---> Combined Node certificate generation refreshed"
  printf 'changed\n'
}

wait_for_container() {
  local name="$1"
  for _ in $(seq 1 "${TP_CONTAINER_ATTEMPTS}"); do
    if container_running "${name}"; then
      return
    fi
    sleep "${TP_CONTAINER_DELAY_SECONDS}"
  done
  echo_content red "---> ${name} is not running"
  docker logs "${name}" 2>/dev/null || true
  exit 1
}

wait_for_mariadb() {
  for _ in $(seq 1 60); do
    if mariadb_query_with_configured_credential 'select 1'; then
      return
    fi
    sleep 2
  done
  echo_content red "---> MariaDB is not ready"
  docker logs "${MARIADB_CONTAINER}" 2>/dev/null || true
  exit 1
}

mariadb_query_with_configured_credential() {
  local query="$1"
  printf '%s\n%s\n' "${MARIADB_PASSWORD}" "${query}" |
    docker exec -i "${MARIADB_CONTAINER}" sh -c '
      credential_file="$(mktemp)" || exit 1
      trap '\''rm -f -- "$credential_file"'\'' EXIT
      chmod 0600 "$credential_file" || exit 1
      IFS= read -r password || exit 1
      IFS= read -r query || exit 1
      printf "[client]\npassword=%s\n" "$password" >"$credential_file" || exit 1
      mariadb --defaults-extra-file="$credential_file" -uroot -e "$query" >/dev/null 2>&1 ||
        mysql --defaults-extra-file="$credential_file" -uroot -e "$query" >/dev/null 2>&1
    '
}

create_database() {
  mariadb_query_with_configured_credential \
    "create database if not exists ${MARIADB_DATABASE} default character set utf8mb4;"
}

write_ui_nginx_config() {
  local listen="${UI_LISTEN:-${BIND_ADDRESS}:${UI_PORT}}"
  cat >"${TP_DATA}/trojan-panel-ui/nginx/default.conf" <<EOF
server {
    listen       ${listen};
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
  local -a combined_label=()
  [[ "${TP_DEPLOYMENT_MODE}" != combined ]] || combined_label=(--label "io.trojanpanelnext.deployment=${COMBINED_ENTRY_DEPLOYMENT_ID}" --label "io.trojanpanelnext.owner-token=${COMBINED_OWNER_TOKEN}")
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
  docker run -d --name "${MARIADB_CONTAINER}" --restart always "${combined_label[@]}" \
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
  local -a combined_label=()
  [[ "${TP_DEPLOYMENT_MODE}" != combined ]] || combined_label=(--label "io.trojanpanelnext.deployment=${COMBINED_ENTRY_DEPLOYMENT_ID}" --label "io.trojanpanelnext.owner-token=${COMBINED_OWNER_TOKEN}")
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
  docker run -d --name "${REDIS_CONTAINER}" --restart always "${combined_label[@]}" \
    --network=host \
    -v "${TP_DATA}/redis/data:/data" \
    "${REDIS_IMAGE}" redis-server --requirepass "${REDIS_PASSWORD}" --port "${REDIS_PORT}"
  wait_for_container "${REDIS_CONTAINER}"
}

deploy_panel_backend() {
  local -a combined_label=()
  [[ "${TP_DEPLOYMENT_MODE}" != combined ]] || combined_label=(--label "io.trojanpanelnext.deployment=${COMBINED_ENTRY_DEPLOYMENT_ID}" --label "io.trojanpanelnext.owner-token=${COMBINED_OWNER_TOKEN}")
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
  docker run -d --name "${PANEL_CONTAINER}" --restart always "${combined_label[@]}" \
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
    -e "TP_INITIAL_SYSADMIN_PASSWORD_FILE=${INITIAL_SYSADMIN_PASSWORD_FILE}" \
    "${PANEL_IMAGE}"
  wait_for_container "${PANEL_CONTAINER}"
}

deploy_panel_ui() {
  local -a combined_label=()
  [[ "${TP_DEPLOYMENT_MODE}" != combined ]] || combined_label=(--label "io.trojanpanelnext.deployment=${COMBINED_ENTRY_DEPLOYMENT_ID}" --label "io.trojanpanelnext.owner-token=${COMBINED_OWNER_TOKEN}")
  remove_container_if_force "${UI_CONTAINER}"
  write_ui_nginx_config
  recreate_container_if_env_changed "${UI_CONTAINER}" TP_BIND_ADDRESS "${BIND_ADDRESS}" 0.0.0.0
  if container_running "${UI_CONTAINER}"; then
    echo_content skyBlue "---> Trojan Panel UI already running"
    return
  fi
  if container_exists "${UI_CONTAINER}"; then
    docker start "${UI_CONTAINER}" >/dev/null
    return
  fi

  ensure_image "${UI_IMAGE}"
  docker run -d --name "${UI_CONTAINER}" --restart always "${combined_label[@]}" \
    --network=host \
    -e "TP_BIND_ADDRESS=${BIND_ADDRESS}" \
    -v "${TP_DATA}/trojan-panel-ui/nginx/default.conf:/etc/nginx/conf.d/default.conf" \
    "${UI_IMAGE}"
  wait_for_container "${UI_CONTAINER}"
}

prepare_combined_node_identity() {
  command -v jq >/dev/null 2>&1 || {
    echo_content red "jq is required to provision the combined Node identity"
    exit 1
  }
  local credential_file="${NODE_IDENTITY_CREDENTIAL_FILE}"
  mkdir -p "$(dirname "${credential_file}")"
  chmod 0700 "$(dirname "${credential_file}")"

  # The API container and host deliberately share this restricted path. The
  # CLI is idempotent for the same name/domain and replays the committed file
  # on a same-version rerun without minting a second identity.
  if ! docker exec "${PANEL_CONTAINER}" /tpdata/trojan-panel/trojan-panel \
    node-identity register \
    --name "${TP_NODE_NAME}" \
    --domain "${TP_NODE_DOMAIN}" \
    --public-ip "${TP_NODE_PUBLIC_IP}" \
    --credential-file "${credential_file}"; then
    echo_content red "Unable to provision the combined Node identity"
    exit 1
  fi
  [[ -r "${credential_file}" && ! -L "${credential_file}" ]] || {
    echo_content red "Combined Node identity did not produce a safe credential file"
    exit 1
  }

  NODE_IDENTITY_ID="$(jq -r '.node_identity_id // empty' "${credential_file}")"
  NODE_SERVER_ID="$(jq -r '.node_server_id // empty' "${credential_file}")"
  NODE_IDENTITY_GENERATION="$(jq -r '.generation // empty' "${credential_file}")"
  MARIADB_USER="$(jq -r '.mariadb.username // empty' "${credential_file}")"
  MARIADB_PASSWORD="$(jq -r '.mariadb.password // empty' "${credential_file}")"
  REDIS_USERNAME="$(jq -r '.redis.username // empty' "${credential_file}")"
  REDIS_PASSWORD="$(jq -r '.redis.password // empty' "${credential_file}")"
  REDIS_AUTH_USERNAME="$(jq -r '.redis_auth.username // empty' "${credential_file}")"
  REDIS_AUTH_PASSWORD="$(jq -r '.redis_auth.password // empty' "${credential_file}")"
  MARIADB_HOST=127.0.0.1
  REDIS_HOST=127.0.0.1
  export NODE_IDENTITY_ID NODE_SERVER_ID NODE_IDENTITY_GENERATION \
    MARIADB_USER MARIADB_PASSWORD REDIS_USERNAME REDIS_PASSWORD \
    REDIS_AUTH_USERNAME REDIS_AUTH_PASSWORD MARIADB_HOST REDIS_HOST

  [[ -n "${NODE_IDENTITY_ID}" && -n "${MARIADB_USER}" && -n "${REDIS_USERNAME}" &&
    -n "${REDIS_AUTH_USERNAME}" ]] || {
    echo_content red "Combined Node credential file is incomplete"
    exit 1
  }
}

revoke_combined_node_identity() {
  local credential_file="${NODE_IDENTITY_CREDENTIAL_FILE}"
  if [[ ! -e "${credential_file}" ]] && ! container_exists "${CORE_CONTAINER}"; then
    echo_content skyBlue "---> Combined Node identity is already absent"
    return 0
  fi
  [[ -r "${credential_file}" && ! -L "${credential_file}" ]] || {
    echo_content red "Cannot remove the combined Node role without its owned identity record: ${credential_file}"
    return 1
  }
  local identity_id
  identity_id="$(jq -r '.node_identity_id // empty' "${credential_file}")"
  [[ "${identity_id}" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$ ]] || {
    echo_content red "Combined Node identity record is invalid"
    return 1
  }
  local status=0
  if container_running "${PANEL_CONTAINER}"; then
    docker exec "${PANEL_CONTAINER}" /tpdata/trojan-panel/trojan-panel \
      node-identity revoke --id "${identity_id}" || status=$?
  else
    # Web role removal leaves the shared database and Redis running. Use the
    # same control-plane binary for a one-shot revocation without restoring
    # its public service or taking ownership of the Node role.
    docker run --rm --network=host \
      --entrypoint /tpdata/trojan-panel/trojan-panel \
      -w /tpdata/trojan-panel \
      -v "${TP_DATA}/trojan-panel/config:/tpdata/trojan-panel/config" \
      "${PANEL_IMAGE}" node-identity revoke --id "${identity_id}" || status=$?
  fi
  if [[ "${status}" != 0 ]]; then
    echo_content red "Combined Node identity revocation failed; local Node resources were retained"
    return 1
  fi
  echo_content skyBlue "---> Combined Node identity revoked: ${identity_id}"
}

probe_mariadb_health() {
  if [[ "${TP_DEPLOYMENT_MODE}" == combined && -n "${WEB_MARIADB_PASSWORD}" ]]; then
    local previous_user="${MARIADB_USER}" previous_password="${MARIADB_PASSWORD}"
    MARIADB_USER="${WEB_MARIADB_USER}"
    MARIADB_PASSWORD="${WEB_MARIADB_PASSWORD}"
    local status=0
    mariadb_query_with_configured_credential 'select 1' || status=$?
    MARIADB_USER="${previous_user}"
    MARIADB_PASSWORD="${previous_password}"
    return "${status}"
  fi
  mariadb_query_with_configured_credential 'select 1'
}

probe_redis_health() {
  local redis_password="${REDIS_PASSWORD}"
  if [[ "${TP_DEPLOYMENT_MODE}" == combined && -n "${WEB_REDIS_PASSWORD}" ]]; then
    redis_password="${WEB_REDIS_PASSWORD}"
  fi
  local response
  response="$(printf 'AUTH %s\r\nPING\r\n' "${redis_password}" |
    docker exec -i "${REDIS_CONTAINER}" redis-cli -p "${REDIS_PORT}" --no-auth-warning 2>/dev/null)" || return
  grep -Fxq PONG <<<"${response}"
}

probe_web_https_health() {
  curl --proto '=https' --tlsv1.2 --fail --silent --show-error \
    --connect-timeout 5 --max-time 15 "https://${TP_WEB_DOMAIN}/" >/dev/null
}

probe_node_https_health() {
  curl --proto '=https' --tlsv1.2 --fail --silent --show-error \
    --connect-timeout 5 --max-time 15 "https://${TP_NODE_DOMAIN}/" >/dev/null
}

probe_sysadmin_credential_health() {
  local status
  if docker exec -e TP_VERIFY_SYSADMIN_CREDENTIAL=1 "${PANEL_CONTAINER}" ./trojan-panel; then
    return 0
  else
    status=$?
  fi
  [[ "${status}" == 2 ]] && return 2
  return 1
}

wait_for_health_probe() {
  local label="$1"
  local probe="$2"
  local attempt
  for ((attempt = 1; attempt <= TP_HEALTH_ATTEMPTS; attempt++)); do
    local probe_status=0
    if "${probe}" >/dev/null 2>&1; then
      echo_content skyBlue "---> Health check passed: ${label}"
      return
    else
      probe_status=$?
    fi
    if [[ "${probe_status}" == 2 ]]; then
      break
    fi
    if ((attempt < TP_HEALTH_ATTEMPTS)); then
      sleep "${TP_HEALTH_DELAY_SECONDS}"
    fi
  done
  echo_content red "---> Health check failed: ${label}"
  case "${label}" in
  MariaDB) echo_content yellow "    Check container ${MARIADB_CONTAINER} and its persisted data." ;;
  Redis) echo_content yellow "    Check container ${REDIS_CONTAINER} and its authentication state." ;;
  "Web HTTPS") echo_content yellow "    Check DNS, certificate issuance, ports 80/443, and the active entry provider for ${TP_WEB_DOMAIN}." ;;
  "sysadmin container credential") echo_content yellow "    Check container ${PANEL_CONTAINER}; the generated credential remains in the restricted configuration." ;;
  esac
  return 1
}

probe_node_mariadb_health() {
  docker exec -e TP_VERIFY_NODE_DATA_SERVICES=mariadb "${CORE_CONTAINER}" \
    /tpdata/trojan-panel-core/trojan-panel-core >/dev/null 2>&1
}

probe_node_redis_health() {
  docker exec -e TP_VERIFY_NODE_DATA_SERVICES=redis "${CORE_CONTAINER}" \
    /tpdata/trojan-panel-core/trojan-panel-core >/dev/null 2>&1
}

probe_node_api_health() {
  curl --fail --silent --show-error --connect-timeout 2 --max-time 5 \
    "http://127.0.0.1:${CORE_PORT}/healthz" >/dev/null
}

verify_node_health() {
  echo_content green "---> Verify Node Agent health"
  wait_for_health_probe "Node MariaDB identity" probe_node_mariadb_health
  wait_for_health_probe "Node Redis identities" probe_node_redis_health
  echo_content yellow "Run this on the Web control-plane host if automatic polling has not verified the Node yet:"
  echo_content yellow "  docker exec ${PANEL_CONTAINER} /tpdata/trojan-panel/trojan-panel node-identity verify --id ${NODE_IDENTITY_ID} --challenge ${NODE_BOOTSTRAP_CHALLENGE}"
  wait_for_health_probe "Web-to-Node mTLS/gRPC and Node API" probe_node_api_health
}

print_node_success() {
  echo_content red "\n=============================================================="
  echo_content skyBlue "Node Agent is healthy"
  echo_content yellow "Node identity: ${NODE_IDENTITY_ID} (generation ${NODE_IDENTITY_GENERATION})"
  echo_content yellow "Node domain: ${TP_NODE_DOMAIN}"
  echo_content yellow "Core gRPC port: ${GRPC_PORT}"
  echo_content yellow "Core API port: ${CORE_PORT}"
  echo_content red "==============================================================\n"
}

verify_web_health() {
  echo_content green "---> Verify Web control plane health"
  wait_for_health_probe MariaDB probe_mariadb_health
  wait_for_health_probe Redis probe_redis_health
  wait_for_health_probe "Web HTTPS" probe_web_https_health
  wait_for_health_probe "sysadmin container credential" probe_sysadmin_credential_health
}

verify_combined_health() {
  verify_web_health
  wait_for_health_probe "Node HTTPS" probe_node_https_health
  verify_node_health
  wait_for_health_probe "Web-to-Node mTLS/gRPC" probe_combined_mtls_grpc_health
}

probe_combined_mtls_grpc_health() {
  docker exec "${PANEL_CONTAINER}" /tpdata/trojan-panel/trojan-panel \
    node-identity verify --id "${NODE_IDENTITY_ID}" --challenge "${NODE_BOOTSTRAP_CHALLENGE}" >/dev/null
}

print_web_success() {
  echo_content red "\n=============================================================="
  echo_content skyBlue "Web control plane is healthy"
  echo_content yellow "URL: https://${TP_WEB_DOMAIN}"
  echo_content yellow "Username: sysadmin"
  echo_content yellow "Credentials are stored in the restricted deployment configuration and are not printed."
  echo_content red "==============================================================\n"
}

print_combined_success() {
  echo_content red "\n=============================================================="
  echo_content skyBlue "Combined deployment is healthy"
  echo_content yellow "Web URL: https://${TP_WEB_DOMAIN}"
  echo_content yellow "Node URL: https://${TP_NODE_DOMAIN}"
  echo_content yellow "Shared Entry owns TCP 80/443; Node kernel protocol listeners remain direct."
  echo_content yellow "Node identity: ${NODE_IDENTITY_ID} (generation ${NODE_IDENTITY_GENERATION})"
  echo_content yellow "Credentials are stored in restricted deployment files and are not printed."
  echo_content red "==============================================================\n"
}

deploy_core() {
  local -a combined_label=()
  [[ "${TP_DEPLOYMENT_MODE}" != combined ]] || combined_label=(--label "io.trojanpanelnext.deployment=${COMBINED_ENTRY_DEPLOYMENT_ID}" --label "io.trojanpanelnext.owner-token=${COMBINED_OWNER_TOKEN}")
  local domain="$1"
  local client_ca_sha256
  client_ca_sha256="$(sha256sum "${GRPC_CLIENT_CA_PATH}" | awk '{print $1}')"
  local runtime_config="${TP_DATA}/trojan-panel-core/config/config.ini"
  local node_config_sha256
  local cert_data="${TP_DATA}/custom/node-caddy/data"
  if [[ "${TP_DEPLOYMENT_MODE}" == combined ]]; then
    # The shared Entry owns the Node certificate too; the Node kernels only
    # consume the material and never bind the Entry's 80/443 listeners.
    cert_data="${TP_DATA}/custom/web-caddy/data"
  fi
  local crt_path="${cert_data}/caddy/certificates/acme-v02.api.letsencrypt.org-directory/${domain}/${domain}.crt"
  local key_path="${cert_data}/caddy/certificates/acme-v02.api.letsencrypt.org-directory/${domain}/${domain}.key"
  if [[ "${TLS_MODE}" == "external" ]]; then
    crt_path="${MANAGED_CERT_DIR}/fullchain.pem"
    key_path="${MANAGED_CERT_DIR}/privkey.pem"
  elif [[ "${TP_DEPLOYMENT_MODE}" == combined ]]; then
    if [[ ! -s "${MANAGED_CERT_DIR}/fullchain.pem" || ! -s "${MANAGED_CERT_DIR}/privkey.pem" ]]; then
      echo_content red "Certificate material is unavailable for Node domain ${domain}"
      exit 1
    fi
    crt_path="${MANAGED_CERT_DIR}/fullchain.pem"
    key_path="${MANAGED_CERT_DIR}/privkey.pem"
  fi

  write_core_runtime_config "${crt_path}" "${key_path}"
  node_config_sha256="$(sha256sum "${runtime_config}" | awk '{print $1}')"
  remove_container_if_force "${CORE_CONTAINER}"
  recreate_container_if_env_changed "${CORE_CONTAINER}" TP_TLS_MODE "${TLS_MODE}" acme
  recreate_container_if_env_changed "${CORE_CONTAINER}" TP_CLIENT_CA_SHA256 "${client_ca_sha256}" ""
  recreate_container_if_env_changed "${CORE_CONTAINER}" TP_NODE_CONFIG_SHA256 "${node_config_sha256}" ""
  if container_running "${CORE_CONTAINER}"; then
    echo_content skyBlue "---> Trojan Panel Core already running"
    if [[ "${TLS_MODE}" == "external" ]]; then
      echo_content yellow "---> Note: re-run with --force when switching tls_mode so the managed certificate mount is applied"
    fi
    if [[ "${TP_DEPLOYMENT_MODE}" == combined ]]; then
      reconcile_combined_node_cert_consumer
    fi
    return
  fi
  if container_exists "${CORE_CONTAINER}"; then
    docker start "${CORE_CONTAINER}" >/dev/null
    if [[ "${TLS_MODE}" == "external" ]]; then
      echo_content yellow "---> Note: re-run with --force when switching tls_mode so the managed certificate mount is applied"
    fi
    if [[ "${TP_DEPLOYMENT_MODE}" == combined ]]; then
      record_combined_node_cert_generation "$(combined_node_cert_sha256)"
    fi
    return
  fi

  ensure_image "${CORE_IMAGE}"
  docker run -d --name "${CORE_CONTAINER}" --restart always "${combined_label[@]}" \
    --network=host \
    -v "${TP_DATA}/trojan-panel-core/bin/xray/config/:${TP_DATA}/trojan-panel-core/bin/xray/config/" \
    -v "${TP_DATA}/trojan-panel-core/bin/naiveproxy/config/:${TP_DATA}/trojan-panel-core/bin/naiveproxy/config/" \
    -v "${TP_DATA}/trojan-panel-core/bin/hysteria2/config/:${TP_DATA}/trojan-panel-core/bin/hysteria2/config/" \
    -v "${TP_DATA}/trojan-panel-core/logs/:${TP_DATA}/trojan-panel-core/logs/" \
    -v "${TP_DATA}/trojan-panel-core/config/:${TP_DATA}/trojan-panel-core/config/" \
    -v "${runtime_config}:${runtime_config}:ro" \
    -v "${TP_DATA}/trojan-panel-core/pki/:${TP_DATA}/trojan-panel-core/pki/:ro" \
    -v "${KERNEL_RUNTIME_PATH}:${TP_DATA}/trojan-panel-core/runtime/" \
    -v "${cert_data}:${cert_data}:ro" \
    -v "${MANAGED_CERT_DIR}:${MANAGED_CERT_DIR}:ro" \
    -v "${EXTERNAL_MANAGED_DIR}:${EXTERNAL_MANAGED_DIR}" \
    -v "${EXTERNAL_ROUTES_DIR}:${EXTERNAL_ROUTES_DIR}" \
    -v "${WEB_PATH}:${WEB_PATH}" \
    -v /etc/localtime:/etc/localtime \
    -e GIN_MODE=release \
    -e "mariadb_ip=${MARIADB_HOST}" \
    -e "mariadb_port=${MARIADB_PORT}" \
    -e "mariadb_user=${MARIADB_USER}" \
    -e "database=${MARIADB_DATABASE}" \
    -e "account_table=${ACCOUNT_TABLE}" \
    -e "redis_host=${REDIS_HOST}" \
    -e "redis_port=${REDIS_PORT}" \
    -e "REDIS_USERNAME=${REDIS_USERNAME}" \
    -e "REDIS_AUTH_USERNAME=${REDIS_AUTH_USERNAME}" \
    -e "crt_path=${crt_path}" \
    -e "key_path=${key_path}" \
    -e "grpc_port=${GRPC_PORT}" \
    -e "NODE_SERVER_ID=${NODE_SERVER_ID}" \
    -e "grpc_tls_mode=${GRPC_TLS_MODE}" \
    -e "grpc_client_ca_path=${GRPC_CLIENT_CA_PATH}" \
    -e "TP_CLIENT_CA_SHA256=${client_ca_sha256}" \
    -e "TP_NODE_CONFIG_SHA256=${node_config_sha256}" \
    -e "TP_KERNEL_RUNTIME=${TP_DATA}/trojan-panel-core/runtime" \
    -e "TP_EXTERNAL_DIR=${EXTERNAL_ROUTES_DIR}" \
    -e "TP_TLS_MODE=${TLS_MODE}" \
    -e "server_port=${CORE_PORT}" \
    -e "TP_NODE_DOMAIN=${domain}" \
    "${CORE_IMAGE}"
  wait_for_container "${CORE_CONTAINER}"
  if [[ "${TP_DEPLOYMENT_MODE}" == combined ]]; then
    record_combined_node_cert_generation "$(combined_node_cert_sha256)"
  fi
}

write_external_entry_contract() {
  local mode="$1"
  local file="${EXTERNAL_MANAGED_DIR}/README.md"
  local routes="${EXTERNAL_ROUTES_DIR}/routes.json"
  mkdir -p "${EXTERNAL_MANAGED_DIR}"

  local domain="${TP_WEB_DOMAIN:-}"
  local managed_domain="${TP_NODE_DOMAIN:-}"
  local kernel_note service_rows
  if [[ "${mode}" == "node" ]]; then
    kernel_note="- Kernels read TLS from \`${MANAGED_CERT_DIR}\` (read-only mount). After renewal the certificate owner must run \`install.sh refresh-cert --mode node --config <file>\`; it atomically refreshes this copy and restarts Core only when the pair changed."
    service_rows="| Core API | ${CORE_PORT} | Hysteria2 authentication callback; restrict with the firewall |
| Core gRPC | ${GRPC_PORT} | Control plane to node agent; restrict to the Web host |
| Camouflage site | - | Static files in \`${WEB_PATH}\`; serve plain HTTP only for routes that require fallback |"
  else
    kernel_note="- The control plane terminates no TLS traffic itself; the external entry terminates TLS and proxies to the panel UI."
    service_rows="| Panel UI | ${UI_LISTEN} | Serves the panel and proxies \`/api\` to the panel API |
| Panel API | 127.0.0.1:${PANEL_PORT} | Internal only; subscriptions are under \`/api/auth/subscribe/:token\` |"
  fi

  cat >"${file}" <<EOF
# External entry point contract (generated by install.sh)

Generated for \`${mode}\` with \`tls_mode: ${TLS_MODE}\`. The installer does not
create or manage any reverse proxy container in this mode. See
\`deploy/installer/EXTERNAL.md\` and \`docs/外部入口实现契约.md\` in the repository.

## Host services and listeners

| Service | Address | Notes |
| --- | --- | --- |
${service_rows}

The camouflage site must be served as **plain HTTP**: an Xray fallback relays the stream it
already decrypted, so an HTTPS listener on that port answers "400 plain HTTP request was sent to
HTTPS port". Port ${NODE_CADDY_HTTP_PORT} is only the panel's default fallback value; if the
external entry already uses that port for ACME challenges, create the node with a different
fallback destination (for example 8443) and serve the same directory there.

Mode: \`${mode}\`, panel/node domain: \`${domain:-${managed_domain:-<unset>}}\`

## TLS material

${kernel_note}

${EXTERNAL_ROUTES_NOTE}

## Machine readable routing list

${routes}

The node agent regenerates this observed-state file whenever a node is added or
removed. Kernels remain the direct public listeners and terminate their own TLS;
do not generate nginx stream forwarding from this file by default. Use it to
audit firewall exposure, detect 443 conflicts and provision only explicitly
required plain-HTTP fallback listeners. Current file:

    cat ${routes}

## Checklist

- Kernel protocol ports are direct public TCP/UDP listeners and terminate their own TLS.
- Keep Core API, Core gRPC and Hysteria2 traffic-stats ports closed except to their authorised callers.
- Serve the camouflage site over plain HTTP only when a route sets \`external_fallback_listener_required: true\`.
- A certificate renewal must atomically refresh the managed copy and restart its kernel consumers.
EOF
  echo_content green "---> External entry point contract: ${file}"
}

deploy_web() {
  require_value TP_WEB_DOMAIN

  install_base_tools
  install_docker
  init_web_secrets
  load_image_archives
  prepare_dirs
  install_pki_material web
  deploy_mariadb
  deploy_redis
  write_panel_runtime_config
  write_initial_sysadmin_password_file
  deploy_panel_backend
  deploy_panel_ui

  if [[ "${TLS_MODE}" == "external" ]]; then
    remove_caddy_container "${WEB_CADDY_CONTAINER}"
    EXTERNAL_ROUTES_NOTE="- The web side has no kernel routes; \`routes.json\` exists on node agents only."
    write_external_entry_contract web
    warn_external_ports web
    return
  fi

  write_web_caddyfile "${TP_WEB_DOMAIN}"
  start_caddy "${WEB_CADDY_CONTAINER}" "${TP_DATA}/custom/web-caddy" "${TP_DATA}/custom/web-caddy/data" "${WEB_PATH}"
  wait_for_cert "${TP_WEB_DOMAIN}" "${TP_DATA}/custom/web-caddy/data"
}

deploy_node() {
  require_value TP_NODE_DOMAIN
  require_value MARIADB_HOST
  require_value MARIADB_PASSWORD
  require_value REDIS_HOST
  require_value REDIS_PASSWORD

  install_base_tools
  install_docker
  load_image_archives
  prepare_dirs
  install_pki_material node
  prepare_static_web

  if [[ "${TLS_MODE}" == "external" ]]; then
    TLS_CERT_PAIR="$(discover_external_cert)"
    install_external_cert "${TLS_CERT_PAIR}"
    remove_caddy_container "${NODE_CADDY_CONTAINER}"
    deploy_core "${TP_NODE_DOMAIN}"
    EXTERNAL_ROUTES_NOTE="- Node kernels are direct listeners. The external entry only serves \`${WEB_PATH}\` on fallback ports explicitly marked as required in \`routes.json\`."
    write_external_entry_contract node
    warn_external_ports node

    return
  fi

  write_node_caddyfile "${TP_NODE_DOMAIN}"
  start_caddy "${NODE_CADDY_CONTAINER}" "${TP_DATA}/custom/node-caddy" "${TP_DATA}/custom/node-caddy/data" "${WEB_PATH}"
  wait_for_cert "${TP_NODE_DOMAIN}" "${TP_DATA}/custom/node-caddy/data"
  deploy_core "${TP_NODE_DOMAIN}"

}

deploy_combined() {
  validate_combined_entry_preconditions
  combined_owner_prepare
  # Establish ownership before any data/container mutation so a failed first
  # deployment remains safely retryable.
  combined_owner_write_marker
  install_base_tools
  install_docker
  init_web_secrets
  load_image_archives
  ensure_image "${CADDY_IMAGE}"
  prepare_dirs
  combined_owner_write_data_markers
  install_pki_material combined
  prepare_static_web
  deploy_mariadb
  deploy_redis
  write_panel_runtime_config
  write_initial_sysadmin_password_file
  deploy_panel_backend
  deploy_panel_ui

  # The API provisions dedicated MariaDB/Redis identities for the local Node;
  # the Core never receives the Web root password or the global Redis secret.
  WEB_MARIADB_USER="${MARIADB_USER}"
  WEB_MARIADB_PASSWORD="${MARIADB_PASSWORD}"
  WEB_REDIS_PASSWORD="${REDIS_PASSWORD}"
  prepare_combined_node_identity
  COMBINED_ENTRY_REVISION="$(jq -r '.desired_revision // 0' "${COMBINED_ENTRY_STATE_ROOT}/${COMBINED_ENTRY_DEPLOYMENT_ID}.json" 2>/dev/null || printf '0')"
  COMBINED_ENTRY_REVISION=$((COMBINED_ENTRY_REVISION + 1))
  combined_entry_write_spec '["web","node"]'
  if container_exists "${CORE_CONTAINER}"; then
    combined_entry_reconcile 0
  else
    combined_entry_reconcile 1
  fi
  deploy_core "${TP_NODE_DOMAIN}"
  combined_entry_reconcile 0
}

remove_web() {
  local containers=("${UI_CONTAINER}" "${PANEL_CONTAINER}" "${REDIS_CONTAINER}" "${MARIADB_CONTAINER}")
  if [[ "${TLS_MODE}" != "external" ]]; then
    containers=("${WEB_CADDY_CONTAINER}" "${containers[@]}")
  fi
  docker rm -f "${containers[@]}" >/dev/null 2>&1 || true
  if [[ "${TP_PURGE_DATA}" == "1" ]]; then
    rm -rf "${TP_DATA}/custom/web-caddy" "${TP_DATA}/trojan-panel" "${TP_DATA}/trojan-panel-ui" "${TP_DATA}/mariadb" "${TP_DATA}/redis" "${EXTERNAL_MANAGED_DIR}" "${NETWORK_PLAN_DIR}" "${INSTALLER_STATE_DIR}"
  fi
  echo_content skyBlue "---> Trojan Panel web side removed"
}

remove_node() {
  local containers=("${CORE_CONTAINER}")
  if [[ "${TLS_MODE}" != "external" ]]; then
    containers+=("${NODE_CADDY_CONTAINER}")
  fi
  docker rm -f "${containers[@]}" >/dev/null 2>&1 || true
  if [[ "${TP_PURGE_DATA}" == "1" ]]; then
    rm -rf "${TP_DATA}/custom/node-caddy" "${TP_DATA}/trojan-panel-core" "${EXTERNAL_MANAGED_DIR}" "${NETWORK_PLAN_DIR}" "${INSTALLER_STATE_DIR}"
  fi
  echo_content skyBlue "---> Trojan Panel node side removed"
}

remove_combined_container_if_exists() {
  local name="$1"
  container_exists "${name}" || return 0
  local owner
  owner="$(docker inspect -f '{{ index .Config.Labels "io.trojanpanelnext.deployment" }}' "${name}" 2>/dev/null || true)"
  [[ "${owner}" == "${COMBINED_ENTRY_DEPLOYMENT_ID}" ]] || {
    echo_content red "Combined resource ownership conflict: ${name}"
    return 1
  }
  [[ "$(combined_container_owner_token "${name}")" == "${COMBINED_OWNER_TOKEN}" ]] || {
    echo_content red "Combined resource owner token conflict: ${name}"
    return 1
  }
  docker rm -f "${name}" >/dev/null || {
    echo_content red "Could not remove combined role container: ${name}"
    return 1
  }
}

remove_combined_role() {
  local role="$1" state="${COMBINED_ENTRY_STATE_ROOT}/${COMBINED_ENTRY_DEPLOYMENT_ID}.json" roles
  [[ -f "${state}" && ! -L "${state}" ]] || { echo_content red "Combined Entry journal is missing"; return 1; }
  combined_owner_prepare || return 1
  roles="$(jq -r '.active_roles | sort | join(",")' "${state}")" || return 1
  combined_resource_check || return 1
  [[ "${TP_PURGE_DATA}" != 1 ]] || combined_owner_purge_preflight || return 1
  require_combined_entry_ownership
  case "${role}" in
  web)
    [[ "${roles}" == node,web || "${roles}" == web ]] || return 1
    if [[ "${roles}" == node,web ]]; then
      COMBINED_ENTRY_REVISION="$(jq -r '.desired_revision' "${state}")"
      COMBINED_ENTRY_REVISION=$((COMBINED_ENTRY_REVISION + 1))
      combined_entry_write_spec '["node"]'
      COMBINED_PURGE_RETIRED_ROLES="${TP_PURGE_DATA}" combined_entry_reconcile 0
    else
      combined_entry_remove "${TP_PURGE_DATA}"
    fi
    remove_combined_container_if_exists "${UI_CONTAINER}"
    remove_combined_container_if_exists "${PANEL_CONTAINER}"
    if [[ "${roles}" == web ]]; then
      remove_combined_container_if_exists "${REDIS_CONTAINER}"
      remove_combined_container_if_exists "${MARIADB_CONTAINER}"
    fi
    if [[ "${TP_PURGE_DATA}" == 1 ]]; then
      rm -rf "${TP_DATA}/trojan-panel-ui"
      if [[ "${roles}" == web ]]; then
        rm -rf "${NETWORK_PLAN_DIR}" "${INSTALLER_STATE_DIR}"
      fi
    fi
    ;;
  node)
    [[ "${roles}" == node,web || "${roles}" == node ]] || return 1
    # Reclaim the control-plane identity before changing the local Node role.
    # A failed revocation leaves every local resource untouched.
    revoke_combined_node_identity
    if [[ "${roles}" == node,web ]]; then
      COMBINED_ENTRY_REVISION="$(jq -r '.desired_revision' "${state}")"
      COMBINED_ENTRY_REVISION=$((COMBINED_ENTRY_REVISION + 1))
      combined_entry_write_spec '["web"]'
      COMBINED_PURGE_RETIRED_ROLES="${TP_PURGE_DATA}" combined_entry_reconcile 0
    fi
    remove_combined_container_if_exists "${CORE_CONTAINER}"
    if [[ "${roles}" == node ]]; then
      combined_entry_remove "${TP_PURGE_DATA}"
      remove_combined_container_if_exists "${REDIS_CONTAINER}"
      remove_combined_container_if_exists "${MARIADB_CONTAINER}"
    fi
    if [[ "${TP_PURGE_DATA}" == 1 ]]; then
      rm -rf "${TP_DATA}/trojan-panel-core"
      rm -f "${NODE_IDENTITY_CREDENTIAL_FILE}"
      rm -rf "${NETWORK_PLAN_DIR}" "${INSTALLER_STATE_DIR}"
    fi
    ;;
  combined)
    # The Entry Adapter validates and removes every owned Entry resource before
    # role containers and shared data are retired. Certificate consumers are
    # still stopped below; purge of their files is only done after ownership
    # has been proven by the Adapter.
    revoke_combined_node_identity
    combined_entry_remove "${TP_PURGE_DATA}"
    local resource
    for resource in "${WEB_CADDY_CONTAINER}" "${UI_CONTAINER}" "${PANEL_CONTAINER}" \
      "${CORE_CONTAINER}" "${REDIS_CONTAINER}" "${MARIADB_CONTAINER}"; do
      remove_combined_container_if_exists "${resource}"
    done
    if [[ "${TP_PURGE_DATA}" == "1" ]]; then
      rm -rf "${TP_DATA}/custom/web-caddy" "${TP_DATA}/custom/node-caddy" \
        "${TP_DATA}/trojan-panel" "${TP_DATA}/trojan-panel-ui" \
        "${TP_DATA}/trojan-panel-core" "${TP_DATA}/mariadb" "${TP_DATA}/redis" \
        "${TP_DATA}/trojanpanelnext-entry" "$(dirname "${NODE_IDENTITY_CREDENTIAL_FILE}")" \
        "${NETWORK_PLAN_DIR}" "${INSTALLER_STATE_DIR}"
    fi
    ;;
  *)
    echo_content red "Unsupported combined removal role: ${role}"
    return 1
    ;;
  esac
  echo_content skyBlue "---> Trojan Panel combined ${role} role removed; unrelated role resources were retained"
}

main() {
  local command="${1:-}"
  TP_REQUEST_COMMAND="${command}"
  local mode=""
  local config_file=""
  local bundle_file=""
  local force_override=""
  local purge_override=""
  local entry_spec_override=""

  case "${command}" in
  -h | --help | help | "")
    usage
    return
    ;;
  install | remove | validate | refresh-cert)
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
    --mode)
      [[ $# -ge 2 ]] || { echo_content red "--mode requires a value"; exit 1; }
      mode="$2"
      shift 2
      ;;
    --config)
      [[ $# -ge 2 ]] || { echo_content red "--config requires a value"; exit 1; }
      config_file="$2"
      shift 2
      ;;
    --bundle)
      [[ $# -ge 2 ]] || { echo_content red "--bundle requires a value"; exit 1; }
      bundle_file="$2"
      shift 2
      ;;
    --entry-spec)
      [[ $# -ge 2 ]] || { echo_content red "--entry-spec requires a value"; exit 1; }
      entry_spec_override="$2"
      shift 2
      ;;
    --restore-role)
      [[ $# -ge 2 ]] || { echo_content red "--restore-role requires web or node"; exit 1; }
      COMBINED_RESTORE_ROLE="$2"
      shift 2
      ;;
    --force)
      force_override=1
      shift
      ;;
    --purge-data)
      [[ -z "${purge_override}" ]] || { echo_content red "--purge-data and --keep-data are mutually exclusive"; exit 1; }
      purge_override=1
      shift
      ;;
    --keep-data)
      [[ -z "${purge_override}" ]] || { echo_content red "--purge-data and --keep-data are mutually exclusive"; exit 1; }
      purge_override=0
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

  require_one_of mode "${mode}" web node combined
  if [[ -n "${COMBINED_RESTORE_ROLE}" ]]; then
    require_one_of restore_role "${COMBINED_RESTORE_ROLE}" web node
    [[ "${command}" == install && "${mode}" == combined ]] || {
      echo_content red "--restore-role is only valid with combined install"
      exit 1
    }
  fi
  if [[ -n "${config_file}" && -n "${bundle_file}" ]] || [[ -z "${config_file}" && -z "${bundle_file}" ]]; then
    echo_content red "Exactly one of --config or --bundle is required"
    usage
    exit 1
  fi
  if [[ -n "${bundle_file}" && ( "${mode}" != node || ( "${command}" != install && "${command}" != validate ) ) ]]; then
    echo_content red "--bundle is only valid with node install or validate"
    exit 1
  fi
  if [[ -n "${force_override}" && "${command}" != install ]]; then
    echo_content red "--force is only valid with install"
    exit 1
  fi
  if [[ -n "${purge_override}" && "${command}" != remove ]]; then
    echo_content red "--purge-data/--keep-data are only valid with remove"
    exit 1
  fi
  [[ -n "${entry_spec_override}" ]] && ENTRY_SPEC_FILE="${entry_spec_override}"
  if [[ -n "${ENTRY_SPEC_FILE}" && "${command}" == refresh-cert ]]; then
    echo_content red "--entry-spec is not valid with refresh-cert"
    exit 1
  fi
  if [[ -n "${bundle_file}" ]]; then
    verify_release_assets_before_host_change "" 1
  else
    verify_release_assets_before_host_change "${config_file}"
  fi
  apply_executable_asset_policy
  if [[ -n "${bundle_file}" ]]; then
    prepare_node_bundle "${bundle_file}"
    config_file="${TP_NODE_BUNDLE_DIR}/config-node.yaml"
  else
    prepare_secure_config "${config_file}"
  fi
  verify_release_assets_before_host_change "${TP_CONFIG_READ_FILE}"
  if [[ "${command}" == validate ]]; then
    load_config "${mode}" "${TP_CONFIG_READ_FILE}" 0
  else
    if [[ "${command}" == install ]]; then
      require_supported_install_platform
    fi
    require_root
    if [[ "${command}" == install ]]; then
      preflight_install_dependencies
      if [[ -n "${ENTRY_SPEC_FILE}" && "${INSTALLER_ASSET_VERSION}" != development ]]; then
        install_entry_runtime_assets || {
          echo_content red "Could not persist verified EntryController runtime assets"
          exit 1
        }
      fi
    fi
    if [[ "${command}" != validate && -x "${ENTRY_RUNTIME_DIR}/entryctl.sh" && ! -L "${ENTRY_RUNTIME_DIR}/entryctl.sh" ]]; then
      ENTRYCTL_PATH="${ENTRY_RUNTIME_DIR}/entryctl.sh"
    fi
    load_config "${mode}" "${TP_CONFIG_READ_FILE}" 1
  fi
  if [[ "${TP_NODE_BUNDLE_ACTIVE}" == 1 ]]; then
    TP_PKI_BUNDLE_DIR="${TP_NODE_BUNDLE_DIR}/pki"
  fi
  [[ -n "${force_override}" ]] && TP_FORCE="${force_override}"
  [[ -n "${purge_override}" ]] && TP_PURGE_DATA="${purge_override}"
  local validation_mode="${mode}"
  if [[ "${command}" == remove && "${TP_DEPLOYMENT_MODE}" == combined &&
    ( "${mode}" == web || "${mode}" == node ) ]]; then
    validation_mode=combined
  fi
  validate_config "${validation_mode}"
  validate_entry_spec_binding "${mode}"

  if [[ "${command}" == install ]]; then
    check_same_version_replay_preconditions "${mode}"
  fi

  if [[ "${command}:${mode}" == install:combined ]]; then
    check_combined_host_preconditions
  fi

  if [[ "${command}" == install ]]; then
    network_plan_write "${mode}"
  fi

  if [[ "${command}:${mode}" == install:node || "${command}:${mode}" == install:combined ]]; then
    initialize_node_bootstrap_challenge
  fi

  if [[ "${command}" == refresh-cert && "${mode}" != node && "${mode}" != combined ]]; then
    echo_content red "refresh-cert is only valid with --mode node or combined"
    exit 1
  fi

  if [[ "${command}" != "remove" && "${TLS_MODE}" == "external" ]]; then
    if [[ "${mode}" == "node" ]]; then
      TLS_CERT_PAIR="$(discover_external_cert)"
      require_external_cert
    elif [[ -n "${TLS_CERT_DIR}" ]]; then
      TLS_CERT_PAIR="$(discover_external_cert)"
      require_external_cert
    fi
  fi

  if [[ "${command}" == remove && -n "${ENTRY_SPEC_FILE}" ]]; then
    entry_controller remove
  fi

  case "${command}:${mode}" in
  validate:web | validate:node | validate:combined)
    echo_content green "Configuration is valid for ${TP_DEPLOYMENT_MODE} deployment mode: ${config_file}"
    ;;
  install:web)
    deploy_web
    ;;
  install:node)
    deploy_node
    ;;
  install:combined)
    deploy_combined
    ;;
  remove:node)
    if [[ "${TP_DEPLOYMENT_MODE}" == combined ]]; then
      remove_combined_role node
    else
      remove_node
    fi
    ;;
  remove:combined)
    remove_combined_role combined
    ;;
  remove:web)
    if [[ "${TP_DEPLOYMENT_MODE}" == combined ]]; then
      remove_combined_role web
    else
      remove_web
    fi
    ;;
  refresh-cert:node)
    refresh_node_certificate
    ;;
  refresh-cert:combined)
    refresh_combined_certificate
    ;;
  esac

  if [[ "${command}" == install && -n "${ENTRY_SPEC_FILE}" ]]; then
    entry_controller reconcile
  fi
  if [[ "${command}:${mode}" == install:web ]]; then
    verify_web_health
    write_installer_state "${mode}"
    print_web_success
  elif [[ "${command}:${mode}" == install:node ]]; then
    verify_node_health
    write_installer_state "${mode}"
    print_node_success
  elif [[ "${command}:${mode}" == install:combined ]]; then
    verify_combined_health
    write_installer_state "${mode}"
    print_combined_success
  fi
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
