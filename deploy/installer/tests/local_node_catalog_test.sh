#!/usr/bin/env bash
set -Eeuo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
api_dir="${root}/apps/control-plane/api"
client_dir="${root}/deploy/installer/client"
work="$(mktemp -d)"
suffix="${RANDOM}-$$"
db_container="tp-catalog-db-${suffix}"
redis_container="tp-catalog-redis-${suffix}"
db_image='mariadb@sha256:07e06f2e7ae9dfc63707a83130a62e00167c827f08fcac7a9aa33f4b6dc34e0e'
redis_image='redis@sha256:a93c14584715ec5bd9d2648d58c3b27f89416242bee0bc9e5fb2edc1a4cbec1d'
db_password='CatalogTestDatabasePassword'
redis_password='CatalogTestRedisPassword'
cleanup() {
  docker rm -fv "${db_container}" "${redis_container}" >/dev/null 2>&1 || true
  rm -r -- "${work}"
}
trap cleanup EXIT
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
reject() {
  local case_name="$1"; shift
  if "$@" >"${work}/reject.out" 2>&1; then fail "accepted ${case_name}"; fi
  printf 'TRACE rejected=%s\n' "${case_name}"
}
if ! docker info >/dev/null 2>&1; then
  docker() { sudo -n /usr/bin/docker "$@"; }
fi
command -v jq >/dev/null || fail 'jq is required'
command -v yq >/dev/null || fail 'mikefarah yq v4 is required'
[[ "$(yq --version)" == *'version v4.'* ]] || fail 'mikefarah yq v4 is required'

sed -e 's/__RELEASE_TAG__/v1.2.3/g' -e "s/__ARCHIVE_SHA256__/$(printf '%064d' 1)/g" \
  "${client_dir}/templates/unified-ssh.yaml" >"${work}/unified.yaml"
first_plan="$(bash "${client_dir}/node-catalog.sh" --config "${work}/unified.yaml" --node-key node-one)"
jq -e '.node_key=="node-one" and .host_id=="node-host" and .name=="node-one" and .grpc_port==8100' \
  <<<"${first_plan}" >/dev/null || fail 'first YAML catalog mapping is wrong'
yq -i '.hosts."second-host" = {"transport":"ssh","ssh":{"user":"root","hostname":"203.0.113.30","port":22,"identity_file":"","config_file":""}} |
  .nodes += [(.nodes[0] | .node_key = "node-two" | .host = "second-host" | .name = "node-two" |
    .domain = "node-two.example.com" | .public_ip = "203.0.113.30" | .settings.grpc_port = 8200)]' "${work}/unified.yaml"
second_plan="$(bash "${client_dir}/node-catalog.sh" --config "${work}/unified.yaml" --node-key node-two)"
jq -e '.node_key=="node-two" and .host_id=="second-host" and .grpc_port==8200' \
  <<<"${second_plan}" >/dev/null || fail 'appended YAML catalog mapping is wrong'
yq -i '.nodes |= reverse' "${work}/unified.yaml"
test "$(bash "${client_dir}/node-catalog.sh" --config "${work}/unified.yaml" --node-key node-one)" = "${first_plan}" ||
  fail 'YAML reorder changed the stable node_key mapping'
test "$(bash "${client_dir}/node-catalog.sh" --config "${work}/unified.yaml" --node-key node-two)" = "${second_plan}" ||
  fail 'YAML reorder changed the appended node_key mapping'
reject 'undeclared YAML node_key' bash "${client_dir}/node-catalog.sh" --config "${work}/unified.yaml" --node-key unknown

(cd "${api_dir}" && CGO_ENABLED=0 go build -buildvcs=false -trimpath -o "${work}/trojan-panel" .)
docker run -d --name "${db_container}" -e "MARIADB_ROOT_PASSWORD=${db_password}" \
  -e MARIADB_DATABASE=trojan_panel_db -p 127.0.0.1::3306 "${db_image}" >/dev/null
docker run -d --name "${redis_container}" -p 127.0.0.1::6379 "${redis_image}" \
  redis-server --requirepass "${redis_password}" >/dev/null
