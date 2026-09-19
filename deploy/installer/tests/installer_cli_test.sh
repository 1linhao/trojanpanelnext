#!/usr/bin/env bash
set -Eeuo pipefail

INSTALLER="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/install.sh"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

assert_fails() {
  if "$@" >/dev/null 2>&1; then
    fail "command unexpectedly succeeded: $*"
  fi
}

# Minimal yq-compatible reader for the flat example schema. This keeps the CLI
# contract test hermetic; production installations use mikefarah/yq.
yq() {
  local operation="$1"
  local expression="$2"
  local file="$3"

  if [[ "${operation}" == -e ]]; then
    grep -q '^trojanpanelnext:' "${file}"
    return
  fi
  [[ "${operation}" == -r ]] || return 2

  local key value
  key="${expression#.trojanpanelnext.}"
  key="${key%% *}"
  value="$(awk -F: -v key="${key}" '
    $1 ~ "^[[:space:]]+" key "$" {
      sub(/^[^:]*:[[:space:]]*/, "")
      sub(/[[:space:]]+#.*$/, "")
      gsub(/^"|"$/, "")
      print
      exit
    }
  ' "${file}")"
  printf '%s\n' "${value}"
}
export -f yq

bash -n "${INSTALLER}"
"${INSTALLER}" --help | grep -q 'install.*--mode web|node'
"${INSTALLER}" --help | grep -q 'refresh-cert.*--mode node'
assert_fails "${INSTALLER}" deploy --mode web --config /dev/null
assert_fails "${INSTALLER}" validate --mode invalid --config /dev/null
assert_fails "${INSTALLER}" validate --mode web
assert_fails "${INSTALLER}" validate --mode web --config /dev/null --force
assert_fails "${INSTALLER}" install --mode web --config /dev/null --purge-data
assert_fails "${INSTALLER}" install --mode web --config /dev/null --keep-data
assert_fails "${INSTALLER}" remove --mode web --config /dev/null --purge-data --keep-data

"${INSTALLER}" validate --mode web \
  --config "$(dirname "${INSTALLER}")/examples/web.yaml" | grep -q 'valid for web deployment mode'
"${INSTALLER}" validate --mode node \
  --config "$(dirname "${INSTALLER}")/examples/node-agent.yaml" | grep -q 'valid for node deployment mode'
assert_fails "${INSTALLER}" validate --mode node \
  --config "$(dirname "${INSTALLER}")/examples/web.yaml"

legacy_config="$(mktemp)"
missing_mode_config="$(mktemp)"
canonical_config="$(mktemp)"
legacy_key_config="$(mktemp)"
conflicting_mode_config="$(mktemp)"
pki_dir="$(mktemp -d)"
node_pki_dir="$(mktemp -d)"
node_runtime_dir="$(mktemp -d)"
path_contract_dir="$(mktemp -d)"
trap 'rm -f "${legacy_config}" "${missing_mode_config}" "${canonical_config}" "${legacy_key_config}" "${conflicting_mode_config}"; rm -rf -- "${pki_dir}" "${node_pki_dir}" "${node_runtime_dir}" "${path_contract_dir}"' EXIT

ln -s "$(command -v bash)" "${path_contract_dir}/bash"
PATH="${path_contract_dir}" "${INSTALLER}" --help >/dev/null ||
  fail 'development installer no longer expands a bash-only caller PATH with system directories'

path_trace="${path_contract_dir}/caller-command.trace"
cat >"${path_contract_dir}/dirname" <<'EOF'
#!/usr/bin/env bash
printf 'called\n' >"${PATH_CONTRACT_TRACE}"
exit 91
EOF
chmod +x "${path_contract_dir}/dirname"
PATH="${path_contract_dir}" PATH_CONTRACT_TRACE="${path_trace}" "${INSTALLER}" --help >/dev/null ||
  fail 'development installer allowed a caller command to shadow the system dirname'
test ! -e "${path_trace}" || fail 'caller dirname shadowed the system command'

sed 's/grpc_tls_mode: mtls/grpc_tls_mode: legacy/' \
  "$(dirname "${INSTALLER}")/examples/node-agent.yaml" >"${legacy_config}"
sed '/deployment_mode: web/d' "$(dirname "${INSTALLER}")/examples/web.yaml" >"${missing_mode_config}"
assert_fails "${INSTALLER}" validate --mode node --config "${legacy_config}"
assert_fails env TP_DEPLOYMENT_MODE=web "${INSTALLER}" validate --mode web --config "${missing_mode_config}"
cp "$(dirname "${INSTALLER}")/examples/web.yaml" "${canonical_config}"
"${INSTALLER}" validate --mode web --config "${canonical_config}" |
  grep -q 'valid for web deployment mode'
sed \
  -e 's/^  deployment_mode:/  purpose:/' \
  -e 's/^  api_image:/  panel_image:/' \
  -e 's/^  web_image:/  ui_image:/' \
  -e 's/^  node_agent_image:/  core_image:/' \
  "${canonical_config}" >"${legacy_key_config}"
"${INSTALLER}" validate --mode web --config "${legacy_key_config}" |
  grep -q 'valid for web deployment mode'
sed '/deployment_mode: web/a\  purpose: node' "${canonical_config}" >"${conflicting_mode_config}"
assert_fails "${INSTALLER}" validate --mode web --config "${conflicting_mode_config}"

bash -c 'set -Eeuo pipefail; source "$1"; TP_PKI_BUNDLE_DIR="$2"; generate_web_client_pki' \
  installer-test "${INSTALLER}" "${pki_dir}"
openssl verify -CAfile "${pki_dir}/client-ca.crt" "${pki_dir}/client.crt" >/dev/null
test "$(stat -c '%a' "${pki_dir}/client-ca.key")" = 600
test "$(stat -c '%a' "${pki_dir}/client.key")" = 600
assert_fails bash -c 'set -Eeuo pipefail; source "$1"; TP_PKI_BUNDLE_DIR="$2"; GRPC_CLIENT_CA_PATH="$3/client-ca.crt"; install_pki_material node' \
  installer-test "${INSTALLER}" "${node_pki_dir}" "${node_runtime_dir}"
cp "${pki_dir}/client-ca.crt" "${node_pki_dir}/client-ca.crt"
bash -c 'set -Eeuo pipefail; source "$1"; TP_PKI_BUNDLE_DIR="$2"; GRPC_CLIENT_CA_PATH="$3/client-ca.crt"; install_pki_material node' \
  installer-test "${INSTALLER}" "${node_pki_dir}" "${node_runtime_dir}"
cmp "${pki_dir}/client-ca.crt" "${node_runtime_dir}/client-ca.crt"

# --- external TLS mode -------------------------------------------------------
EXAMPLES="$(dirname "${INSTALLER}")/examples"

external_cases_dir="$(mktemp -d)"
external_tls_dir="$(mktemp -d)"
external_pairs_dir="$(mktemp -d)"
external_data_dir="$(mktemp -d)"
external_mismatch_dir="$(mktemp -d)"
external_wrong_domain_dir="$(mktemp -d)"
external_refresh_dir="$(mktemp -d)"
entry_spec="$(mktemp)"
entry_trace="$(mktemp)"
fake_entryctl="$(mktemp)"
trap 'rm -f "${legacy_config}" "${missing_mode_config}" "${canonical_config}" "${legacy_key_config}" "${conflicting_mode_config}" "${entry_spec}" "${entry_trace}" "${fake_entryctl}"; rm -rf -- "${pki_dir}" "${node_pki_dir}" "${node_runtime_dir}" "${path_contract_dir}" "${external_cases_dir}" "${external_tls_dir}" "${external_pairs_dir}" "${external_data_dir}" "${external_mismatch_dir}" "${external_wrong_domain_dir}" "${external_refresh_dir}"' EXIT

# The shipped template points at a real external certificate directory, which
# cannot exist on a test host. Validate the template against a local pair.
openssl req -x509 -newkey rsa:2048 -sha256 -days 3650 -nodes \
  -subj "/CN=node.example.com" \
  -keyout "${external_tls_dir}/privkey.pem" -out "${external_tls_dir}/fullchain.pem" >/dev/null 2>&1
sed "s#tls_cert_dir: /etc/vps-factory/certs/node.example.com#tls_cert_dir: ${external_tls_dir}#" \
  "${EXAMPLES}/external-node.yaml" >"${external_cases_dir}/ready.yaml"

jq -n '{
  schema_version: 1, revision: 1, deployment_id: "trojanpanelnext-node",
  provider: "external", purpose: "node", domain: "node.example.com",
  certificate: {
    managed_dir: "/tpdata/trojan-panel-core/cert",
    source_dir: "/etc/vps-factory/certs/node.example.com"
  },
  ingress: {
    node_exposure: "direct",
    route_manifest: "/tpdata/trojan-panel-core/external/routes.json",
    fallbacks: []
  },
  external_driver: {
    protocol_version: 1,
    path: "/usr/local/libexec/trojanpanelnext-entry-provider"
  }
}' >"${entry_spec}"
chmod 0600 "${entry_spec}"

