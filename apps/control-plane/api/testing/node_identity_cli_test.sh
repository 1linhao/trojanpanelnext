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
INSTALLER_DIR="$(cd "${API_DIR}/../../../deploy/installer" && pwd)"
work="$(mktemp -d)"
suffix="${RANDOM}-$$"
mariadb_container="tp-node-identity-mariadb-${suffix}"
redis_container="tp-node-identity-redis-${suffix}"
node_container="tp-node-bootstrap-agent-${suffix}"
mariadb_image='mariadb@sha256:07e06f2e7ae9dfc63707a83130a62e00167c827f08fcac7a9aa33f4b6dc34e0e'
redis_image='redis@sha256:a93c14584715ec5bd9d2648d58c3b27f89416242bee0bc9e5fb2edc1a4cbec1d'

cleanup() {
  docker rm -fv "${node_container}" "${mariadb_container}" "${redis_container}" >/dev/null 2>&1 || true
  rm -rf -- "${work}"
}
trap cleanup EXIT

(cd "${API_DIR}" && CGO_ENABLED=0 go build -trimpath -o "${work}/trojan-panel" .)
(cd "${API_DIR}" && CGO_ENABLED=0 go build -trimpath -tags nodeidentitycrashtest -o "${work}/trojan-panel-crash-test" .)
(cd "${INSTALLER_DIR}/nodebundle" && CGO_ENABLED=0 go build -trimpath -o "${work}/node-bundle" .)
(cd "${API_DIR}/../../node-agent" && CGO_ENABLED=0 go build -trimpath -o "${work}/trojan-panel-core" .)
if strings "${work}/trojan-panel" | grep -Fq 'TP_NODE_IDENTITY_TEST_CRASH_AT'; then
  fail 'production API binary contains the Node identity crash-test hook'
fi

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
docker exec -i -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
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

db_scalar() {
  docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
    mariadb -N -uroot trojan_panel_db -e "$1"
}

assert_crashed_registration_has_no_data_identity() {
  local name="$1"
  local mariadb_username redis_username redis_auth_username
  mariadb_username="$(db_scalar "SELECT mariadb_username FROM node_identity WHERE name='${name}'")"
  redis_username="$(db_scalar "SELECT redis_username FROM node_identity WHERE name='${name}'")"
  redis_auth_username="$(db_scalar "SELECT redis_auth_username FROM node_identity WHERE name='${name}'")"
  test "$(docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
    mariadb -N -uroot mysql -e "SELECT COUNT(1) FROM user WHERE User='${mariadb_username}'")" = 0 ||
    fail "${name} crash left an orphan MariaDB identity"
  test -z "$(docker exec -e "REDISCLI_AUTH=${admin_redis_password}" "${redis_container}" \
    redis-cli --raw ACL GETUSER "${redis_username}")" || fail "${name} crash left an orphan cache Redis identity"
  test -z "$(docker exec -e "REDISCLI_AUTH=${admin_redis_password}" "${redis_container}" \
    redis-cli --raw ACL GETUSER "${redis_auth_username}")" || fail "${name} crash left an orphan auth Redis identity"
}

# The crash injector exists only in the tagged test binary. Each invocation
# exercises the public CLI in a separate process, then retries the exact user
# command and proves that the reservation, generated file, and digest commit
# are all recoverable without regenerating credentials or provisioning users
# ahead of the durable commitment.
crash_reservation_file="${work}/runtime/config/node-crash-reservation.json"
set +e
(cd "${work}/runtime" && TP_NODE_IDENTITY_TEST_CRASH_AT=register_after_reservation \
  "${work}/trojan-panel-crash-test" node-identity register \
  --name node-crash-reservation --domain node-crash-reservation.example.com --public-ip 203.0.113.20 \
  --credential-file "${crash_reservation_file}" >"${work}/crash-reservation.out" 2>"${work}/crash-reservation.err")
crash_status=$?
set -e
test "${crash_status}" = 86 || fail 'reservation crash hook did not terminate at the public CLI boundary'
test ! -e "${crash_reservation_file}" || fail 'reservation crash published a credential file too early'
test "$(db_scalar "SELECT status FROM node_identity WHERE name='node-crash-reservation'")" = provisioning ||
  fail 'reservation crash did not leave a recoverable provisioning intent'
assert_crashed_registration_has_no_data_identity node-crash-reservation
(cd "${work}/runtime" && "${work}/trojan-panel-crash-test" node-identity register \
  --name node-crash-reservation --domain node-crash-reservation.example.com --public-ip 203.0.113.20 \
  --credential-file "${crash_reservation_file}" >"${work}/recover-reservation.out" 2>"${work}/recover-reservation.err") ||
  fail 'registration did not recover after the reservation crash point'
test "$(db_scalar "SELECT status FROM node_identity WHERE name='node-crash-reservation'")" = active ||
  fail 'reservation crash recovery did not activate the identity'

crash_file_file="${work}/runtime/config/node-crash-file.json"
set +e
(cd "${work}/runtime" && TP_NODE_IDENTITY_TEST_CRASH_AT=register_after_credential_file \
  "${work}/trojan-panel-crash-test" node-identity register \
  --name node-crash-file --domain node-crash-file.example.com --public-ip 203.0.113.21 \
  --credential-file "${crash_file_file}" >"${work}/crash-file.out" 2>"${work}/crash-file.err")