for _ in $(seq 1 90); do
  docker exec -e "MYSQL_PWD=${db_password}" "${db_container}" mariadb -uroot -e 'SELECT 1' >/dev/null 2>&1 && break
  sleep 1
done
docker exec -e "MYSQL_PWD=${db_password}" "${db_container}" mariadb -uroot -e 'SELECT 1' >/dev/null || fail 'MariaDB unavailable'
for _ in $(seq 1 30); do
  docker exec -e "REDISCLI_AUTH=${redis_password}" "${redis_container}" redis-cli ping 2>/dev/null | grep -Fxq PONG && break
  sleep 1
done
docker exec -e "REDISCLI_AUTH=${redis_password}" "${redis_container}" redis-cli ping | grep -Fxq PONG || fail 'Redis unavailable'

docker exec -i -e "MYSQL_PWD=${db_password}" "${db_container}" mariadb -uroot trojan_panel_db <<'SQL'
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
INSERT INTO account (username) VALUES ('catalog-test');
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
CREATE TABLE account_traffic_total (account_id bigint unsigned NOT NULL, upload bigint unsigned NOT NULL DEFAULT 0, download bigint unsigned NOT NULL DEFAULT 0, PRIMARY KEY (account_id));
CREATE TABLE account_traffic_daily (traffic_date date NOT NULL, account_id bigint unsigned NOT NULL, upload bigint unsigned NOT NULL DEFAULT 0, download bigint unsigned NOT NULL DEFAULT 0, PRIMARY KEY (traffic_date, account_id));
CREATE TABLE account_server_traffic_daily (traffic_date date NOT NULL, account_id bigint unsigned NOT NULL, node_server_id bigint unsigned NOT NULL, upload bigint unsigned NOT NULL DEFAULT 0, download bigint unsigned NOT NULL DEFAULT 0, PRIMARY KEY (traffic_date, account_id, node_server_id));
SQL
db_port="$(docker port "${db_container}" 3306/tcp | sed -n 's/.*://p' | head -n1)"
redis_port="$(docker port "${redis_container}" 6379/tcp | sed -n 's/.*://p' | head -n1)"
mkdir -m 0700 -p "${work}/runtime/config" "${work}/credentials"
chmod 0700 "${work}/credentials"
printf '%s\n' '[mysql]' 'host=127.0.0.1' 'user=root' "password=${db_password}" "port=${db_port}" \
  '[redis]' 'host=127.0.0.1' "port=${redis_port}" "password=${redis_password}" \
  'db=0' 'max_idle=2' 'max_active=4' 'wait=true' >"${work}/runtime/config/config.ini"
chmod 0600 "${work}/runtime/config/config.ini"
db_scalar() {
  docker exec -e "MYSQL_PWD=${db_password}" "${db_container}" mariadb -N -uroot trojan_panel_db -e "$1"
}
db_exec() {
  docker exec -e "MYSQL_PWD=${db_password}" "${db_container}" mariadb -uroot trojan_panel_db -e "$1" >/dev/null
}
catalog() {
  (cd "${work}/runtime" && "${work}/trojan-panel" node-identity catalog "$@" --credential-dir "${work}/credentials")
}
node_one=(--node-key node-one --host-id node-host --name node-one --domain node-one.example.com --public-ip 203.0.113.20 --grpc-port 8100)
node_two=(--node-key node-two --host-id second-host --name node-two --domain node-two.example.com --public-ip 203.0.113.30 --grpc-port 8200)

