#!/usr/bin/env bash
set -Eeuo pipefail

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

if ! docker info >/dev/null 2>&1; then
  docker() { sudo -n /usr/bin/docker "$@"; }
fi

work="$(mktemp -d)"
suffix="${RANDOM}-$$"
network="tp-combined-${suffix}"
entry="tp-combined-entry-${suffix}"
web="tp-combined-web-${suffix}"
core="tp-combined-core-${suffix}"
entry_volume="tp-combined-entry-data-${suffix}"
image="caddy:2.8.4"

cleanup_test() {
  docker rm -fv "${entry}" "${web}" "${core}" >/dev/null 2>&1 || true
  docker volume rm "${entry_volume}" >/dev/null 2>&1 || true
  docker network rm "${network}" >/dev/null 2>&1 || true
  rm -rf -- "${work}"
}
trap cleanup_test EXIT

mkdir -p "${work}/entry-config" "${work}/web-config" "${work}/core-config"
cat >"${work}/web-config/Caddyfile" <<'EOF'
:8080 {
    respond "web-control-plane"
}
EOF
cat >"${work}/entry-config/Caddyfile" <<'EOF'
panel.example.test {
    tls internal
    reverse_proxy web:8080
}

node.example.test {
    tls internal
    respond "node-domain"
}
EOF
cat >"${work}/core-config/Caddyfile" <<'EOF'
core.example.test:9443 {
    tls internal
    respond "direct-node-kernel"
}
EOF

docker pull "${image}" >/dev/null
docker network create "${network}" >/dev/null
docker volume create "${entry_volume}" >/dev/null
docker run -d --name "${web}" --network "${network}" --network-alias web \
  -v "${work}/web-config/Caddyfile:/etc/caddy/Caddyfile:ro" "${image}" >/dev/null
docker run -d --name "${core}" --network "${network}" -p 127.0.0.1::9443 \
  -v "${work}/core-config/Caddyfile:/etc/caddy/Caddyfile:ro" "${image}" >/dev/null
docker run -d --name "${entry}" --network "${network}" \
  -p 127.0.0.1::80 -p 127.0.0.1::443 \
  -v "${work}/entry-config/Caddyfile:/etc/caddy/Caddyfile:ro" \
  -v "${entry_volume}:/data" "${image}" >/dev/null

entry_https_port="$(docker port "${entry}" 443/tcp | sed -n 's/.*://p' | head -n 1)"
core_port="$(docker port "${core}" 9443/tcp | sed -n 's/.*://p' | head -n 1)"
[[ "${entry_https_port}" =~ ^[0-9]+$ ]] || fail 'shared Entry did not publish HTTPS'
[[ "${core_port}" =~ ^[0-9]+$ ]] || fail 'direct Node listener was not published independently'

probe_https() {
  local port="$1" server_name="$2" expected="$3"
  local output
  output="$(curl --insecure --fail --silent --show-error --noproxy '*' \
    --resolve "${server_name}:${port}:127.0.0.1" "https://${server_name}:${port}/" 2>/dev/null || true)"
  grep -Fq "${expected}" <<<"${output}"
}

for _ in $(seq 1 30); do
  if probe_https "${entry_https_port}" panel.example.test web-control-plane &&
    probe_https "${entry_https_port}" node.example.test node-domain &&
    probe_https "${core_port}" core.example.test direct-node-kernel; then
    ready=1
    break
  fi
  sleep 1
done
[[ "${ready:-0}" == 1 ]] || {
  docker logs "${entry}" >&2 || true
  fail 'combined Docker HTTPS smoke did not become ready'
}

entry_ports="$(docker inspect "${entry}" --format '{{json .HostConfig.PortBindings}}')"
web_ports="$(docker inspect "${web}" --format '{{json .HostConfig.PortBindings}}')"
core_ports="$(docker inspect "${core}" --format '{{json .HostConfig.PortBindings}}')"
grep -Fq '80/tcp' <<<"${entry_ports}" || fail 'shared Entry does not own TCP 80'
grep -Fq '443/tcp' <<<"${entry_ports}" || fail 'shared Entry does not own TCP 443'
[[ "${web_ports}" == null || "${web_ports}" == '{}' ]] || fail 'Web process unexpectedly published host ports'
grep -Fq '9443/tcp' <<<"${core_ports}" || fail 'Node listener is not direct'
! grep -Eq '"(80|443)/tcp"' <<<"${core_ports}" || fail 'Node process competes for shared Entry ports'

for domain in panel.example.test node.example.test; do
  certificate="/data/caddy/certificates/local/${domain}/${domain}.crt"
  docker exec "${entry}" test -s "${certificate}" || fail "shared Entry did not manage a certificate for ${domain}"
  docker exec "${entry}" cat "${certificate}" | openssl x509 -noout -checkhost "${domain}" 2>&1 |
    grep -Fq 'does match certificate' || fail "certificate does not cover ${domain}"
done

printf 'PASS combined Docker shared listener, dual-domain HTTPS, certificates, and direct Node exposure smoke\n'
