#!/usr/bin/env bash
set -Eeuo pipefail

INSTALLER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
work="$(mktemp -d)"
bundle_tmpfs_root="$(mktemp -d /dev/shm/trojanpanelnext-node-bundle-test.XXXXXX)"
cleanup() {
  rm -rf -- "${work}" "${bundle_tmpfs_root}"
}
trap cleanup EXIT

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
cp "${INSTALLER_DIR}/release/templates/config-node.yaml" "${work}/config-node.yaml"
sed -i \
  -e 's/__ASSET_VERSION__/1.2.3/' \
  -e 's|__CADDY_IMAGE__|caddy@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa|' \
  -e 's|__MARIADB_IMAGE__|mariadb@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb|' \
  -e 's|__REDIS_IMAGE__|redis@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc|' \
  -e 's|__API_IMAGE__|api@sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd|' \
  -e 's|__WEB_IMAGE__|web@sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee|' \
  -e 's|__NODE_AGENT_IMAGE__|node-agent@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff|' \
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

test -z "$(find "${bundle_tmpfs_root}" -mindepth 1 -maxdepth 1 -print -quit)"
TP_NODE_BUNDLE_PASSWORD="${password}" NODE_BUNDLE_HELPER="${work}/node-bundle" \
  TP_NODE_BUNDLE_TMP_ROOT="${bundle_tmpfs_root}" \
  "${INSTALLER_DIR}/install.sh" validate --mode node --bundle "${work}/node.age" |
  grep -q 'valid for node deployment mode'
test -z "$(find "${bundle_tmpfs_root}" -mindepth 1 -maxdepth 1 -print -quit)" ||
  fail 'installer left decrypted Node bundle files in its isolated tmpfs root'

printf 'PASS encrypted Node bootstrap bundle CLI and cleanup contract\n'
