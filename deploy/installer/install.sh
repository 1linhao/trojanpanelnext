#!/usr/bin/env bash
set -euo pipefail

SCRIPT_VERSION="0.1.0-rc.6"
TP_SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if [[ ! -f "${TP_SCRIPT_DIR}/common.sh" ]]; then
  printf 'Missing common.sh. Use tp.sh to download the command and its dependencies.\n' >&2
  exit 1
fi
# shellcheck source=deploy/installer/common.sh
source "${TP_SCRIPT_DIR}/common.sh"
require_matching_version "${SCRIPT_VERSION}"

usage() {
  cat <<EOF
TrojanPanel Next installation ${INSTALLER_VERSION}
Usage: $0 --config <file> [--force]
  --config <file>  YAML deployment configuration; purpose selects web or node
  --force          Recreate API, UI, Agent and Caddy containers
  -V, --version    Show version
  -h, --help       Show help
Requires root, Docker, mikefarah/yq v4, curl, OpenSSL, tar,
coreutils, findutils and awk. Install these dependencies before use.
Use tp.sh config, validate or remove for the other commands.
EOF
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
    -addext 'basicConstraints=critical,CA:TRUE' \
    -addext 'keyUsage=critical,keyCertSign,cRLSign' \
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
    # Preserve the live trust bundle maintained by the Agent during rotation.
    if [[ -f "${GRPC_CLIENT_CA_PATH}" ]]; then
      mkdir -p "${TP_PKI_BUNDLE_DIR}"
      chmod 700 "${TP_PKI_BUNDLE_DIR}"
      if [[ "${GRPC_CLIENT_CA_PATH}" != "${TP_PKI_BUNDLE_DIR}/client-ca.crt" ]]; then
        install -m 0644 "${GRPC_CLIENT_CA_PATH}" "${TP_PKI_BUNDLE_DIR}/client-ca.crt"
      fi
      return
    fi
    if [[ ! -f "${TP_PKI_BUNDLE_DIR}/client-ca.crt" ]]; then
      echo_content red "Missing control-plane CA: ${TP_PKI_BUNDLE_DIR}/client-ca.crt"
      exit 1
    fi
    mkdir -p "$(dirname "${GRPC_CLIENT_CA_PATH}")"
    chmod 700 "${TP_PKI_BUNDLE_DIR}"
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
    -v "${TP_PKI_BUNDLE_DIR}:${TP_PKI_BUNDLE_DIR}" \
    -v /etc/localtime:/etc/localtime \
    -e GIN_MODE=release \
    -e "TP_HOST_REMOVAL_CALLBACK_URL=https://${TP_WEB_DOMAIN}/api/nodeServer/completeHostRemoval" \
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
    -e "TP_PKI_AUTHORITY_DIR=${TP_PKI_BUNDLE_DIR}" \
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
  local ca_directory
  ca_directory="$(dirname "${GRPC_CLIENT_CA_PATH}")"
  local -a pki_mounts=(-v "${TP_PKI_BUNDLE_DIR}:${TP_PKI_BUNDLE_DIR}")
  if [[ "${ca_directory}" != "${TP_PKI_BUNDLE_DIR}" ]]; then
    pki_mounts+=(-v "${ca_directory}:${ca_directory}")
  fi

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
    "${pki_mounts[@]}" \
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
    -e "TP_PKI_BOOTSTRAP_CA_PATH=${TP_PKI_BUNDLE_DIR}/client-ca.crt" \
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
  require_commands systemctl
  if [[ ! -d /run/systemd/system || ! -f "${TP_SCRIPT_DIR}/uninstall.sh" ]]; then
    echo_content red "Node installation requires systemd and matching uninstall.sh; use tp.sh install" >&2
    exit 1
  fi

  load_image_archives
  prepare_dirs
  install_pki_material node
  prepare_static_web
  write_node_caddyfile "${TP_NODE_DOMAIN}"
  start_caddy "${NODE_CADDY_CONTAINER}" "${TP_DATA}/custom/node-caddy" "${TP_DATA}/custom/node-caddy/data" "${WEB_PATH}"
  wait_for_cert "${TP_NODE_DOMAIN}" "${TP_DATA}/custom/node-caddy/data"
  deploy_core "${TP_NODE_DOMAIN}"
  install_host_removal_service

  echo_content red "\n=============================================================="
  echo_content skyBlue "Trojan Panel node side deployed"
  echo_content yellow "Node domain: ${TP_NODE_DOMAIN}"
  echo_content yellow "Core gRPC port: ${GRPC_PORT}"
  echo_content yellow "Core API port: ${CORE_PORT}"
  echo_content red "==============================================================\n"
}

