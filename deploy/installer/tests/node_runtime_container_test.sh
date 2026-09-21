#!/usr/bin/env bash
set -Eeuo pipefail

INSTALLER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FIXTURES="${INSTALLER_DIR}/tests/fixtures"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

if ! docker info >/dev/null 2>&1; then
  docker() {
    sudo -n /usr/bin/docker "$@"
  }
fi

work="$(mktemp -d)"
suffix="${RANDOM}-$$"
container="tp-core-credential-${suffix}"
image="tp-core-credential-test:${suffix}"

cleanup_test() {
  docker rm -fv "${container}" >/dev/null 2>&1 || true
  docker image rm -f "${image}" >/dev/null 2>&1 || true
  if [[ -n "${TP_SECURE_CONFIG_DIR:-}" && -d "${TP_SECURE_CONFIG_DIR}" ]]; then
    rm -rf -- "${TP_SECURE_CONFIG_DIR}"
  fi
  rm -rf -- "${work}"
}
trap cleanup_test EXIT

docker build --quiet --tag "${image}" --file "${FIXTURES}/Dockerfile.node-runtime-test" "${FIXTURES}" >/dev/null

# shellcheck source=../install.sh
source "${INSTALLER_DIR}/install.sh"
trap cleanup_test EXIT

TP_DATA="${work}/data"
CORE_CONTAINER="${container}"
CORE_IMAGE="${image}"
TP_FORCE=0
TLS_MODE=acme
MARIADB_HOST=db.example.test
MARIADB_PORT=3306
MARIADB_DATABASE=trojan_panel_db
MARIADB_USER=tpn_node_test
MARIADB_PASSWORD='generation-one-mariadb-secret'
REDIS_HOST=redis.example.test
REDIS_PORT=6379
REDIS_USERNAME=tpn-cache-test
REDIS_PASSWORD='generation-one-cache-secret'
REDIS_AUTH_USERNAME=tpn-auth-test
REDIS_AUTH_PASSWORD='generation-one-auth-secret'
NODE_SERVER_ID=42
NODE_IDENTITY_ID=11111111-2222-4333-8444-555555555555
NODE_IDENTITY_GENERATION=1
NODE_BOOTSTRAP_CHALLENGE=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
TP_NODE_DOMAIN=node.example.test
GRPC_PORT=18100
CORE_PORT=18082
GRPC_TLS_MODE=mtls
GRPC_CLIENT_CA_PATH="${TP_DATA}/trojan-panel-core/pki/client-ca.crt"
KERNEL_RUNTIME_PATH="${TP_DATA}/trojan-panel-core/runtime"
MANAGED_CERT_DIR="${TP_DATA}/trojan-panel-core/cert"
EXTERNAL_MANAGED_DIR="${TP_DATA}/trojan-panel-core/external-managed"
EXTERNAL_ROUTES_DIR="${TP_DATA}/trojan-panel-core/external"
WEB_PATH="${TP_DATA}/web"

mkdir -p \
  "${TP_DATA}/trojan-panel-core/bin/xray/config" \
  "${TP_DATA}/trojan-panel-core/bin/naiveproxy/config" \
  "${TP_DATA}/trojan-panel-core/bin/hysteria2/config" \
  "${TP_DATA}/trojan-panel-core/logs" \
  "${TP_DATA}/trojan-panel-core/config" \
  "${TP_DATA}/trojan-panel-core/pki" \
  "${KERNEL_RUNTIME_PATH}" \
  "${TP_DATA}/custom/node-caddy/data" \
  "${MANAGED_CERT_DIR}" \
  "${EXTERNAL_MANAGED_DIR}" \
  "${EXTERNAL_ROUTES_DIR}" \
  "${WEB_PATH}"
printf 'test-ca\n' >"${GRPC_CLIENT_CA_PATH}"

