#!/usr/bin/env bash
set -euo pipefail

SCRIPT_VERSION="1.0.2-rc.8"
TP_SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if [[ ! -f "${TP_SCRIPT_DIR}/common.sh" ]]; then
  printf 'Missing common.sh. Use tp.sh to download the command and its dependencies.\n' >&2
  exit 1
fi
# shellcheck source=scripts/deploy/common.sh
source "${TP_SCRIPT_DIR}/common.sh"
require_matching_version "${SCRIPT_VERSION}"

usage() {
  cat <<EOF
TrojanPanel Next removal ${INSTALLER_VERSION}
Usage: $0 --config <file> [--keep-data | --purge-data]
  --config <file>  YAML deployment configuration; purpose selects web or node
  --keep-data      Retain data even when YAML purge_data is 1
  --purge-data     Delete service data, PKI, camouflage site, custom runtime,
                   deployment YAML and installed host maintenance files
  --purge          Alias for --purge-data
  -V, --version    Show version
  -h, --help       Show help
Requires root, Docker, mikefarah/yq v4 and coreutils. Removes project containers
and their images; images still used by other containers are retained.
EOF
}

validate_purge_path() {
  local path="$1" resolved
  if [[ "${path}" != /* ]]; then
    echo_content red "Refusing to delete a non-absolute path: ${path}" >&2
    exit 1
  fi
  resolved="$(realpath -m -- "${path}")"
  case "${resolved}" in
  / | /etc | /usr | /usr/* | /bin | /bin/* | /sbin | /sbin/* | /lib | /lib/* | /lib64 | /lib64/* | /boot | /boot/* | /var | /var/lib | /var/lib/docker | /var/lib/docker/* | /home | /root | /opt | /tmp | /run | /run/* | /srv | /dev | /dev/* | /proc | /proc/* | /sys | /sys/* | /etc/ssh | /etc/ssh/* | /etc/ssl | /etc/ssl/* | /etc/systemd | /etc/systemd/* | /root/.ssh | /root/.ssh/* | /home/*/.ssh | /home/*/.ssh/*)
    echo_content red "Refusing to delete protected path: ${resolved}" >&2
    exit 1
    ;;
  esac
}

prepare_removal() {
  if [[ "${TP_PURPOSE}" == web ]]; then
    TP_REMOVE_CONTAINERS=("${WEB_CADDY_CONTAINER}" "${UI_CONTAINER}" "${PANEL_CONTAINER}" "${REDIS_CONTAINER}" "${MARIADB_CONTAINER}")
    TP_REMOVE_IMAGES=("${CADDY_IMAGE}" "${UI_IMAGE}" "${PANEL_IMAGE}" "${REDIS_IMAGE}" "${MARIADB_IMAGE}")
    TP_REMOVE_IMAGES+=("ghcr.io/1linhao/trojanpanelnext-api:${INSTALLER_VERSION}" "ghcr.io/1linhao/trojanpanelnext-web:${INSTALLER_VERSION}")
    TP_OTHER_CONTAINERS=("${CORE_CONTAINER}" "${NODE_CADDY_CONTAINER}")
    TP_REMOVE_PATHS=("${TP_DATA}/custom/web-caddy" "${TP_DATA}/trojan-panel" "${TP_DATA}/trojan-panel-ui" "${TP_DATA}/mariadb" "${TP_DATA}/redis")
  else
    TP_REMOVE_CONTAINERS=("${CORE_CONTAINER}")
    TP_REMOVE_IMAGES=("${CORE_IMAGE}")
    if [[ "${NODE_CERTIFICATE_MODE}" == caddy ]]; then
      TP_REMOVE_CONTAINERS+=("${NODE_CADDY_CONTAINER}")
      TP_REMOVE_IMAGES+=("${CADDY_IMAGE}")
    fi
    TP_REMOVE_IMAGES+=("ghcr.io/1linhao/trojanpanelnext-node-agent:${INSTALLER_VERSION}")
    TP_OTHER_CONTAINERS=("${WEB_CADDY_CONTAINER}" "${UI_CONTAINER}" "${PANEL_CONTAINER}" "${REDIS_CONTAINER}" "${MARIADB_CONTAINER}")
    TP_REMOVE_PATHS=("${TP_DATA}/custom/node-caddy" "${TP_DATA}/trojan-panel-core")
  fi
  local name image path
  for name in "${TP_REMOVE_CONTAINERS[@]}"; do
    if image="$(docker inspect --format '{{.Config.Image}}' "${name}" 2>/dev/null)"; then
      TP_REMOVE_IMAGES+=("${image}")
    fi
  done
  if [[ "${TP_PURGE_DATA}" != 1 ]]; then return; fi
  # Shared PKI/site directories cannot be purged while the other purpose is
  # installed on this host. Separate those deployments before full removal.
  for name in "${TP_OTHER_CONTAINERS[@]}"; do
    if docker container inspect "${name}" >/dev/null 2>&1; then
      echo_content red "Cannot purge shared project data while ${name} exists on this host" >&2
      exit 1
    fi
  done
  TP_REMOVE_PATHS+=("${TP_PKI_BUNDLE_DIR}" "${WEB_PATH}" "${KERNEL_RUNTIME_PATH}"
    "${TP_DATA}/trojan-panel" "${TP_DATA}/trojan-panel-ui" "${TP_DATA}/trojan-panel-core"
    "${TP_DATA}/mariadb" "${TP_DATA}/redis" "${TP_DATA}/custom/web-caddy" "${TP_DATA}/custom/node-caddy")
  for path in "${TP_REMOVE_PATHS[@]}"; do validate_purge_path "${path}"; done
  for path in "${GRPC_CLIENT_CERT_PATH}" "${GRPC_CLIENT_KEY_PATH}" "${GRPC_CLIENT_CA_PATH}"; do
    validate_purge_path "${path}"
  done
  protect_external_certificates "${TP_REMOVE_PATHS[@]}" \
    "${GRPC_CLIENT_CERT_PATH}" "${GRPC_CLIENT_KEY_PATH}" "${GRPC_CLIENT_CA_PATH}" \
    "${TP_CONFIG_FILE}" "${TP_ORIGINAL_CONFIG_FILE:-${TP_CONFIG_FILE}}"
}

remove_project_images() {
  local image id used container_ids repository tag
  local -a references=("${TP_REMOVE_IMAGES[@]}")
  # Include unused tags of the exact repositories used by this deployment.
  # No global prune or forced removal of images used by another container.
  for image in "${references[@]}"; do
    repository="${image%%@*}"
    if [[ "${repository##*/}" == *:* ]]; then repository="${repository%:*}"; fi
    while IFS= read -r tag; do
      [[ "${tag}" == *':<none>' ]] || TP_REMOVE_IMAGES+=("${tag}")
    done < <(docker image ls "${repository}" --format '{{.Repository}}:{{.Tag}}')
  done
  local -A seen=()
  for image in "${TP_REMOVE_IMAGES[@]}"; do
    [[ -n "${image}" && -z "${seen[${image}]:-}" ]] || continue
    seen["${image}"]=1
    if ! id="$(docker image inspect --format '{{.Id}}' "${image}" 2>/dev/null)"; then continue; fi
    used=0
    container_ids="$(docker ps -aq)"
    local container
    for container in ${container_ids}; do
      if [[ "$(docker inspect --format '{{.Image}}' "${container}")" == "${id}" ]]; then used=1; break; fi
    done
    if [[ "${used}" == 1 ]]; then
      echo_content yellow "Retaining shared image still used by another container: ${image}"
      continue
    fi
    docker image rm "${image}"
  done
}

cleanup_host_maintenance() {
  # Remote removal keeps its control channel until Web has committed deletion.
  if [[ "${TP_DEFER_HOST_CLEANUP:-0}" == 1 ]]; then return; fi
  if [[ "${TP_PURPOSE}" == node ]]; then
    if [[ -f /etc/systemd/system/trojanpanelnext-host.service ]]; then
      systemctl disable --now trojanpanelnext-host.service
      rm -f -- /etc/systemd/system/trojanpanelnext-host.service
      systemctl daemon-reload
    fi
    rm -rf -- /etc/trojanpanelnext-host /usr/local/lib/trojanpanelnext-host
  fi
}

remove_project() {
  local name
  prepare_removal
  for name in "${TP_REMOVE_CONTAINERS[@]}"; do
    if docker container inspect "${name}" >/dev/null 2>&1; then
      docker rm -fv "${name}"
    fi
  done
  remove_project_images
  if [[ "${TP_PURGE_DATA}" == 1 ]]; then
    rm -rf -- "${TP_REMOVE_PATHS[@]}"
    rm -f -- "${GRPC_CLIENT_CERT_PATH}" "${GRPC_CLIENT_KEY_PATH}" "${GRPC_CLIENT_CA_PATH}"
    if [[ "${TP_DEFER_HOST_CLEANUP:-0}" != 1 ]]; then rm -f -- "${TP_CONFIG_FILE}"; fi
    if [[ -n "${TP_ORIGINAL_CONFIG_FILE:-}" ]]; then rm -f -- "${TP_ORIGINAL_CONFIG_FILE}"; fi
    rmdir --ignore-fail-on-non-empty -- "${TP_DATA}/custom" "${TP_DATA}" 2>/dev/null || true
  fi
  cleanup_host_maintenance
  echo_content skyBlue "---> Trojan Panel ${TP_PURPOSE} side removed (purge data: ${TP_PURGE_DATA})"
}

main() {
  if handle_metadata "$@"; then return; fi
  parse_config_options remove "$@"
  validate_config
  echo_content skyBlue "Operation: remove; purpose: ${TP_PURPOSE}; config: ${TP_CONFIG_FILE}; release: ${INSTALLER_VERSION}"
  require_commands docker realpath rm rmdir
  docker info >/dev/null
  remove_project
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