catalog reconcile "${node_one[@]}" >"${work}/one.json"
one_id="$(jq -r '.node_identity_id' "${work}/one.json")"
one_server="$(jq -r '.node_server_id' "${work}/one.json")"
[[ "${one_id}" =~ ^[0-9a-f-]{36}$ && "${one_server}" =~ ^[1-9][0-9]*$ ]] || fail 'first IDs are invalid'
test "$(db_scalar 'SELECT COUNT(*) FROM node_identity')" = 1 || fail 'first identity row missing'
test "$(db_scalar 'SELECT COUNT(*) FROM node_server')" = 1 || fail 'first server row missing'
test "$(stat -c %a "${work}/credentials/node-one.g1.json")" = 600 || fail 'credential file mode is not 0600'
test "$(stat -c %a "${work}/credentials/node-one.binding.json")" = 600 || fail 'binding file mode is not 0600'
test "$(stat -c %a "${work}/credentials/node-one.identity.json")" = 600 || fail 'ID commitment mode is not 0600'
one_credential_sha="$(sha256sum "${work}/credentials/node-one.g1.json" | awk '{print $1}')"
catalog reconcile "${node_one[@]}" >"${work}/one-replay.json"
cmp "${work}/one.json" "${work}/one-replay.json" || fail 'replay changed ID, generation, fields or health status'
catalog lookup "${node_one[@]}" >"${work}/one-lookup.json"
cmp "${work}/one.json" "${work}/one-lookup.json" || fail 'lookup differs from reconciliation'

catalog reconcile "${node_two[@]}" >"${work}/two.json"
two_id="$(jq -r '.node_identity_id' "${work}/two.json")"
two_server="$(jq -r '.node_server_id' "${work}/two.json")"
test "${one_id}" != "${two_id}" && test "${one_server}" != "${two_server}" || fail 'Node identities collide'
test "$(db_scalar 'SELECT COUNT(*) FROM node_identity')" = 2 || fail 'append did not create exactly one identity'
test "$(db_scalar 'SELECT COUNT(*) FROM node_server')" = 2 || fail 'append did not create exactly one server'
test "$(db_scalar "SELECT grpc_port FROM node_server WHERE id=${two_server}")" = 8200 || fail 'configured gRPC port was not committed'
test "$(db_scalar "SELECT COUNT(*) FROM mysql.user WHERE User=(SELECT mariadb_username FROM node_identity WHERE identity_id='${one_id}')")" = 1 || fail 'first MariaDB ACL user missing'
test "$(db_scalar "SELECT COUNT(*) FROM mysql.user WHERE User=(SELECT mariadb_username FROM node_identity WHERE identity_id='${two_id}')")" = 1 || fail 'second MariaDB ACL user missing'
test "$(docker exec -e "REDISCLI_AUTH=${redis_password}" "${redis_container}" redis-cli --raw ACL USERS | grep -c '^tpn-')" = 4 || fail 'independent Redis ACL users missing'
catalog reconcile "${node_two[@]}" >"${work}/two-replay.json"
catalog reconcile "${node_one[@]}" >"${work}/one-reordered.json"
cmp "${work}/one.json" "${work}/one-reordered.json" || fail 'reordered Node changed its registration'
test "$(db_scalar 'SELECT COUNT(*) FROM node_identity')" = 2 || fail 'replay inserted an identity'
test "$(db_scalar 'SELECT COUNT(*) FROM node_server')" = 2 || fail 'replay inserted a server'
test "$(sha256sum "${work}/credentials/node-one.g1.json" | awk '{print $1}')" = "${one_credential_sha}" ||
  fail 'replay rewrote or rotated the old Node credential'
db_exec "UPDATE node_server SET traffic_period='month',traffic_limit_mode='upload',traffic_total_limit=50,traffic_upload_limit=40,traffic_download_limit=30 WHERE id=${one_server}"
catalog lookup "${node_one[@]}" >"${work}/one-web-policy.json"
catalog reconcile "${node_one[@]}" >"${work}/one-web-policy-replay.json"
cmp "${work}/one.json" "${work}/one-web-policy-replay.json" || fail 'Web-owned traffic policy changed catalog identity facts'
test "$(db_scalar "SELECT CONCAT_WS(',',traffic_period,traffic_limit_mode,traffic_total_limit,traffic_upload_limit,traffic_download_limit) FROM node_server WHERE id=${one_server}")" = 'month,upload,50,40,30' ||
  fail 'catalog replay overwrote Web-owned traffic policy'