crash_status=$?
set -e
test "${crash_status}" = 86 || fail 'credential-file crash hook did not terminate at the public CLI boundary'
test -f "${crash_file_file}" || fail 'credential-file crash point did not publish the intended file'
test "$(stat -c '%a' "${crash_file_file}")" = 600 || fail 'crash-recovery credential file is not 0600'
test -z "$(db_scalar "SELECT credential_sha256 FROM node_identity WHERE name='node-crash-file'")" ||
  fail 'credential-file crash point committed the digest too early'
assert_crashed_registration_has_no_data_identity node-crash-file
cp "${crash_file_file}" "${work}/crash-file.original"
(cd "${work}/runtime" && "${work}/trojan-panel-crash-test" node-identity register \
  --name node-crash-file --domain node-crash-file.example.com --public-ip 203.0.113.21 \
  --credential-file "${crash_file_file}" >"${work}/recover-file.out" 2>"${work}/recover-file.err") ||
  fail 'registration did not recover after credential-file publication'
cmp "${work}/crash-file.original" "${crash_file_file}" ||
  fail 'credential-file crash recovery regenerated the committed intent'
test "$(db_scalar "SELECT status FROM node_identity WHERE name='node-crash-file'")" = active ||
  fail 'credential-file crash recovery did not activate the identity'

crash_digest_file="${work}/runtime/config/node-crash-digest.json"
set +e
(cd "${work}/runtime" && TP_NODE_IDENTITY_TEST_CRASH_AT=register_after_digest_commit \
  "${work}/trojan-panel-crash-test" node-identity register \
  --name node-crash-digest --domain node-crash-digest.example.com --public-ip 203.0.113.22 \
  --credential-file "${crash_digest_file}" >"${work}/crash-digest.out" 2>"${work}/crash-digest.err")
crash_status=$?
set -e
test "${crash_status}" = 86 || fail 'digest-commit crash hook did not terminate at the public CLI boundary'
test "$(db_scalar "SELECT credential_sha256 FROM node_identity WHERE name='node-crash-digest'")" = \
  "$(sha256sum "${crash_digest_file}" | awk '{print $1}')" || fail 'digest crash point was not durably committed'
assert_crashed_registration_has_no_data_identity node-crash-digest
cp "${crash_digest_file}" "${work}/crash-digest.original"
(cd "${work}/runtime" && "${work}/trojan-panel-crash-test" node-identity register \
  --name node-crash-digest --domain node-crash-digest.example.com --public-ip 203.0.113.22 \
  --credential-file "${crash_digest_file}" >"${work}/recover-digest.out" 2>"${work}/recover-digest.err") ||
  fail 'registration did not recover after the credential digest commit'
cmp "${work}/crash-digest.original" "${crash_digest_file}" ||
  fail 'digest-commit crash recovery changed credential contents'
test "$(db_scalar "SELECT status FROM node_identity WHERE name='node-crash-digest'")" = active ||
  fail 'digest-commit crash recovery did not activate the identity'

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
redis_auth_username="$(jq -r '.redis_auth.username' "${credential_file}")"
redis_auth_password="$(jq -r '.redis_auth.password' "${credential_file}")"
test "$(jq -r '.generation' "${credential_file}")" = 1 || fail 'initial credential generation is not 1'
credential_commitment="$(docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
  mariadb -N -uroot trojan_panel_db -e "SELECT credential_sha256 FROM node_identity WHERE identity_id='${node_identity_id}'")"
test "${credential_commitment}" = "$(sha256sum "${credential_file}" | awk '{print $1}')" ||
  fail 'control plane did not bind the exact credential file contents'
[[ "${node_identity_id}" =~ ^[0-9a-f-]{36}$ ]] || fail 'Node identity id is not stable UUID text'
[[ "${node_server_id}" =~ ^[1-9][0-9]*$ ]] || fail 'node_server_id is invalid'
for secret in "${db_password}" "${redis_password}" "${redis_auth_password}" "${admin_db_password}" "${admin_redis_password}"; do
  ! grep -Fq -- "${secret}" "${work}/register.out" "${work}/register.err" || fail 'register leaked a secret'
done
test ! -s "${work}/register.err" || fail 'register wrote unexpected stderr'
grep -Fq "Node identity registered: ${node_identity_id}" "${work}/register.out" ||
  fail 'register did not report the stable Node identity'

# Seal the exact generation-one data identities and public client CA into the
# product bundle format before rotating them. Later failures therefore prove
# that a previously valid encrypted bundle cannot be installed after rotation.
cp "${INSTALLER_DIR}/examples/node-agent.yaml" "${work}/node-a.yaml"
sed -i \
  -e 's/hostname: node.example.com/hostname: node-a.example.com/' \
  -e 's/mariadb_host: panel.example.com/mariadb_host: 127.0.0.1/' \
  -e "s/mariadb_port: 9507/mariadb_port: ${mariadb_port}/" \
  -e 's/redis_host: panel.example.com/redis_host: 127.0.0.1/' \
  -e "s/redis_port: 6378/redis_port: ${redis_port}/" \
  "${work}/node-a.yaml"
chmod 0600 "${work}/node-a.yaml"
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj /CN=control-plane-ca \
  -addext basicConstraints=critical,CA:TRUE \
  -keyout "${work}/discarded-control-ca.key" -out "${work}/client-ca.crt" >/dev/null 2>&1
bundle_password='node-a integration bundle password'
TP_NODE_BUNDLE_PASSWORD="${bundle_password}" "${work}/node-bundle" create \
  --credential-file "${credential_file}" --node-config "${work}/node-a.yaml" \
  --client-ca "${work}/client-ca.crt" --output "${work}/node-a.g1.age" >/dev/null ||
  fail 'generation-one Node bootstrap bundle creation failed'
