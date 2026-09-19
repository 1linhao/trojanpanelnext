#!/usr/bin/env bash
set -Eeuo pipefail

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

if ! docker info >/dev/null 2>&1; then
  docker() {
    sudo -n /usr/bin/docker "$@"
  }
fi

API_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
work="$(mktemp -d)"
suffix="${RANDOM}-$$"
mariadb_container="tp-node-identity-mariadb-${suffix}"
redis_container="tp-node-identity-redis-${suffix}"
mariadb_image='mariadb@sha256:07e06f2e7ae9dfc63707a83130a62e00167c827f08fcac7a9aa33f4b6dc34e0e'
redis_image='redis@sha256:a93c14584715ec5bd9d2648d58c3b27f89416242bee0bc9e5fb2edc1a4cbec1d'

cleanup() {
  docker rm -fv "${mariadb_container}" "${redis_container}" >/dev/null 2>&1 || true
  rm -rf -- "${work}"
}
trap cleanup EXIT

(cd "${API_DIR}" && CGO_ENABLED=0 go build -trimpath -o "${work}/trojan-panel" .)

mkdir -p "${work}/runtime"
if ! (cd "${work}/runtime" && "${work}/trojan-panel" node-identity --help >"${work}/help.out" 2>"${work}/help.err"); then
  fail 'node-identity --help returned non-zero'
fi
grep -Fq 'register --name <name> --domain <domain> --public-ip <ip> --credential-file <0600-file>' "${work}/help.out" ||
  fail 'node identity register contract is missing from help'
grep -Fq 'rotate --id <node-identity-id> --credential-file <0600-file>' "${work}/help.out" ||
  fail 'node identity rotate contract is missing from help'
grep -Fq 'revoke --id <node-identity-id>' "${work}/help.out" ||
  fail 'node identity revoke contract is missing from help'
grep -Fq 'force-evict --id <node-identity-id>' "${work}/help.out" ||
  fail 'node identity force-evict contract is missing from help'

test ! -e "${work}/runtime/config" || fail 'help changed runtime state'
test ! -s "${work}/help.err" || fail 'help wrote unexpected stderr'

admin_db_password='IntegrationRootPassword-5'
admin_redis_password='IntegrationRedisPassword-5'
docker run -d --name "${mariadb_container}" \
  -e "MARIADB_ROOT_PASSWORD=${admin_db_password}" \
  -e MARIADB_DATABASE=trojan_panel_db \
  -p 127.0.0.1::3306 "${mariadb_image}" >/dev/null
docker run -d --name "${redis_container}" \
  -p 127.0.0.1::6379 "${redis_image}" \
  redis-server --requirepass "${admin_redis_password}" >/dev/null

for _ in $(seq 1 90); do
  if docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
      mariadb -uroot -e 'SELECT 1' >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
  mariadb -uroot -e 'SELECT 1' >/dev/null 2>&1 || fail 'MariaDB did not become ready'
for _ in $(seq 1 30); do
  if docker exec -e "REDISCLI_AUTH=${admin_redis_password}" "${redis_container}" \
      redis-cli ping 2>/dev/null | grep -Fxq PONG; then
    break
  fi
  sleep 1
done
docker exec -e "REDISCLI_AUTH=${admin_redis_password}" "${redis_container}" \
  redis-cli ping 2>/dev/null | grep -Fxq PONG || fail 'Redis did not become ready'

docker exec -i -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
  mariadb -uroot trojan_panel_db <<'SQL'
