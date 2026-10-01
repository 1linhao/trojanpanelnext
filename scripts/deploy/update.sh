#!/usr/bin/env bash
# Configuration values below are consumed by the sourced installation helpers.
# shellcheck disable=SC2034
set -Eeuo pipefail

SCRIPT_VERSION="1.0.2-rc.7"
TP_SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if [[ ! -f "${TP_SCRIPT_DIR}/install.sh" || ! -f "${TP_SCRIPT_DIR}/uninstall.sh" ]]; then
  printf 'Missing matching install.sh/uninstall.sh. Use tp.sh update.\n' >&2
  exit 1
fi
# Reuse only product-container creation and Node maintenance installation.
# shellcheck source=scripts/deploy/install.sh
source "${TP_SCRIPT_DIR}/install.sh"
require_matching_version "${SCRIPT_VERSION}"

usage() {
  cat <<EOF
TrojanPanel Next image update ${INSTALLER_VERSION}
Usage: tp.sh --version ${INSTALLER_VERSION} update --config <existing-deployment.yaml>
  --config <file>  Original deployed Web or Node YAML (supported schema 1)
  -V, --version    Show this script library's target version
  -h, --help       Show help
Requires root, Docker, mikefarah/yq v4, curl, OpenSSL and coreutils.
Node also requires its existing systemd host maintenance service.
Only official TPNext API/Web or Agent images are updated. Caddy, database,
Redis, certificates, credentials and data are retained. Products stop briefly.
Old images and a mode-0600 configuration backup beside the original are kept.
Failed switching restores the deployment, not database migrations or writes.
Back up business data before updating. Downgrades are not supported.
EOF
}

update_error() { printf 'Update refused: %s\n' "$1" >&2; return 1; }