if TP_NODE_BUNDLE_PASSWORD='incorrect integration password' "${work}/node-bundle" inspect \
  --bundle "${work}/node-a.g1.age" >/dev/null 2>&1; then
  fail 'generation-one Node bootstrap bundle accepted an incorrect password'
fi
mkdir -m 0700 "${work}/node-a-g1"
TP_NODE_BUNDLE_PASSWORD="${bundle_password}" "${work}/node-bundle" extract \
  --bundle "${work}/node-a.g1.age" --directory "${work}/node-a-g1" ||
  fail 'generation-one Node bootstrap bundle extraction failed'
for bundled_value in "${db_username}" "${db_password}" "${redis_username}" "${redis_password}" \
  "${redis_auth_username}" "${redis_auth_password}"; do
  grep -Fq -- "${bundled_value}" "${work}/node-a-g1/config-node.yaml" ||
    fail 'generation-one bundle did not carry its dedicated Node identity'
done
mapfile -t node_a_g1_inventory < <(cd "${work}/node-a-g1" && find . -type f -printf '%P\n' | sort)
test "${node_a_g1_inventory[*]}" = 'config-node.yaml manifest.json pki/client-ca.crt' ||
  fail 'generation-one bundle contained an unsafe inventory'

# Run the shipped Web and Node binaries in separate containers. The Node API
# stays unavailable until the Web container reaches the state RPC with its
# client certificate; the successful RPC records the exact identity generation.
mkdir -p "${work}/node-runtime/config" "${work}/node-runtime/pki" \
  "${work}/node-runtime/cert" "${work}/node-runtime/runtime" \
  "${work}/node-runtime/logs" "${work}/node-runtime/config/sqlite" \
  "${work}/node-runtime/bin/xray/config" "${work}/node-runtime/bin/naiveproxy/config" \
  "${work}/node-runtime/bin/hysteria2/config" "${work}/node-runtime/external" \
  "${work}/web-runtime/config" "${work}/web-runtime/pki"
cp "${work}/trojan-panel-core" "${work}/node-runtime/trojan-panel-core"
cp "${work}/trojan-panel" "${work}/web-runtime/trojan-panel"
for kernel_binary in bin/xray/xray bin/naiveproxy/naiveproxy bin/hysteria2/hysteria2; do
  printf '#!/bin/sh\nexit 0\n' >"${work}/node-runtime/${kernel_binary}"
  chmod 0755 "${work}/node-runtime/${kernel_binary}"
done
cp "${work}/client-ca.crt" "${work}/node-runtime/pki/client-ca.crt"
openssl req -newkey rsa:2048 -nodes -subj /CN=trojanpanelnext-control-plane \
  -keyout "${work}/web-runtime/pki/client.key" -out "${work}/client.csr" >/dev/null 2>&1
printf '%s\n' 'basicConstraints=CA:FALSE' 'keyUsage=digitalSignature,keyEncipherment' \
  'extendedKeyUsage=clientAuth' >"${work}/client.ext"
openssl x509 -req -days 1 -sha256 -in "${work}/client.csr" \
  -CA "${work}/client-ca.crt" -CAkey "${work}/discarded-control-ca.key" -CAcreateserial \
  -extfile "${work}/client.ext" -out "${work}/web-runtime/pki/client.crt" >/dev/null 2>&1
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj /CN=node-server-ca \
  -addext basicConstraints=critical,CA:TRUE \
  -keyout "${work}/server-ca.key" -out "${work}/web-runtime/pki/server-ca.crt" >/dev/null 2>&1
openssl req -newkey rsa:2048 -nodes -subj /CN=node-a.example.com \
  -keyout "${work}/node-runtime/cert/server.key" -out "${work}/server.csr" >/dev/null 2>&1
printf '%s\n' 'basicConstraints=CA:FALSE' 'keyUsage=digitalSignature,keyEncipherment' \
  'extendedKeyUsage=serverAuth' 'subjectAltName=DNS:node-a.example.com' >"${work}/server.ext"
openssl x509 -req -days 1 -sha256 -in "${work}/server.csr" \
  -CA "${work}/web-runtime/pki/server-ca.crt" -CAkey "${work}/server-ca.key" -CAcreateserial \
  -extfile "${work}/server.ext" -out "${work}/node-runtime/cert/server.crt" >/dev/null 2>&1
chmod 0600 "${work}/web-runtime/pki/client.key" "${work}/node-runtime/cert/server.key"

cat >"${work}/node-runtime/config/config.ini" <<EOF
[mysql]
host=127.0.0.1
user=${db_username}
password=${db_password}
port=${mariadb_port}
database=trojan_panel_db
account_table=account
[redis]
host=127.0.0.1
port=${redis_port}
username=${redis_username}
password=${redis_password}
auth_username=${redis_auth_username}
auth_password=${redis_auth_password}
db=0
max_idle=2
max_active=4
wait=true
[cert]
crt_path=/tpdata/trojan-panel-core/cert/server.crt
key_path=/tpdata/trojan-panel-core/cert/server.key
[log]
filename=/tpdata/trojan-panel-core/logs/core.log
max_size=1
max_backups=1
max_age=1
compress=false
[grpc]
port=8100
tls_mode=mtls
client_ca_path=/tpdata/trojan-panel-core/pki/client-ca.crt
[server]
port=18082
[node]
server_id=${node_server_id}
domain=node-a.example.com
identity_id=${node_identity_id}
identity_generation=1
EOF
chmod 0600 "${work}/node-runtime/config/config.ini"
cat >"${work}/web-runtime/config/config.ini" <<EOF
[mysql]
host=127.0.0.1
user=root
password=${admin_db_password}
port=${mariadb_port}
[redis]
host=127.0.0.1
port=${redis_port}
password=${admin_redis_password}
db=0
max_idle=2
max_active=4
wait=true
[grpc]
client_cert_path=/work/pki/client.crt
client_key_path=/work/pki/client.key
server_ca_path=/work/pki/server-ca.crt
EOF
chmod 0600 "${work}/web-runtime/config/config.ini"
docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
  mariadb -uroot trojan_panel_db -e "UPDATE node_identity SET public_ip='127.0.0.1' WHERE identity_id='${node_identity_id}'; UPDATE node_server SET ip='127.0.0.1' WHERE id=${node_server_id}" >/dev/null

