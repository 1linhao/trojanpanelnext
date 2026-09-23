#!/usr/bin/env bash
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
API_DIR="${ROOT}/apps/control-plane/api"
WEB_DIR="${ROOT}/apps/control-plane/web"
INSTALLER_DIR="${ROOT}/deploy/installer"
SMOKE_HELPERS="${ROOT}/deploy/installer/tests/fixtures/release_web_smoke_helpers.sh"
source "${SMOKE_HELPERS}"

fail() { printf 'FAIL released Web smoke: %s\n' "$1" >&2; exit 1; }
if ! docker info >/dev/null 2>&1; then docker() { sudo -n /usr/bin/docker "$@"; }; fi
docker info >/dev/null 2>&1 || fail 'Docker daemon is required'

work="$(mktemp -d)"
work_root="$(realpath -e "${work}")"
suffix="${RANDOM}-$$"
suffix="${TP_RELEASE_WEB_SMOKE_SUFFIX:-${suffix}}"
registry="tp-web-smoke-registry-${suffix}"
entry="tp-web-smoke-entry-${suffix}"
api_container="tp-web-smoke-api-${suffix}"
ui_container="tp-web-smoke-ui-${suffix}"
mariadb_container="tp-web-smoke-mariadb-${suffix}"
redis_container="tp-web-smoke-redis-${suffix}"
SMOKE_DATA_DIR="${TP_RELEASE_WEB_SMOKE_DATA_DIR:-${work}/tpdata}"
SMOKE_DATA_DIR="$(realpath -m -- "${SMOKE_DATA_DIR}")"
registry_created=0
entry_created=0
api_created=0
ui_created=0
mariadb_created=0
redis_created=0
data_created=0
trap 'rm -rf -- "${work}"' EXIT

