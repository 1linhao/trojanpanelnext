#!/usr/bin/env bash
set -Eeuo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
work="$(mktemp -d)"
suffix="${RANDOM}-$$"
db_container="tp-combined-rotate-db-${suffix}"
redis_container="tp-combined-rotate-redis-${suffix}"
db_password='CombinedRotationRoot-5'
redis_password='CombinedRotationRedis-5'

if ! docker info >/dev/null 2>&1; then
  docker() { sudo -n /usr/bin/docker "$@"; }
fi
cleanup_test() {
  docker rm -fv "${db_container}" "${redis_container}" >/dev/null 2>&1 || true
  rm -rf -- "${work}"
}
trap cleanup_test EXIT
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

(cd "${repo}/apps/control-plane/api" && CGO_ENABLED=0 go build -trimpath -o "${work}/trojan-panel" .)
docker run -d --name "${db_container}" -e "MARIADB_ROOT_PASSWORD=${db_password}" \
  -e MARIADB_DATABASE=trojan_panel_db -p 127.0.0.1::3306 mariadb:10.7.3 >/dev/null
docker run -d --name "${redis_container}" -p 127.0.0.1::6379 redis:6.2.7 \
  redis-server --requirepass "${redis_password}" >/dev/null

for _ in $(seq 1 60); do
  if docker exec -e "MYSQL_PWD=${db_password}" "${db_container}" mariadb -uroot -e 'SELECT 1' >/dev/null 2>&1 &&
    docker exec -e "REDISCLI_AUTH=${redis_password}" "${redis_container}" redis-cli ping 2>/dev/null | grep -Fxq PONG; then
    break
  fi
  sleep 1
done
docker exec -e "MYSQL_PWD=${db_password}" "${db_container}" mariadb -uroot -e 'SELECT 1' >/dev/null || fail 'MariaDB is unavailable'
docker exec -e "REDISCLI_AUTH=${redis_password}" "${redis_container}" redis-cli ping | grep -Fxq PONG || fail 'Redis is unavailable'

docker exec -i -e "MYSQL_PWD=${db_password}" "${db_container}" mariadb -uroot trojan_panel_db <<'SQL'
CREATE TABLE account (id bigint unsigned NOT NULL AUTO_INCREMENT, username varchar(64) NOT NULL,
  pass varchar(64) NOT NULL DEFAULT '', hash varchar(64) NOT NULL DEFAULT '',
  quota bigint NOT NULL DEFAULT -1, download bigint unsigned NOT NULL DEFAULT 0,
  upload bigint unsigned NOT NULL DEFAULT 0, PRIMARY KEY (id));
INSERT INTO account (username) VALUES ('integration-user');
CREATE TABLE node_server (id bigint unsigned NOT NULL AUTO_INCREMENT,
  ip varchar(64) NOT NULL DEFAULT '', name varchar(64) NOT NULL DEFAULT '',
  grpc_port int unsigned NOT NULL DEFAULT 8100, grpc_tls_mode varchar(16) NOT NULL DEFAULT 'mtls',
  grpc_tls_server_name varchar(253) NOT NULL DEFAULT '', traffic_period varchar(8) NOT NULL DEFAULT 'none',
  traffic_limit_mode varchar(8) NOT NULL DEFAULT 'combined', traffic_total_limit bigint unsigned NOT NULL DEFAULT 0,
  traffic_upload_limit bigint unsigned NOT NULL DEFAULT 0, traffic_download_limit bigint unsigned NOT NULL DEFAULT 0,
  create_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP, PRIMARY KEY (id));
CREATE TABLE account_traffic_total (account_id bigint unsigned NOT NULL,
  upload bigint unsigned NOT NULL DEFAULT 0, download bigint unsigned NOT NULL DEFAULT 0, PRIMARY KEY (account_id));
CREATE TABLE account_traffic_daily (traffic_date date NOT NULL, account_id bigint unsigned NOT NULL,
  upload bigint unsigned NOT NULL DEFAULT 0, download bigint unsigned NOT NULL DEFAULT 0,
  PRIMARY KEY (traffic_date, account_id));
CREATE TABLE account_server_traffic_daily (traffic_date date NOT NULL, account_id bigint unsigned NOT NULL,
  node_server_id bigint unsigned NOT NULL, upload bigint unsigned NOT NULL DEFAULT 0,
  download bigint unsigned NOT NULL DEFAULT 0, PRIMARY KEY (traffic_date, account_id, node_server_id));