"${INSTALLER}" validate --mode node --config "${external_cases_dir}/ready.yaml" | grep -q 'valid for node deployment mode'
"${INSTALLER}" validate --mode node --config "${external_cases_dir}/ready.yaml" \
  --entry-spec "${entry_spec}" | grep -q 'valid for node deployment mode'
assert_fails "${INSTALLER}" validate --mode web --config "${EXAMPLES}/external-web.yaml" \
  --entry-spec "${entry_spec}"
assert_fails "${INSTALLER}" validate --mode node --config "${EXAMPLES}/node-agent.yaml" \
  --entry-spec "${entry_spec}"
assert_fails "${INSTALLER}" refresh-cert --mode node --config "${external_cases_dir}/ready.yaml" \
  --entry-spec "${entry_spec}"
"${INSTALLER}" validate --mode node --config "${external_cases_dir}/ready.yaml" | grep -q 'External TLS material'
"${INSTALLER}" validate --mode web --config "${EXAMPLES}/external-web.yaml" | grep -q 'valid for web deployment mode'
assert_fails "${INSTALLER}" validate --mode web --config "${external_cases_dir}/ready.yaml"

cat >"${fake_entryctl}" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >>"${ENTRY_TEST_TRACE}"
EOF
chmod 0700 "${fake_entryctl}"
ENTRY_TEST_TRACE="${entry_trace}" ENTRYCTL_PATH="${fake_entryctl}" ENTRY_SPEC_FILE="${entry_spec}" \
  bash -c 'set -Eeuo pipefail; source "$1"; TP_PURGE_DATA=0; entry_controller reconcile; entry_controller remove; TP_PURGE_DATA=1; entry_controller remove' \
  installer-test "${INSTALLER}"