CREATE TABLE account (
  id bigint unsigned NOT NULL AUTO_INCREMENT,
  username varchar(64) NOT NULL,
  pass varchar(64) NOT NULL DEFAULT '',
  hash varchar(64) NOT NULL DEFAULT '',
  quota bigint NOT NULL DEFAULT -1,
  download bigint unsigned NOT NULL DEFAULT 0,
  upload bigint unsigned NOT NULL DEFAULT 0,
  PRIMARY KEY (id)
);
INSERT INTO account (username) VALUES ('integration-user');
CREATE TABLE node_server (
  id bigint unsigned NOT NULL AUTO_INCREMENT,
  ip varchar(64) NOT NULL DEFAULT '',
  name varchar(64) NOT NULL DEFAULT '',
  grpc_port int unsigned NOT NULL DEFAULT 8100,
  grpc_tls_mode varchar(16) NOT NULL DEFAULT 'mtls',
  grpc_tls_server_name varchar(253) NOT NULL DEFAULT '',
  traffic_period varchar(8) NOT NULL DEFAULT 'none',
  traffic_limit_mode varchar(8) NOT NULL DEFAULT 'combined',
  traffic_total_limit bigint unsigned NOT NULL DEFAULT 0,
  traffic_upload_limit bigint unsigned NOT NULL DEFAULT 0,
  traffic_download_limit bigint unsigned NOT NULL DEFAULT 0,
  create_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id)
);
CREATE TABLE account_traffic_total (
  account_id bigint unsigned NOT NULL,
  upload bigint unsigned NOT NULL DEFAULT 0,
  download bigint unsigned NOT NULL DEFAULT 0,
  PRIMARY KEY (account_id)
);
CREATE TABLE account_traffic_daily (
  traffic_date date NOT NULL,
  account_id bigint unsigned NOT NULL,
  upload bigint unsigned NOT NULL DEFAULT 0,
  download bigint unsigned NOT NULL DEFAULT 0,
  PRIMARY KEY (traffic_date, account_id)
);
CREATE TABLE account_server_traffic_daily (
  traffic_date date NOT NULL,
  account_id bigint unsigned NOT NULL,
  node_server_id bigint unsigned NOT NULL,
  upload bigint unsigned NOT NULL DEFAULT 0,
  download bigint unsigned NOT NULL DEFAULT 0,
  PRIMARY KEY (traffic_date, account_id, node_server_id)
);
CREATE TABLE system (id bigint unsigned NOT NULL PRIMARY KEY, name varchar(32) NOT NULL);
INSERT INTO system VALUES (1, 'private-control-data');
SQL

mariadb_port="$(docker port "${mariadb_container}" 3306/tcp | sed -n 's/.*://p' | head -n1)"
redis_port="$(docker port "${redis_container}" 6379/tcp | sed -n 's/.*://p' | head -n1)"
mkdir -p "${work}/runtime/config"
chmod 0700 "${work}/runtime/config"
printf '%s\n' \
  '[mysql]' \
  'host=127.0.0.1' \
  'user=root' \
  "password=${admin_db_password}" \
  "port=${mariadb_port}" \
  '[redis]' \
  'host=127.0.0.1' \
  "port=${redis_port}" \
  "password=${admin_redis_password}" \
  'db=0' \
  'max_idle=2' \
  'max_active=4' \
  'wait=true' \
  >"${work}/runtime/config/config.ini"
chmod 0600 "${work}/runtime/config/config.ini"

credential_file="${work}/runtime/config/node-a-credentials.json"
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity register \
  --name node-a \
  --domain node-a.example.com \
  --public-ip 203.0.113.10 \
  --credential-file "${credential_file}" \
  >"${work}/register.out" 2>"${work}/register.err") || {
    sed -n '1,80p' "${work}/register.out" >&2
    sed -n '1,80p' "${work}/register.err" >&2
    docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
      mariadb -uroot trojan_panel_db -e 'SELECT action,result,error_code FROM node_identity_event' >&2 || true
    fail 'register returned non-zero'
  }