# Names include a per-run nonce. Verify them unused before the installer can
# create them, so cleanup only targets containers this invocation owns.
case "${SMOKE_DATA_DIR}" in
  "${work_root}"/*) ;;
  *) fail "smoke data directory must be inside the per-run temporary workspace: ${SMOKE_DATA_DIR}" ;;
esac
[[ ! -e "${SMOKE_DATA_DIR}" && ! -L "${SMOKE_DATA_DIR}" ]] ||
  fail "smoke data directory is unexpectedly occupied: ${SMOKE_DATA_DIR}"
for name in "${registry}" "${entry}" "${api_container}" "${ui_container}" "${mariadb_container}" "${redis_container}"; do
  if docker inspect "${name}" >/dev/null 2>&1; then
    fail "smoke container name is unexpectedly occupied: ${name}"
  fi
done
mkdir -m 0755 "${SMOKE_DATA_DIR}"
data_created=1

cleanup() {
  [[ "${entry_created}" == 0 ]] || docker rm -fv "${entry}" >/dev/null 2>&1 || true
  [[ "${registry_created}" == 0 ]] || docker rm -fv "${registry}" >/dev/null 2>&1 || true
  [[ "${api_created}" == 0 ]] || docker rm -fv "${api_container}" >/dev/null 2>&1 || true
  [[ "${ui_created}" == 0 ]] || docker rm -fv "${ui_container}" >/dev/null 2>&1 || true
  [[ "${mariadb_created}" == 0 ]] || docker rm -fv "${mariadb_container}" >/dev/null 2>&1 || true
  [[ "${redis_created}" == 0 ]] || docker rm -fv "${redis_container}" >/dev/null 2>&1 || true
  [[ "${data_created}" == 0 ]] || sudo -n rm -rf -- "${SMOKE_DATA_DIR}"
  rm -rf -- "${work}"
}
trap cleanup EXIT

registry_created=1
docker run -d --name "${registry}" -p 127.0.0.1::5000 registry@sha256:a3d8aaa63ed8681a604f1dea0aa03f100d5895b6a58ace528858a7b332415373 >/dev/null
registry_port="$(docker inspect --format '{{(index (index .NetworkSettings.Ports "5000/tcp") 0).HostPort}}' "${registry}")"
for _ in $(seq 1 30); do curl -fsS "http://127.0.0.1:${registry_port}/v2/" >/dev/null 2>&1 && break; sleep 1; done
curl -fsS "http://127.0.0.1:${registry_port}/v2/" >/dev/null || fail 'local registry did not become ready'

push_image() {
  local source="$1" name="$2" tag="127.0.0.1:${registry_port}/${2}:smoke"
  docker tag "${source}" "${tag}"
  docker push "${tag}" >"${work}/${name}.push" 2>&1 || fail "could not push ${name}"
  local digest
  digest="$(awk '/digest: sha256:/ {print $3}' "${work}/${name}.push" | tail -n1)"
  [[ "${digest}" =~ ^sha256:[0-9a-f]{64}$ ]] || fail "registry omitted ${name} digest"
  printf '127.0.0.1:%s/%s@%s\n' "${registry_port}" "${name}" "${digest}"
}

(cd "${API_DIR}" && mkdir -p build && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o build/trojan-panel-linux-amd64 .)
docker build -q --build-arg TARGETOS=linux --build-arg TARGETARCH=amd64 -t "tp-web-api-${suffix}" "${API_DIR}" >/dev/null
cat >"${work}/api-runtime.Dockerfile" <<EOF
FROM tp-web-api-${suffix}
ARG TP_DATA
RUN mkdir -p "\${TP_DATA}/trojan-panel" && cp /tpdata/trojan-panel/trojan-panel "\${TP_DATA}/trojan-panel/trojan-panel"
WORKDIR \${TP_DATA}/trojan-panel/
EOF
docker build -q --build-arg TP_DATA="${SMOKE_DATA_DIR}" -f "${work}/api-runtime.Dockerfile" \
  -t "tp-web-api-runtime-${suffix}" "${API_DIR}" >/dev/null
(cd "${WEB_DIR}" && npx --yes yarn@1.22.22 install --frozen-lockfile && npx --yes yarn@1.22.22 build) >/dev/null
docker build -q -t "tp-web-ui-${suffix}" "${WEB_DIR}" >/dev/null

admin_password='SmokeAdminPass1234'
mkdir -p "${work}/mariadb"
cp "${API_DIR}/resource/sql/trojan_panel_db_v2.3.0.sql" "${work}/mariadb/schema.sql"
cat >"${work}/mariadb/Dockerfile" <<'EOF'
FROM mariadb@sha256:07e06f2e7ae9dfc63707a83130a62e00167c827f08fcac7a9aa33f4b6dc34e0e
COPY schema.sql /docker-entrypoint-initdb.d/00-schema.sql
EOF
docker build -q -t "tp-web-mariadb-${suffix}" "${work}/mariadb" >/dev/null
api_image="$(push_image "tp-web-api-runtime-${suffix}" tpn-api)"
web_image="$(push_image "tp-web-ui-${suffix}" tpn-web)"
mariadb_image="$(push_image "tp-web-mariadb-${suffix}" tpn-mariadb)"
docker pull redis@sha256:a93c14584715ec5bd9d2648d58c3b27f89416242bee0bc9e5fb2edc1a4cbec1d >/dev/null
redis_image="$(push_image redis@sha256:a93c14584715ec5bd9d2648d58c3b27f89416242bee0bc9e5fb2edc1a4cbec1d tpn-redis)"

# This is a real external TLS entry. The test CA is trusted through
# CURL_CA_BUNDLE, so the installer still performs certificate verification.
mkdir -p "${work}/entry"
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj /CN=localhost -addext 'subjectAltName=DNS:localhost' -keyout "${work}/entry/key" -out "${work}/entry/cert" >/dev/null 2>&1
cat >"${work}/entry/Caddyfile" <<'EOF'
localhost {
  tls /etc/caddy/cert/cert /etc/caddy/cert/key
  reverse_proxy 127.0.0.1:8888
}
EOF
entry_created=1
docker run -d --name "${entry}" --network host -v "${work}/entry/Caddyfile:/etc/caddy/Caddyfile:ro" -v "${work}/entry:/etc/caddy/cert:ro" caddy:2.8.4 >/dev/null

assets="${work}/assets"
"${INSTALLER_DIR}/release/generate-assets.sh" --version 1.2.3 --source-commit 0123456789abcdef0123456789abcdef01234567 --output "${assets}" --api-image "${api_image}" --web-image "${web_image}" --node-agent-image example.invalid/tpn-node@sha256:3333333333333333333333333333333333333333333333333333333333333333 --caddy-image caddy@sha256:4444444444444444444444444444444444444444444444444444444444444444 --mariadb-image "${mariadb_image}" --redis-image "${redis_image}" >/dev/null
config="${work}/web.yaml"
cp "${assets}/config-web.yaml" "${config}"
sed -i \
  -e 's/hostname: panel.example.com/hostname: localhost/' \
  -e 's#^  grpc_client_cert_path: .*#  grpc_client_cert_path: '"${SMOKE_DATA_DIR}"'/trojan-panel/pki/client.crt#' \
  -e 's#^  grpc_client_key_path: .*#  grpc_client_key_path: '"${SMOKE_DATA_DIR}"'/trojan-panel/pki/client.key#' \
  -e 's#^  pki_bundle_dir: .*#  pki_bundle_dir: '"${SMOKE_DATA_DIR}"'/trojanpanelnext-pki#' \
  -e 's/^  sysadmin_password: ""/  sysadmin_password: "SmokeAdminPass1234"/' \
  "${config}"
printf '  tls_mode: external\n' >>"${config}"
chmod 0600 "${config}"

mkdir -p "${work}/tools"
curl -fsSL https://github.com/mikefarah/yq/releases/download/v4.53.6/yq_linux_amd64 -o "${work}/tools/yq"
printf '%s  %s\n' c5f056448f973ae7d39b5401949648a78f2dc1947d6a8eb65be60d5c504b9385 "${work}/tools/yq" | sha256sum -c - >/dev/null
chmod 0755 "${work}/tools/yq"
printf '#!/bin/sh\nexit 0\n' >"${work}/tools/age"; chmod 0755 "${work}/tools/age"
printf 'ID=debian\nVERSION_ID="12"\n' >"${work}/debian-12"
docker_hook='() {
  if [[ "${TP_RELEASE_WEB_SMOKE_INJECT_ADMIN_FAILURE:-0}" == 1 &&
    "${1:-}" == exec && " $* " == *" TP_VERIFY_SYSADMIN_CREDENTIAL=1 "* ]]; then
    printf "%s\\n" WrongSmokeAdmin9876 >"${TP_DATA}/trojan-panel/config/initial-admin-password"
    chmod 0600 "${TP_DATA}/trojan-panel/config/initial-admin-password"
  fi
  /usr/bin/docker "$@"
}'

run_installer() {
  local inject_admin_failure="$1"
  local stdout_file="$2"
  local stderr_file="$3"
  local -a docker_hook_env=("BASH_FUNC_docker%%=${docker_hook}")

  sudo -n env "${docker_hook_env[@]}" \
    PATH="${work}/tools:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin" \
    CURL_CA_BUNDLE="${work}/entry/cert" \
    TP_DATA="${SMOKE_DATA_DIR}" \
    TP_RELEASE_WEB_SMOKE_INJECT_ADMIN_FAILURE="${inject_admin_failure}" \
    TP_INSTALL_DEPS=0 TP_OS_RELEASE_FILE="${work}/debian-12" \
    TP_HEALTH_ATTEMPTS=30 TP_HEALTH_DELAY_SECONDS=1 \
    PANEL_CONTAINER="${api_container}" UI_CONTAINER="${ui_container}" \
    MARIADB_CONTAINER="${mariadb_container}" REDIS_CONTAINER="${redis_container}" \
    "${assets}/install.sh" install --mode web --config "${config}" \
    >"${stdout_file}" 2>"${stderr_file}"
}

# All product container names were confirmed absent above and are unique to
# this run; from this point the installer may create them, so cleanup owns them.
api_created=1
ui_created=1
mariadb_created=1
redis_created=1
if run_installer 1 "${work}/injected-failure.out" "${work}/injected-failure.err"; then
  fail 'injected unhealthy administrator credential unexpectedly passed'
fi
docker logs "${api_container}" >"${work}/api.log" 2>&1 || true
sudo -n bash -c \
  'source "$1"; smoke_print_install_failure_diagnostics "$2" "$3" "$4" "$5"' \
  _ "${SMOKE_HELPERS}" "${config}" "${work}/injected-failure.out" \
  "${work}/injected-failure.err" "${work}/api.log" \
  >"${work}/failure-diagnostics" 2>&1
grep -Fq 'Health check failed: sysadmin container credential' "${work}/failure-diagnostics" ||
  fail 'injected unhealthy credential lacked a useful health diagnostic'
grep -Fq "Check container ${api_container}" "${work}/failure-diagnostics" ||
  fail 'injected unhealthy credential lacked an API container locator'
sudo -n bash -c \
  'source "$1"; smoke_assert_diagnostics_sanitized "$2" "$3"' \
  _ "${SMOKE_HELPERS}" "${config}" "${work}/failure-diagnostics" ||
  fail 'injected failure diagnostics leaked a configured credential'

if ! run_installer 0 "${work}/install.out" "${work}/install.err"; then
  docker logs "${api_container}" >"${work}/api.log" 2>&1 || true
  sudo -n bash -c \
    'source "$1"; smoke_print_install_failure_diagnostics "$2" "$3" "$4" "$5"' \
    _ "${SMOKE_HELPERS}" "${config}" "${work}/install.out" "${work}/install.err" "${work}/api.log"
  fail 'formal release installer failed after health failure injection'
fi
grep -Fq 'Web control plane is healthy' "${work}/install.out" || fail 'health success marker missing'
for probe in MariaDB Redis 'Web HTTPS' 'sysadmin container credential'; do grep -Fq "Health check passed: ${probe}" "${work}/install.out" || fail "health evidence missing: ${probe}"; done
login=""
for _ in $(seq 1 20); do
  if login="$(curl --fail --silent --show-error --cacert "${work}/entry/cert" -H 'content-type: application/json' --data "{\"username\":\"sysadmin\",\"pass\":\"${admin_password}\"}" https://localhost/api/auth/login 2>/dev/null)"; then break; fi
  sleep 1
done
grep -Fq '"token"' <<<"${login}" || fail 'administrator login through TLS entry did not return a token'
sudo -n bash -c \
  'source "$1"; smoke_assert_diagnostics_sanitized "$2" "$3"' \
  _ "${SMOKE_HELPERS}" "${config}" "${work}/install.out" ||
  fail 'successful installer output leaked a configured credential'
sudo -n bash -c \
  'source "$1"; smoke_assert_diagnostics_sanitized "$2" "$3"' \
  _ "${SMOKE_HELPERS}" "${config}" "${work}/install.err" ||
  fail 'successful installer error output leaked a configured credential'
printf '%s\n' 'PASS released Web Docker smoke (formal installer, real TLS, MariaDB, Redis, and administrator API)'
