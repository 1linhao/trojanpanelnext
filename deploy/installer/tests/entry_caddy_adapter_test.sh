#!/usr/bin/env bash
set -Eeuo pipefail

repo="$(cd "$(dirname "$0")/../../.." && pwd)"
source "$repo/deploy/installer/entry/controller.sh"
source "$repo/deploy/installer/entry/adapters/caddy.sh"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
export ENTRY_SPEC_OWNER_UID="$(id -u)"
export CADDY_ADAPTER_FAKE=1
export CADDY_ADAPTER_SKIP_PORT_CHECK=1
export CADDY_ADAPTER_ROOT="$tmp/caddy"
export ENTRY_STATE_ROOT="$tmp/state"

fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
expect_fail() { if "$@" >"$tmp/out" 2>&1; then cat "$tmp/out" >&2; fail "unexpected success: $*"; fi; }

mkdir -p "$tmp/certs/web" "$tmp/certs/node"
for role in web node; do
  domain="${role}.example.com"
  openssl req -x509 -newkey rsa:2048 -nodes \
    -keyout "$tmp/certs/$role/key.pem" -out "$tmp/certs/$role/cert.pem" \
    -days 2 -subj "/CN=${domain}" -addext "subjectAltName=DNS:${domain}" >/dev/null 2>&1
done
cat "$tmp/certs/web/cert.pem" "$tmp/certs/node/cert.pem" >"$tmp/test-ca.pem"
export CADDY_ADAPTER_CA_FILE="$tmp/test-ca.pem"
printf '{"routes":[{"network":"tcp","port":8443}]}' >"$tmp/routes.json"

# The canonical v2 contract stores managed_dir at the data level; without an
# installer override the adapter root is its parent, not the data directory.
jq '.certificate_targets.web.managed_dir = "/tpdata/custom/web-caddy/data" |
   .certificate_targets.node.managed_dir = "/tpdata/custom/web-caddy/data"' \
  "$repo/docs/entry-controller/examples/combined-caddy-v2.json" >"$tmp/canonical-root-spec"
unset CADDY_ADAPTER_ROOT
[[ "$(caddy_adapter_root "$tmp/canonical-root-spec")" == /tpdata/custom/web-caddy ]] ||
  fail 'canonical managed_dir did not resolve to the Caddy root'
export CADDY_ADAPTER_ROOT="$tmp/caddy"

jq --arg root "$CADDY_ADAPTER_ROOT" \
  --arg routes "$tmp/routes.json" \
  --arg wc "$tmp/certs/web/cert.pem" --arg wk "$tmp/certs/web/key.pem" \
  --arg nc "$tmp/certs/node/cert.pem" --arg nk "$tmp/certs/node/key.pem" \
  '.domains.web = "web.example.com" | .domains.node = "node.example.com" |
   .roles.node.route_manifest=$routes |
   .certificate_targets.web.managed_dir = $root | .certificate_targets.node.managed_dir = $root |
   .certificate_targets.web.cert_path = $wc | .certificate_targets.web.key_path = $wk |
   .certificate_targets.node.cert_path = $nc | .certificate_targets.node.key_path = $nk' \
  "$repo/docs/entry-controller/examples/combined-caddy-v2.json" >"$tmp/spec"
chmod 0600 "$tmp/spec"

render="$(caddy_adapter_render "$tmp/spec")"
grep -Fq 'web.example.com' <<<"$render" || fail 'web site missing from Caddyfile'
grep -Fq 'node.example.com' <<<"$render" || fail 'node site missing from Caddyfile'
grep -Fq 'reverse_proxy 127.0.0.1:8888' <<<"$render" || fail 'web upstream missing'
grep -Fq 'file_server' <<<"$render" || fail 'node fallback missing'

probe="$(entry_v2_adapter_probe "$tmp/spec" null)"
[[ "$(jq '.resources | length' <<<"$probe")" == 0 ]] || fail 'probe adopted an unowned root'

created="$(entry_v2_reconcile "$tmp/spec" "$ENTRY_STATE_ROOT")"
[[ "$(jq -r '.phase' <<<"$created")" == stable ]] || fail 'Caddy transaction did not commit'
[[ "$(jq -r '.health' <<<"$created")" == healthy ]] || fail 'Caddy transaction is not healthy'
[[ "$(jq '[.listeners[] | select(.owner == "provider" and (.port == 80 or .port == 443))] | length' <<<"$created")" == 2 ]] || fail 'Caddy did not own one 80/443 pair'
[[ "$(jq '[.certificates | keys[]] | length' <<<"$created")" == 2 ]] || fail 'both certificates were not observed'
[[ "$(jq '[.resources[] | select(.kind == "certificate")] | length' <<<"$created")" == 4 ]] || fail 'certificate resources were not journaled'
file_digest="$(printf '%s' "$(sha256sum "$CADDY_ADAPTER_ROOT/Caddyfile" | awk '{print $1}')" | sha256sum | awk '{print $1}')"
[[ "$(jq -r --arg id "$CADDY_ADAPTER_ROOT/Caddyfile" '.resources[] | select(.id == $id) | .identity.digest' <<<"$created")" == "$file_digest" ]] || fail 'file identity was not based on content'

export CADDY_ADAPTER_PORT_CHECK_CMD='false'
unchanged="$(entry_v2_reconcile "$tmp/spec" "$ENTRY_STATE_ROOT")"
[[ "$(jq -r '.result' <<<"$unchanged")" == unchanged ]] || fail 'same target was not idempotent'
unset CADDY_ADAPTER_PORT_CHECK_CMD

