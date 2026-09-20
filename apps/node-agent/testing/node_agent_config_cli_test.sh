#!/usr/bin/env bash
set -Eeuo pipefail

AGENT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf -- "${work}"' EXIT

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

(cd "${AGENT_DIR}" && CGO_ENABLED=0 go build -trimpath -o "${work}/trojan-panel-core" .)

write_config() {
  local destination="$1"
  local mariadb_password="$2"
  local cache_password="$3"
  local auth_password="$4"
  mkdir -p "${destination}/config"
  cat >"${destination}/config/config.ini" <<EOF
[mysql]
host=127.0.0.1
user=tpn_node_test
password=${mariadb_password}
port=3306
database=trojan_panel_db
account_table=account
[redis]
host=127.0.0.1
port=6379
username=tpn-cache-test
password=${cache_password}
auth_username=tpn-auth-test
auth_password=${auth_password}
db=0
max_idle=2
max_active=4
wait=true
[cert]
crt_path=cert.pem
key_path=key.pem
[log]
filename=logs/trojan-panel-core.log
max_size=1
max_backups=1
max_age=1
compress=true
[grpc]
port=8100
tls_mode=mtls
client_ca_path=client-ca.crt
[server]
port=8082
[node]
server_id=1
domain=node.example.test
EOF
  chmod 0600 "${destination}/config/config.ini"
}

assert_startup_rejects() {
  local name="$1"
  local expected="$2"
  local mariadb_password="$3"
  local cache_password="$4"
  local auth_password="$5"
  local runtime="${work}/${name}"
  write_config "${runtime}" "${mariadb_password}" "${cache_password}" "${auth_password}"
  if (cd "${runtime}" && "${work}/trojan-panel-core" >"${runtime}.out" 2>"${runtime}.err"); then
    fail "Node Agent accepted ${name} at its process startup boundary"
  fi
  grep -Fq "${expected}" "${runtime}.err" || fail "Node Agent did not report ${name} fail-closed rejection"
  for secret in valid-mariadb-secret valid-cache-secret valid-auth-secret; do
    ! grep -Fq -- "${secret}" "${runtime}.out" "${runtime}.err" || fail "Node Agent startup rejection leaked a credential"
  done
}

assert_startup_rejects empty-mariadb-password 'non-empty MariaDB password' '' valid-cache-secret valid-auth-secret
assert_startup_rejects empty-cache-password 'non-empty Redis cache password' valid-mariadb-secret '' valid-auth-secret
assert_startup_rejects empty-auth-password 'non-empty Redis auth password' valid-mariadb-secret valid-cache-secret ''

printf 'PASS Node Agent public startup rejects empty data-layer passwords\n'