deploy_core "${TP_NODE_DOMAIN}" >/dev/null
first_id="$(docker inspect --format '{{.Id}}' "${container}")"
runtime_config="${TP_DATA}/trojan-panel-core/config/config.ini"
first_digest="$(sha256sum "${runtime_config}" | awk '{print $1}')"
for _ in $(seq 1 30); do
  observed_digest="$(docker exec "${container}" cat /tmp/observed-node-config-sha256 2>/dev/null || true)"
  [[ "${observed_digest}" == "${first_digest}" ]] && break
  sleep 0.1
done
[[ "${observed_digest:-}" == "${first_digest}" ]] || fail 'running Node Agent fixture did not read generation-one credentials from the mounted file'
test "$(stat -c '%a' "${runtime_config}")" = 600 || fail 'Node Agent runtime credential file is not 0600'
docker inspect "${container}" --format '{{range .Config.Env}}{{println .}}{{end}}' >"${work}/env-one"
docker inspect "${container}" --format '{{.Path}} {{json .Args}}' >"${work}/argv-one"
docker logs "${container}" >"${work}/logs-one" 2>&1
for secret in "${MARIADB_PASSWORD}" "${REDIS_PASSWORD}" "${REDIS_AUTH_PASSWORD}"; do
  ! grep -Fq -- "${secret}" "${work}/env-one" "${work}/argv-one" "${work}/logs-one" ||
    fail 'generation-one Node data credential leaked through container metadata or logs'
done
docker inspect "${container}" --format '{{range .Mounts}}{{if eq .Destination "'"${runtime_config}"'"}}{{.RW}}{{end}}{{end}}' |
  grep -Fxq false || fail 'Node Agent credential file mount is not read-only'
if docker exec "${container}" sh -c "printf tampered >>'${runtime_config}'" >/dev/null 2>&1; then
  fail 'running Node Agent could write its credential file mount'
fi

MARIADB_PASSWORD='generation-two-mariadb-secret'
REDIS_PASSWORD='generation-two-cache-secret'
REDIS_AUTH_PASSWORD='generation-two-auth-secret'
NODE_IDENTITY_GENERATION=2
deploy_core "${TP_NODE_DOMAIN}" >/dev/null
second_id="$(docker inspect --format '{{.Id}}' "${container}")"
[[ "${second_id}" != "${first_id}" ]] || fail 'credential generation change did not recreate the running Node Agent'
if docker inspect "${first_id}" >/dev/null 2>&1; then
  fail 'old Node Agent process still exists after credential rotation reconciliation'
fi
second_digest="$(sha256sum "${runtime_config}" | awk '{print $1}')"
for _ in $(seq 1 30); do
  observed_digest="$(docker exec "${container}" cat /tmp/observed-node-config-sha256 2>/dev/null || true)"
  [[ "${observed_digest}" == "${second_digest}" ]] && break
  sleep 0.1
done
[[ "${observed_digest:-}" == "${second_digest}" ]] || fail 'replacement Node Agent did not read generation-two credentials'
docker inspect "${container}" --format '{{range .Config.Env}}{{println .}}{{end}}' >"${work}/env-two"
docker inspect "${container}" --format '{{.Path}} {{json .Args}}' >"${work}/argv-two"
docker logs "${container}" >"${work}/logs-two" 2>&1
for secret in \
  'generation-one-mariadb-secret' 'generation-one-cache-secret' 'generation-one-auth-secret' \
  "${MARIADB_PASSWORD}" "${REDIS_PASSWORD}" "${REDIS_AUTH_PASSWORD}"; do
  ! grep -Fq -- "${secret}" "${work}/env-two" "${work}/argv-two" "${work}/logs-two" ||
    fail 'Node data credential leaked through replacement container metadata or logs'
done

deploy_core "${TP_NODE_DOMAIN}" >/dev/null
third_id="$(docker inspect --format '{{.Id}}' "${container}")"
[[ "${third_id}" == "${second_id}" ]] || fail 'unchanged same-version replay restarted the Node Agent unnecessarily'

printf 'PASS Node Agent credential-file mount and same-version rotation reconciliation\n'
