#!/usr/bin/env bash
set -euo pipefail

SCRIPT_VERSION="1.0.2-rc.7"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ ! -f "${SCRIPT_DIR}/common.sh" ]]; then
  printf 'Missing common.sh. Use tp.sh to download this command.\n' >&2
  exit 1
fi
# shellcheck source=scripts/deploy/common.sh
source "${SCRIPT_DIR}/common.sh"
require_matching_version "${SCRIPT_VERSION}"

TP_DEPENDENCY_STATE_DIR=/var/lib/trojanpanelnext-dependencies
TP_DEPENDENCY_YQ_PATH=/usr/local/bin/yq
TP_DEPENDENCY_HOST_UNIT_PATH=/etc/systemd/system/trojanpanelnext-host.service
TP_DEPENDENCY_CONTAINERD_SOCKET=/run/containerd/containerd.sock
TP_DEPENDENCY_YQ_VERSION=4.53.6
TP_DEPENDENCY_TEMP=""
TP_DEPENDENCY_YQ_STAGE=""
TP_DEPENDENCY_DOCKER_PACKAGES=(docker.io containerd runc)

usage() {
  cat <<EOF
TrojanPanel Next dependency manager v${INSTALLER_VERSION}

Usage:
  $0 install
  $0 remove
  $0 --help

Requires root, Debian 12/13 or Ubuntu 22.04/24.04, amd64/arm64 and systemd.
install prepares system tools, Docker Engine and verified mikefarah/yq v4.
Existing Docker and compatible yq are reused. Deployment does not install tools.
remove uninstalls only Docker packages and yq added by this command, using its
record in ${TP_DEPENDENCY_STATE_DIR}. Remove services and containers first.
System tools, pre-existing software and Docker data are retained. No autoremove
or Docker prune is performed. Full guide: docs/deployment.md#dependencies.
EOF
}

dependency_error() { printf '%s\n' "$*" >&2; return 1; }

detect_dependency_platform() {
  local ID="" VERSION_ID=""
  # shellcheck source=/dev/null
  source /etc/os-release
  case "${ID}:${VERSION_ID}" in
  debian:12 | ubuntu:22.04 | ubuntu:24.04) ;;
  debian:13) TP_DEPENDENCY_DOCKER_PACKAGES+=(docker-cli) ;;
  *) dependency_error 'Automatic dependencies support Debian 12/13 and Ubuntu 22.04/24.04. Prepare other systems manually.'; return 1 ;;
  esac
  TP_DEPENDENCY_ARCH="$(dpkg --print-architecture)"
  case "${TP_DEPENDENCY_ARCH}" in
  amd64) TP_DEPENDENCY_YQ_HASH=c5f056448f973ae7d39b5401949648a78f2dc1947d6a8eb65be60d5c504b9385 ;;
  arm64) TP_DEPENDENCY_YQ_HASH=88a1016bc1d657375a35864e4f44b6f333df8ff97b559f51bba0adcb2169df09 ;;
  *) dependency_error 'Automatic dependencies require Linux amd64 or arm64.'; return 1 ;;
  esac
  [[ -d /run/systemd/system ]] || dependency_error 'A running systemd host is required.'
}

dependency_package_installed() {
  [[ "$(dpkg-query -W -f='${db:Status-Status}' "$1" 2>/dev/null)" == installed ]]
}

dependency_docker() { docker --host unix:///var/run/docker.sock "$@"; }

lock_dependency_state() {
  if [[ -L "${TP_DEPENDENCY_STATE_DIR}" ]] ||
    { [[ -e "${TP_DEPENDENCY_STATE_DIR}" ]] && [[ ! -d "${TP_DEPENDENCY_STATE_DIR}" ]]; }; then
    dependency_error 'Dependency record directory must be a real directory.'; return 1
  fi
  mkdir -p -- "${TP_DEPENDENCY_STATE_DIR}"
  [[ "$(stat -c %u "${TP_DEPENDENCY_STATE_DIR}")" == "$(id -u)" ]] || {
    dependency_error 'Dependency record directory has an unexpected owner.'; return 1
  }
  chmod 700 -- "${TP_DEPENDENCY_STATE_DIR}"
  local file
  for file in lock docker-packages yq.sha256; do
    if [[ -L "${TP_DEPENDENCY_STATE_DIR}/${file}" ]] ||
      { [[ -e "${TP_DEPENDENCY_STATE_DIR}/${file}" ]] && [[ ! -f "${TP_DEPENDENCY_STATE_DIR}/${file}" ]]; }; then
      dependency_error "Invalid dependency record: ${file}"; return 1
    fi
  done
  exec {TP_DEPENDENCY_LOCK_FD}>"${TP_DEPENDENCY_STATE_DIR}/lock"
  flock -n "${TP_DEPENDENCY_LOCK_FD}" || dependency_error 'Another dependency command is running.'
}

read_dependency_packages() {
  TP_DEPENDENCY_OWNED_PACKAGES=()
  [[ -f "${TP_DEPENDENCY_STATE_DIR}/docker-packages" ]] || return 0
  local package
  while IFS= read -r package; do
    case "${package}" in
    docker.io | docker-cli | containerd | runc) TP_DEPENDENCY_OWNED_PACKAGES+=("${package}") ;;
    *) dependency_error 'Invalid Docker package ownership record.'; return 1 ;;
    esac
  done <"${TP_DEPENDENCY_STATE_DIR}/docker-packages"
}

