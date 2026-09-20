#!/usr/bin/env bash
set -Eeuo pipefail

INSTALLER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf -- "${work}"' EXIT

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

assert_fails() {
  if "$@" >/dev/null 2>&1; then
    fail "command unexpectedly succeeded: $*"
  fi
}

(cd "${INSTALLER_DIR}/nodebundle" && go test ./...)
(cd "${INSTALLER_DIR}/nodebundle" && go build -trimpath -o "${work}/node-bundle" .)

cat >"${work}/credential.json" <<'JSON'
{
  "schema_version": 2,
  "node_identity_id": "11111111-2222-4333-8444-555555555555",
  "node_server_id": 42,
  "node_name": "node-sg",
  "node_domain": "node.example.com",
  "public_ip": "203.0.113.42",
  "generation": 7,
  "mariadb": {"database":"trojan_panel_db","username":"tpn_example","password":"db-secret"},
  "redis": {"username":"tpn-cache-example","password":"cache-secret","key_patterns":["trojan-panel-core:*"]},
  "redis_auth": {"username":"tpn-auth-example","password":"auth-secret","key_patterns":["trojan-panel:jwt-key","trojan-panel:token:*"]}
}
JSON
cp "${INSTALLER_DIR}/examples/node-agent.yaml" "${work}/config-node.yaml"
sed -i \
  -e 's/mariadb_user: .*/mariadb_user: replace/' \
  -e 's/mariadb_password: .*/mariadb_password: replace/' \
  -e 's/redis_username: .*/redis_username: replace/' \
  -e 's/redis_password: .*/redis_password: replace/' \
  -e 's/redis_auth_username: .*/redis_auth_username: replace/' \
  -e 's/redis_auth_password: .*/redis_auth_password: replace/' \
  "${work}/config-node.yaml"
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj /CN=control-plane-ca \
  -addext basicConstraints=critical,CA:TRUE \
  -keyout "${work}/discarded-ca.key" -out "${work}/client-ca.crt" >/dev/null 2>&1
chmod 0600 "${work}/credential.json" "${work}/config-node.yaml"

password='correct horse battery staple'
TP_NODE_BUNDLE_PASSWORD="${password}" "${work}/node-bundle" create \
  --credential-file "${work}/credential.json" \
  --node-config "${work}/config-node.yaml" \
  --client-ca "${work}/client-ca.crt" \
  --output "${work}/node.age" >/dev/null
test "$(stat -c '%a' "${work}/node.age")" = 600
grep -a -q -- '-> scrypt ' "${work}/node.age"
TP_NODE_BUNDLE_PASSWORD="${password}" "${work}/node-bundle" inspect \
  --bundle "${work}/node.age" | grep -q '"generation": 7'
assert_fails env TP_NODE_BUNDLE_PASSWORD='wrong password value' \
  "${work}/node-bundle" inspect --bundle "${work}/node.age"

mkdir -m 0700 "${work}/plain"
TP_NODE_BUNDLE_PASSWORD="${password}" "${work}/node-bundle" extract \
  --bundle "${work}/node.age" --directory "${work}/plain"
mapfile -t entries < <(cd "${work}/plain" && find . -type f -printf '%P\n' | sort)
test "${entries[*]}" = 'config-node.yaml manifest.json pki/client-ca.crt' ||
  fail "unexpected plaintext inventory: ${entries[*]}"
if find "${work}/plain" -type f -iname '*key*' -print -quit | grep -q .; then
  fail 'private key-like file leaked from Node bootstrap bundle'
fi

before_count="$(find /dev/shm -maxdepth 1 -type d -name 'trojanpanelnext-node-bundle.*' | wc -l)"
TP_NODE_BUNDLE_PASSWORD="${password}" NODE_BUNDLE_HELPER="${work}/node-bundle" \
  "${INSTALLER_DIR}/install.sh" validate --mode node --bundle "${work}/node.age" |
  grep -q 'valid for node deployment mode'
after_count="$(find /dev/shm -maxdepth 1 -type d -name 'trojanpanelnext-node-bundle.*' | wc -l)"
test "${before_count}" = "${after_count}" || fail 'installer left decrypted Node bundle files in tmpfs'

printf 'PASS encrypted Node bootstrap bundle CLI and cleanup contract\n'
