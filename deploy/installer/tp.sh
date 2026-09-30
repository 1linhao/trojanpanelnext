#!/usr/bin/env bash
set -euo pipefail

export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:${PATH:-}"
INSTALLER_VERSION="0.1.0-rc.7"
GITHUB_RAW_BASE="https://raw.githubusercontent.com/1linhao/trojanpanelnext"
DEFAULT_SCRIPT_REF="v${INSTALLER_VERSION}"
TP_SCRIPT_REF="${TP_SCRIPT_REF:-${DEFAULT_SCRIPT_REF}}"
TP_DOWNLOAD_DIR=""

cleanup() {
  if [[ -n "${TP_DOWNLOAD_DIR}" ]]; then
    rm -rf -- "${TP_DOWNLOAD_DIR}"
  fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

usage() {
  cat <<EOF
TrojanPanel Next command entrypoint ${INSTALLER_VERSION}

Usage:
  $0 config web|node [--output <file>]
  $0 validate --config <file>
  $0 install --config <file> [--force]
  $0 remove --config <file> [--keep-data | --purge-data]
  $0 <command> --help
  $0 --version

Fetches the selected command and common.sh from GitHub Raw at
${DEFAULT_SCRIPT_REF}, then executes it with the arguments supplied above.
Install software dependencies first. No software tools are installed by this
entrypoint. Downloads require Bash, curl, CA certificates and coreutils.
install/remove require root. validate/install/remove require mikefarah/yq v4;
install/remove also require a running Docker Engine.
remove deletes project containers and images. --purge-data (alias --purge)
also deletes service data, PKI, camouflage site and deployment YAML.
--keep-data overrides YAML purge_data: 1 and retains data explicitly.
EOF
}

download_script() {
  local file="$1" marker="$2"
  local url="${GITHUB_RAW_BASE}/${TP_SCRIPT_REF}/deploy/installer/${file}"
  local destination="${TP_DOWNLOAD_DIR}/${file}"
  if ! curl --fail --location --silent --show-error \
    --proto '=https' --proto-redir '=https' \
    --retry 2 --retry-max-time 180 --connect-timeout 10 --max-time 60 \
    --max-filesize 5242880 "${url}" -o "${destination}"; then
    printf 'Failed to download script: %s\n' "${url}" >&2
    return 1
  fi
  # Inspect text before executing any downloaded code. In particular, reject
  # HTML error pages, empty responses and scripts from another release.
  if [[ ! -s "${destination}" ]] || \
    ! grep -Fxq "${marker}=\"${INSTALLER_VERSION}\"" "${destination}" || \
    ! bash -n "${destination}"; then
    printf 'Invalid script or mismatched version: %s\n' "${url}" >&2
    return 1
  fi
  chmod 600 -- "${destination}"
}

main() {
  local command="${1:-}" script=""
  case "${command}" in
  -h | --help | help | "") usage; return ;;
  -V | --version | version) printf '%s\n' "${INSTALLER_VERSION}"; return ;;
  config) script=config.sh ;;
  validate) script=validate.sh ;;
  install) script=install.sh ;;
  remove) script=uninstall.sh ;;
  *) printf 'Unknown command: %s\n' "${command}" >&2; usage >&2; return 1 ;;
  esac
  shift
  local dependency
  for dependency in bash curl mktemp chmod rm grep; do
    if ! command -v "${dependency}" >/dev/null 2>&1; then
      printf 'Missing dependency: %s. Install it first.\n' "${dependency}" >&2
      return 1
    fi
  done
  if [[ ! "${TP_SCRIPT_REF}" =~ ^[a-zA-Z0-9][a-zA-Z0-9._/-]*$ ]] || \
    [[ "${TP_SCRIPT_REF}" == *'..'* ]]; then
    printf 'Invalid TP_SCRIPT_REF\n' >&2
    return 1
  fi
  export TP_SCRIPT_REF
  TP_DOWNLOAD_DIR="$(mktemp -d)"
  chmod 700 -- "${TP_DOWNLOAD_DIR}"
  download_script common.sh INSTALLER_VERSION
  download_script "${script}" SCRIPT_VERSION
  if [[ "${command}" == install ]]; then
    download_script uninstall.sh SCRIPT_VERSION
  fi
  # Keep the caller's working directory and stdin/stdout. The command status is
  # propagated by Bash, and the EXIT trap cleans both files on success/failure.
  bash "${TP_DOWNLOAD_DIR}/${script}" "$@"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
