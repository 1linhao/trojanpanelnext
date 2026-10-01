#!/usr/bin/env bash
set -euo pipefail

export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:${PATH:-}"
DEFAULT_VERSION="1.0.2-rc.2"
GITHUB_RAW_BASE="https://raw.githubusercontent.com/1linhao/trojanpanelnext"
TP_DOWNLOAD_DIR=""

cleanup() {
  [[ -z "${TP_DOWNLOAD_DIR}" ]] || rm -rf -- "${TP_DOWNLOAD_DIR}"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

usage() {
  cat <<EOF
TrojanPanel Next script library

Usage:
  $0 [--version <release>] deps install|remove
  $0 [--version <release>] web|node [options]
  $0 [--version <release>] config web|node [--output <file>]
  $0 [--version <release>] validate --config <file>
  $0 [--version <release>] install --config <file> [--force]
  $0 --version <release> update --config <file>
  $0 [--version <release>] remove --config <file> [--keep-data | --purge-data]
  $0 <command> --help
  $0 --entry-version

Default release: ${DEFAULT_VERSION}. --version accepts ${DEFAULT_VERSION} or v${DEFAULT_VERSION}.
The selected release supplies its commands, templates and product image tags.
web/node create a protected YAML and deploy it; missing values are prompted.
deps install prepares deployment dependencies on Debian/Ubuntu systemd hosts.
deps remove uninstalls the Docker/yq it added, retaining system tools and data.
The entrypoint needs Bash, curl, CA certificates, grep and coreutils first.
Deployment, dependency management and removal require root.
Full guide: docs/deployment.md.
EOF
}

download_script() {
  local file="$1" marker="$2" version="$3"
  local url="${GITHUB_RAW_BASE}/v${version}/scripts/deploy/${file}"
  local destination="${TP_DOWNLOAD_DIR}/${file}"
  if ! curl --fail --location --silent --show-error \
    --proto '=https' --proto-redir '=https' \
    --retry 2 --retry-max-time 180 --connect-timeout 10 --max-time 60 \
    --max-filesize 5242880 "${url}" -o "${destination}"; then
    printf 'Cannot download the script library for release v%s: %s\n' "${version}" "${url}" >&2
    return 1
  fi
  if [[ ! -s "${destination}" ]] ||
    ! grep -Fxq "${marker}=\"${version}\"" "${destination}" ||
    ! bash -n "${destination}"; then
    printf 'Invalid script or release mismatch: %s\n' "${url}" >&2
    return 1
  fi
  chmod 600 -- "${destination}"
}

main() {
  local version="${DEFAULT_VERSION}" command="" script="" selected_version=0
  local -a args=()
  while (($#)); do
    case "$1" in
    --version)
      if [[ "${selected_version}" == 1 || -z "${2:-}" || "${2}" == -* ]]; then
        printf '%s\n' '--version requires one release value and may only be supplied once' >&2
        return 1
      fi
      version="${2#v}"
      selected_version=1
      shift 2
      ;;
    --entry-version)
      if (($# != 1)) || [[ -n "${command}" ]]; then
        printf '%s\n' '--entry-version does not accept other arguments' >&2
        return 1
      fi
      printf '%s\n' "${DEFAULT_VERSION}"
      return
      ;;
    *)
      if [[ -z "${command}" ]]; then command="$1"; else args+=("$1"); fi
      shift
      ;;
    esac
  done
  if [[ ! "${version}" =~ ^[0-9]+\.[0-9]+(\.[0-9]+)?(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$ ]]; then
    printf 'Invalid release version: %s\n' "${version}" >&2
    return 1
  fi
  case "${command}" in
  -h | --help | help | "") usage; return ;;
  web | node) script="${command}.sh" ;;
  deps) script=dependencies.sh ;;
  config) script=config.sh ;;
  validate) script=validate.sh ;;
  install) script=install.sh ;;
  update)
    if [[ "${selected_version}" != 1 ]] &&
       ! { [[ "${#args[@]}" == 1 ]] && [[ "${args[0]}" == --help || "${args[0]}" == -h ]]; }; then
      printf '%s\n' 'update requires an explicit --version target release' >&2
      return 1
    fi
    script=update.sh
    ;;
  remove) script=uninstall.sh ;;
  *) printf 'Unknown command: %s\n' "${command}" >&2; usage >&2; return 1 ;;
  esac
  local dependency
  for dependency in bash curl mktemp chmod rm grep; do
    if ! command -v "${dependency}" >/dev/null 2>&1; then
      printf 'Missing dependency: %s. Install it first.\n' "${dependency}" >&2
      return 1
    fi
  done
  export TP_RELEASE_REF="v${version}"
  TP_DOWNLOAD_DIR="$(mktemp -d)"
  chmod 700 -- "${TP_DOWNLOAD_DIR}"
  download_script common.sh INSTALLER_VERSION "${version}"
  download_script "${script}" SCRIPT_VERSION "${version}"
  case "${command}" in
  install) download_script uninstall.sh SCRIPT_VERSION "${version}" ;;
  update)
    download_script install.sh SCRIPT_VERSION "${version}"
    download_script uninstall.sh SCRIPT_VERSION "${version}"
    ;;
  web | node)
    for dependency in quick.sh config.sh install.sh uninstall.sh; do
      download_script "${dependency}" SCRIPT_VERSION "${version}"
    done
    ;;
  esac
  bash "${TP_DOWNLOAD_DIR}/${script}" "${args[@]}"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then main "$@"; fi