install_host_removal_service() {
  local host_dir=/etc/trojanpanelnext-host
  local library_dir=/usr/local/lib/trojanpanelnext-host
  # Stop an existing helper before replacing its executable or persisted state.
  if [[ -f /etc/systemd/system/trojanpanelnext-host.service ]]; then
    systemctl stop trojanpanelnext-host.service
  fi
  mkdir -p "${host_dir}" "${library_dir}"
  chmod 700 "${host_dir}" "${library_dir}"
  install -m 0600 "${TP_SCRIPT_DIR}/common.sh" "${library_dir}/common.sh"
  install -m 0600 "${TP_SCRIPT_DIR}/uninstall.sh" "${library_dir}/uninstall.sh"
  install -m 0600 "${TP_CONFIG_FILE}" "${host_dir}/node.yaml"
  docker cp "${CORE_CONTAINER}:/usr/local/bin/tp-host-agent" "${library_dir}/tp-host-agent"
  chmod 700 "${library_dir}/tp-host-agent"
  local cert_base="${TP_DATA}/custom/node-caddy/data/caddy/certificates/acme-v02.api.letsencrypt.org-directory/${TP_NODE_DOMAIN}/${TP_NODE_DOMAIN}"
  local original_config
  original_config="$(realpath -- "${TP_CONFIG_FILE}")"
  export NODE_SERVER_ID GRPC_PORT GRPC_CLIENT_CA_PATH TP_DATA WEB_PATH TP_PKI_BUNDLE_DIR KERNEL_RUNTIME_PATH
  export MARIADB_CONTAINER REDIS_CONTAINER PANEL_CONTAINER UI_CONTAINER CORE_CONTAINER WEB_CADDY_CONTAINER NODE_CADDY_CONTAINER
  TP_HOST_CERT="${cert_base}.crt" TP_HOST_KEY="${cert_base}.key" TP_ORIGINAL_CONFIG="${original_config}" \
    yq -n -o=json '{
      "nodeId": (strenv(NODE_SERVER_ID) | tonumber),
      "port": ((strenv(GRPC_PORT) | tonumber) + 1),
      "certificate": strenv(TP_HOST_CERT), "key": strenv(TP_HOST_KEY),
      "clientCA": strenv(GRPC_CLIENT_CA_PATH), "originalConfig": strenv(TP_ORIGINAL_CONFIG),
      "environment": {
        "TP_DATA": strenv(TP_DATA), "WEB_PATH": strenv(WEB_PATH),
        "TP_PKI_BUNDLE_DIR": strenv(TP_PKI_BUNDLE_DIR), "KERNEL_RUNTIME_PATH": strenv(KERNEL_RUNTIME_PATH),
        "MARIADB_CONTAINER": strenv(MARIADB_CONTAINER), "REDIS_CONTAINER": strenv(REDIS_CONTAINER),
        "PANEL_CONTAINER": strenv(PANEL_CONTAINER), "UI_CONTAINER": strenv(UI_CONTAINER),
        "CORE_CONTAINER": strenv(CORE_CONTAINER), "WEB_CADDY_CONTAINER": strenv(WEB_CADDY_CONTAINER),
        "NODE_CADDY_CONTAINER": strenv(NODE_CADDY_CONTAINER)
      }
    }' >"${host_dir}/config.json"
  chmod 600 "${host_dir}/config.json"
  # A fresh installation supersedes any completed removal receipt.
  rm -f -- "${host_dir}/result.json" "${host_dir}/finalize.json" "${host_dir}/server.crt" "${host_dir}/server.key" "${host_dir}/client-ca.crt" "${library_dir}/cleanup-ready"
  cat >/etc/systemd/system/trojanpanelnext-host.service <<'EOF'
[Unit]
Description=TrojanPanel Next authenticated host removal
After=network-online.target docker.service
Wants=network-online.target
[Service]
Type=simple
ExecStart=/usr/local/lib/trojanpanelnext-host/tp-host-agent
Restart=on-failure
RestartSec=5
UMask=0077
[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable trojanpanelnext-host.service
  systemctl restart trojanpanelnext-host.service
  systemctl is-active --quiet trojanpanelnext-host.service
}
main() {
  if handle_metadata "$@"; then return; fi
  parse_config_options install "$@"
  validate_config
  echo_content skyBlue "Operation: install; purpose: ${TP_PURPOSE}; config: ${TP_CONFIG_FILE}; installer: ${INSTALLER_VERSION}"
  require_commands docker curl openssl tar od sha256sum find seq awk
  docker info >/dev/null
  case "${TP_PURPOSE}" in
  web) deploy_web ;;
  node) deploy_node ;;
  esac
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