test -f "${credential_file}" || fail 'register did not create a credential file'
test "$(stat -c '%a' "${credential_file}")" = 600 || fail 'credential file mode is not 0600'
node_identity_id="$(jq -r '.node_identity_id' "${credential_file}")"
node_server_id="$(jq -r '.node_server_id' "${credential_file}")"
db_username="$(jq -r '.mariadb.username' "${credential_file}")"
db_password="$(jq -r '.mariadb.password' "${credential_file}")"
redis_username="$(jq -r '.redis.username' "${credential_file}")"
redis_password="$(jq -r '.redis.password' "${credential_file}")"
test "$(jq -r '.generation' "${credential_file}")" = 1 || fail 'initial credential generation is not 1'
[[ "${node_identity_id}" =~ ^[0-9a-f-]{36}$ ]] || fail 'Node identity id is not stable UUID text'
[[ "${node_server_id}" =~ ^[1-9][0-9]*$ ]] || fail 'node_server_id is invalid'
for secret in "${db_password}" "${redis_password}" "${admin_db_password}" "${admin_redis_password}"; do
  ! grep -Fq -- "${secret}" "${work}/register.out" "${work}/register.err" || fail 'register leaked a secret'
done
test ! -s "${work}/register.err" || fail 'register wrote unexpected stderr'
grep -Fq "Node identity registered: ${node_identity_id}" "${work}/register.out" ||
  fail 'register did not report the stable Node identity'

docker exec -e "MYSQL_PWD=${db_password}" "${mariadb_container}" \
  mariadb -h127.0.0.1 "-u${db_username}" trojan_panel_db \
  -e 'SELECT username FROM account' >/dev/null || fail 'Node MariaDB identity cannot read accounts'
docker exec -e "MYSQL_PWD=${db_password}" "${mariadb_container}" \
  mariadb -h127.0.0.1 "-u${db_username}" trojan_panel_db \
  -e 'UPDATE account SET download=download+1, upload=upload+1 WHERE id=1' >/dev/null ||
  fail 'Node MariaDB identity cannot account for traffic'
if docker exec -e "MYSQL_PWD=${db_password}" "${mariadb_container}" \
    mariadb -h127.0.0.1 "-u${db_username}" trojan_panel_db \
    -e 'SELECT name FROM system' >/dev/null 2>&1; then
  fail 'Node MariaDB identity can read control-plane-only data'
fi
if docker exec -e "MYSQL_PWD=${db_password}" "${mariadb_container}" \
    mariadb -h127.0.0.1 "-u${db_username}" trojan_panel_db \
    -e 'DROP TABLE account' >/dev/null 2>&1; then
  fail 'Node MariaDB identity can perform destructive DDL'
fi
if docker exec -e "MYSQL_PWD=${db_password}" "${mariadb_container}" \
    mariadb -h127.0.0.1 "-u${db_username}" trojan_panel_db \
    -e "CREATE USER forbidden@'%' IDENTIFIED BY 'forbidden'" >/dev/null 2>&1; then
  fail 'Node MariaDB identity can administer users'
fi

docker exec -e "REDISCLI_AUTH=${redis_password}" "${redis_container}" \
  redis-cli --user "${redis_username}" ping 2>/dev/null | grep -Fxq PONG ||
  fail 'Node Redis ACL identity cannot authenticate'
docker exec -e "REDISCLI_AUTH=${redis_password}" "${redis_container}" \
  redis-cli --user "${redis_username}" set trojan-panel-core:node-config:443-1 ok 2>/dev/null | grep -Fxq OK ||
  fail 'Node Redis ACL identity cannot write an allowed key'
if docker exec -e "REDISCLI_AUTH=${redis_password}" "${redis_container}" \
    redis-cli --user "${redis_username}" set control-plane:private denied 2>/dev/null | grep -Fxq OK; then
  fail 'Node Redis ACL identity can write an unrelated key'
fi
if docker exec -e "REDISCLI_AUTH=${redis_password}" "${redis_container}" \
    redis-cli --user "${redis_username}" --raw ACL LIST 2>/dev/null | grep -q '^user '; then
  fail 'Node Redis ACL identity can administer ACLs'
fi
unexpected_register_file="${work}/runtime/config/node-a-untracked-reregister.json"
if (cd "${work}/runtime" && "${work}/trojan-panel" node-identity register \
  --name node-a --domain node-a.example.com --public-ip 203.0.113.10 \
  --credential-file "${unexpected_register_file}" \
  >"${work}/register-new-path.out" 2>"${work}/register-new-path.err"); then
  fail 'active Node registration accepted a new untracked credential file'