install_dependency_yq() {
  if command -v yq >/dev/null 2>&1; then
    require_yq
    printf 'Reusing existing mikefarah/yq v4.\n'
    return
  fi
  if [[ -e "${TP_DEPENDENCY_YQ_PATH}" || -L "${TP_DEPENDENCY_YQ_PATH}" ]]; then
    dependency_error "Refusing to overwrite existing ${TP_DEPENDENCY_YQ_PATH}."; return 1
  fi
  TP_DEPENDENCY_TEMP="$(mktemp -d)"
  curl --fail --location --silent --show-error --proto '=https' --proto-redir '=https' \
    --retry 2 --connect-timeout 10 --max-time 180 \
    "https://github.com/mikefarah/yq/releases/download/v${TP_DEPENDENCY_YQ_VERSION}/yq_linux_${TP_DEPENDENCY_ARCH}" \
    -o "${TP_DEPENDENCY_TEMP}/yq"
  printf '%s  %s\n' "${TP_DEPENDENCY_YQ_HASH}" "${TP_DEPENDENCY_TEMP}/yq" | sha256sum -c -
  # Link a verified staged file without overwriting a path created concurrently.
  TP_DEPENDENCY_YQ_STAGE="$(mktemp "$(dirname "${TP_DEPENDENCY_YQ_PATH}")/.tpnext-yq.XXXXXX")"
  install -m 0755 "${TP_DEPENDENCY_TEMP}/yq" "${TP_DEPENDENCY_YQ_STAGE}"
  ln -T -- "${TP_DEPENDENCY_YQ_STAGE}" "${TP_DEPENDENCY_YQ_PATH}"
  rm -f -- "${TP_DEPENDENCY_YQ_STAGE}"
  TP_DEPENDENCY_YQ_STAGE=""
  printf '%s\n' "${TP_DEPENDENCY_YQ_HASH}" >"${TP_DEPENDENCY_STATE_DIR}/yq.sha256"
  chmod 600 -- "${TP_DEPENDENCY_STATE_DIR}/yq.sha256"
  require_yq
  rm -rf -- "${TP_DEPENDENCY_TEMP}"
  TP_DEPENDENCY_TEMP=""
}

