#!/usr/bin/env bash
set -Eeuo pipefail

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

if ! docker info >/dev/null 2>&1; then
  docker() { sudo -n /usr/bin/docker "$@"; }
fi

api_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
work="$(mktemp -d)"
suffix="${RANDOM}-$$"
mariadb_container="tp-receipt-mariadb-${suffix}"
redis_container="tp-receipt-redis-${suffix}"
mariadb_image='mariadb@sha256:07e06f2e7ae9dfc63707a83130a62e00167c827f08fcac7a9aa33f4b6dc34e0e'
redis_image='redis@sha256:a93c14584715ec5bd9d2648d58c3b27f89416242bee0bc9e5fb2edc1a4cbec1d'
gate_pid=""
uncertain_pid=""
cleanup() {
  if [[ -n "${uncertain_pid}" ]]; then
    kill "${uncertain_pid}" >/dev/null 2>&1 || true
    wait "${uncertain_pid}" >/dev/null 2>&1 || true
  fi
  if [[ -n "${gate_pid}" ]]; then
    kill "${gate_pid}" >/dev/null 2>&1 || true
    wait "${gate_pid}" >/dev/null 2>&1 || true
  fi
  docker rm -fv "${mariadb_container}" "${redis_container}" >/dev/null 2>&1 || true
  rm -rf -- "${work}"
}
trap cleanup EXIT

(cd "${api_dir}" && CGO_ENABLED=0 go build -trimpath -o "${work}/trojan-panel" .)
(cd "${api_dir}" && CGO_ENABLED=0 go build -trimpath -o "${work}/verify-receipt" ./testing/node_revocation_receipt_verify.go)
(cd "${api_dir}" && CGO_ENABLED=0 go build -trimpath -o "${work}/redis-exec-gate" ./testing/redis_exec_gate.go)
(cd "${api_dir}/../../../deploy/installer/nodebundle" && CGO_ENABLED=0 go build -trimpath -o "${work}/node-bundle" .)

admin_db_password="$(openssl rand -hex 24)"
admin_redis_password="$(openssl rand -hex 24)"
docker run -d --name "${mariadb_container}" \
  -e "MARIADB_ROOT_PASSWORD=${admin_db_password}" -e MARIADB_DATABASE=trojan_panel_db \
  -p 127.0.0.1::3306 "${mariadb_image}" >/dev/null
docker run -d --name "${redis_container}" -p 127.0.0.1::6379 "${redis_image}" \
  redis-server --requirepass "${admin_redis_password}" >/dev/null

db() {
  docker exec -e "MYSQL_PWD=${admin_db_password}" "${mariadb_container}" \
    mariadb -N -uroot trojan_panel_db -e "$1"
}
redis_admin() {
  docker exec -e "REDISCLI_AUTH=${admin_redis_password}" "${redis_container}" redis-cli "$@"
}
for _ in $(seq 1 90); do
  if db 'SELECT 1' >/dev/null 2>&1; then break; fi
  sleep 1
done
db 'SELECT 1' >/dev/null 2>&1 || fail 'MariaDB did not become ready'
for _ in $(seq 1 30); do
  if redis_admin ping 2>/dev/null | grep -Fxq PONG; then break; fi
  sleep 1
done
redis_admin ping 2>/dev/null | grep -Fxq PONG || fail 'Redis did not become ready'

