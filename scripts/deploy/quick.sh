#!/usr/bin/env bash
# Shared implementation for the Web and Node one-command deployments.
# shellcheck disable=SC2034
set -euo pipefail
SCRIPT_VERSION="1.0.2-rc.12"
require_matching_version "${SCRIPT_VERSION}"
QUICK_TEMP_DIR=""

quick_cleanup() {
  [[ -z "${QUICK_TEMP_DIR}" ]] || rm -rf -- "${QUICK_TEMP_DIR}"
}

quick_usage() {
  cat <<EOF
TrojanPanel Next ${INSTALLER_VERSION} one-command ${1} deployment
Usage: $0 [options]
  --hostname <domain>      Certificate and server domain (prompted if omitted)
  --email <address>        ACME contact email (prompted if omitted)
  --output <file>          New YAML destination (default: ./${1}.yaml)
  --config <file>          Deploy an existing YAML instead of creating one
  --force                  Recreate application containers, retaining data
  --help                   Show help
EOF
  if [[ "$1" == node ]]; then
    cat <<'EOF'
  --web-host <host>        Web database and Redis host
  --node-id <id>           Registered node server ID
  --client-ca <file>       Web public client CA certificate
  --certificate-mode caddy|external  Certificate management (default: caddy)
  --certificate <file>     Existing fullchain PEM in external mode
  --private-key <file>     Existing unencrypted key in external mode
Database/Redis credentials are prompted privately. Prepare the server record
and copy Web's public client-ca.crt before deploying Node. Advanced settings
use --config. No software dependency is installed automatically.
EOF
  fi
}

quick_prompt() {
  local variable="$1" label="$2" secret="${3:-0}" value=""
  [[ -z "${!variable:-}" ]] || return 0
  if ! { exec 8<>/dev/tty; } 2>/dev/null; then
    printf 'Cannot prompt for %s. Supply options or use --config in a terminal.\n' "${label}" >&2
    return 1
  fi
  printf '%s: ' "${label}" >&8
  if [[ "${secret}" == 1 ]]; then
    IFS= read -r -s -u 8 value || { exec 8>&-; return 1; }
    printf '\n' >&8
  else
    IFS= read -r -u 8 value || { exec 8>&-; return 1; }
  fi
  exec 8>&-
  if [[ -z "${value}" ]]; then
    printf '%s is required\n' "${label}" >&2
    return 1
  fi
  printf -v "${variable}" '%s' "${value}"
}

quick_set() {
  local file="$1" key="$2" value="$3"
  TP_QUICK_VALUE="${value}" yq -i ".trojanpanelnext.${key} = strenv(TP_QUICK_VALUE)" "${file}"
}

quick_validate_client_ca() {
  validate_public_client_ca "$1"
}