grep -Fxq "reconcile --spec ${entry_spec}" "${entry_trace}"
test "$(grep -Fxc "remove --spec ${entry_spec}" "${entry_trace}")" = 1
grep -Fxq "remove --spec ${entry_spec} --purge" "${entry_trace}"

# Defaults stay acme, so the pre-existing templates must keep validating.
"${INSTALLER}" validate --mode web --config "${EXAMPLES}/web.yaml" | grep -q 'valid for web deployment mode'
"${INSTALLER}" validate --mode node --config "${EXAMPLES}/node-agent.yaml" | grep -q 'valid for node deployment mode'

sed '/tls_cert_dir:/d' "${EXAMPLES}/external-node.yaml" >"${external_cases_dir}/missing-cert-dir.yaml"
sed "s#tls_cert_dir: /etc/vps-factory/certs/node.example.com#tls_cert_dir: ${external_tls_dir}#; s/tls_mode: external/tls_mode: bogus/" \
  "${EXAMPLES}/external-node.yaml" >"${external_cases_dir}/bogus-tls-mode.yaml"
sed "s#tls_cert_dir: /etc/vps-factory/certs/node.example.com#tls_cert_dir: ${external_tls_dir}#; s/bind_address: 127.0.0.1/bind_address: not-an-ip/" \
  "${EXAMPLES}/external-node.yaml" >"${external_cases_dir}/bogus-bind.yaml"
sed "s#tls_cert_dir: /etc/vps-factory/certs/node.example.com#tls_cert_dir: ${external_cases_dir}#" \
  "${EXAMPLES}/external-node.yaml" >"${external_cases_dir}/empty-cert-dir.yaml"