fi
docker exec -e "MYSQL_PWD=${db_password}" "${mariadb_container}" \
  mariadb -h127.0.0.1 "-u${db_username}" trojan_panel_db -e 'SELECT 1' >/dev/null ||
  fail 'rejected re-registration changed the active MariaDB credential'
docker exec -e "REDISCLI_AUTH=${redis_password}" "${redis_container}" \
  redis-cli --user "${redis_username}" ping 2>/dev/null | grep -Fxq PONG ||
  fail 'rejected re-registration changed the active Redis credential'
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity register \
  --name node-a --domain node-a.example.com --public-ip 203.0.113.10 \
  --credential-file "${credential_file}" \
  >"${work}/register-replay.out" 2>"${work}/register-replay.err") ||
  fail 're-registration with the original credential file did not converge'
original_credential_file="${work}/runtime/config/node-a-credentials-original.json"
tampered_credential_file="${work}/runtime/config/node-a-credentials-tampered.json"
cp "${credential_file}" "${original_credential_file}"
jq '.mariadb.password = "tampered-without-rotation"' "${credential_file}" \
  >"${tampered_credential_file}"
chmod 0600 "${tampered_credential_file}"
mv "${tampered_credential_file}" "${credential_file}"
if (cd "${work}/runtime" && "${work}/trojan-panel" node-identity register \
  --name node-a --domain node-a.example.com --public-ip 203.0.113.10 \
  --credential-file "${credential_file}" \
  >"${work}/register-tampered.out" 2>"${work}/register-tampered.err"); then
  fail 're-registration accepted tampered credential contents without rotation'
fi
docker exec -e "MYSQL_PWD=${db_password}" "${mariadb_container}" \
  mariadb -h127.0.0.1 "-u${db_username}" trojan_panel_db -e 'SELECT 1' >/dev/null ||
  fail 'tampered re-registration changed the active MariaDB credential'
docker exec -e "REDISCLI_AUTH=${redis_password}" "${redis_container}" \
  redis-cli --user "${redis_username}" ping 2>/dev/null | grep -Fxq PONG ||
  fail 'tampered re-registration changed the active Redis credential'
mv "${original_credential_file}" "${credential_file}"
chmod 0644 "${credential_file}"
if (cd "${work}/runtime" && "${work}/trojan-panel" node-identity register \
  --name node-a --domain node-a.example.com --public-ip 203.0.113.10 \
  --credential-file "${credential_file}" \
  >"${work}/register-loose-mode.out" 2>"${work}/register-loose-mode.err"); then
  fail 're-registration read a credential file whose mode was not 0600'
fi
chmod 0600 "${credential_file}"
credential_backup="${work}/runtime/config/node-a-credentials-backup.json"
cp "${credential_file}" "${credential_backup}"
chmod 0600 "${credential_backup}"
rm "${credential_file}"
ln -s "${credential_backup}" "${credential_file}"
if (cd "${work}/runtime" && "${work}/trojan-panel" node-identity register \
  --name node-a --domain node-a.example.com --public-ip 203.0.113.10 \
  --credential-file "${credential_file}" \
  >"${work}/register-symlink-file.out" 2>"${work}/register-symlink-file.err"); then
  fail 're-registration followed a symlinked credential file'
fi
rm "${credential_file}"
mv "${credential_backup}" "${credential_file}"
docker exec -e "REDISCLI_AUTH=${admin_redis_password}" "${redis_container}" \
  redis-cli set trojan-panel:jwt-key integration-jwt-key >/dev/null
(cd "${API_DIR}/../../node-agent" && \
  TP_REDIS_ACL_INTEGRATION_ADDRESS="127.0.0.1:${redis_port}" \
  TP_REDIS_ACL_INTEGRATION_USERNAME="${redis_username}" \
  TP_REDIS_ACL_INTEGRATION_PASSWORD="${redis_password}" \
  go test ./dao/redis -run '^TestNodeAgentAuthenticatesWithRedisACLIdentity$' -count=1) >/dev/null ||
  fail 'Node Agent could not consume its Redis ACL identity through the public Redis seam'

