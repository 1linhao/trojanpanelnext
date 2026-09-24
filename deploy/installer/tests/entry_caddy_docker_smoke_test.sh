#!/usr/bin/env bash
set -Eeuo pipefail

repo="$(cd "$(dirname "$0")/../../.." && pwd)"
source "$repo/deploy/installer/entry/controller.sh"
source "$repo/deploy/installer/entry/adapters/caddy.sh"

[[ "$(id -u)" == 0 ]] || { echo 'Run this isolated host-network smoke as root' >&2; exit 2; }
command -v ss >/dev/null 2>&1 || { echo 'ss is required for listener ownership checks' >&2; exit 2; }
[[ -z "$(ss -Hlnpt '( sport = :80 or sport = :443 or sport = :8888 )')" ]] || {
  echo 'Ports 80, 443 or 8888 are already in use' >&2; exit 2;
}
tmp="$(mktemp -d)"
container="tpn-entry-smoke-$$"
core="tpn-core-smoke-$$"
server_pid=''
cleanup() {
  docker rm -f "$core" >/dev/null 2>&1 || true
  docker rm -f "$container" >/dev/null 2>&1 || true
  [[ -z "$server_pid" ]] || kill "$server_pid" 2>/dev/null || true
  rm -rf "$tmp"
}
trap cleanup EXIT

export ENTRY_SPEC_OWNER_UID=0 ENTRY_STATE_ROOT="$tmp/state"
export CADDY_ADAPTER_ROOT="$tmp/caddy" CADDY_ADAPTER_CONTAINER="$container"
export CADDY_ADAPTER_NODE_CONTAINER="$core"
export CADDY_ADAPTER_WEB_ROOT="$tmp/webroot" CADDY_ADAPTER_TEST_INTERNAL_TLS=1
export CADDY_ADAPTER_SKIP_DNS_CHECK=1 CADDY_ADAPTER_CERT_WAIT_ATTEMPTS=20 CADDY_ADAPTER_CERT_WAIT_SECONDS=1
export CADDY_ADAPTER_IMAGE='caddy@sha256:226d1f059b75399fe19182893c7184591c07b97afc8dfcf44eeb80c9a77a530f'
docker image inspect "$CADDY_ADAPTER_IMAGE" >/dev/null 2>&1 || docker pull "$CADDY_ADAPTER_IMAGE" >/dev/null
mkdir -p "$tmp/webroot" "$tmp/upstream"
printf 'node-route-ok\n' >"$tmp/webroot/index.html"
printf 'web-route-ok\n' >"$tmp/upstream/index.html"
python3 -m http.server 8888 --bind 127.0.0.1 --directory "$tmp/upstream" >"$tmp/upstream.log" 2>&1 &
server_pid=$!
sleep 1

jq --arg root "$CADDY_ADAPTER_ROOT" --arg tmp "$tmp" '
  .domains.web="web.entry-smoke.test" | .domains.node="node.entry-smoke.test" |
  .certificate_targets.web.managed_dir=$root | .certificate_targets.node.managed_dir=$root |
  .certificate_targets.web.cert_path=($tmp+"/cert/web/fullchain.pem") |
  .certificate_targets.web.key_path=($tmp+"/cert/web/privkey.pem") |
  .certificate_targets.node.cert_path=($tmp+"/cert/node/fullchain.pem") |
  .certificate_targets.node.key_path=($tmp+"/cert/node/privkey.pem") |
  .roles.node.route_manifest=($tmp+"/routes.json") |
  .roles.node.certificate_consumer=($tmp+"/consumer")' \
  "$repo/docs/entry-controller/examples/combined-caddy-v2.json" >"$tmp/spec"
chmod 0600 "$tmp/spec"
printf '{"routes":[{"network":"tcp","port":8443}]}' >"$tmp/routes.json"