assert_fails "${INSTALLER}" validate --mode node --config "${external_cases_dir}/missing-cert-dir.yaml"
assert_fails "${INSTALLER}" validate --mode node --config "${external_cases_dir}/bogus-tls-mode.yaml"
assert_fails "${INSTALLER}" validate --mode node --config "${external_cases_dir}/bogus-bind.yaml"
sed "s#tls_cert_dir: /etc/vps-factory/certs/node.example.com#tls_cert_dir: ${external_tls_dir}#; s/bind_address: 127.0.0.1/bind_address: dead/" \
  "${EXAMPLES}/external-node.yaml" >"${external_cases_dir}/hex-word-bind.yaml"
assert_fails "${INSTALLER}" validate --mode node --config "${external_cases_dir}/hex-word-bind.yaml"
sed "s#tls_cert_dir: /etc/vps-factory/certs/node.example.com#tls_cert_dir: ${external_tls_dir}#; s/bind_address: 127.0.0.1/bind_address: \"::1\"/" \
  "${EXAMPLES}/external-node.yaml" >"${external_cases_dir}/ipv6-bind.yaml"
"${INSTALLER}" validate --mode node --config "${external_cases_dir}/ipv6-bind.yaml" | grep -q 'valid for node deployment mode'
# A tls_cert_dir that exists but holds no pair must fail the same way.
assert_fails "${INSTALLER}" validate --mode node --config "${external_cases_dir}/empty-cert-dir.yaml"

# Validation rejects syntactically valid but mismatched key material and a
# certificate that does not cover the configured node hostname.
cp "${external_tls_dir}/fullchain.pem" "${external_mismatch_dir}/fullchain.pem"
openssl req -x509 -newkey rsa:2048 -sha256 -days 3650 -nodes \
  -subj "/CN=other.example.com" \
  -keyout "${external_mismatch_dir}/privkey.pem" -out "${external_wrong_domain_dir}/fullchain.pem" >/dev/null 2>&1
cp "${external_mismatch_dir}/privkey.pem" "${external_wrong_domain_dir}/privkey.pem"
sed "s#tls_cert_dir: /etc/vps-factory/certs/node.example.com#tls_cert_dir: ${external_mismatch_dir}#" \
  "${EXAMPLES}/external-node.yaml" >"${external_cases_dir}/mismatched-key.yaml"
sed "s#tls_cert_dir: /etc/vps-factory/certs/node.example.com#tls_cert_dir: ${external_wrong_domain_dir}#" \
  "${EXAMPLES}/external-node.yaml" >"${external_cases_dir}/wrong-domain.yaml"
assert_fails "${INSTALLER}" validate --mode node --config "${external_cases_dir}/mismatched-key.yaml"
assert_fails "${INSTALLER}" validate --mode node --config "${external_cases_dir}/wrong-domain.yaml"

# Nested (named directory) layout and same-stem layout.
mkdir -p "${external_pairs_dir}/site"
openssl req -x509 -newkey rsa:2048 -sha256 -days 3650 -nodes -subj "/CN=node.example.com" \
  -keyout "${external_pairs_dir}/site/server.key" -out "${external_pairs_dir}/site/server.crt" >/dev/null 2>&1

read -r -d '' external_probe <<'EOS' || true
set -Eeuo pipefail
source "$1"
PROBE_ROOT="$2"
TLS_CERT_DIR="${PROBE_CERT_DIR}"
TLS_CERT_FILE="${PROBE_CERT_FILE:-}"
TLS_KEY_FILE="${PROBE_KEY_FILE:-}"
MANAGED_CERT_DIR="${PROBE_ROOT}/trojan-panel-core/cert"
EXTERNAL_MANAGED_DIR="${PROBE_ROOT}/trojanpanelnext-external"
TP_DATA="${PROBE_ROOT}"
TP_WEB_DOMAIN=""
TP_NODE_DOMAIN="node.example.com"
UI_LISTEN="127.0.0.1:8888"
PANEL_PORT=8081
CORE_PORT=8082
GRPC_PORT=8100
NODE_CADDY_HTTP_PORT=80
WEB_PATH="${PROBE_ROOT}/web"
TLS_CERT_PAIR="$(discover_external_cert)"
printf 'PAIR=%s\n' "${TLS_CERT_PAIR}"
install_external_cert "${TLS_CERT_PAIR}"
EXTERNAL_ROUTES_NOTE="- probe"
write_external_entry_contract node
EOS

