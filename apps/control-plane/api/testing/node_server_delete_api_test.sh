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
suffix="${RANDOM}-$$"
mariadb_container="tp-node-delete-mariadb-${suffix}"
redis_container="tp-node-delete-redis-${suffix}"
mariadb_image='mariadb@sha256:07e06f2e7ae9dfc63707a83130a62e00167c827f08fcac7a9aa33f4b6dc34e0e'
redis_image='redis@sha256:a93c14584715ec5bd9d2648d58c3b27f89416242bee0bc9e5fb2edc1a4cbec1d'
admin_db_password="node-delete-db-${RANDOM}-${RANDOM}"
admin_redis_password="node-delete-redis-${RANDOM}-${RANDOM}"

cleanup() {
  docker rm -fv "${mariadb_container}" "${redis_container}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

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

mariadb_port="$(docker port "${mariadb_container}" 3306/tcp | sed -n 's/.*://p' | head -n1)"
redis_port="$(docker port "${redis_container}" 6379/tcp | sed -n 's/.*://p' | head -n1)"

(
  cd "${API_DIR}"
  TP_TEST_MARIADB_HOST=127.0.0.1 \
  TP_TEST_MARIADB_PORT="${mariadb_port}" \
  TP_TEST_MARIADB_PASSWORD="${admin_db_password}" \
  TP_TEST_REDIS_ADDRESS="127.0.0.1:${redis_port}" \
  TP_TEST_REDIS_PASSWORD="${admin_redis_password}" \
    go test ./router -run '^TestDeleteNodeServerHTTPProtectsManagedIdentityLifecycle$' -count=1 -v
)

printf 'PASS public node_server delete API lifecycle boundary\n'
