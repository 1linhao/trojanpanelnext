#!/usr/bin/env bash
set -euo pipefail

SCRIPT_VERSION="0.1.0-rc.9"
TP_SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if [[ ! -f "${TP_SCRIPT_DIR}/common.sh" ]]; then
  printf 'Missing common.sh. Use tp.sh to download the command and its dependencies.\n' >&2
  exit 1
fi
# shellcheck source=deploy/installer/common.sh
source "${TP_SCRIPT_DIR}/common.sh"
require_matching_version "${SCRIPT_VERSION}"

TP_TEMP_CONFIG_FILE=""

cleanup() {
  if [[ -n "${TP_TEMP_CONFIG_FILE}" ]]; then
    rm -f -- "${TP_TEMP_CONFIG_FILE}"
  fi
}

trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

usage() {
  cat <<EOF
TrojanPanel Next configuration download ${INSTALLER_VERSION}
Usage: $0 web|node [--output <file>]
  --output <file>  Destination (default: ./web.yaml or ./node.yaml)
  -V, --version    Show version
  -h, --help       Show help
Requires curl, CA certificates and coreutils. Existing files and symlinks are
never overwritten. Configuration templates default to ${DEFAULT_CONFIG_REF}.
EOF
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
  echo_content skyBlue "Edit this file, then run: ./tp.sh validate --config ${output}"
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
    echo_content red "Usage: $0 web|node [--output <file>]" >&2
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
main() {
  if handle_metadata "$@"; then return; fi
  handle_config_command "$@"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