PROBE_CERT_DIR="${external_tls_dir}" PROBE_CERT_FILE= PROBE_KEY_FILE= \
  bash -c "${external_probe}" installer-test "${INSTALLER}" "${external_data_dir}" >/dev/null
test "$(stat -c '%a' "${external_data_dir}/trojan-panel-core/cert/privkey.pem")" = 600
test "$(stat -c '%a' "${external_data_dir}/trojan-panel-core/cert/fullchain.pem")" = 644
test "$(stat -c '%a' "${external_data_dir}/trojan-panel-core/cert")" = 700
cmp "${external_tls_dir}/fullchain.pem" "${external_data_dir}/trojan-panel-core/cert/fullchain.pem"
if find "${external_data_dir}/trojan-panel-core/cert" -maxdepth 1 -name '.*.pem.*' -print -quit | grep -q .; then
  fail "temporary TLS material was left behind"
fi

# Re-running over the same destination must not truncate the pair.
PROBE_CERT_DIR="${external_tls_dir}" PROBE_CERT_FILE= PROBE_KEY_FILE= \
  bash -c "${external_probe}" installer-test "${INSTALLER}" "${external_data_dir}" >/dev/null
cmp "${external_tls_dir}/privkey.pem" "${external_data_dir}/trojan-panel-core/cert/privkey.pem"

# refresh-cert reports certificate generations accurately and restarts the
# consumer only after a changed pair has been atomically installed.
mkdir -p "${external_refresh_dir}/source"
cp "${external_tls_dir}/fullchain.pem" "${external_refresh_dir}/source/fullchain.pem"
cp "${external_tls_dir}/privkey.pem" "${external_refresh_dir}/source/privkey.pem"
bash -c '
  set -Eeuo pipefail
  source "$1"
  TLS_MODE=external
  TLS_CERT_DIR="$2/source"
  TLS_CERT_FILE=
  TLS_KEY_FILE=
  TP_NODE_DOMAIN=node.example.com
  MANAGED_CERT_DIR="$2/managed"
  EXTERNAL_MANAGED_DIR="$2/state"
  CORE_CONTAINER=trojan-panel-core
  TRACE_FILE="$2/docker.trace"
  container_exists() { return 0; }
  wait_for_container() { :; }
  docker() { printf "docker %s\n" "$*" >>"${TRACE_FILE}"; }
  refresh_node_certificate
  refresh_node_certificate
' installer-test "${INSTALLER}" "${external_refresh_dir}" >"${external_refresh_dir}/refresh.out"
test "$(grep -c '^changed$' "${external_refresh_dir}/refresh.out")" = 1
test "$(grep -c '^unchanged$' "${external_refresh_dir}/refresh.out")" = 1
test "$(grep -c '^docker restart trojan-panel-core$' "${external_refresh_dir}/docker.trace")" = 1

# Pointing tls_cert_dir at the managed directory itself is a no-op copy.
PROBE_CERT_DIR="${external_data_dir}/trojan-panel-core/cert" PROBE_CERT_FILE= PROBE_KEY_FILE= \
  bash -c "${external_probe}" installer-test "${INSTALLER}" "${external_data_dir}" >/dev/null
cmp "${external_tls_dir}/fullchain.pem" "${external_data_dir}/trojan-panel-core/cert/fullchain.pem"

# Same-stem layout inside a named directory. This runs before a second pair is
# added, because a directory with two pairs is deliberately ambiguous.
PROBE_CERT_DIR="${external_pairs_dir}" PROBE_CERT_FILE= PROBE_KEY_FILE= \
  bash -c "${external_probe}" installer-test "${INSTALLER}" "${external_data_dir}/stem" \
  >"${external_cases_dir}/stem.out"
grep -q "${external_pairs_dir}/site/server.crt" "${external_cases_dir}/stem.out"

# A directory with two pairs is ambiguous unless tls_cert_file selects one.
cp "${external_tls_dir}/fullchain.pem" "${external_pairs_dir}/second.crt"
cp "${external_tls_dir}/privkey.pem" "${external_pairs_dir}/second.key"
assert_fails env PROBE_CERT_DIR="${external_pairs_dir}" PROBE_CERT_FILE= PROBE_KEY_FILE= \
  bash -c "${external_probe}" installer-test "${INSTALLER}" "${external_data_dir}/ambiguous"