db 'CREATE TABLE account (id bigint unsigned NOT NULL AUTO_INCREMENT PRIMARY KEY, username varchar(64) NOT NULL, download bigint unsigned NOT NULL DEFAULT 0, upload bigint unsigned NOT NULL DEFAULT 0); INSERT INTO account (username) VALUES ("integration-user");'
db 'CREATE TABLE node_server (id bigint unsigned NOT NULL AUTO_INCREMENT PRIMARY KEY, ip varchar(64) NOT NULL DEFAULT "", name varchar(64) NOT NULL DEFAULT "", grpc_port int unsigned NOT NULL DEFAULT 8100, grpc_tls_mode varchar(16) NOT NULL DEFAULT "mtls", grpc_tls_server_name varchar(253) NOT NULL DEFAULT "", traffic_period varchar(8) NOT NULL DEFAULT "none", traffic_limit_mode varchar(8) NOT NULL DEFAULT "combined", traffic_total_limit bigint unsigned NOT NULL DEFAULT 0, traffic_upload_limit bigint unsigned NOT NULL DEFAULT 0, traffic_download_limit bigint unsigned NOT NULL DEFAULT 0, create_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP, update_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP);'
db 'CREATE TABLE node (id bigint unsigned NOT NULL AUTO_INCREMENT PRIMARY KEY, node_server_id bigint unsigned NOT NULL);'
db 'CREATE TABLE account_traffic_total (account_id bigint unsigned NOT NULL PRIMARY KEY, upload bigint unsigned NOT NULL DEFAULT 0, download bigint unsigned NOT NULL DEFAULT 0);'
db 'CREATE TABLE account_traffic_daily (traffic_date date NOT NULL, account_id bigint unsigned NOT NULL, upload bigint unsigned NOT NULL DEFAULT 0, download bigint unsigned NOT NULL DEFAULT 0, PRIMARY KEY (traffic_date, account_id));'
db 'CREATE TABLE account_server_traffic_daily (traffic_date date NOT NULL, account_id bigint unsigned NOT NULL, node_server_id bigint unsigned NOT NULL, upload bigint unsigned NOT NULL DEFAULT 0, download bigint unsigned NOT NULL DEFAULT 0, PRIMARY KEY (traffic_date, account_id, node_server_id));'

mariadb_port="$(docker port "${mariadb_container}" 3306/tcp | sed -n 's/.*://p' | head -n1)"
redis_port="$(docker port "${redis_container}" 6379/tcp | sed -n 's/.*://p' | head -n1)"
mkdir -m 0700 "${work}/runtime" "${work}/runtime/config" "${work}/receipts" "${work}/unsafe"
chmod 0755 "${work}/unsafe"
printf '%s\n' '[mysql]' 'host=127.0.0.1' 'user=root' "password=${admin_db_password}" \
  "port=${mariadb_port}" '[redis]' 'host=127.0.0.1' "port=${redis_port}" \
  "password=${admin_redis_password}" 'db=0' 'max_idle=2' 'max_active=4' 'wait=true' \
  >"${work}/runtime/config/config.ini"
chmod 0600 "${work}/runtime/config/config.ini"