quick_deploy() {
  local purpose="$1"
  shift
  local hostname="" email="" output="./${purpose}.yaml" config="" force=0 custom_values=0
  local web_host="" node_id="" client_ca="" certificate_mode=caddy certificate="" private_key=""
  local database_password="" redis_password=""
  while (($#)); do
    case "$1" in
    -h | --help) quick_usage "${purpose}"; return ;;
    --force) force=1; shift ;;
    --config) require_option_value "$1" "${2:-}"; config="$2"; shift 2 ;;
    --output) require_option_value "$1" "${2:-}"; output="$2"; custom_values=1; shift 2 ;;
    --hostname) require_option_value "$1" "${2:-}"; hostname="$2"; custom_values=1; shift 2 ;;
    --email) require_option_value "$1" "${2:-}"; email="$2"; custom_values=1; shift 2 ;;
    --web-host | --node-id | --client-ca | --certificate-mode | --certificate | --private-key)
      if [[ "${purpose}" != node ]]; then printf 'Node-only option: %s\n' "$1" >&2; return 1; fi
      require_option_value "$1" "${2:-}"
      case "$1" in
      --web-host) web_host="$2" ;;
      --node-id) node_id="$2" ;;
      --client-ca) client_ca="$2" ;;
      --certificate-mode) certificate_mode="$2" ;;
      --certificate) certificate="$2" ;;
      --private-key) private_key="$2" ;;
      esac
      custom_values=1; shift 2
      ;;
    *) printf 'Unknown deployment option: %s\n' "$1" >&2; return 1 ;;
    esac
  done
  require_root
  local -a install_args=()
  [[ "${force}" == 0 ]] || install_args+=(--force)
  if [[ -n "${config}" ]]; then
    if [[ "${custom_values}" == 1 ]]; then
      printf '%s\n' '--config can only be combined with --force; configure all values in the YAML' >&2
      return 1
    fi
    load_config "${config}"
    if [[ "${TP_PURPOSE}" != "${purpose}" ]]; then printf 'Expected %s YAML\n' "${purpose}" >&2; return 1; fi
    bash "${TP_SCRIPT_DIR}/install.sh" --config "${config}" "${install_args[@]}"
    return
  fi
  require_commands docker curl openssl mktemp dirname ln rm
  require_yq
  if [[ "${purpose}" == node ]]; then
    require_commands systemctl install cmp
    require_one_of node_certificate_mode "${certificate_mode}" caddy external
  fi
  if [[ -e "${output}" || -L "${output}" ]]; then
    printf 'Refusing to overwrite %s; deploy it with --config\n' "${output}" >&2
    return 1
  fi
  quick_prompt hostname 'Server domain'
  if [[ "${purpose}" == web || "${certificate_mode}" == caddy ]]; then quick_prompt email 'ACME contact email'; fi
  if [[ "${purpose}" == node ]]; then
    quick_prompt web_host 'Web database and Redis host'
    quick_prompt node_id 'Registered node server ID'
    if [[ -z "${client_ca}" && -s "${TP_PKI_BUNDLE_DIR}/client-ca.crt" ]]; then client_ca="${TP_PKI_BUNDLE_DIR}/client-ca.crt"; fi
    quick_prompt client_ca 'Path to Web public client-ca.crt'
    if ! quick_validate_client_ca "${client_ca}"; then printf '%s\n' 'Invalid public client CA certificate' >&2; return 1; fi
    if [[ "${certificate_mode}" == external ]]; then
      quick_prompt certificate 'Existing fullchain PEM path'
      quick_prompt private_key 'Existing private key PEM path'
    elif [[ -n "${certificate}" || -n "${private_key}" ]]; then
      printf '%s\n' '--certificate and --private-key require --certificate-mode external' >&2
      return 1
    fi
    quick_prompt database_password 'Web MariaDB password' 1
    quick_prompt redis_password 'Web Redis password' 1
  fi
  trap quick_cleanup EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
  QUICK_TEMP_DIR="$(mktemp -d -- "$(dirname -- "${output}")/.tpnext.XXXXXX")"
  chmod 700 -- "${QUICK_TEMP_DIR}"
  local temporary="${QUICK_TEMP_DIR}/config.yaml" result=""
  if ! result="$(bash "${TP_SCRIPT_DIR}/config.sh" "${purpose}" --output "${temporary}" 2>&1)"; then
    printf '%s\n' "${result}" >&2
    return 1
  fi
  quick_set "${temporary}" hostname "${hostname}"
  quick_set "${temporary}" email "${email}"
  if [[ "${purpose}" == node ]]; then
    quick_set "${temporary}" grpc_tls_server_name "${hostname}"
    quick_set "${temporary}" mariadb_host "${web_host}"
    quick_set "${temporary}" redis_host "${web_host}"
    quick_set "${temporary}" mariadb_password "${database_password}"
    quick_set "${temporary}" redis_password "${redis_password}"
    TP_QUICK_VALUE="${node_id}" yq -i '.trojanpanelnext.node_server_id = (strenv(TP_QUICK_VALUE) | tonumber)' "${temporary}"
    quick_set "${temporary}" node_certificate_mode "${certificate_mode}"
    quick_set "${temporary}" node_certificate_path "${certificate}"
    quick_set "${temporary}" node_private_key_path "${private_key}"
  fi
  if ! result="$(bash "${TP_SCRIPT_DIR}/validate.sh" --config "${temporary}" 2>&1)"; then
    printf '%s\n' "${result}" >&2
    return 1
  fi
  if [[ "${purpose}" == node ]]; then
    local destination="${TP_PKI_BUNDLE_DIR}/client-ca.crt"
    if [[ -e "${destination}" ]] && ! cmp -s -- "${client_ca}" "${destination}"; then
      printf '%s\n' 'A different client CA is already installed; use the documented CA rotation procedure' >&2
      return 1
    fi
    if ! cmp -s -- "${client_ca}" "${destination}"; then
      mkdir -p -- "${TP_PKI_BUNDLE_DIR}"
      chmod 700 -- "${TP_PKI_BUNDLE_DIR}"
      install -m 0644 -- "${client_ca}" "${destination}"
    fi
  fi
  ln -T -- "${temporary}" "${output}"
  quick_cleanup
  QUICK_TEMP_DIR=""
  printf 'Deployment configuration saved: %s\n' "${output}"
  bash "${TP_SCRIPT_DIR}/install.sh" --config "${output}" "${install_args[@]}"
}