debian_image='debian@sha256:f37a335e82bca302e955fa39f9dfe28f1be618f016f8a2b56318e5a5111afc26'
docker run -d --name "${node_container}" --network host --user "$(id -u):$(id -g)" \
  -e TP_KERNEL_RUNTIME=/tpdata/trojan-panel-core/runtime \
  -e TP_NODE_CREDENTIAL_RECHECK_SECONDS=1 \
  -v "${work}/node-runtime:/tpdata/trojan-panel-core" \
  -w /tpdata/trojan-panel-core "${debian_image}" ./trojan-panel-core >/dev/null
for _ in $(seq 1 40); do
  node_http_status="$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:18082/healthz || true)"
  [[ "${node_http_status}" == 503 ]] && break
  if ! docker inspect --format '{{.State.Running}}' "${node_container}" 2>/dev/null | grep -Fxq true; then
    docker logs "${node_container}" >&2 || true
    fail 'Node Agent container exited before bootstrap verification'
  fi
  sleep 0.25
done
test "${node_http_status:-}" = 503 || fail 'Node API did not wait for Web mTLS/gRPC verification'
docker exec -e TP_VERIFY_NODE_DATA_SERVICES=mariadb "${node_container}" \
  /tpdata/trojan-panel-core/trojan-panel-core >/dev/null || fail 'Node container MariaDB identity probe failed'
docker exec -e TP_VERIFY_NODE_DATA_SERVICES=redis "${node_container}" \
  /tpdata/trojan-panel-core/trojan-panel-core >/dev/null || fail 'Node container Redis identity probes failed'
docker run --rm --network host --user "$(id -u):$(id -g)" -v "${work}/web-runtime:/work" -w /work "${debian_image}" \
  ./trojan-panel node-identity verify --id "${node_identity_id}" >"${work}/verify.out" ||
  fail 'Web container could not verify Node over mTLS/gRPC'
grep -Fq 'verified over Web-to-Node mTLS/gRPC' "${work}/verify.out" ||
  fail 'Web verification did not report the mTLS/gRPC contract'
for _ in $(seq 1 20); do
  node_http_status="$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:18082/healthz || true)"
  [[ "${node_http_status}" == 200 ]] && break
  sleep 0.25
done
test "${node_http_status:-}" = 200 || fail 'Node API did not become healthy after Web mTLS/gRPC verification'
test "$(jq -r '.identity_generation' "${work}/node-runtime/runtime/bootstrap-verified.json")" = 1 ||
  fail 'Node readiness marker did not bind generation one'
test ! -e "${work}/node-runtime/pki/client.key" || fail 'Web mTLS client private key leaked to Node'
test ! -e "${work}/node-runtime/pki/client-ca.key" || fail 'Web client CA private key leaked to Node'
docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
  mariadb -uroot trojan_panel_db -e "UPDATE node_identity SET public_ip='203.0.113.10' WHERE identity_id='${node_identity_id}'; UPDATE node_server SET ip='203.0.113.10' WHERE id=${node_server_id}" >/dev/null

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
docker exec -e "REDISCLI_AUTH=${admin_redis_password}" "${redis_container}" \
  redis-cli set trojan-panel:jwt-key integration-jwt-key >/dev/null
if docker exec -e "REDISCLI_AUTH=${redis_password}" "${redis_container}" \
    redis-cli --user "${redis_username}" set trojan-panel:jwt-key attacker 2>/dev/null | grep -Fxq OK; then
  fail 'write-capable Node cache identity can overwrite the shared JWT key'
fi
if docker exec -e "REDISCLI_AUTH=${redis_password}" "${redis_container}" \
    redis-cli --user "${redis_username}" eval "return redis.call('set',KEYS[1],ARGV[1])" 1 trojan-panel:jwt-key attacker 2>/dev/null | grep -Fxq OK; then
  fail 'Node cache identity can overwrite the shared JWT key through EVAL'
fi
docker exec -e "REDISCLI_AUTH=${redis_auth_password}" "${redis_container}" \
  redis-cli --user "${redis_auth_username}" get trojan-panel:jwt-key 2>/dev/null | grep -Fxq integration-jwt-key ||
  fail 'Node Redis auth identity cannot read the shared JWT key'
if docker exec -e "REDISCLI_AUTH=${redis_auth_password}" "${redis_container}" \
    redis-cli --user "${redis_auth_username}" set trojan-panel:jwt-key attacker 2>/dev/null | grep -Fxq OK; then
  fail 'read-only Node auth identity can overwrite the shared JWT key'