run_cli() {
  local label="$1"
  shift
  (cd "${work}/runtime" && "${work}/trojan-panel" node-identity "$@" \
    >"${work}/${label}.out" 2>"${work}/${label}.err")
}
reject_cli() {
  local label="$1"
  shift
  if run_cli "${label}" "$@"; then fail "${label} unexpectedly succeeded"; fi
}
register_node() {
  local name="$1"
  run_cli "register-${name}" register --name "${name}" --domain "${name}.example.com" \
    --public-ip 203.0.113.10 --credential-file "${work}/runtime/config/${name}.json" ||
    { sed -n '1,12p' "${work}/register-${name}.err" >&2; fail "${name} registration failed"; }
  test -s "${work}/runtime/config/${name}.json" || fail "${name} credential file missing"
}
verify_receipt() {
  local name="$1" path="$2" status="$3" generation="${4:-1}"
  "${work}/verify-receipt" "${work}/runtime/config/revocation/public-key.txt" "${path}" \
    "$(jq -r .node_identity_id "${work}/runtime/config/${name}.json")" \
    "$(jq -r .node_server_id "${work}/runtime/config/${name}.json")" \
    "${generation}" "${status}" || fail "${name} receipt did not verify as ${status}"
}
assert_live() {
  local name="$1" credential="${work}/runtime/config/${1}.json" db_user db_pass redis_user redis_pass auth_user auth_pass
  db_user="$(jq -r .mariadb.username "${credential}")"
  db_pass="$(jq -r .mariadb.password "${credential}")"
  redis_user="$(jq -r .redis.username "${credential}")"
  redis_pass="$(jq -r .redis.password "${credential}")"
  auth_user="$(jq -r .redis_auth.username "${credential}")"
  auth_pass="$(jq -r .redis_auth.password "${credential}")"
  docker exec -e "MYSQL_PWD=${db_pass}" "${mariadb_container}" \
    mariadb -h127.0.0.1 "-u${db_user}" trojan_panel_db -e 'SELECT username FROM account' >/dev/null 2>&1 ||
    fail "${name} MariaDB identity is not live"
  docker exec -e "REDISCLI_AUTH=${redis_pass}" "${redis_container}" \
    redis-cli --user "${redis_user}" ping 2>/dev/null | grep -Fxq PONG ||
    fail "${name} cache Redis identity is not live"
  docker exec -e "REDISCLI_AUTH=${auth_pass}" "${redis_container}" \
    redis-cli --user "${auth_user}" ping 2>/dev/null | grep -Fxq PONG ||
    fail "${name} auth Redis identity is not live"
}
assert_revoked() {
  local name="$1" credential="${work}/runtime/config/${1}.json" db_user db_pass redis_user redis_pass auth_user auth_pass
  db_user="$(jq -r .mariadb.username "${credential}")"
  db_pass="$(jq -r .mariadb.password "${credential}")"
  redis_user="$(jq -r .redis.username "${credential}")"
  redis_pass="$(jq -r .redis.password "${credential}")"
  auth_user="$(jq -r .redis_auth.username "${credential}")"
  auth_pass="$(jq -r .redis_auth.password "${credential}")"
  if docker exec -e "MYSQL_PWD=${db_pass}" "${mariadb_container}" \
    mariadb -h127.0.0.1 "-u${db_user}" trojan_panel_db -e 'SELECT 1' >/dev/null 2>&1; then
    fail "${name} MariaDB identity survived revocation"
  fi
  if docker exec -e "REDISCLI_AUTH=${redis_pass}" "${redis_container}" \
    redis-cli --user "${redis_user}" ping 2>/dev/null | grep -Fxq PONG; then
    fail "${name} cache Redis identity survived revocation"
  fi
  if docker exec -e "REDISCLI_AUTH=${auth_pass}" "${redis_container}" \
    redis-cli --user "${auth_user}" ping 2>/dev/null | grep -Fxq PONG; then
    fail "${name} auth Redis identity survived revocation"
  fi
  test "$(db "SELECT COUNT(*) FROM mysql.user WHERE User='${db_user}'")" = 0 ||
    fail "${name} MariaDB ACL still contains the user"
  test -z "$(redis_admin --raw ACL GETUSER "${redis_user}")" ||
    fail "${name} cache Redis ACL still contains the user"
  test -z "$(redis_admin --raw ACL GETUSER "${auth_user}")" ||
    fail "${name} auth Redis ACL still contains the user"
}
assert_state() {
  local name="$1" expected="$2" server_count="$3" id server
  id="$(jq -r .node_identity_id "${work}/runtime/config/${name}.json")"
  server="$(jq -r .node_server_id "${work}/runtime/config/${name}.json")"
  test "$(db "SELECT status FROM node_identity WHERE identity_id='${id}'")" = "${expected}" ||
    fail "${name} terminal status differs from ${expected}"
  test "$(db "SELECT COUNT(*) FROM node_server WHERE id=${server}")" = "${server_count}" ||
    fail "${name} control registration count differs from ${server_count}"
}
assert_audit() {
  local name="$1" action="$2" minimum="$3" id
  id="$(jq -r .node_identity_id "${work}/runtime/config/${name}.json")"
  test "$(db "SELECT COUNT(*) FROM node_identity_event WHERE identity_id='${id}' AND action='${action}' AND result='succeeded'")" -ge "${minimum}" ||
    fail "${name} successful ${action} audit missing"
}
assert_control_targets() {
  local expected="" name server
  for name in "$@"; do
    server="$(jq -r .node_server_id "${work}/runtime/config/${name}.json")"
    expected+="${expected:+,}${server}"
  done
  (cd "${api_dir}" && TP_NODE_CONTROL_TEST_DSN="root:${admin_db_password}@tcp(127.0.0.1:${mariadb_port})/trojan_panel_db?parseTime=true" \
    TP_NODE_CONTROL_TEST_EXPECT="${expected}" \
    go test ./dao -run '^TestNodeControlTargetsRespectIdentityStatus$' -count=1) || {
    db 'SELECT ns.id,ns.name,COALESCE(ni.status,"unmanaged") FROM node_server ns LEFT JOIN node_identity ni ON ni.node_server_id=ns.id ORDER BY ns.id' >&2
    fail "control target query included a revoked identity or excluded a live Node"
  }
}