node_b_credential_file="${work}/runtime/config/node-b-credentials.json"
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity register \
  --name node-b \
  --domain node-b.example.com \
  --public-ip 203.0.113.11 \
  --credential-file "${node_b_credential_file}" \
  >"${work}/register-b.out" 2>"${work}/register-b.err") || fail 'second Node registration returned non-zero'
node_b_db_username="$(jq -r '.mariadb.username' "${node_b_credential_file}")"
node_b_db_password="$(jq -r '.mariadb.password' "${node_b_credential_file}")"
node_b_redis_username="$(jq -r '.redis.username' "${node_b_credential_file}")"
node_b_redis_password="$(jq -r '.redis.password' "${node_b_credential_file}")"
test "${node_b_db_username}" != "${db_username}" || fail 'two Nodes share a MariaDB identity'
test "${node_b_redis_username}" != "${redis_username}" || fail 'two Nodes share a Redis ACL identity'

rotated_credential_file="${work}/runtime/config/node-a-credentials-generation-2.json"
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity rotate \
  --id "${node_identity_id}" \
  --credential-file "${rotated_credential_file}" \
  >"${work}/rotate.out" 2>"${work}/rotate.err") || fail 'rotate returned non-zero'
test "$(stat -c '%a' "${rotated_credential_file}")" = 600 || fail 'rotated credential file mode is not 0600'
test "$(jq -r '.generation' "${rotated_credential_file}")" = 2 || fail 'rotated credential generation is not 2'
test "$(jq -r '.node_identity_id' "${rotated_credential_file}")" = "${node_identity_id}" || fail 'rotation changed Node identity'
test "$(jq -r '.mariadb.username' "${rotated_credential_file}")" = "${db_username}" || fail 'rotation changed MariaDB identity'
test "$(jq -r '.redis.username' "${rotated_credential_file}")" = "${redis_username}" || fail 'rotation changed Redis ACL identity'
rotated_db_password="$(jq -r '.mariadb.password' "${rotated_credential_file}")"
rotated_redis_password="$(jq -r '.redis.password' "${rotated_credential_file}")"
test "${rotated_db_password}" != "${db_password}" || fail 'rotation reused the MariaDB password'
test "${rotated_redis_password}" != "${redis_password}" || fail 'rotation reused the Redis password'
for secret in "${rotated_db_password}" "${rotated_redis_password}" "${db_password}" "${redis_password}"; do
  ! grep -Fq -- "${secret}" "${work}/rotate.out" "${work}/rotate.err" || fail 'rotate leaked a secret'
done
if docker exec -e "MYSQL_PWD=${db_password}" "${mariadb_container}" \
    mariadb -h127.0.0.1 "-u${db_username}" trojan_panel_db -e 'SELECT 1' >/dev/null 2>&1; then
  fail 'old MariaDB credential remained valid after rotation'
fi
if docker exec -e "REDISCLI_AUTH=${redis_password}" "${redis_container}" \
    redis-cli --user "${redis_username}" ping 2>/dev/null | grep -Fxq PONG; then
  fail 'old Redis credential remained valid after rotation'
fi
docker exec -e "MYSQL_PWD=${rotated_db_password}" "${mariadb_container}" \
  mariadb -h127.0.0.1 "-u${db_username}" trojan_panel_db -e 'SELECT username FROM account' >/dev/null ||
  fail 'rotated MariaDB credential is invalid'
docker exec -e "REDISCLI_AUTH=${rotated_redis_password}" "${redis_container}" \
  redis-cli --user "${redis_username}" ping 2>/dev/null | grep -Fxq PONG ||
  fail 'rotated Redis credential is invalid'
docker exec -e "MYSQL_PWD=${node_b_db_password}" "${mariadb_container}" \
  mariadb -h127.0.0.1 "-u${node_b_db_username}" trojan_panel_db -e 'SELECT username FROM account' >/dev/null ||
  fail 'rotating Node A invalidated Node B MariaDB credentials'