fi
if docker exec -e "REDISCLI_AUTH=${redis_auth_password}" "${redis_container}" \
    redis-cli --user "${redis_auth_username}" del trojan-panel:jwt-key 2>/dev/null | grep -Fxq 1; then
  fail 'read-only Node auth identity can delete the shared JWT key'
fi
if docker exec -e "REDISCLI_AUTH=${redis_auth_password}" "${redis_container}" \
    redis-cli --user "${redis_auth_username}" set trojan-panel-core:node-config:443-1 attacker 2>/dev/null | grep -Fxq OK; then
  fail 'read-only Node auth identity can write a Node cache key'
fi
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
# A row created before the recoverable-intent migration has no nonce. Its
# committed active credential must remain replayable, and the next rotate will
# establish a nonce for the new generation.
docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
  mariadb -uroot trojan_panel_db -e "UPDATE node_identity SET credential_nonce='' WHERE identity_id='${node_identity_id}'"
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
(cd "${API_DIR}/../../node-agent" && \
  TP_REDIS_ACL_INTEGRATION_ADDRESS="127.0.0.1:${redis_port}" \
  TP_REDIS_ACL_INTEGRATION_USERNAME="${redis_username}" \
  TP_REDIS_ACL_INTEGRATION_PASSWORD="${redis_password}" \
  TP_REDIS_AUTH_INTEGRATION_USERNAME="${redis_auth_username}" \
  TP_REDIS_AUTH_INTEGRATION_PASSWORD="${redis_auth_password}" \
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
node_b_redis_auth_username="$(jq -r '.redis_auth.username' "${node_b_credential_file}")"
node_b_redis_auth_password="$(jq -r '.redis_auth.password' "${node_b_credential_file}")"
test "${node_b_db_username}" != "${db_username}" || fail 'two Nodes share a MariaDB identity'
test "${node_b_redis_username}" != "${redis_username}" || fail 'two Nodes share a Redis ACL identity'
test "${node_b_redis_auth_username}" != "${redis_auth_username}" || fail 'two Nodes share a Redis auth identity'

# A pre-positioned next-generation file is untrusted even when all public
# identity metadata is correct: only content committed by the control plane is accepted.
rotated_credential_file="${work}/runtime/config/node-a-credentials-generation-2.json"
jq '.generation = 2 | .mariadb.password = "attacker-db" | .redis.password = "attacker-cache" | .redis_auth.password = "attacker-auth"' \
  "${credential_file}" >"${rotated_credential_file}"
chmod 0600 "${rotated_credential_file}"
if (cd "${work}/runtime" && "${work}/trojan-panel" node-identity rotate \
  --id "${node_identity_id}" --credential-file "${rotated_credential_file}" \
  >"${work}/rotate-preplaced.out" 2>"${work}/rotate-preplaced.err"); then
  fail 'rotation accepted a pre-positioned uncommitted credential file'
fi
rm "${rotated_credential_file}"

(cd "${work}/runtime" && "${work}/trojan-panel" node-identity rotate \
  --id "${node_identity_id}" \
  --credential-file "${rotated_credential_file}" \
  >"${work}/rotate.out" 2>"${work}/rotate.err") || fail 'rotate returned non-zero'
test "$(stat -c '%a' "${rotated_credential_file}")" = 600 || fail 'rotated credential file mode is not 0600'
test "$(jq -r '.generation' "${rotated_credential_file}")" = 2 || fail 'rotated credential generation is not 2'
test "$(jq -r '.node_identity_id' "${rotated_credential_file}")" = "${node_identity_id}" || fail 'rotation changed Node identity'
test "$(jq -r '.mariadb.username' "${rotated_credential_file}")" = "${db_username}" || fail 'rotation changed MariaDB identity'
test "$(jq -r '.redis.username' "${rotated_credential_file}")" = "${redis_username}" || fail 'rotation changed Redis ACL identity'
test "$(jq -r '.redis_auth.username' "${rotated_credential_file}")" = "${redis_auth_username}" || fail 'rotation changed Redis auth identity'
test -n "$(db_scalar "SELECT credential_nonce FROM node_identity WHERE identity_id='${node_identity_id}'")" ||
  fail 'rotation did not migrate the identity to a recoverable credential intent'
rotated_db_password="$(jq -r '.mariadb.password' "${rotated_credential_file}")"
rotated_redis_password="$(jq -r '.redis.password' "${rotated_credential_file}")"
rotated_redis_auth_password="$(jq -r '.redis_auth.password' "${rotated_credential_file}")"
test "${rotated_db_password}" != "${db_password}" || fail 'rotation reused the MariaDB password'
test "${rotated_redis_password}" != "${redis_password}" || fail 'rotation reused the Redis password'
test "${rotated_redis_auth_password}" != "${redis_auth_password}" || fail 'rotation reused the Redis auth password'
for secret in "${rotated_db_password}" "${rotated_redis_password}" "${rotated_redis_auth_password}" "${db_password}" "${redis_password}" "${redis_auth_password}"; do
  ! grep -Fq -- "${secret}" "${work}/rotate.out" "${work}/rotate.err" || fail 'rotate leaked a secret'
done

for _ in $(seq 1 30); do
  running="$(docker inspect --format '{{.State.Running}}' "${node_container}" 2>/dev/null || true)"
  [[ "${running}" == false ]] && break
  sleep 0.2
done
test "${running:-}" = false || fail 'running Node Agent did not stop after its bundle credentials were rotated'
docker rm -fv "${node_container}" >/dev/null

# A fresh Node process using the now-old decrypted generation-one bundle must
# fail its mandatory startup data-service probes and never expose the API.
docker run -d --name "${node_container}" --network host --user "$(id -u):$(id -g)" \
  -e TP_KERNEL_RUNTIME=/tpdata/trojan-panel-core/runtime \
  -v "${work}/node-runtime:/tpdata/trojan-panel-core" \
  -w /tpdata/trojan-panel-core "${debian_image}" ./trojan-panel-core >/dev/null
for _ in $(seq 1 30); do
  running="$(docker inspect --format '{{.State.Running}}' "${node_container}" 2>/dev/null || true)"
  [[ "${running}" == false ]] && break
  sleep 0.2
done
test "${running:-}" = false || fail 'old encrypted bundle started a Node Agent after credential rotation'
docker rm -fv "${node_container}" >/dev/null
if docker exec -e "MYSQL_PWD=${db_password}" "${mariadb_container}" \
    mariadb -h127.0.0.1 "-u${db_username}" trojan_panel_db -e 'SELECT 1' >/dev/null 2>&1; then
  fail 'old encrypted bundle MariaDB credential remained valid after rotation'
fi
if docker exec -e "REDISCLI_AUTH=${redis_password}" "${redis_container}" \
    redis-cli --user "${redis_username}" ping 2>/dev/null | grep -Fxq PONG; then
  fail 'old encrypted bundle Redis credential remained valid after rotation'
fi
if docker exec -e "REDISCLI_AUTH=${redis_auth_password}" "${redis_container}" \
    redis-cli --user "${redis_auth_username}" ping 2>/dev/null | grep -Fxq PONG; then
  fail 'old encrypted bundle Redis auth credential remained valid after rotation'
fi
docker exec -e "MYSQL_PWD=${rotated_db_password}" "${mariadb_container}" \
  mariadb -h127.0.0.1 "-u${db_username}" trojan_panel_db -e 'SELECT username FROM account' >/dev/null ||
  fail 'rotated MariaDB credential is invalid'
docker exec -e "REDISCLI_AUTH=${rotated_redis_password}" "${redis_container}" \
  redis-cli --user "${redis_username}" ping 2>/dev/null | grep -Fxq PONG ||
  fail 'rotated Redis credential is invalid'
docker exec -e "REDISCLI_AUTH=${rotated_redis_auth_password}" "${redis_container}" \
  redis-cli --user "${redis_auth_username}" get trojan-panel:jwt-key 2>/dev/null | grep -Fxq integration-jwt-key ||
  fail 'rotated Redis auth credential is invalid'
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

preplaced_credential_file="${work}/runtime/config/node-preplaced-credentials.json"
printf '{"mariadb":{"password":"attacker-selected"}}\n' >"${preplaced_credential_file}"
chmod 0600 "${preplaced_credential_file}"
if (cd "${work}/runtime" && "${work}/trojan-panel" node-identity register \
  --name node-preplaced --domain node-preplaced.example.com --public-ip 203.0.113.15 \
  --credential-file "${preplaced_credential_file}" \
  >"${work}/register-preplaced.out" 2>"${work}/register-preplaced.err"); then
  fail 'register accepted a pre-positioned credential file'
fi
test "$(docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
  mariadb -N -uroot trojan_panel_db -e "SELECT COUNT(1) FROM node_identity WHERE name='node-preplaced'")" = 0 ||
  fail 'rejected pre-positioned credential file left an identity reservation'