cp "$tmp/routes.json" "$tmp/routes-valid.json"
jq '.routes[0].port=443' "$tmp/routes-valid.json" >"$tmp/routes.json"
if entry_v2_reconcile "$tmp/spec" "$ENTRY_STATE_ROOT" >"$tmp/port-failure"; then
  echo 'Node route on Caddy port 443 was accepted' >&2; exit 1
fi
[[ "$(jq -r '.code' "$tmp/port-failure")" == ownership_conflict && ! -e "$CADDY_ADAPTER_ROOT" ]]
cp "$tmp/routes-valid.json" "$tmp/routes.json"

mkdir -p "$tmp/cert/web"
printf 'unowned\n' >"$tmp/cert/web/fullchain.pem"
if entry_v2_reconcile "$tmp/spec" "$ENTRY_STATE_ROOT" >"$tmp/preexisting-failure"; then
  echo 'Preexisting certificate was adopted' >&2; exit 1
fi
[[ "$(jq -r '.code' "$tmp/preexisting-failure")" == ownership_conflict ]]
[[ "$(cat "$tmp/cert/web/fullchain.pem")" == unowned ]]
rm -f "$tmp/cert/web/fullchain.pem"

mv "$tmp/webroot/index.html" "$tmp/webroot/held-index.html"
if entry_v2_reconcile "$tmp/spec" "$ENTRY_STATE_ROOT" >"$tmp/first-verify-failure"; then
  echo 'Missing node route was accepted' >&2; exit 1
fi
[[ "$(jq -r '.code' "$tmp/first-verify-failure")" == verification_failed ]]
[[ ! -e "$CADDY_ADAPTER_ROOT/data" && ! -e "$tmp/cert/web/fullchain.pem" ]]
[[ "$(docker inspect -f '{{.Id}}' "$container" 2>/dev/null || true)" == '' ]]
mv "$tmp/webroot/held-index.html" "$tmp/webroot/index.html"

created="$(entry_v2_reconcile "$tmp/spec" "$ENTRY_STATE_ROOT")"
[[ "$(jq -r '.phase + ":" + .health' <<<"$created")" == stable:healthy ]]
[[ "$(jq -r '.certificates | keys | join(",")' <<<"$created")" == node,web ]]
cmp -s "$tmp/cert/node/fullchain.pem" "$tmp/consumer/fullchain.pem"
cmp -s "$tmp/cert/node/privkey.pem" "$tmp/consumer/privkey.pem"
ca="$CADDY_ADAPTER_ROOT/data/caddy/pki/authorities/local/root.crt"
[[ -s "$ca" ]]
for role in web node; do
  content="$(curl --silent --show-error --fail --cacert "$ca" --resolve "${role}.entry-smoke.test:443:127.0.0.1" "https://${role}.entry-smoke.test/")"
  [[ "$content" == "${role}-route-ok" ]]
done
[[ "$(docker inspect -f '{{.State.Running}}' "$container")" == true ]]
[[ "$(entry_v2_reconcile "$tmp/spec" "$ENTRY_STATE_ROOT" | jq -r '.result')" == unchanged ]]
docker run -d --name "$core" --network none \
  -v "$tmp/consumer:$tmp/consumer:ro" \
  -e "crt_path=$tmp/consumer/fullchain.pem" -e "key_path=$tmp/consumer/privkey.pem" \
  "$CADDY_ADAPTER_IMAGE" sleep 3600 >/dev/null
core_before="$(docker inspect -f '{{.State.StartedAt}}' "$core")"

# Simulate a Caddy storage renewal with a fresh leaf from this isolated CA.
source_dir="$CADDY_ADAPTER_ROOT/data/caddy/certificates/local/node.entry-smoke.test"
[[ -s "$source_dir/node.entry-smoke.test.crt" && -s "$source_dir/node.entry-smoke.test.key" ]]
openssl req -new -newkey rsa:2048 -nodes -keyout "$tmp/renewed.key" -out "$tmp/renewed.csr" \
  -subj '/CN=node.entry-smoke.test' >/dev/null 2>&1