PROBE_CERT_DIR="${external_pairs_dir}" PROBE_CERT_FILE='second.crt' PROBE_KEY_FILE='second.key' \
  bash -c "${external_probe}" installer-test "${INSTALLER}" "${external_data_dir}/explicit" \
  >"${external_cases_dir}/explicit.out"
grep -q 'second.crt|' "${external_cases_dir}/explicit.out"

# certd exports each domain as fullchain.pem + chain.pem + privkey.pem. The
# intermediate chain.pem must not make the directory look ambiguous.
mkdir -p "${external_cases_dir}/certd"
cp "${external_tls_dir}/fullchain.pem" "${external_cases_dir}/certd/fullchain.pem"
cp "${external_tls_dir}/fullchain.pem" "${external_cases_dir}/certd/chain.pem"
cp "${external_tls_dir}/privkey.pem" "${external_cases_dir}/certd/privkey.pem"
PROBE_CERT_DIR="${external_cases_dir}/certd" PROBE_CERT_FILE= PROBE_KEY_FILE= \
  bash -c "${external_probe}" installer-test "${INSTALLER}" "${external_data_dir}/certd" \
  >"${external_cases_dir}/certd.out"
grep -q 'PAIR=.*certd/fullchain.pem' "${external_cases_dir}/certd.out"
grep -q 'certd/privkey.pem' "${external_cases_dir}/certd.out"
test "$(stat -c '%a' "${external_data_dir}/certd/trojan-panel-core/cert/privkey.pem")" = 600

# Removing an external deployment must keep working after the external
# certificate directory has disappeared. Uninstall does not consume TLS input.
bash -c '
  set -Eeuo pipefail
  source "$1"
  prepare_secure_config() { TP_CONFIG_READ_FILE="$1"; }
  require_root() { :; }
  load_config() { TLS_MODE=external; }
  validate_config() { :; }
  discover_external_cert() { return 99; }
  remove_node() { printf "REMOVED\n"; }
  main remove --mode node --config /does/not-need-to-exist
' installer-test "${INSTALLER}" | grep -q '^REMOVED$'

# Automation can force a non-destructive removal even when an existing config
# opted into purge_data.
bash -c '
  set -Eeuo pipefail
  source "$1"
  prepare_secure_config() { TP_CONFIG_READ_FILE="$1"; }
  require_root() { :; }
  load_config() { TLS_MODE=external; TP_PURGE_DATA=1; }
  validate_config() { :; }
  remove_node() { test "${TP_PURGE_DATA}" = 0; printf "KEPT\n"; }
  main remove --mode node --config /contains-purge-data --keep-data
' installer-test "${INSTALLER}" | grep -q '^KEPT$'

# Mode/bind migrations recreate only affected containers. An old container
# without the marker is the legacy default (acme / 0.0.0.0).
bash -c '
  set -Eeuo pipefail
  source "$1"
  container_exists() { return 0; }
  container_env_value() { printf "%s\n" "${CURRENT_VALUE:-}"; }
  docker() { printf "%s\n" "$*"; }
  CURRENT_VALUE= recreate_container_if_env_changed core TP_TLS_MODE acme acme
  CURRENT_VALUE= recreate_container_if_env_changed core TP_TLS_MODE external acme
  CURRENT_VALUE=external recreate_container_if_env_changed core TP_TLS_MODE external acme
' installer-test "${INSTALLER}" >"${external_cases_dir}/migration.out"
test "$(grep -c 'recreate container' "${external_cases_dir}/migration.out")" = 1

# Core reads the client CA at startup. A legacy container without the marker,
# or a container pinned to another CA, must be recreated; an unchanged CA is a
# no-op on later installer replays.
bash -c '
  set -Eeuo pipefail
  source "$1"
  container_exists() { return 0; }
  container_env_value() { printf "%s\n" "${CURRENT_VALUE:-}"; }
  docker() { printf "%s\n" "$*"; }
  CURRENT_VALUE= recreate_container_if_env_changed core TP_CLIENT_CA_SHA256 abc ""
  CURRENT_VALUE=old recreate_container_if_env_changed core TP_CLIENT_CA_SHA256 abc ""
  CURRENT_VALUE=abc recreate_container_if_env_changed core TP_CLIENT_CA_SHA256 abc ""