test "$(docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
  mariadb -N -uroot trojan_panel_db -e "SELECT COUNT(1) FROM node_server WHERE name='node-preplaced'")" = 0 ||
  fail 'rejected pre-positioned credential file left a node_server registration'

dotdot_credential_file="${work}/runtime/config/../node-dotdot-credentials.json"
if (cd "${work}/runtime" && "${work}/trojan-panel" node-identity register \
  --name node-dotdot --domain node-dotdot.example.com --public-ip 203.0.113.17 \
  --credential-file "${dotdot_credential_file}" \
  >"${work}/register-dotdot.out" 2>"${work}/register-dotdot.err"); then
  fail 'register accepted a credential path containing a .. component'
fi
test ! -e "${work}/runtime/node-dotdot-credentials.json" || fail 'register wrote through a .. credential path'

(cd "${work}/runtime" && "${work}/trojan-panel" node-identity status --id "${node_identity_id}" \
  >"${work}/status-active.out" 2>"${work}/status-active.err") || fail 'status returned non-zero for active identity'
test "$(jq -r '.status' "${work}/status-active.out")" = active || fail 'status did not report active identity'
test "$(jq -r '.generation' "${work}/status-active.out")" = 2 || fail 'status did not report rotated generation'
for secret in "${rotated_db_password}" "${rotated_redis_password}"; do
  ! grep -Fq -- "${secret}" "${work}/status-active.out" "${work}/status-active.err" || fail 'status leaked a secret'
done

TP_NODE_BUNDLE_PASSWORD="${bundle_password}" "${work}/node-bundle" create \
  --credential-file "${rotated_credential_file}" --node-config "${work}/node-a.yaml" \
  --client-ca "${work}/client-ca.crt" --output "${work}/node-a.g2.age" >/dev/null ||
  fail 'generation-two Node bootstrap bundle creation failed'
TP_NODE_BUNDLE_PASSWORD="${bundle_password}" "${work}/node-bundle" inspect \
  --bundle "${work}/node-a.g2.age" | grep -q '"generation": 2' ||
  fail 'generation-two bundle did not bind the rotated identity generation'