# A renewal changes only the node CertificateRef and its generation.
openssl req -x509 -newkey rsa:2048 -nodes -keyout "$tmp/new-node.key" \
  -out "$tmp/new-node.crt" -days 3 -subj '/CN=node.example.com' \
  -addext 'subjectAltName=DNS:node.example.com' >/dev/null 2>&1
cat "$tmp/new-node.crt" >>"$tmp/test-ca.pem"
original_refresh="$(declare -f entry_v2_adapter_refresh)"
entry_v2_adapter_refresh() {
  cp "$tmp/new-node.crt" "$tmp/certs/node/cert.pem"
  cp "$tmp/new-node.key" "$tmp/certs/node/key.pem"
  local observed
  observed="$(caddy_adapter_observation "$1" "$CADDY_ADAPTER_ROOT" 1)" || return 1
  caddy_adapter_apply_generations "$observed" "$2"
}
renewed="$(entry_v2_reconcile "$tmp/spec" "$ENTRY_STATE_ROOT")"
[[ "$(jq -r '.result' <<<"$renewed")" == renewed ]] || fail 'renewal was not observed'
[[ "$(jq -r '.certificates.node.generation' <<<"$renewed")" == 2 ]] || fail 'node certificate generation did not advance'
[[ "$(jq -r '.certificates.web.generation' <<<"$renewed")" == 1 ]] || fail 'unchanged web certificate generation advanced'
[[ "$(jq -r '.generation' <<<"$renewed")" == 2 ]] || fail 'deployment renewal generation did not advance'
eval "$original_refresh"

printf 'deployment=other\n' >"$CADDY_ADAPTER_ROOT/.trojanpanelnext-owner"
jq '.revision = 2 | .roles.web.web_upstream = "127.0.0.1:8899"' "$tmp/spec" >"$tmp/changed"
chmod 0600 "$tmp/changed"
if foreign_error="$(entry_v2_reconcile "$tmp/changed" "$ENTRY_STATE_ROOT" 2>&1)"; then
  fail 'foreign Caddy root was adopted'
fi
printf 'deployment=trojanpanelnext-combined\nowner_token=%s\n' "$(jq -r '.owner_token' "$tmp/spec")" >"$CADDY_ADAPTER_ROOT/.trojanpanelnext-owner"

cp "$tmp/certs/node/cert.pem" "$tmp/node-cert-backup"
rm -f "$tmp/certs/node/cert.pem"
if entry_v2_reconcile "$tmp/changed" "$ENTRY_STATE_ROOT" >"$tmp/certificate-failure" 2>&1; then
  fail 'certificate failure was not reported'
fi
[[ -s "$tmp/certificate-failure" ]] || fail 'certificate failure had no structured output'
certificate_error="$(jq -r '.code' "$tmp/certificate-failure")"
[[ "$certificate_error" == ownership_conflict || "$certificate_error" == verification_failed ]] || fail 'certificate failure was not structured'
[[ "$(jq -r '.phase' "$ENTRY_STATE_ROOT/trojanpanelnext-combined.json")" == stable ]] || fail 'rollback did not restore stable journal'

cp "$tmp/node-cert-backup" "$tmp/certs/node/cert.pem"
recovered="$(entry_v2_reconcile "$tmp/changed" "$ENTRY_STATE_ROOT")"
health="$(jq -r '.health' <<<"$recovered")"
[[ "$health" == healthy ]] || fail 'reconcile did not recover after certificate returned'

printf 'tampered\n' >"$CADDY_ADAPTER_ROOT/Caddyfile"
{
  jq '.revision = 3 | .roles.web.web_upstream = "127.0.0.1:8898"' "$tmp/changed" >"$tmp/tampered-spec"
  chmod 0600 "$tmp/tampered-spec"
}
if tampered_error="$(entry_v2_reconcile "$tmp/tampered-spec" "$ENTRY_STATE_ROOT" 2>&1)"; then
  fail 'tampered Caddyfile was accepted'
fi
[[ "$(jq -r '.code' <<<"$tampered_error")" == ownership_conflict ]] || fail 'tampered Caddyfile error was not structured'
render="$(caddy_adapter_render "$tmp/changed")"
printf '%s' "$render" >"$CADDY_ADAPTER_ROOT/Caddyfile"

jq '.revision = 4 | .active_roles = ["web"] | del(.roles.node, .certificate_targets.node)' "$tmp/changed" >"$tmp/web"
chmod 0600 "$tmp/web"
one_role="$(entry_v2_reconcile "$tmp/web" "$ENTRY_STATE_ROOT")"
[[ "$(jq -r '.active_roles | join(",")' <<<"$one_role")" == web ]] || fail 'role removal did not commit'
bash -c 'exec 9<"$1"; sleep 30' held-cert "$tmp/certs/web/cert.pem" &
consumer_pid=$!
sleep 1
if entry_v2_adapter_remove "$tmp/web" "$one_role" 1 >/dev/null 2>&1; then
  kill "$consumer_pid" 2>/dev/null || true
  fail 'purge accepted an open certificate consumer'
fi
kill "$consumer_pid" 2>/dev/null || true
wait "$consumer_pid" 2>/dev/null || true
removed="$(entry_v2_remove "$tmp/web" "$ENTRY_STATE_ROOT" 1)"
[[ "$(jq -r '.result' <<<"$removed")" == removed ]] || fail 'owned Caddy resources were not removed'
[[ ! -e "$CADDY_ADAPTER_ROOT/Caddyfile" && ! -e "$tmp/certs/web/cert.pem" ]] || fail 'purge crossed ownership boundary'

printf 'Caddy Adapter combined transaction: PASS\n'