' installer-test "${INSTALLER}" >"${external_cases_dir}/client-ca-migration.out"
test "$(grep -c 'recreate container' "${external_cases_dir}/client-ca-migration.out")" = 2

# External mode must fail closed when a stale Caddy container cannot be
# removed; otherwise install would claim success while 80/443 may stay owned.
assert_fails bash -c '
  set -Eeuo pipefail
  source "$1"
  container_exists() { return 0; }
  docker() { return 42; }
  remove_caddy_container stale-caddy
' installer-test "${INSTALLER}"

# External removal must never target the legacy Caddy names. The caller may
# verify ownership of the workload containers, but cannot safely authorize a
# later, unrelated container that races into a legacy Caddy name.
remove_trace="${external_cases_dir}/remove-containers.trace"
REMOVE_TRACE="${remove_trace}" bash -c '
  set -Eeuo pipefail
  source "$1"
  docker() { printf "%s\n" "$*" >>"${REMOVE_TRACE}"; }
  TP_PURGE_DATA=0
  TLS_MODE=external
  remove_web
  remove_node
' installer-test "${INSTALLER}"
grep -Fq 'trojan-panel-ui' "${remove_trace}"
grep -Fq 'trojan-panel' "${remove_trace}"
grep -Fq 'trojan-panel-redis' "${remove_trace}"
grep -Fq 'trojan-panel-mariadb' "${remove_trace}"
grep -Fq 'trojan-panel-core' "${remove_trace}"
if grep -Fq -- '-caddy' "${remove_trace}"; then
  echo "external remove unexpectedly targeted a Caddy container" >&2
  exit 1
fi

: >"${remove_trace}"
REMOVE_TRACE="${remove_trace}" bash -c '
  set -Eeuo pipefail
  source "$1"
  docker() { printf "%s\n" "$*" >>"${REMOVE_TRACE}"; }
  TP_PURGE_DATA=0
  TLS_MODE=acme
  remove_web
  remove_node
' installer-test "${INSTALLER}"
grep -Fq 'trojan-panel-web-caddy' "${remove_trace}"
grep -Fq 'trojan-panel-node-caddy' "${remove_trace}"

# The generated core config carries the node domain used as the routes.json SNI
# fallback, including after migration from an older config.
bash -c '
  set -Eeuo pipefail
  source "$1"
  TP_DATA="$2"
  mkdir -p "${TP_DATA}/trojan-panel-core/config"
  MARIADB_HOST=db.example.com
  MARIADB_PASSWORD=db-secret
  REDIS_HOST=redis.example.com
  REDIS_PASSWORD=redis-secret
  TP_NODE_DOMAIN=node.example.com
  write_core_runtime_config cert.pem key.pem
' installer-test "${INSTALLER}" "${external_data_dir}/core-config"
grep -q '^domain=node.example.com$' "${external_data_dir}/core-config/trojan-panel-core/config/config.ini"

# The documented manifest path, image default, and installer mount must agree.
grep -q 'TP_EXTERNAL_DIR=/tpdata/trojan-panel-core/external' \
  "$(dirname "${INSTALLER}")/../../apps/node-agent/Dockerfile"
grep -q 'EXTERNAL_ROUTES_DIR}:${EXTERNAL_ROUTES_DIR}' "${INSTALLER}"

# The generated on-host contract must exist, name the routes file, and never
# leak a credential.
test -f "${external_data_dir}/trojanpanelnext-external/README.md"
grep -q 'trojan-panel-core/external/routes.json' "${external_data_dir}/trojanpanelnext-external/README.md"
if grep -q '| Panel UI |' "${external_data_dir}/trojanpanelnext-external/README.md"; then
  fail "node contract lists web-only services"
fi
if grep -qiE 'password|mariadb_pas|redis_pass' "${external_data_dir}/trojanpanelnext-external/README.md"; then
  fail "external contract README leaks a credential"
fi

printf 'PASS installer CLI and deployment mode/config contract\n'