run_cli key-init revocation-key-init || fail 'revocation-key-init failed'
public_key="${work}/runtime/config/revocation/public-key.txt"
private_key="${work}/runtime/config/revocation/signing-key.pem"
test "$(stat -c %a "${private_key}")" = 600 || fail 'signing key mode is unsafe'
test "$(stat -c %a "${public_key}")" = 600 || fail 'public key mode is unsafe'
test "$(stat -c %a "${work}/runtime/config/revocation")" = 700 || fail 'signing key directory mode is unsafe'
cp "${public_key}" "${work}/public-original"
run_cli key-init-repeat revocation-key-init || fail 'repeat key initialization failed'
cmp -s "${public_key}" "${work}/public-original" || fail 'repeat key initialization changed the trust root'

register_node node-a
register_node node-b
register_node node-c
register_node node-d
for name in node-a node-b node-c node-d; do
  db "INSERT INTO node (node_server_id) VALUES ($(jq -r .node_server_id "${work}/runtime/config/${name}.json"))" >/dev/null
done
assert_control_targets node-a node-b node-c node-d
# A registered but not yet active identity must also be excluded from both
# list and by-ID control; this status transition is only a DAO fixture.
node_d_id="$(jq -r .node_identity_id "${work}/runtime/config/node-d.json")"
db "UPDATE node_identity SET status='provisioning' WHERE identity_id='${node_d_id}'" >/dev/null
assert_control_targets node-a node-b node-c
db "UPDATE node_identity SET status='active' WHERE identity_id='${node_d_id}'" >/dev/null
assert_control_targets node-a node-b node-c node-d
assert_live node-a
assert_live node-b
assert_live node-c
assert_live node-d

node_a_id="$(jq -r .node_identity_id "${work}/runtime/config/node-a.json")"
node_a_receipt="${work}/receipts/node-a.json"
reject_cli unsafe-parent revoke --id "${node_a_id}" --receipt-file "${work}/unsafe/receipt.json"
test ! -e "${work}/unsafe/receipt.json" || fail 'unsafe parent produced a receipt'
assert_live node-a
printf 'existing receipt sentinel\n' >"${node_a_receipt}"
reject_cli existing-file revoke --id "${node_a_id}" --receipt-file "${node_a_receipt}"
grep -Fxq 'existing receipt sentinel' "${node_a_receipt}" || fail 'existing output was overwritten'
assert_live node-a
ln -s "${node_a_receipt}" "${work}/receipts/link.json"
reject_cli symlink-file revoke --id "${node_a_id}" --receipt-file "${work}/receipts/link.json"
assert_live node-a
rm -- "${node_a_receipt}"

run_cli revoke-a revoke --id "${node_a_id}" --receipt-file "${node_a_receipt}" || fail 'Node A revoke failed'
test "$(stat -c %a "${node_a_receipt}")" = 600 || fail 'Node A receipt mode is unsafe'
assert_revoked node-a
assert_state node-a revoked 1
assert_control_targets node-b node-c node-d
assert_audit node-a revoke 1
verify_receipt node-a "${node_a_receipt}" revoked
"${work}/node-bundle" verify-receipt --receipt-file "${node_a_receipt}" \
  --pinned-public-key "${public_key}" --identity-id "${node_a_id}" \
  --server-id "$(jq -r .node_server_id "${work}/runtime/config/node-a.json")" --generation 1 >/dev/null ||
  fail 'installed Node verifier rejected Web-issued terminal receipt'
if "${work}/node-bundle" verify-receipt --receipt-file "${node_a_receipt}" \
  --pinned-public-key "${public_key}" --identity-id "${node_a_id}" \
  --server-id "$(jq -r .node_server_id "${work}/runtime/config/node-a.json")" --generation 2 >/dev/null 2>&1; then
  fail 'installed Node verifier accepted a newer generation'
fi
if "${work}/verify-receipt" "${public_key}" "${node_a_receipt}" \
  "${node_a_id}" "$(jq -r .node_server_id "${work}/runtime/config/node-a.json")" \
  2 revoked >/dev/null 2>&1; then
  fail 'Node A receipt authorized a newer generation'