docker exec -e "REDISCLI_AUTH=${node_b_redis_password}" "${redis_container}" \
  redis-cli --user "${node_b_redis_username}" ping 2>/dev/null | grep -Fxq PONG ||
  fail 'rotating Node A invalidated Node B Redis credentials'

mkdir -p "${work}/real-credential-parent/nested"
ln -s "${work}/real-credential-parent" "${work}/symlink-credential-parent"
symlink_credential_file="${work}/symlink-credential-parent/nested/node-d-credentials.json"
if (cd "${work}/runtime" && "${work}/trojan-panel" node-identity register \
  --name node-d --domain node-d.example.com --public-ip 203.0.113.13 \
  --credential-file "${symlink_credential_file}" \
  >"${work}/register-symlink.out" 2>"${work}/register-symlink.err"); then
  fail 'register followed a symlink in the credential file path'
fi
test ! -e "${work}/real-credential-parent/nested/node-d-credentials.json" ||
  fail 'register wrote credentials through a symlinked ancestor'
test "$(docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
  mariadb -N -uroot trojan_panel_db -e "SELECT COUNT(1) FROM node_identity WHERE name='node-d'")" = 0 ||
  fail 'invalid credential path left a Node identity reservation behind'
test "$(docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
  mariadb -N -uroot trojan_panel_db -e "SELECT COUNT(1) FROM node_server WHERE name='node-d'")" = 0 ||
  fail 'invalid credential path left an active control-plane registration behind'

missing_parent_credential_file="${work}/missing-credential-parent/node-e-credentials.json"
if (cd "${work}/runtime" && "${work}/trojan-panel" node-identity register \
  --name node-e --domain node-e.example.com --public-ip 203.0.113.14 \
  --credential-file "${missing_parent_credential_file}" \
  >"${work}/register-missing-parent.out" 2>"${work}/register-missing-parent.err"); then
  fail 'register accepted a missing credential parent directory'
fi
test "$(docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
  mariadb -N -uroot trojan_panel_db -e "SELECT COUNT(1) FROM node_identity WHERE name='node-e'")" = 0 ||
  fail 'missing credential parent left a Node identity reservation behind'
test "$(docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
  mariadb -N -uroot trojan_panel_db -e "SELECT COUNT(1) FROM node_server WHERE name='node-e'")" = 0 ||
  fail 'missing credential parent left an active control-plane registration behind'

(cd "${work}/runtime" && "${work}/trojan-panel" node-identity status --id "${node_identity_id}" \
  >"${work}/status-active.out" 2>"${work}/status-active.err") || fail 'status returned non-zero for active identity'
test "$(jq -r '.status' "${work}/status-active.out")" = active || fail 'status did not report active identity'
test "$(jq -r '.generation' "${work}/status-active.out")" = 2 || fail 'status did not report rotated generation'
for secret in "${rotated_db_password}" "${rotated_redis_password}"; do
  ! grep -Fq -- "${secret}" "${work}/status-active.out" "${work}/status-active.err" || fail 'status leaked a secret'
done

(cd "${work}/runtime" && "${work}/trojan-panel" node-identity revoke --id "${node_identity_id}" \
  >"${work}/revoke.out" 2>"${work}/revoke.err") || fail 'revoke returned non-zero'
if docker exec -e "MYSQL_PWD=${rotated_db_password}" "${mariadb_container}" \
    mariadb -h127.0.0.1 "-u${db_username}" trojan_panel_db -e 'SELECT 1' >/dev/null 2>&1; then
  fail 'revoked MariaDB credential remained valid'
fi
if docker exec -e "REDISCLI_AUTH=${rotated_redis_password}" "${redis_container}" \
    redis-cli --user "${redis_username}" ping 2>/dev/null | grep -Fxq PONG; then
  fail 'revoked Redis credential remained valid'