# Return true when the target is not below the installed release. Compare
# numeric core/pre-release identifiers; a final release sorts above its RCs.
version_can_update() {
  local installed="$1" target="$2" i a b installed_pre target_pre
  local -a installed_core target_core old_ids new_ids
  [[ "${installed}" =~ ^[1-9][0-9]*\.[0-9]+(\.[0-9]+)?(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$ ]] || return 1
  [[ "${target}" =~ ^[1-9][0-9]*\.[0-9]+(\.[0-9]+)?(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$ ]] || return 1
  IFS=. read -r -a installed_core <<<"${installed%%-*}"
  IFS=. read -r -a target_core <<<"${target%%-*}"
  for i in 0 1 2; do
    a="${installed_core[i]:-0}"; b="${target_core[i]:-0}"
    ((10#${b} > 10#${a})) && return 0
    ((10#${b} < 10#${a})) && return 1
  done
  installed_pre="${installed#*-}"; target_pre="${target#*-}"
  [[ "${target}" == *-* ]] || return 0
  [[ "${installed}" == *-* ]] || return 1
  IFS=. read -r -a old_ids <<<"${installed_pre}"
  IFS=. read -r -a new_ids <<<"${target_pre}"
  for ((i=0; i<${#old_ids[@]} || i<${#new_ids[@]}; i++)); do
    [[ -n "${new_ids[i]:-}" ]] || return 1
    [[ -n "${old_ids[i]:-}" ]] || return 0
    a="${old_ids[i]}"; b="${new_ids[i]}"
    if [[ "${a}" =~ ^[0-9]+$ && "${b}" =~ ^[0-9]+$ ]]; then
      ((10#${b} > 10#${a})) && return 0
      ((10#${b} < 10#${a})) && return 1
    elif [[ "${a}" =~ ^[0-9]+$ ]]; then return 0
    elif [[ "${b}" =~ ^[0-9]+$ ]]; then return 1
    else
      [[ "${b}" > "${a}" ]] && return 0
      [[ "${b}" < "${a}" ]] && return 1
    fi
  done
  return 0
}

read_update_options() {
  local config=""
  while (($#)); do
    case "$1" in
    --config)
      require_option_value "$1" "${2:-}"
      [[ -z "${config}" ]] || { update_error '--config may only be supplied once'; return 1; }
      config="$2"; shift 2 ;;
    -h | --help) usage; exit 0 ;;
    *) update_error "Unknown option: $1"; return 1 ;;
    esac
  done
  [[ -n "${config}" ]] || { update_error '--config is required'; return 1; }
  [[ -f "${config}" && ! -L "${config}" ]] || { update_error 'configuration must be an existing regular file, not a symlink'; return 1; }
  UPDATE_ORIGINAL_CONFIG="$(realpath -- "${config}")"
}

prepare_update_config() {
  detect_config_root "${UPDATE_ORIGINAL_CONFIG}"
  UPDATE_FROM_VERSION="$(yaml_read_raw "${UPDATE_ORIGINAL_CONFIG}" release)"
  version_can_update "${UPDATE_FROM_VERSION}" "${INSTALLER_VERSION}" || {
    update_error "unsupported release or downgrade: ${UPDATE_FROM_VERSION:-<missing>} -> ${INSTALLER_VERSION}"; return 1;
  }
  local schema purpose key repository image
  schema="$(yaml_read_raw "${UPDATE_ORIGINAL_CONFIG}" schema_version)"
  [[ "${schema}" == "${SUPPORTED_SCHEMA_VERSION}" ]] || { update_error "unsupported schema version: ${schema}"; return 1; }
  purpose="$(yaml_read_raw "${UPDATE_ORIGINAL_CONFIG}" purpose)"
  case "${purpose}" in
  web) UPDATE_IMAGE_KEYS=(panel_image ui_image); UPDATE_REPOSITORIES=(trojanpanelnext-api trojanpanelnext-web) ;;
  node) UPDATE_IMAGE_KEYS=(core_image); UPDATE_REPOSITORIES=(trojanpanelnext-node-agent) ;;
  *) update_error "unsupported purpose: ${purpose}"; return 1 ;;
  esac
  local i
  for i in "${!UPDATE_IMAGE_KEYS[@]}"; do
    key="${UPDATE_IMAGE_KEYS[i]}"; repository="${UPDATE_REPOSITORIES[i]}"
    image="$(yaml_read_raw "${UPDATE_ORIGINAL_CONFIG}" "${key}")"
    [[ "${image}" == "ghcr.io/1linhao/${repository}:${UPDATE_FROM_VERSION}" ]] || {
      update_error "${key} does not match the official image for configuration release ${UPDATE_FROM_VERSION}"; return 1;
    }
  done
  UPDATE_WORKSPACE="$(mktemp -d "$(dirname -- "${UPDATE_ORIGINAL_CONFIG}")/.tpnext-update.XXXXXX")"
  chmod 700 -- "${UPDATE_WORKSPACE}"
  UPDATE_STAGED_CONFIG="${UPDATE_WORKSPACE}/target.yaml"
  install -m 0600 "${UPDATE_ORIGINAL_CONFIG}" "${UPDATE_WORKSPACE}/original.yaml"
  install -m 0600 "${UPDATE_WORKSPACE}/original.yaml" "${UPDATE_STAGED_CONFIG}"
  TP_TARGET_RELEASE="${INSTALLER_VERSION}" yq -i '.trojanpanelnext.release = strenv(TP_TARGET_RELEASE)' "${UPDATE_STAGED_CONFIG}"
  for i in "${!UPDATE_IMAGE_KEYS[@]}"; do
    key="${UPDATE_IMAGE_KEYS[i]}"; repository="${UPDATE_REPOSITORIES[i]}"
    TP_TARGET_IMAGE="ghcr.io/1linhao/${repository}:${INSTALLER_VERSION}" yq -i ".trojanpanelnext.${key} = strenv(TP_TARGET_IMAGE)" "${UPDATE_STAGED_CONFIG}"
  done
  load_config "${UPDATE_STAGED_CONFIG}"
  validate_config
  TP_FORCE=0
  [[ -n "${MARIADB_PASSWORD:-}" && -n "${REDIS_PASSWORD:-}" ]] || {
    update_error 'original configuration must contain the existing database and Redis credentials'; return 1;
  }
  if [[ "${TP_PURPOSE}" == web ]]; then
    UPDATE_CONTAINERS=("${PANEL_CONTAINER}" "${UI_CONTAINER}")
    UPDATE_IMAGES=("${PANEL_IMAGE}" "${UI_IMAGE}")
  else
    UPDATE_CONTAINERS=("${CORE_CONTAINER}"); UPDATE_IMAGES=("${CORE_IMAGE}")
    prepare_node_certificate
  fi
}

inspect_update_container() {
  local name="$1" image="$2" file="${UPDATE_WORKSPACE}/${1}.json"
  docker inspect "${name}" >"${file}"
  [[ "$(yq -r '.[0].State.Running' "${file}")" == true ]] || { update_error "${name} must be running before updating"; return 1; }
  [[ "$(yq -r '.[0].Config.Image' "${file}")" == "${image}" ]] || { update_error "${name} image differs from the original configuration"; return 1; }
  [[ "$(yq -r '.[0].HostConfig.NetworkMode' "${file}")" == host &&
     "$(yq -r '.[0].HostConfig.RestartPolicy.Name' "${file}")" == always &&
     "$(yq -r '.[0].HostConfig.Privileged // false' "${file}")" == false ]] || {
    update_error "${name} uses an unsupported host configuration"; return 1;
  }
  UPDATE_CHECK_FILE="${file}"
  UPDATE_EXPECTED_MOUNTS=0
}

assert_update_env() {
  local key="$1" expected="$2" actual
  actual="$(TP_ENV_KEY="${key}" yq -r '.[0].Config.Env[] | select((split("=") | .[0]) == strenv(TP_ENV_KEY))' "${UPDATE_CHECK_FILE}")"
  [[ "${actual}" == "${key}=${expected}" ]] || { update_error "container environment ${key} differs from the deployment configuration"; return 1; }
}

assert_update_runtime() {
  local file="$1" section="$2" key="$3" expected="$4" actual
  [[ -f "${file}" ]] || { update_error 'runtime configuration is missing'; return 1; }
  actual="$(awk -v wanted_section="${section}" -v wanted_key="${key}" '
    /^\[/ {current=$0; sub(/^\[/, "", current); sub(/\].*$/, "", current); next}
    current == wanted_section && index($0, wanted_key "=") == 1 {
      value=substr($0, length(wanted_key) + 2); sub(/\r$/, "", value); print value
    }
  ' "${file}")"
  [[ "${actual}" == "${expected}" ]] || { update_error "runtime ${section}.${key} differs from the deployment configuration"; return 1; }
}

assert_update_mount() {
  local source="$1" destination="${2%/}" writable="$3" actual type rw
  actual="$(TP_MOUNT_DST="${destination}" yq -r '.[0].Mounts[] | select((.Destination | sub("/$", "")) == strenv(TP_MOUNT_DST)) | .Source' "${UPDATE_CHECK_FILE}")"
  type="$(TP_MOUNT_DST="${destination}" yq -r '.[0].Mounts[] | select((.Destination | sub("/$", "")) == strenv(TP_MOUNT_DST)) | .Type' "${UPDATE_CHECK_FILE}")"
  rw="$(TP_MOUNT_DST="${destination}" yq -r '.[0].Mounts[] | select((.Destination | sub("/$", "")) == strenv(TP_MOUNT_DST)) | .RW' "${UPDATE_CHECK_FILE}")"
  [[ -n "${actual}" && "${type}" == bind && "${rw}" == "${writable}" &&
     "$(realpath -m -- "${actual}")" == "$(realpath -m -- "${source}")" ]] || {
    update_error "container mount ${destination} differs from the deployment configuration"; return 1;
  }
  UPDATE_EXPECTED_MOUNTS=$((UPDATE_EXPECTED_MOUNTS + 1))
}

assert_update_mount_count() {
  [[ "$(yq -r '.[0].Mounts | length' "${UPDATE_CHECK_FILE}")" == "${UPDATE_EXPECTED_MOUNTS}" ]] || {
    update_error 'container contains unsupported extra mounts'; return 1;
  }
}

check_pending_host_removal() {
  local path
  # The host helper snapshots TLS before launching remove, and writes the
  # result/finalize receipts only afterward. Any such file means an operation
  # has started; an update must not erase or replace that state.
  for path in /etc/trojanpanelnext-host/{result.json,finalize.json,server.crt,server.key,client-ca.crt} /usr/local/lib/trojanpanelnext-host/cleanup-ready; do
    [[ ! -e "${path}" ]] || { update_error 'Node maintenance removal is pending; finish it before updating'; return 1; }
  done
}

check_update_deployment() {
  local directory mount runtime
  if [[ "${TP_PURPOSE}" == web ]]; then
    inspect_update_container "${PANEL_CONTAINER}" "ghcr.io/1linhao/trojanpanelnext-api:${UPDATE_FROM_VERSION}"
    assert_update_env mariadb_ip 127.0.0.1
    assert_update_env mariadb_port "${MARIADB_PORT}"
    assert_update_env mariadb_user "${MARIADB_USER}"
    assert_update_env mariadb_pas "${MARIADB_PASSWORD}"
    assert_update_env redis_host 127.0.0.1
    assert_update_env redis_port "${REDIS_PORT}"
    assert_update_env redis_pass "${REDIS_PASSWORD}"
    assert_update_env server_port "${PANEL_PORT}"
    assert_update_env GRPC_CLIENT_CERT_PATH "${GRPC_CLIENT_CERT_PATH}"
    assert_update_env GRPC_CLIENT_KEY_PATH "${GRPC_CLIENT_KEY_PATH}"
    assert_update_env GRPC_SERVER_CA_PATH "${GRPC_SERVER_CA_PATH}"
    assert_update_env TP_PKI_AUTHORITY_DIR "${TP_PKI_BUNDLE_DIR}"
    assert_update_env TP_HOST_REMOVAL_CALLBACK_URL "https://${TP_WEB_DOMAIN}/api/nodeServer/completeHostRemoval"
    assert_update_mount "${WEB_PATH}" "${TP_DATA}/trojan-panel/webfile" true
    for directory in logs config; do assert_update_mount "${TP_DATA}/trojan-panel/${directory}" "${TP_DATA}/trojan-panel/${directory}" true; done
    assert_update_mount "${TP_DATA}/trojan-panel/pki" "${TP_DATA}/trojan-panel/pki" false
    assert_update_mount "${TP_PKI_BUNDLE_DIR}" "${TP_PKI_BUNDLE_DIR}" true
    assert_update_mount /etc/localtime /etc/localtime true
    assert_update_mount_count
    runtime="${TP_DATA}/trojan-panel/config/config.ini"
    assert_update_runtime "${runtime}" mysql host 127.0.0.1
    assert_update_runtime "${runtime}" mysql port "${MARIADB_PORT}"
    assert_update_runtime "${runtime}" mysql user "${MARIADB_USER}"
    assert_update_runtime "${runtime}" mysql password "${MARIADB_PASSWORD}"
    assert_update_runtime "${runtime}" redis host 127.0.0.1
    assert_update_runtime "${runtime}" redis port "${REDIS_PORT}"
    assert_update_runtime "${runtime}" redis password "${REDIS_PASSWORD}"
    assert_update_runtime "${runtime}" server port "${PANEL_PORT}"
    assert_update_runtime "${runtime}" grpc client_cert_path "${GRPC_CLIENT_CERT_PATH}"
    assert_update_runtime "${runtime}" grpc client_key_path "${GRPC_CLIENT_KEY_PATH}"
    assert_update_runtime "${runtime}" grpc server_ca_path "${GRPC_SERVER_CA_PATH}"
    inspect_update_container "${UI_CONTAINER}" "ghcr.io/1linhao/trojanpanelnext-web:${UPDATE_FROM_VERSION}"
    assert_update_mount "${TP_DATA}/trojan-panel-ui/nginx/default.conf" /etc/nginx/conf.d/default.conf true
    assert_update_mount_count
    if ! grep -Fxq "    listen       ${UI_PORT};" "${TP_DATA}/trojan-panel-ui/nginx/default.conf" ||
      ! grep -Fxq "        proxy_pass http://127.0.0.1:${PANEL_PORT};" "${TP_DATA}/trojan-panel-ui/nginx/default.conf"; then
      update_error 'UI ports differ from the original configuration'; return 1;
    fi
  else
    inspect_update_container "${CORE_CONTAINER}" "ghcr.io/1linhao/trojanpanelnext-node-agent:${UPDATE_FROM_VERSION}"
    assert_update_env mariadb_ip "${MARIADB_HOST}"
    assert_update_env mariadb_port "${MARIADB_PORT}"
    assert_update_env mariadb_user "${MARIADB_USER}"
    assert_update_env mariadb_pas "${MARIADB_PASSWORD}"
    assert_update_env database "${MARIADB_DATABASE}"
    assert_update_env account_table "${ACCOUNT_TABLE}"
    assert_update_env redis_host "${REDIS_HOST}"
    assert_update_env redis_port "${REDIS_PORT}"
    assert_update_env redis_pass "${REDIS_PASSWORD}"
    assert_update_env server_port "${CORE_PORT}"
    assert_update_env grpc_port "${GRPC_PORT}"
    assert_update_env NODE_SERVER_ID "${NODE_SERVER_ID}"
    assert_update_env grpc_tls_mode "${GRPC_TLS_MODE}"
    assert_update_env grpc_client_ca_path "${GRPC_CLIENT_CA_PATH}"
    assert_update_env TP_PKI_BOOTSTRAP_CA_PATH "${TP_PKI_BUNDLE_DIR}/client-ca.crt"
    assert_update_env TP_KERNEL_RUNTIME "${TP_DATA}/trojan-panel-core/runtime"
    assert_update_env TP_NODE_CERTIFICATE_MODE "${NODE_CERTIFICATE_MODE}"
    assert_update_env crt_path "${NODE_CERTIFICATE_PATH}"
    assert_update_env key_path "${NODE_PRIVATE_KEY_PATH}"
    for directory in bin/xray/config bin/naiveproxy/config bin/hysteria2/config logs config; do
      assert_update_mount "${TP_DATA}/trojan-panel-core/${directory}" "${TP_DATA}/trojan-panel-core/${directory}" true
    done
    assert_update_mount "${TP_PKI_BUNDLE_DIR}" "${TP_PKI_BUNDLE_DIR}" true
    directory="$(dirname -- "${GRPC_CLIENT_CA_PATH}")"
    if [[ "${directory}" != "${TP_PKI_BUNDLE_DIR}" ]]; then assert_update_mount "${directory}" "${directory}" true; fi
    assert_update_mount "${KERNEL_RUNTIME_PATH}" "${TP_DATA}/trojan-panel-core/runtime" true
    if [[ "${NODE_CERTIFICATE_MODE}" == caddy ]]; then
      assert_update_mount "${TP_DATA}/custom/node-caddy/data" "${TP_DATA}/custom/node-caddy/data" true
    else
      for mount in "${NODE_CERTIFICATE_MOUNTS[@]}"; do
        [[ "${mount}" == type=bind,* ]] || continue
        directory="${mount#type=bind,src=}"; directory="${directory%%,dst=*}"
        assert_update_mount "${directory}" "${directory}" false
      done
    fi
    assert_update_mount "${WEB_PATH}" "${WEB_PATH}" true
    assert_update_mount /etc/localtime /etc/localtime true
    assert_update_mount_count
    runtime="${TP_DATA}/trojan-panel-core/config/config.ini"
    assert_update_runtime "${runtime}" mysql host "${MARIADB_HOST}"
    assert_update_runtime "${runtime}" mysql port "${MARIADB_PORT}"
    assert_update_runtime "${runtime}" mysql user "${MARIADB_USER}"
    assert_update_runtime "${runtime}" mysql password "${MARIADB_PASSWORD}"
    assert_update_runtime "${runtime}" mysql database "${MARIADB_DATABASE}"
    assert_update_runtime "${runtime}" mysql account_table "${ACCOUNT_TABLE}"
    assert_update_runtime "${runtime}" redis host "${REDIS_HOST}"
    assert_update_runtime "${runtime}" redis port "${REDIS_PORT}"
    assert_update_runtime "${runtime}" redis password "${REDIS_PASSWORD}"
    assert_update_runtime "${runtime}" cert crt_path "${NODE_CERTIFICATE_PATH}"
    assert_update_runtime "${runtime}" cert key_path "${NODE_PRIVATE_KEY_PATH}"
    assert_update_runtime "${runtime}" grpc port "${GRPC_PORT}"
    assert_update_runtime "${runtime}" grpc tls_mode "${GRPC_TLS_MODE}"
    assert_update_runtime "${runtime}" grpc client_ca_path "${GRPC_CLIENT_CA_PATH}"
    assert_update_runtime "${runtime}" server port "${CORE_PORT}"
    assert_update_runtime "${runtime}" node server_id "${NODE_SERVER_ID}"
    require_commands systemctl
    [[ -f /etc/systemd/system/trojanpanelnext-host.service && -f /etc/trojanpanelnext-host/config.json &&
       -f /usr/local/lib/trojanpanelnext-host/uninstall.sh && -f /usr/local/lib/trojanpanelnext-host/common.sh ]] || {
      update_error 'Node maintenance service is missing; repair the existing installation first'; return 1;
    }
    check_pending_host_removal
    if [[ "${UPDATE_HELPER_STOPPED:-0}" != 1 ]]; then systemctl is-active --quiet trojanpanelnext-host.service; fi
    [[ "$(yq -r '.originalConfig' /etc/trojanpanelnext-host/config.json)" == "${UPDATE_ORIGINAL_CONFIG}" &&
       "$(yq -r '.nodeId' /etc/trojanpanelnext-host/config.json)" == "${NODE_SERVER_ID}" ]] || {
      update_error 'Node maintenance identity or original configuration path differs'; return 1;
    }
  fi
}

backup_update_files() {
  UPDATE_BACKUP_CONFIG="${UPDATE_ORIGINAL_CONFIG}.backup-${UPDATE_FROM_VERSION}-$(date -u +%Y%m%dT%H%M%SZ)-${UPDATE_WORKSPACE##*.}"
  # Hard-link creation avoids overwriting an existing backup.
  cmp -s -- "${UPDATE_WORKSPACE}/original.yaml" "${UPDATE_ORIGINAL_CONFIG}" || {
    update_error 'original configuration changed during image pulls'; return 1;
  }
  ln -T -- "${UPDATE_WORKSPACE}/original.yaml" "${UPDATE_BACKUP_CONFIG}"
  UPDATE_RUNTIME_FILES=()
  if [[ "${TP_PURPOSE}" == web ]]; then
    UPDATE_RUNTIME_FILES=("${TP_DATA}/trojan-panel/config/config.ini" "${TP_DATA}/trojan-panel-ui/nginx/default.conf")
  else
    UPDATE_RUNTIME_FILES=("${TP_DATA}/trojan-panel-core/config/config.ini")
    cp -a -- /etc/trojanpanelnext-host "${UPDATE_WORKSPACE}/host-state"
    cp -a -- /usr/local/lib/trojanpanelnext-host "${UPDATE_WORKSPACE}/host-library"
    cp -a -- /etc/systemd/system/trojanpanelnext-host.service "${UPDATE_WORKSPACE}/host-service"
  fi
  local i
  for i in "${!UPDATE_RUNTIME_FILES[@]}"; do
    [[ -f "${UPDATE_RUNTIME_FILES[i]}" ]] || { update_error "missing runtime configuration: ${UPDATE_RUNTIME_FILES[i]}"; return 1; }
    cp -a -- "${UPDATE_RUNTIME_FILES[i]}" "${UPDATE_WORKSPACE}/runtime-${i}"
  done
}

update_health_check() {
  local timeout=90 deadline=$((SECONDS + 90)) stable=0 name state code body
  printf 'Waiting up to %s seconds for product readiness.\n' "${timeout}"
  while ((SECONDS < deadline)); do
    state=1
    for name in "${UPDATE_CONTAINERS[@]}"; do
      [[ "$(docker inspect --format '{{.State.Running}}:{{.State.Restarting}}:{{.RestartCount}}' "${name}")" == true:false:0 ]] || state=0
    done
    if [[ "${TP_PURPOSE}" == web ]]; then
      body="$(curl --fail --silent --show-error --noproxy '*' --max-time 3 "http://127.0.0.1:${PANEL_PORT}/api/auth/setting" 2>/dev/null)" || state=0
      [[ "$(yq -r '.code // 0' <<<"${body}" 2>/dev/null)" == 20000 ]] || state=0
      [[ "$(curl --fail --silent --show-error --noproxy '*' --max-time 3 "http://127.0.0.1:${UI_PORT}/version" 2>/dev/null)" == "v${INSTALLER_VERSION}" ]] || state=0
    else
      # The Agent binds its HTTP router only after DB/Redis/SQLite/gRPC and
      # application initialization. Its root route deliberately returns 404.
      code="$(curl --silent --show-error --noproxy '*' --output /dev/null --write-out '%{http_code}' --max-time 3 "http://127.0.0.1:${CORE_PORT}/" 2>/dev/null)" || state=0
      [[ "${code}" == 404 ]] || state=0
    fi
    if [[ "${state}" == 1 ]]; then stable=$((stable + 1)); else stable=0; fi
    ((stable >= 3)) && return 0
    sleep 2
  done
  printf 'Product readiness timed out; restoring the prior deployment.\n' >&2
  return 1
}

restore_update() {
  local i name saved failed=0
  printf 'Restoring prior product containers and deployment configuration.\n' >&2
  if [[ "${UPDATE_HELPER_STOPPED:-0}" == 1 ]]; then systemctl stop trojanpanelnext-host.service || failed=1; fi
  for i in "${!UPDATE_RENAMED[@]}"; do
    name="${UPDATE_CONTAINERS[i]}"; saved="${UPDATE_SAVED_CONTAINERS[i]}"
    [[ "${UPDATE_RENAMED[i]}" == 1 ]] || continue
    if container_exists "${name}"; then docker rm -f "${name}" >/dev/null || failed=1; fi
    docker rename "${saved}" "${name}" || failed=1
  done
  if [[ "${UPDATE_PRODUCT_SWITCH_STARTED:-0}" == 1 ]]; then
    for i in "${!UPDATE_RUNTIME_FILES[@]}"; do cp -a -- "${UPDATE_WORKSPACE}/runtime-${i}" "${UPDATE_RUNTIME_FILES[i]}" || failed=1; done
  fi
  if [[ "${UPDATE_CONFIG_COMMITTED:-0}" == 1 ]]; then
    install -m 0600 "${UPDATE_BACKUP_CONFIG}" "${UPDATE_WORKSPACE}/restore.yaml" &&
      mv -T -- "${UPDATE_WORKSPACE}/restore.yaml" "${UPDATE_ORIGINAL_CONFIG}" || failed=1
  fi
  if [[ "${UPDATE_HELPER_STOPPED:-0}" == 1 ]]; then
    if [[ "${UPDATE_HELPER_REPLACED:-0}" == 1 ]]; then
      cp -a -- "${UPDATE_WORKSPACE}/host-state/." /etc/trojanpanelnext-host/ || failed=1
      cp -a -- "${UPDATE_WORKSPACE}/host-library/." /usr/local/lib/trojanpanelnext-host/ || failed=1
      cp -a -- "${UPDATE_WORKSPACE}/host-service" /etc/systemd/system/trojanpanelnext-host.service || failed=1
    fi
    systemctl daemon-reload || failed=1
    systemctl restart trojanpanelnext-host.service || failed=1
  fi
  # Include the case where stop succeeded but rename failed.
  for i in "${!UPDATE_STOPPED[@]}"; do
    [[ "${UPDATE_STOPPED[i]}" == 1 ]] || continue
    docker start "${UPDATE_CONTAINERS[i]}" >/dev/null || failed=1
  done
  if [[ "${failed}" != 0 ]]; then
    printf 'Automatic restoration needs attention. Retained recovery files: %s\n' "${UPDATE_WORKSPACE}" >&2
    UPDATE_KEEP_WORKSPACE=1
  fi
  return "${failed}"
}

finish_update() {
  local status="$?"
  trap - EXIT INT TERM
  if [[ "${UPDATE_SWITCH_STARTED:-0}" == 1 && "${UPDATE_COMMITTED:-0}" != 1 ]]; then
    set +e
    restore_update
    set -e
    ((status != 0)) || status=1
  fi
  if [[ -n "${UPDATE_WORKSPACE:-}" && "${UPDATE_KEEP_WORKSPACE:-0}" != 1 ]]; then rm -rf -- "${UPDATE_WORKSPACE}"; fi
  [[ -z "${UPDATE_LOCK_DIR:-}" ]] || rmdir -- "${UPDATE_LOCK_DIR}"
  exit "${status}"
}

perform_update() {
  prepare_update_config
  check_update_deployment
  local image i name saved
  for image in "${UPDATE_IMAGES[@]}"; do docker pull "${image}"; done
  # Pulling can take time. Recheck the deployment and pending host removal
  # before taking snapshots and stopping anything.
  check_update_deployment
  backup_update_files
  UPDATE_STOPPED=(); UPDATE_RENAMED=(); UPDATE_SAVED_CONTAINERS=()
  for i in "${!UPDATE_CONTAINERS[@]}"; do
    name="${UPDATE_CONTAINERS[i]}"
    saved="${name}-tp-update-${UPDATE_WORKSPACE##*.}"
    container_exists "${saved}" && { update_error "recovery container name already exists: ${saved}"; return 1; }
    UPDATE_SAVED_CONTAINERS+=("${saved}"); UPDATE_STOPPED+=(0); UPDATE_RENAMED+=(0)
  done
  UPDATE_SWITCH_STARTED=1
  if [[ "${TP_PURPOSE}" == node ]]; then
    UPDATE_HELPER_STOPPED=1
    systemctl stop trojanpanelnext-host.service
    # The helper could have accepted removal while images were being pulled.
    # Its entire systemd control group is now stopped; inspect its latest state
    # and Agent before modifying either. A refusal preserves fresh receipts.
    check_update_deployment
  fi
  UPDATE_PRODUCT_SWITCH_STARTED=1
  for i in "${!UPDATE_CONTAINERS[@]}"; do
    name="${UPDATE_CONTAINERS[i]}"; saved="${UPDATE_SAVED_CONTAINERS[i]}"
    UPDATE_STOPPED[i]=1
    docker stop "${name}" >/dev/null
    docker rename "${name}" "${saved}"
    UPDATE_RENAMED[i]=1
  done
  # Existing runtime files can contain supported pool/log settings and nginx
  # additions that are outside deployment YAML. Keep those files byte-for-byte.
  TP_KEEP_RUNTIME_CONFIG=1
  if [[ "${TP_PURPOSE}" == web ]]; then
    deploy_panel_backend
    deploy_panel_ui
  else
    OLD_NODE_CERTIFICATE_PATH=""; OLD_NODE_PRIVATE_KEY_PATH=""
    deploy_core
  fi
  update_health_check
  # Refuse to overwrite concurrent manual changes to the original YAML.
  cmp -s -- "${UPDATE_BACKUP_CONFIG}" "${UPDATE_ORIGINAL_CONFIG}" || {
    update_error 'original configuration changed during update; restoring containers without overwriting the changed file'; return 1;
  }
  if [[ "${TP_PURPOSE}" == node ]]; then
    TP_ORIGINAL_CONFIG_FILE="${UPDATE_ORIGINAL_CONFIG}"
    TP_DEFER_HOST_SERVICE_START=1
    UPDATE_HELPER_REPLACED=1
    install_host_removal_service
  fi
  cmp -s -- "${UPDATE_BACKUP_CONFIG}" "${UPDATE_ORIGINAL_CONFIG}" || {
    update_error 'original configuration changed while preparing maintenance; restoring without overwriting it'; return 1;
  }
  chmod 600 -- "${UPDATE_STAGED_CONFIG}"
  UPDATE_CONFIG_COMMITTED=1
  mv -T -- "${UPDATE_STAGED_CONFIG}" "${UPDATE_ORIGINAL_CONFIG}"
  if [[ "${TP_PURPOSE}" == node ]]; then
    systemctl restart trojanpanelnext-host.service
    systemctl is-active --quiet trojanpanelnext-host.service
  fi
  UPDATE_COMMITTED=1
  for saved in "${UPDATE_SAVED_CONTAINERS[@]}"; do
    if ! docker rm "${saved}" >/dev/null; then
      printf 'Updated successfully; old stopped container still exists: %s\n' "${saved}" >&2
    fi
  done
  printf 'Updated %s from v%s to v%s. Configuration backup: %s\n' "${TP_PURPOSE}" "${UPDATE_FROM_VERSION}" "${INSTALLER_VERSION}" "${UPDATE_BACKUP_CONFIG}"
  printf 'Old product images are retained. Business database changes are not automatically reversible.\n'
}

main() {
  if handle_metadata "$@"; then return; fi
  read_update_options "$@"
  require_root
  require_commands docker curl openssl yq realpath mktemp install cp mv ln chmod rm cmp date grep awk seq
  require_yq
  docker info >/dev/null
  trap finish_update EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
  # All deployment configurations on this host share the lock, because two
  # YAML files can otherwise name the same product containers.
  local lock=/run/trojanpanelnext-update.lock
  if ! mkdir -m 0700 -- "${lock}"; then
    update_error "another update or stale update lock exists: ${lock}"; return 1
  fi
  UPDATE_LOCK_DIR="${lock}"
  perform_update
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then main "$@"; fi