printf 'subjectAltName=DNS:node.entry-smoke.test\nextendedKeyUsage=serverAuth\n' >"$tmp/renewed.ext"
openssl x509 -req -in "$tmp/renewed.csr" -CA "$ca" \
  -CAkey "$CADDY_ADAPTER_ROOT/data/caddy/pki/authorities/local/root.key" -CAcreateserial \
  -out "$tmp/renewed.crt" -days 2 -extfile "$tmp/renewed.ext" >/dev/null 2>&1
cp "$tmp/renewed.crt" "$source_dir/node.entry-smoke.test.crt"
cp "$tmp/renewed.key" "$source_dir/node.entry-smoke.test.key"
docker restart "$container" >/dev/null
sleep 2
started_before="$(docker inspect -f '{{.State.StartedAt}}' "$container")"
renewed="$(entry_v2_reconcile "$tmp/spec" "$ENTRY_STATE_ROOT")"
[[ "$(jq -r '.result' <<<"$renewed")" == renewed ]]
[[ "$(jq -r '.certificates.node.generation' <<<"$renewed")" == 2 ]]
[[ "$(jq -r '.certificates.web.generation' <<<"$renewed")" == 1 ]]
cmp -s "$tmp/renewed.crt" "$tmp/cert/node/fullchain.pem"
cmp -s "$tmp/renewed.crt" "$tmp/consumer/fullchain.pem"
[[ "$(docker inspect -f '{{.State.StartedAt}}' "$container")" == "$started_before" ]]
[[ "$(docker inspect -f '{{.State.StartedAt}}' "$core")" != "$core_before" ]]
core_after="$(docker inspect -f '{{.State.StartedAt}}' "$core")"
[[ "$(entry_v2_reconcile "$tmp/spec" "$ENTRY_STATE_ROOT" | jq -r '.result')" == unchanged ]]
[[ "$(docker inspect -f '{{.State.StartedAt}}' "$core")" == "$core_after" ]]

# A failed new target must restore the committed Caddyfile and working HTTPS.
jq '.revision=2 | .roles.web.web_upstream="127.0.0.1:8999"' "$tmp/spec" >"$tmp/bad"
chmod 0600 "$tmp/bad"
if entry_v2_reconcile "$tmp/bad" "$ENTRY_STATE_ROOT" >"$tmp/failure"; then
  echo 'Broken web upstream was accepted' >&2; exit 1
fi
[[ "$(jq -r '.code' "$tmp/failure")" == verification_failed ]]
[[ "$(curl --silent --fail --cacert "$ca" --resolve 'web.entry-smoke.test:443:127.0.0.1' 'https://web.entry-smoke.test/')" == web-route-ok ]]
jq '.revision=3 | .active_roles=["node"] | del(.roles.web, .certificate_targets.web)' "$tmp/spec" >"$tmp/node-spec"
chmod 0600 "$tmp/node-spec"
node_only="$(entry_v2_reconcile "$tmp/node-spec" "$ENTRY_STATE_ROOT")"
[[ "$(jq -r '.active_roles | join(",")' <<<"$node_only")" == node ]]
[[ "$(curl --silent --fail --cacert "$ca" --resolve 'node.entry-smoke.test:443:127.0.0.1' 'https://node.entry-smoke.test/')" == node-route-ok ]]
if entry_v2_adapter_remove "$tmp/node-spec" "$node_only" 1; then
  echo 'Purge removed a certificate still consumed by running Core' >&2; exit 1
fi
[[ "$(docker inspect -f '{{.State.Running}}' "$container")" == true ]]
[[ "$(entry_v2_remove "$tmp/node-spec" "$ENTRY_STATE_ROOT" 0 | jq -r '.result')" == removed ]]
[[ "$(docker inspect -f '{{.State.Running}}' "$container" 2>/dev/null || true)" == '' ]]
echo 'PASS real Caddy isolated Docker HTTPS, renewal consumer, rollback, role removal and remove'