fi
docker exec -e "MYSQL_PWD=${node_b_db_password}" "${mariadb_container}" \
  mariadb -h127.0.0.1 "-u${node_b_db_username}" trojan_panel_db -e 'SELECT 1' >/dev/null ||
  fail 'revoking Node A invalidated Node B MariaDB credentials'
docker exec -e "REDISCLI_AUTH=${node_b_redis_password}" "${redis_container}" \
  redis-cli --user "${node_b_redis_username}" ping 2>/dev/null | grep -Fxq PONG ||
  fail 'revoking Node A invalidated Node B Redis credentials'
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity revoke --id "${node_identity_id}" \
  >"${work}/revoke-replay.out" 2>"${work}/revoke-replay.err") || fail 'repeated revoke was not idempotent'
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity status --id "${node_identity_id}" \
  >"${work}/status-revoked.out" 2>"${work}/status-revoked.err") || fail 'status returned non-zero for revoked identity'
test "$(jq -r '.status' "${work}/status-revoked.out")" = revoked || fail 'status did not report revoked identity'
test "$(docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
  mariadb -N -uroot trojan_panel_db -e "SELECT COUNT(1) FROM node_server WHERE id=${node_server_id}")" = 0 ||
  fail 'revoke retained an active control-plane registration'

node_b_identity_id="$(jq -r '.node_identity_id' "${node_b_credential_file}")"
node_b_server_id="$(jq -r '.node_server_id' "${node_b_credential_file}")"
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity force-evict --id "${node_b_identity_id}" \
  >"${work}/evict.out" 2>"${work}/evict.err") || fail 'force-evict returned non-zero while the Node was offline'
if docker exec -e "MYSQL_PWD=${node_b_db_password}" "${mariadb_container}" \
    mariadb -h127.0.0.1 "-u${node_b_db_username}" trojan_panel_db -e 'SELECT 1' >/dev/null 2>&1; then
  fail 'force-evicted MariaDB credential remained valid'
fi
if docker exec -e "REDISCLI_AUTH=${node_b_redis_password}" "${redis_container}" \
    redis-cli --user "${node_b_redis_username}" ping 2>/dev/null | grep -Fxq PONG; then
  fail 'force-evicted Redis credential remained valid'
fi
test "$(docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
  mariadb -N -uroot trojan_panel_db -e "SELECT COUNT(1) FROM node_server WHERE id=${node_b_server_id}")" = 0 ||
  fail 'force-evict retained the active control-plane registration'
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity status --id "${node_b_identity_id}" \
  >"${work}/status-evicted.out" 2>"${work}/status-evicted.err") || fail 'status returned non-zero for evicted identity'
test "$(jq -r '.status' "${work}/status-evicted.out")" = evicted || fail 'status did not report evicted identity'
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity force-evict --id "${node_b_identity_id}" \
  >"${work}/evict-replay.out" 2>"${work}/evict-replay.err") || fail 'repeated force-evict was not idempotent'

node_c_credential_file="${work}/runtime/config/node-c-credentials.json"
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity register \
  --name node-c --domain node-c.example.com --public-ip 203.0.113.12 \
  --credential-file "${node_c_credential_file}" \
  >"${work}/register-c.out" 2>"${work}/register-c.err") || fail 'Node C registration returned non-zero'
node_c_identity_id="$(jq -r '.node_identity_id' "${node_c_credential_file}")"
docker exec -e "REDISCLI_AUTH=${admin_redis_password}" "${redis_container}" \
  redis-cli ACL SETUSER default -acl >/dev/null
node_c_rotated_file="${work}/runtime/config/node-c-credentials-generation-2.json"
if (cd "${work}/runtime" && "${work}/trojan-panel" node-identity rotate \
  --id "${node_c_identity_id}" --credential-file "${node_c_rotated_file}" \
  >"${work}/rotate-c-failed.out" 2>"${work}/rotate-c-failed.err"); then
  fail 'rotation succeeded after the Redis ACL provisioning boundary failed'