(cd "${work}/runtime" && "${work}/trojan-panel" node-identity revoke --id "${node_identity_id}" \
  >"${work}/revoke.out" 2>"${work}/revoke.err") || fail 'revoke returned non-zero'
if docker exec -e "MYSQL_PWD=${rotated_db_password}" "${mariadb_container}" \
    mariadb -h127.0.0.1 "-u${db_username}" trojan_panel_db -e 'SELECT 1' >/dev/null 2>&1; then
  fail 'revoked encrypted bundle MariaDB credential remained valid'
fi
if docker exec -e "REDISCLI_AUTH=${rotated_redis_password}" "${redis_container}" \
    redis-cli --user "${redis_username}" ping 2>/dev/null | grep -Fxq PONG; then
  fail 'revoked encrypted bundle Redis credential remained valid'
fi
if docker exec -e "REDISCLI_AUTH=${rotated_redis_auth_password}" "${redis_container}" \
    redis-cli --user "${redis_auth_username}" ping 2>/dev/null | grep -Fxq PONG; then
  fail 'revoked encrypted bundle Redis auth credential remained valid'
fi
docker exec -e "MYSQL_PWD=${node_b_db_password}" "${mariadb_container}" \
  mariadb -h127.0.0.1 "-u${node_b_db_username}" trojan_panel_db -e 'SELECT 1' >/dev/null ||
  fail 'revoking Node A invalidated Node B MariaDB credentials'
docker exec -e "REDISCLI_AUTH=${node_b_redis_password}" "${redis_container}" \
  redis-cli --user "${node_b_redis_username}" ping 2>/dev/null | grep -Fxq PONG ||
  fail 'revoking Node A invalidated Node B Redis credentials'
docker exec -e "REDISCLI_AUTH=${node_b_redis_auth_password}" "${redis_container}" \
  redis-cli --user "${node_b_redis_auth_username}" ping 2>/dev/null | grep -Fxq PONG ||
  fail 'revoking Node A invalidated Node B Redis auth credentials'
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity revoke --id "${node_identity_id}" \
  >"${work}/revoke-replay.out" 2>"${work}/revoke-replay.err") || fail 'repeated revoke was not idempotent'
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity status --id "${node_identity_id}" \
  >"${work}/status-revoked.out" 2>"${work}/status-revoked.err") || fail 'status returned non-zero for revoked identity'
test "$(jq -r '.status' "${work}/status-revoked.out")" = revoked || fail 'status did not report revoked identity'
test "$(docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
  mariadb -N -uroot trojan_panel_db -e "SELECT COUNT(1) FROM node_server WHERE id=${node_server_id}")" = 1 ||
  fail 'revoke removed the control-plane audit tombstone'
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity force-evict --id "${node_identity_id}" \
  >"${work}/evict-after-revoke.out" 2>"${work}/evict-after-revoke.err") || fail 'revoke to force-evict did not converge'
test "$(docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
  mariadb -N -uroot trojan_panel_db -e "SELECT COUNT(1) FROM node_server WHERE id=${node_server_id}")" = 0 ||
  fail 'force-evict retained the control-plane registration after revoke'

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
node_c_redis_username="$(jq -r '.redis.username' "${node_c_credential_file}")"
node_c_old_redis_password="$(jq -r '.redis.password' "${node_c_credential_file}")"
node_c_redis_auth_username="$(jq -r '.redis_auth.username' "${node_c_credential_file}")"
node_c_old_redis_auth_password="$(jq -r '.redis_auth.password' "${node_c_credential_file}")"
# Force a cross-service partial failure after the atomic Redis rotation by
# removing a table required by the subsequent MariaDB grant sequence.
docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
  mariadb -uroot trojan_panel_db -e 'DROP TABLE account'
node_c_rotated_file="${work}/runtime/config/node-c-credentials-generation-2.json"
if (cd "${work}/runtime" && "${work}/trojan-panel" node-identity rotate \
  --id "${node_c_identity_id}" --credential-file "${node_c_rotated_file}" \
  >"${work}/rotate-c-failed.out" 2>"${work}/rotate-c-failed.err"); then
  fail 'rotation succeeded after the MariaDB boundary failed following Redis rotation'
fi
test -f "${node_c_rotated_file}" || fail 'failed rotation did not retain retry credentials'
test "$(stat -c '%a' "${node_c_rotated_file}")" = 600 || fail 'retry credential file mode is not 0600'
node_c_rotated_db_password="$(jq -r '.mariadb.password' "${node_c_rotated_file}")"
node_c_rotated_redis_password="$(jq -r '.redis.password' "${node_c_rotated_file}")"
node_c_rotated_redis_auth_password="$(jq -r '.redis_auth.password' "${node_c_rotated_file}")"
for secret in "${node_c_rotated_db_password}" "${node_c_rotated_redis_password}" "${node_c_rotated_redis_auth_password}"; do
  ! grep -Fq -- "${secret}" "${work}/rotate-c-failed.out" "${work}/rotate-c-failed.err" ||
    fail 'failed rotation leaked a retry credential'
done
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity status --id "${node_c_identity_id}" \
  >"${work}/status-c-rotating.out" 2>"${work}/status-c-rotating.err") || fail 'status failed for interrupted rotation'
test "$(jq -r '.status' "${work}/status-c-rotating.out")" = rotating || fail 'interrupted rotation was not recorded as rotating'
if docker exec -e "REDISCLI_AUTH=${node_c_old_redis_password}" "${redis_container}" \
    redis-cli --user "${node_c_redis_username}" ping 2>/dev/null | grep -Fxq PONG; then
  fail 'old Redis cache password remained valid after partial rotation'