install_dependencies() {
  # An incompatible yq must be resolved explicitly instead of shadowed/replaced.
  if command -v yq >/dev/null 2>&1; then require_yq; fi
  read_dependency_packages
  local package
  local -a missing=() added_docker=()
  for package in bash curl ca-certificates grep coreutils openssl tar findutils; do
    if ! dependency_package_installed "${package}"; then missing+=("${package}"); fi
  done
  if ! command -v awk >/dev/null 2>&1; then missing+=(gawk); fi
  if ! command -v docker >/dev/null 2>&1; then
    for package in docker-ce docker-ce-cli containerd.io moby-engine moby-cli; do
      if dependency_package_installed "${package}"; then
        dependency_error "Existing ${package} needs manual repair; refusing to mix Docker distributions."; return 1
      fi
    done
    for package in "${TP_DEPENDENCY_DOCKER_PACKAGES[@]}"; do
      if ! dependency_package_installed "${package}"; then
        missing+=("${package}")
        added_docker+=("${package}")
      fi
    done
  fi
  if ((${#missing[@]})); then
    apt-get update
    local status=0
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends --no-remove "${missing[@]}" || status=$?
    # Failed APT transactions can still install some packages. Record only those
    # actually present on return; never claim a future manual installation.
    for package in "${added_docker[@]}"; do
      if dependency_package_installed "${package}" &&
        [[ " ${TP_DEPENDENCY_OWNED_PACKAGES[*]} " != *" ${package} "* ]]; then
        TP_DEPENDENCY_OWNED_PACKAGES+=("${package}")
      fi
    done
    if ((${#TP_DEPENDENCY_OWNED_PACKAGES[@]})); then
      printf '%s\n' "${TP_DEPENDENCY_OWNED_PACKAGES[@]}" >"${TP_DEPENDENCY_STATE_DIR}/docker-packages"
      chmod 600 -- "${TP_DEPENDENCY_STATE_DIR}/docker-packages"
    fi
    if ((status != 0)); then return "${status}"; fi
  fi
  install_dependency_yq
  if ! dependency_docker info >/dev/null 2>&1; then
    systemctl enable --now docker
  fi
  dependency_docker info >/dev/null
  openssl version
  printf 'Deployment dependencies are ready.\n'
}

check_dependency_removal() {
  # Node removal depends on yq; do not leave a deployed host agent without it.
  if [[ -f "${TP_DEPENDENCY_HOST_UNIT_PATH}" ]] ||
    systemctl is-active --quiet trojanpanelnext-host.service; then
    dependency_error 'Remove the Node host maintenance service with tp.sh remove before uninstalling dependencies.'; return 1
  fi
  if ((${#TP_DEPENDENCY_INSTALLED_PACKAGES[@]})); then
    local containers namespaces namespace tasks
    if ! containers="$(dependency_docker ps -aq)"; then
      dependency_error 'Cannot verify Docker containers. Start/repair Docker before uninstalling its dependencies.'; return 1
    fi
    if [[ -n "${containers}" ]]; then
      dependency_error 'Docker still has containers (including stopped containers). Remove them before uninstalling dependencies.'; return 1
    fi
    # containerd may also serve Kubernetes or other container workloads.
    if [[ -S "${TP_DEPENDENCY_CONTAINERD_SOCKET}" ]] || systemctl is-active --quiet containerd; then
      namespaces="$(ctr --address "${TP_DEPENDENCY_CONTAINERD_SOCKET}" namespaces list --quiet)" || return 1
      while IFS= read -r namespace; do
        [[ -n "${namespace}" ]] || continue
        tasks="$(ctr --address "${TP_DEPENDENCY_CONTAINERD_SOCKET}" --namespace "${namespace}" containers list --quiet)" || return 1
        if [[ -n "${tasks}" ]]; then
          dependency_error "containerd namespace ${namespace} still has containers."; return 1
        fi
      done <<<"${namespaces}"
    fi
    local plan action package rest
    plan="$(LC_ALL=C apt-get --simulate remove "${TP_DEPENDENCY_INSTALLED_PACKAGES[@]}")" || return 1
    while read -r action package rest; do
      if [[ "${action}" == Remv && " ${TP_DEPENDENCY_INSTALLED_PACKAGES[*]} " != *" ${package} "* ]]; then
        dependency_error "APT would also remove unowned package ${package}; refusing dependency removal."; return 1
      fi
    done <<<"${plan}"
  fi
  if [[ -f "${TP_DEPENDENCY_STATE_DIR}/yq.sha256" ]]; then
    local expected actual
    expected="$(cat "${TP_DEPENDENCY_STATE_DIR}/yq.sha256")"
    [[ "${expected}" =~ ^[0-9a-f]{64}$ ]] || { dependency_error 'Invalid yq ownership record.'; return 1; }
    if [[ -e "${TP_DEPENDENCY_YQ_PATH}" || -L "${TP_DEPENDENCY_YQ_PATH}" ]]; then
      [[ -f "${TP_DEPENDENCY_YQ_PATH}" && ! -L "${TP_DEPENDENCY_YQ_PATH}" ]] || {
        dependency_error 'Managed yq has been replaced; refusing to delete it.'; return 1
      }
      actual="$(sha256sum "${TP_DEPENDENCY_YQ_PATH}")"
      if [[ "${actual%% *}" != "${expected}" ]]; then
        dependency_error 'Managed yq has changed; refusing to delete it. Resolve its ownership record manually.'; return 1
      fi
    fi
  fi
}

remove_dependencies() {
  read_dependency_packages
  local package
  TP_DEPENDENCY_INSTALLED_PACKAGES=()
  for package in "${TP_DEPENDENCY_OWNED_PACKAGES[@]}"; do
    if dependency_package_installed "${package}"; then TP_DEPENDENCY_INSTALLED_PACKAGES+=("${package}"); fi
  done
  if ((${#TP_DEPENDENCY_OWNED_PACKAGES[@]} == 0)) && [[ ! -f "${TP_DEPENDENCY_STATE_DIR}/yq.sha256" ]]; then
    printf 'No dependencies owned by this command. Existing software is retained.\n'; return
  fi
  check_dependency_removal
  if ((${#TP_DEPENDENCY_INSTALLED_PACKAGES[@]})); then
    DEBIAN_FRONTEND=noninteractive apt-get remove -y --no-auto-remove "${TP_DEPENDENCY_INSTALLED_PACKAGES[@]}"
  fi
  if [[ -f "${TP_DEPENDENCY_STATE_DIR}/yq.sha256" ]]; then rm -f -- "${TP_DEPENDENCY_YQ_PATH}"; fi
  rm -f -- "${TP_DEPENDENCY_STATE_DIR}/docker-packages" "${TP_DEPENDENCY_STATE_DIR}/yq.sha256"
  printf 'Managed Docker packages and yq removed. System tools and Docker data retained.\n'
}

main() {
  if handle_metadata "$@"; then return; fi
  if (($# != 1)) || [[ "$1" != install && "$1" != remove ]]; then usage >&2; return 1; fi
  require_root
  require_commands apt-get dpkg dpkg-query systemctl flock stat install sha256sum curl
  detect_dependency_platform
  umask 077
  lock_dependency_state
  trap '[[ -z "${TP_DEPENDENCY_TEMP}" ]] || rm -rf -- "${TP_DEPENDENCY_TEMP}"; [[ -z "${TP_DEPENDENCY_YQ_STAGE}" ]] || rm -f -- "${TP_DEPENDENCY_YQ_STAGE}"' EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
  case "$1" in
  install) install_dependencies ;;
  remove) remove_dependencies ;;
  esac
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then main "$@"; fi