fi
test -f "${node_c_rotated_file}" || fail 'failed rotation did not retain retry credentials'
test "$(stat -c '%a' "${node_c_rotated_file}")" = 600 || fail 'retry credential file mode is not 0600'
node_c_rotated_db_password="$(jq -r '.mariadb.password' "${node_c_rotated_file}")"
node_c_rotated_redis_password="$(jq -r '.redis.password' "${node_c_rotated_file}")"
for secret in "${node_c_rotated_db_password}" "${node_c_rotated_redis_password}"; do
  ! grep -Fq -- "${secret}" "${work}/rotate-c-failed.out" "${work}/rotate-c-failed.err" ||
    fail 'failed rotation leaked a retry credential'
done
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity status --id "${node_c_identity_id}" \
  >"${work}/status-c-rotating.out" 2>"${work}/status-c-rotating.err") || fail 'status failed for interrupted rotation'
test "$(jq -r '.status' "${work}/status-c-rotating.out")" = rotating || fail 'interrupted rotation was not recorded as rotating'

docker rm -fv "${redis_container}" >/dev/null
docker run -d --name "${redis_container}" \
  -p 127.0.0.1::6379 "${redis_image}" \
  redis-server --requirepass "${admin_redis_password}" >/dev/null
recovered_redis_port="$(docker port "${redis_container}" 6379/tcp | sed -n 's/.*://p' | head -n1)"
sed -i "s/^port=${redis_port}$/port=${recovered_redis_port}/" "${work}/runtime/config/config.ini"
redis_port="${recovered_redis_port}"
for _ in $(seq 1 30); do
  if docker exec -e "REDISCLI_AUTH=${admin_redis_password}" "${redis_container}" \
      redis-cli ping 2>/dev/null | grep -Fxq PONG; then
    break
  fi
  sleep 1
done
for _ in $(seq 1 15); do
  if (cd "${work}/runtime" && "${work}/trojan-panel" node-identity status --id "${node_c_identity_id}" \
      >"${work}/status-c-restart-ready.out" 2>"${work}/status-c-restart-ready.err"); then
    break
  fi
  sleep 1
done
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity status --id "${node_c_identity_id}" \
  >"${work}/status-c-restart-ready.out" 2>"${work}/status-c-restart-ready.err") ||
  fail 'control-plane data services did not become reachable after Redis restart'
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity rotate \
  --id "${node_c_identity_id}" --credential-file "${node_c_rotated_file}" \
  >"${work}/rotate-c-retry.out" 2>"${work}/rotate-c-retry.err") ||
  {
    sed -n '1,80p' "${work}/rotate-c-retry.err" >&2
    docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
      mariadb -uroot trojan_panel_db -e "SELECT action,result,error_code FROM node_identity_event WHERE identity_id='${node_c_identity_id}'" >&2 || true
    fail 'rotation did not converge when retried with the same credential file'
  }
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity status --id "${node_c_identity_id}" \
  >"${work}/status-c-active.out" 2>"${work}/status-c-active.err") || fail 'status failed after rotation recovery'
test "$(jq -r '.status' "${work}/status-c-active.out")" = active || fail 'retried rotation did not return to active'
test "$(jq -r '.generation' "${work}/status-c-active.out")" = 2 || fail 'retried rotation advanced generation more than once'
node_c_db_username="$(jq -r '.mariadb.username' "${node_c_rotated_file}")"
node_c_redis_username="$(jq -r '.redis.username' "${node_c_rotated_file}")"
docker exec -e "MYSQL_PWD=${node_c_rotated_db_password}" "${mariadb_container}" \
  mariadb -h127.0.0.1 "-u${node_c_db_username}" trojan_panel_db -e 'SELECT 1' >/dev/null ||
  fail 'recovered MariaDB credential is invalid'
docker exec -e "REDISCLI_AUTH=${node_c_rotated_redis_password}" "${redis_container}" \
  redis-cli --user "${node_c_redis_username}" ping 2>/dev/null | grep -Fxq PONG ||
  fail 'recovered Redis credential is invalid'

printf 'PASS node identity CLI lifecycle, isolation, failure recovery, and Node Agent ACL contract\n'