fi
if docker exec -e "REDISCLI_AUTH=${node_c_old_redis_auth_password}" "${redis_container}" \
    redis-cli --user "${node_c_redis_auth_username}" ping 2>/dev/null | grep -Fxq PONG; then
  fail 'old Redis auth password remained valid after partial rotation'
fi
docker exec -e "REDISCLI_AUTH=${node_c_rotated_redis_password}" "${redis_container}" \
  redis-cli --user "${node_c_redis_username}" ping 2>/dev/null | grep -Fxq PONG ||
  fail 'new Redis cache credential was not retained after partial rotation'
node_c_committed_backup="${work}/runtime/config/node-c-generation-2-committed.json"
cp "${node_c_rotated_file}" "${node_c_committed_backup}"
jq '.redis.password = "changed-after-commit"' "${node_c_rotated_file}" >"${node_c_rotated_file}.changed"
chmod 0600 "${node_c_rotated_file}.changed"
mv "${node_c_rotated_file}.changed" "${node_c_rotated_file}"
if (cd "${work}/runtime" && "${work}/trojan-panel" node-identity rotate \
  --id "${node_c_identity_id}" --credential-file "${node_c_rotated_file}" \
  >"${work}/rotate-c-tampered.out" 2>"${work}/rotate-c-tampered.err"); then
  fail 'rotation retry accepted credential contents that differed from the committed generation'
fi
mv "${node_c_committed_backup}" "${node_c_rotated_file}"
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
SQL
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
docker exec -e "MYSQL_PWD=${node_c_rotated_db_password}" "${mariadb_container}" \
  mariadb -h127.0.0.1 "-u${node_c_db_username}" trojan_panel_db -e 'SELECT 1' >/dev/null ||
  fail 'recovered MariaDB credential is invalid'
docker exec -e "REDISCLI_AUTH=${node_c_rotated_redis_password}" "${redis_container}" \
  redis-cli --user "${node_c_redis_username}" ping 2>/dev/null | grep -Fxq PONG ||
  fail 'recovered Redis credential is invalid'

# Queue rotate and revoke behind the same real MariaDB lifecycle lock. Whichever
# operation wins is serialized; revoke must be the terminal state and no ACL
# identity may be recreated after it completes.
node_f_credential_file="${work}/runtime/config/node-f-credentials.json"
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity register \
  --name node-f --domain node-f.example.com --public-ip 203.0.113.16 \
  --credential-file "${node_f_credential_file}" \
  >"${work}/register-f.out" 2>"${work}/register-f.err") || fail 'Node F registration returned non-zero'
node_f_identity_id="$(jq -r '.node_identity_id' "${node_f_credential_file}")"
node_f_redis_username="$(jq -r '.redis.username' "${node_f_credential_file}")"
node_f_redis_auth_username="$(jq -r '.redis_auth.username' "${node_f_credential_file}")"
docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
  mariadb -uroot trojan_panel_db -e "SELECT GET_LOCK('tpn-node-identity:${node_f_identity_id}',0); DO SLEEP(4); SELECT RELEASE_LOCK('tpn-node-identity:${node_f_identity_id}')" \
  >"${work}/lock-f.out" 2>"${work}/lock-f.err" &
lock_holder_pid=$!
for _ in $(seq 1 30); do
  lock_owner="$(docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
    mariadb -N -uroot trojan_panel_db -e "SELECT COALESCE(IS_USED_LOCK('tpn-node-identity:${node_f_identity_id}'),0)")"
  [[ "${lock_owner}" != 0 ]] && break
  sleep 0.1
done
[[ "${lock_owner:-0}" != 0 ]] || fail 'test could not acquire the lifecycle serialization lock'
node_f_rotated_file="${work}/runtime/config/node-f-credentials-generation-2.json"
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity rotate \
  --id "${node_f_identity_id}" --credential-file "${node_f_rotated_file}" \
  >"${work}/rotate-f.out" 2>"${work}/rotate-f.err") &
rotate_f_pid=$!
sleep 0.3
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity revoke --id "${node_f_identity_id}" \
  >"${work}/revoke-f.out" 2>"${work}/revoke-f.err") &
revoke_f_pid=$!
wait "${lock_holder_pid}" || fail 'lifecycle lock holder failed'
wait "${rotate_f_pid}" || true
wait "${revoke_f_pid}" || fail 'concurrent revoke did not converge'
(cd "${work}/runtime" && "${work}/trojan-panel" node-identity status --id "${node_f_identity_id}" \
  >"${work}/status-f.out" 2>"${work}/status-f.err") || fail 'status failed after concurrent lifecycle operations'
test "$(jq -r '.status' "${work}/status-f.out")" = revoked || fail 'concurrent lifecycle operations recreated a revoked identity'
test -z "$(docker exec -e "REDISCLI_AUTH=${admin_redis_password}" "${redis_container}" redis-cli --raw ACL GETUSER "${node_f_redis_username}")" ||
  fail 'concurrent lifecycle operations recreated the cache ACL identity'
test -z "$(docker exec -e "REDISCLI_AUTH=${admin_redis_password}" "${redis_container}" redis-cli --raw ACL GETUSER "${node_f_redis_auth_username}")" ||
  fail 'concurrent lifecycle operations recreated the auth ACL identity'

printf 'PASS node identity CLI lifecycle, isolation, failure recovery, and Node Agent ACL contract\n'