fi
if "${work}/verify-receipt" "${public_key}" "${node_a_receipt}" \
  "$(jq -r .node_identity_id "${work}/runtime/config/node-b.json")" \
  "$(jq -r .node_server_id "${work}/runtime/config/node-b.json")" 1 revoked >/dev/null 2>&1; then
  fail 'Node A receipt verified for Node B'
fi
assert_live node-b
assert_live node-c
assert_live node-d
cp "${node_a_receipt}" "${work}/node-a-original"
reject_cli overwrite-a revoke --id "${node_a_id}" --receipt-file "${node_a_receipt}"
cmp -s "${node_a_receipt}" "${work}/node-a-original" || fail 'receipt changed after overwrite rejection'
run_cli revoke-a-retry revoke --id "${node_a_id}" --receipt-file "${work}/receipts/node-a-retry.json" ||
  fail 'Node A terminal retry failed'
verify_receipt node-a "${work}/receipts/node-a-retry.json" revoked
assert_audit node-a revoke 2

node_b_id="$(jq -r .node_identity_id "${work}/runtime/config/node-b.json")"
run_cli evict-b force-evict --id "${node_b_id}" --receipt-file "${work}/receipts/node-b.json" ||
  fail 'offline Node B force-evict failed'
assert_revoked node-b
assert_state node-b evicted 0
assert_control_targets node-c node-d
assert_audit node-b force-evict 1
verify_receipt node-b "${work}/receipts/node-b.json" evicted
run_cli evict-b-retry force-evict --id "${node_b_id}" --receipt-file "${work}/receipts/node-b-retry.json" ||
  fail 'Node B terminal retry failed'
verify_receipt node-b "${work}/receipts/node-b-retry.json" evicted
assert_audit node-b force-evict 2
assert_live node-c
assert_state node-c active 1
assert_live node-d
assert_state node-d active 1

# A bounded interruption after a real partial revoke leaves the Web result
# uncertain. The test-only Redis wire gate allows AUTH/SELECT but withholds
# EXEC after MariaDB has entered revoking and removed this identity's user.
# The installer trace below uses the exact absent receipt output path.
register_node node-e
db "INSERT INTO node (node_server_id) VALUES ($(jq -r .node_server_id "${work}/runtime/config/node-e.json"))" >/dev/null
assert_control_targets node-c node-d node-e
node_e_id="$(jq -r .node_identity_id "${work}/runtime/config/node-e.json")"
node_e_receipt="${work}/receipts/node-e-uncertain.json"
node_e_db_user="$(jq -r .mariadb.username "${work}/runtime/config/node-e.json")"
node_e_redis_user="$(jq -r .redis.username "${work}/runtime/config/node-e.json")"
"${work}/redis-exec-gate" "127.0.0.1:${redis_port}" "${work}/gate-port" "${work}/gate-held" &
gate_pid=$!
for _ in $(seq 1 50); do
  [[ -s "${work}/gate-port" ]] && break
  kill -0 "${gate_pid}" >/dev/null 2>&1 || fail 'Redis gate exited before listening'
  sleep 0.1
done
[[ -s "${work}/gate-port" ]] || fail 'Redis gate did not become ready'
gate_port="$(<"${work}/gate-port")"
[[ "${gate_port}" =~ ^[0-9]+$ ]] || fail 'Redis gate emitted an invalid port'
cp -- "${work}/runtime/config/config.ini" "${work}/config-real.ini"
sed -i "/^\[redis\]/,\$ s/^port=${redis_port}\$/port=${gate_port}/" "${work}/runtime/config/config.ini"
uncertain_status=0
(cd "${work}/runtime" && timeout -s TERM -k 2s 8s "${work}/trojan-panel" node-identity revoke \
  --id "${node_e_id}" --receipt-file "${node_e_receipt}" \
  >"${work}/uncertain.out" 2>"${work}/uncertain.err") &
uncertain_pid=$!
observed_partial=0
for _ in $(seq 1 50); do
  if [[ -s "${work}/gate-held" ]] &&
    [[ "$(db "SELECT status FROM node_identity WHERE identity_id='${node_e_id}'")" == revoking ]] &&
    [[ "$(db "SELECT COUNT(*) FROM mysql.user WHERE User='${node_e_db_user}'")" == 0 ]]; then
    observed_partial=1
    break
  fi
  kill -0 "${uncertain_pid}" >/dev/null 2>&1 || break
  sleep 0.1