CREATE TABLE system (id bigint unsigned NOT NULL PRIMARY KEY, name varchar(32) NOT NULL);
INSERT INTO system VALUES (1, 'private-control-data');
SQL

db_port="$(docker port "${db_container}" 3306/tcp | sed -n 's/.*://p' | head -n 1)"
redis_port="$(docker port "${redis_container}" 6379/tcp | sed -n 's/.*://p' | head -n 1)"
mkdir -p "${work}/runtime/config" "${work}/data/trojan-panel/config/node-identities"
chmod 0700 "${work}/runtime/config" "${work}/data/trojan-panel/config/node-identities"
printf '[mysql]\nhost=127.0.0.1\nuser=root\npassword=%s\nport=%s\n[redis]\nhost=127.0.0.1\nport=%s\npassword=%s\ndb=0\nmax_idle=2\nmax_active=4\nwait=true\n' \
  "${db_password}" "${db_port}" "${redis_port}" "${redis_password}" >"${work}/runtime/config/config.ini"
chmod 0600 "${work}/runtime/config/config.ini"

first="${work}/data/trojan-panel/config/node-identities/combined-node.json"
second="${work}/data/trojan-panel/config/node-identities/combined-node.g2.json"
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity register \
  --name combined-node --domain node.example.com --public-ip 203.0.113.10 \
  --credential-file "${first}" >"${work}/register.out" 2>"${work}/register.err") || fail 'official Node registration failed'

TP_DATA="${work}/data"
MARIADB_CONTAINER="${db_container}"
MARIADB_PASSWORD="${db_password}"
MARIADB_DATABASE=trojan_panel_db
TP_ASSET_VERSION=development
TP_WEB_DOMAIN=panel.example.com
TP_NODE_DOMAIN=node.example.com
TP_NODE_NAME=combined-node
TP_NODE_PUBLIC_IP=203.0.113.10
NODE_IDENTITY_CREDENTIAL_FILE="${first}"
INSTALLER_STATE_DIR="${TP_DATA}/trojanpanelnext-installer"
source "${repo}/deploy/installer/install.sh"
trap cleanup_test EXIT
installer_owned_dir_prepare() { mkdir -p "$1"; }
load_combined_identity_metadata
write_installer_state combined
check_same_version_replay_preconditions combined || fail 'installer rejected official registered credential'

identity_id="${NODE_IDENTITY_ID}"
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity rotate \
  --id "${identity_id}" --credential-file "${second}" >"${work}/rotate.out" 2>"${work}/rotate.err") || fail 'official Node rotation failed'
[[ -f "${first}" && -f "${second}" && "$(jq -r '.generation' "${second}")" == 2 &&
  "$(stat -c %a "${second}")" == 600 ]] || fail 'official rotation omitted its restricted new credential file'
snapshot_managed() {
  sha256sum "${INSTALLER_STATE_DIR}/combined.state" "${first}" "${second}"
  docker exec -e "MYSQL_PWD=${db_password}" "${db_container}" \
    mariadb --batch --skip-column-names -uroot --database=trojan_panel_db \
    -e "SELECT generation,credential_sha256,status,credential_path FROM node_identity WHERE identity_id='${identity_id}'"
}
NODE_IDENTITY_CREDENTIAL_FILE="${second}"
load_combined_identity_metadata
check_same_version_replay_preconditions combined || fail 'installer rejected official rotated credential and new path'

NODE_IDENTITY_CREDENTIAL_FILE="${first}"
load_combined_identity_metadata
before="$(snapshot_managed)"
if check_same_version_replay_preconditions combined >"${work}/old-path.out" 2>&1; then
  fail 'installer accepted the stale credential path after official rotation'
fi
[[ "${before}" == "$(snapshot_managed)" ]] || fail 'stale-path rejection changed managed state'
NODE_IDENTITY_CREDENTIAL_FILE="${second}"
load_combined_identity_metadata
write_installer_state combined

cp "${second}" "${work}/committed-rotation.json"
jq '.redis.password = "tampered-after-rotation"' "${work}/committed-rotation.json" >"${second}"
chmod 0600 "${second}"
before="$(snapshot_managed)"
if check_same_version_replay_preconditions combined >"${work}/same-generation-tamper.out" 2>&1; then
  fail 'installer accepted same-generation credential tampering'
fi
[[ "${before}" == "$(snapshot_managed)" ]] || fail 'same-generation rejection changed managed state'
printf 'PASS official Node CLI rotation produces accepted new-path replay and rejects stale/tampered credentials\n'