yq -i 'del(.nodes[] | select(.node_key == "node-two")) | del(.hosts."second-host")' "${work}/unified.yaml"
test "$(bash "${client_dir}/node-catalog.sh" --config "${work}/unified.yaml" --node-key node-one)" = "${first_plan}" ||
  fail 'removing another YAML Node changed the remaining node_key mapping'
test "$(db_scalar 'SELECT COUNT(*) FROM node_identity')" = 2 || fail 'YAML deletion revoked or removed a Node identity'

reject 'host drift' catalog reconcile --node-key node-one --host-id different-host "${node_one[@]:4}"
reject 'domain drift' catalog reconcile --node-key node-one --host-id node-host --name node-one --domain drift.example.com --public-ip 203.0.113.20 --grpc-port 8100
reject 'IP drift' catalog reconcile --node-key node-one --host-id node-host --name node-one --domain node-one.example.com --public-ip 203.0.113.21 --grpc-port 8100
reject 'port drift' catalog reconcile --node-key node-one --host-id node-host --name node-one --domain node-one.example.com --public-ip 203.0.113.20 --grpc-port 8101
reject 'name collision' catalog reconcile --node-key node-three --host-id third-host --name node-one --domain node-three.example.com --public-ip 203.0.113.40 --grpc-port 8100
reject 'IP collision' catalog reconcile --node-key node-three --host-id third-host --name node-three --domain node-three.example.com --public-ip 203.0.113.20 --grpc-port 8100
db_exec "UPDATE node_identity SET credential_path='/tmp/wrong-credential.json' WHERE identity_id='${one_id}'"
reject 'credential path drift' catalog reconcile "${node_one[@]}"
db_exec "UPDATE node_identity SET credential_path='${work}/credentials/node-one.g1.json' WHERE identity_id='${one_id}'"
db_exec "UPDATE node_identity SET generation=2 WHERE identity_id='${one_id}'"
reject 'generation drift' catalog lookup "${node_one[@]}"
db_exec "UPDATE node_identity SET generation=1 WHERE identity_id='${one_id}'"
db_exec "UPDATE node_identity SET status='revoked' WHERE identity_id='${one_id}'"
reject 'revoked identity' catalog reconcile "${node_one[@]}"
db_exec "UPDATE node_identity SET status='active' WHERE identity_id='${one_id}'"
db_exec "UPDATE node_server SET grpc_tls_server_name='drift.example.com' WHERE id=${one_server}"
reject 'server table drift' catalog reconcile "${node_one[@]}"
db_exec "UPDATE node_server SET grpc_tls_server_name='node-one.example.com' WHERE id=${one_server}"
db_exec "UPDATE node_identity SET redis_auth_username='shared-user' WHERE identity_id='${one_id}'"
reject 'ACL identity drift' catalog lookup "${node_one[@]}"
db_exec "UPDATE node_identity SET redis_auth_username='$(jq -r '.redis_auth.username' "${work}/credentials/node-one.g1.json")' WHERE identity_id='${one_id}'"
cp "${work}/credentials/node-one.identity.json" "${work}/identity-commitment.backup"
jq '.node_server_id += 100' "${work}/identity-commitment.backup" >"${work}/identity-commitment.tampered"
chmod 0600 "${work}/identity-commitment.tampered"
mv "${work}/identity-commitment.tampered" "${work}/credentials/node-one.identity.json"
reject 'committed server ID drift' catalog reconcile "${node_one[@]}"
cp "${work}/identity-commitment.backup" "${work}/credentials/node-one.identity.json"
test "$(db_scalar 'SELECT COUNT(*) FROM node_identity')" = 2 || fail 'conflict inserted an identity'
test "$(db_scalar 'SELECT COUNT(*) FROM node_server')" = 2 || fail 'conflict inserted a server'

printf 'TRACE ids=node-one:%s/%s,node-two:%s/%s generation=1 rows=2+2 ACL=2-MariaDB+4-Redis\n' \
  "${one_id}" "${one_server}" "${two_id}" "${two_server}"
printf 'PASS real MariaDB/Redis Node catalog append, replay, reorder and conflict\n'