done
wait "${uncertain_pid}" || uncertain_status=$?
uncertain_pid=""
cp -- "${work}/config-real.ini" "${work}/runtime/config/config.ini"
kill "${gate_pid}" >/dev/null 2>&1 || true
wait "${gate_pid}" >/dev/null 2>&1 || true
gate_pid=""
test "${observed_partial}" = 1 || fail 'Web revoke did not reach the partial-revocation stage before timeout'
test "${uncertain_status}" = 1 || fail 'Web revoke did not fail through its bounded Redis read timeout'
test ! -e "${node_e_receipt}" || fail 'uncertain Web revoke emitted a success receipt'
assert_state node-e revoking 1
redis_admin ACL USERS | grep -Fxq "${node_e_redis_user}" || fail 'Redis ACL changed before the held EXEC'
test "$(db "SELECT COUNT(*) FROM node_identity_event WHERE identity_id='${node_e_id}' AND action='revoke' AND result='succeeded'")" = 0 ||
  fail 'uncertain Web revoke recorded success before issuing a receipt'
test "$(db "SELECT COUNT(*) FROM node_identity_event WHERE identity_id='${node_e_id}' AND action='revoke' AND result='failed' AND error_code='redis_revocation_failed'")" = 1 ||
  fail 'partial Web revoke lacks an auditable Redis failure'
TP_NODE_REVOCATION_TIMEOUT_RECEIPT="${node_e_receipt}" \
  bash "${api_dir}/../../../deploy/installer/tests/node_revocation_remove_test.sh" ||
  fail 'Node remove crossed its local side-effect boundary after uncertain Web revoke'
assert_live node-d
run_cli uncertain-retry revoke --id "${node_e_id}" --receipt-file "${work}/receipts/node-e-retry.json" ||
  fail 'Web revoke did not recover after uncertain result'
verify_receipt node-e "${work}/receipts/node-e-retry.json" revoked
assert_control_targets node-c node-d

node_c_id="$(jq -r .node_identity_id "${work}/runtime/config/node-c.json")"
node_c_receipt="${work}/receipts/node-c.json"
# At this point the only writer is Node C's revoke. A single-statement trigger
# keeps the fault isolated to this throwaway database without client DELIMITER.
db "CREATE TRIGGER reject_node_c_revoke_audit BEFORE INSERT ON node_identity_event FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected audit failure'" >/dev/null
reject_cli audit-failure revoke --id "${node_c_id}" --receipt-file "${node_c_receipt}"
test ! -e "${node_c_receipt}" || fail 'audit failure created a success receipt'
assert_revoked node-c
assert_state node-c revoked 1
assert_control_targets node-d
test "$(db "SELECT COUNT(*) FROM node_identity_event WHERE identity_id='${node_c_id}' AND action='revoke' AND result='succeeded'")" = 0 ||
  fail 'failed audit unexpectedly committed success'
db 'DROP TRIGGER reject_node_c_revoke_audit' >/dev/null
run_cli audit-retry revoke --id "${node_c_id}" --receipt-file "${node_c_receipt}" ||
  fail 'retry after audit repair failed'
assert_revoked node-c
assert_audit node-c revoke 1
verify_receipt node-c "${node_c_receipt}" revoked
assert_live node-d
assert_state node-d active 1

for secret in "${admin_db_password}" "${admin_redis_password}"; do
  ! grep -Fq -- "${secret}" "${work}"/*.out "${work}"/*.err || fail 'CLI output leaked admin credentials'
done
for name in node-a node-b node-c node-d node-e; do
  credential="${work}/runtime/config/${name}.json"
  for field in .mariadb.password .redis.password .redis_auth.password; do
    secret="$(jq -r "${field}" "${credential}")"
    ! grep -Fq -- "${secret}" "${work}"/*.out "${work}"/*.err || fail 'CLI output leaked Node credentials'
  done
done
printf '%s\n' 'PASS: real MariaDB/Redis receipt CLI, offline eviction, audit failure and safe retry'
