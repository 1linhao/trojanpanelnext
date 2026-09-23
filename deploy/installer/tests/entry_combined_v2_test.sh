#!/usr/bin/env bash
set -Eeuo pipefail

repo="$(cd "$(dirname "$0")/../../.." && pwd)"
source "$repo/deploy/installer/entry/controller.sh"
source "$repo/deploy/installer/entry/entryctl.sh"
fixture="$repo/docs/entry-controller/examples/combined-caddy-v2.json"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
export ENTRY_SPEC_OWNER_UID="$(id -u)"
export ENTRY_STATE_ROOT="$tmp/state"
trace="$tmp/trace"
: >"$trace"
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
expect_fail() { if "$@" >"$tmp/out" 2>&1; then cat "$tmp/out" >&2; cat "$trace" >&2; fail "unexpected success: $*"; fi; }
make_spec() { jq "$1" "$fixture" >"$2"; chmod 0600 "$2"; }
make_spec '.' "$tmp/spec"
entry_validate_spec "$tmp/spec" || fail 'valid combined spec rejected'
for change in '.domains.node = .domains.web' '.active_roles = ["web","web"]' 'del(.roles.node)' '.active_roles = ["web"]' '.certificate_targets.node.cert_path = .certificate_targets.web.cert_path' '.roles.node.node_exposure = "proxy"'; do
  make_spec "$change" "$tmp/bad"
  expect_fail entry_validate_spec "$tmp/bad"
done

fake_observation() {
  local spec="$1" state="$2" stage="$3"
  jq -cn --argjson spec "$(jq -c . "$spec")" --argjson state "$state" --arg stage "$stage" \
    --arg partial "${FAKE_PARTIAL:-0}" --arg drift "${FAKE_IDENTITY_DRIFT:-0}" \
    --arg resource_digest "${FAKE_RESOURCE_DIGEST:-a}" --arg candidate_digest "${FAKE_CANDIDATE_DIGEST:-a}" \
    --arg unknown_candidate "${FAKE_UNKNOWN_CANDIDATE:-0}" '
    def identity($id;$digest): {marker:("owned:" + $spec.deployment_id + ":" + $id),digest:($digest*64)};
    def resource($id;$scope;$role):
      {kind:"file",id:("/managed/" + $id),owner:"provider",deployment_id:$spec.deployment_id,
       scope:$scope,retention:"managed",identity:identity($id;$candidate_digest)} +
      (if $scope == "role" then {role:$role} else {} end);
    ($spec.active_roles | map(resource(.;"role";.))) as $roles |
    ([resource("shared";"shared";null)] + $roles) as $candidate |
    ($spec.active_roles | map({key:.,value:{domain:$spec.domains[.],cert_path:$spec.certificate_targets[.].cert_path,
      key_path:$spec.certificate_targets[.].key_path,fingerprint:("fingerprint-" + .),generation:1,
      renewal_owner:$spec.provider,last_hook_status:"ok",not_after:"2030-01-01T00:00:00Z"}}) | from_entries) as $certs |
    ([{transport:"tcp",address:"0.0.0.0",port:80,purpose:"acme-http01",owner:"provider",scope:"shared"},
      {transport:"tcp",address:"0.0.0.0",port:443,purpose:"web-https",owner:"provider",scope:"shared"}] +
      (if $spec.active_roles | index("node") then
        [{transport:"tcp",address:"127.0.0.1",port:2443,purpose:"node-direct",owner:"kernel",scope:"role",role:"node"}]
       else [] end)) as $listeners |
    {schema_version:2,deployment_id:$spec.deployment_id,provider:$spec.provider,ownership_verified:true,
     resources:(if $stage == "probe" then
       (if $partial == "1" and $state.phase != "stable" and $state != null then ($state.candidate_resources // [])
        else ($state.resources // []) end)
       else $candidate end),
     candidate_resources:(if $unknown_candidate == "1" then
       $candidate + [($candidate[0] | .id = "/managed/unknown" |
         .identity.marker = "owned:trojanpanelnext-combined:unknown" |
         .identity.digest = ("c" * 64))]
      else $candidate end),certificates:$certs,listeners:$listeners,capabilities:[]} |
     if $drift == "1" and $stage == "probe" and (.resources | length) > 0 then
       .resources[0].identity.digest = ($resource_digest*64)
     else . end'
}
entry_v2_adapter_probe() { printf 'probe\n' >>"$trace"; fake_observation "$1" "$2" probe; }
entry_v2_adapter_prepare() {
  printf 'prepare\n' >>"$trace"
  [[ -z "${FAIL_AT:-}" ]] || printf 'fail-at:%s\n' "$FAIL_AT" >>"$trace"
  [[ "$(jq -r '.phase' <"$ENTRY_STATE_ROOT/trojanpanelnext-combined.json")" == preparing ]] || fail 'prepare lacks journal'
  [[ "${FAIL_AT:-}" != prepare ]] || return 1
  fake_observation "$1" "$2" prepare
}
entry_v2_adapter_activate() {
  printf 'activate\n' >>"$trace"
  [[ "$(jq -r '.phase' <"$ENTRY_STATE_ROOT/trojanpanelnext-combined.json")" == activating ]] || fail 'activate lacks journal'
  [[ "${FAIL_AT:-}" != activate ]]
}
entry_v2_adapter_verify() {
  printf 'verify\n' >>"$trace"
  [[ "$(jq -r '.phase' <"$ENTRY_STATE_ROOT/trojanpanelnext-combined.json")" == verifying ]] || fail 'verify lacks journal'
  [[ "${FAIL_AT:-}" != verify ]] || return 1
  fake_observation "$1" "$2" verify
}
entry_v2_adapter_rollback() {
  printf 'rollback\n' >>"$trace"
  printf 'rollback-spec-revision:%s\n' "$(jq -r '.revision' "$1")" >>"$trace"
  printf 'rollback-kind:%s\n' "${ENTRY_V2_ROLLBACK_KIND:-unset}" >>"$trace"
  [[ "$(jq -r '.phase' <"$ENTRY_STATE_ROOT/trojanpanelnext-combined.json")" == rolling_back ]] || fail 'rollback lacks journal'
  [[ "${FAIL_ROLLBACK:-0}" != 1 ]]
}
entry_v2_adapter_remove() {
  printf 'remove\n' >>"$trace"
  [[ "$(jq -r '.phase' <"$ENTRY_STATE_ROOT/trojanpanelnext-combined.json")" == activating ]] || fail 'remove lacks journal'
  [[ "${FAIL_REMOVE:-0}" != 1 ]]
}

plan="$(main plan --spec "$tmp/spec" --state-root "$ENTRY_STATE_ROOT")"
[[ "$(jq -r '.schema_version' <<<"$plan")" == 2 ]] || fail 'v2 plan did not dispatch'
[[ "$(jq -r '.executable' <<<"$plan")" == false ]] || fail 'production plan enabled mutation'
expect_fail bash "$repo/deploy/installer/entry/entryctl.sh" reconcile --spec "$tmp/spec" --state-root "$ENTRY_STATE_ROOT"
[[ "$(jq -r '.code' <"$tmp/out")" == unsupported_capability ]] || fail 'production v2 mutation did not fail closed'
make_spec '.active_roles = ["web"] | del(.roles.node, .certificate_targets.node)' "$tmp/initial-one-role"
expect_fail entry_v2_reconcile "$tmp/initial-one-role" "$ENTRY_STATE_ROOT"
[[ "$(jq -r '.code' <"$tmp/out")" == invalid_spec ]] || fail 'one-role initial deployment accepted'

# No committed target exists after a failed first prepare. Recovery removes the
# tombstone only after rollback and then retries the same two-role target.
normal_root="$ENTRY_STATE_ROOT"
ENTRY_STATE_ROOT="$tmp/first-failure"
FAIL_AT=prepare expect_fail entry_v2_reconcile "$tmp/spec" "$ENTRY_STATE_ROOT"
[[ "$(jq -r '.phase' <"$ENTRY_STATE_ROOT/trojanpanelnext-combined.json")" == failed ]] || { cat "$tmp/out" >&2; jq -c '{phase,health,last_error}' "$ENTRY_STATE_ROOT/trojanpanelnext-combined.json" >&2; fail 'initial failure lost journal'; }
entry_v2_reconcile "$tmp/spec" "$ENTRY_STATE_ROOT" >/dev/null
[[ "$(jq -r '.phase' <"$ENTRY_STATE_ROOT/trojanpanelnext-combined.json")" == stable ]] || fail 'initial failure recovery failed'
ENTRY_STATE_ROOT="$normal_root"
: >"$trace"

created="$(entry_v2_reconcile "$tmp/spec" "$ENTRY_STATE_ROOT")"
[[ -z "${ENTRY_V2_TEST_CAPTURE:-}" ]] || printf '%s\n' "$created" >"$ENTRY_V2_TEST_CAPTURE"
[[ "$(jq -r '.phase' <<<"$created")" == stable ]] || fail 'initial commit failed'
[[ "$(jq -r '.active_roles | length' <<<"$created")" == 2 ]] || fail 'missing roles'
[[ "$(stat -c %a "$ENTRY_STATE_ROOT/trojanpanelnext-combined.json")" == 600 ]] || fail 'journal mode'
[[ "$(tr '\n' ' ' <"$trace")" == 'probe prepare activate verify ' ]] || fail 'stage order'
: >"$trace"
unchanged="$(entry_v2_reconcile "$tmp/spec" "$ENTRY_STATE_ROOT")"
[[ "$(jq -r '.result' <<<"$unchanged")" == unchanged && ! -s "$trace" ]] || fail 'idempotency'
make_spec '.revision = 1 | .roles.web.web_upstream = "127.0.0.1:9999"' "$tmp/conflict"
expect_fail entry_v2_reconcile "$tmp/conflict" "$ENTRY_STATE_ROOT"
[[ "$(jq -r '.code' <"$tmp/out")" == invalid_spec && ! -s "$trace" ]] || fail 'revision conflict had side effects'
make_spec '.revision = 2 | .active_roles = ["web"] | del(.roles.node, .certificate_targets.node)' "$tmp/web"
new_digest="$(entry_v2_target_digest "$tmp/web")"
jq --argjson spec "$(jq -c . "$tmp/web")" --arg digest "$new_digest" '
  .phase = "activating" | .health = "unknown" | .desired_revision = $spec.revision |
  .desired_digest = $digest | .candidate_target = {digest:$digest,spec:$spec} |
  .previous_resources = .resources |
  .candidate_resources = [.resources[] | select(.scope == "shared" or .role == "web") |
    .identity.digest = ("b" * 64)]
' "$ENTRY_STATE_ROOT/trojanpanelnext-combined.json" >"$tmp/crashed-journal"
chmod 0600 "$tmp/crashed-journal"
mv "$tmp/crashed-journal" "$ENTRY_STATE_ROOT/trojanpanelnext-combined.json"
: >"$trace"
recovery_observation="$(FAKE_CANDIDATE_DIGEST=b fake_observation "$tmp/web" "$(cat "$ENTRY_STATE_ROOT/trojanpanelnext-combined.json")" probe | jq '.candidate_resources += [(.candidate_resources[0] | .id = "/managed/unknown" | .identity.marker = "owned:trojanpanelnext-combined:unknown" | .identity.digest = ("c" * 64))]')"
if entry_v2_validate_observation "$recovery_observation" "$tmp/web" recovery "$(cat "$ENTRY_STATE_ROOT/trojanpanelnext-combined.json")"; then
  fail 'recovery accepted an unknown candidate resource'
fi
FAKE_UNKNOWN_CANDIDATE=1 FAKE_CANDIDATE_DIGEST=b expect_fail entry_v2_reconcile "$tmp/web" "$ENTRY_STATE_ROOT"
[[ "$(jq -r '.code' <"$tmp/out")" == ownership_conflict ]] || fail 'unknown recovery digest was accepted'
[[ "$(tr '\n' ' ' <"$trace")" == 'probe ' ]] || fail 'unknown recovery digest caused side effects'
: >"$trace"
FAKE_CANDIDATE_DIGEST=b FAKE_RESOURCE_DIGEST=a entry_v2_reconcile "$tmp/web" "$ENTRY_STATE_ROOT" >"$tmp/crash-out" || {
  cat "$tmp/crash-out" >&2
  jq -c '{phase,health,active_roles,resources,previous_resources,candidate_resources,committed_target,candidate_target,last_error}' "$ENTRY_STATE_ROOT/trojanpanelnext-combined.json" >&2
  fail 'crash recovery reconcile failed'
}
grep -Fqx 'rollback' "$trace" || fail 'crash recovery skipped rollback'
grep -Fqx 'rollback-spec-revision:1' "$trace" || fail 'crash rollback did not use committed spec'
grep -Fqx 'rollback-kind:committed-target' "$trace" || fail 'crash rollback kind was not committed-target'
[[ "$(jq -r '.active_roles | join(",")' <"$ENTRY_STATE_ROOT/trojanpanelnext-combined.json")" == web ]] || fail 'crash recovery did not commit role reduction'
bad_role_observation="$(fake_observation "$tmp/web" "$(cat "$ENTRY_STATE_ROOT/trojanpanelnext-combined.json")" verify | jq '.candidate_resources += [(.candidate_resources[] | select(.role == "web") | .role = "node" | .id = "/managed/node")] | .resources = .candidate_resources')"
if entry_v2_validate_observation "$bad_role_observation" "$tmp/web" verify; then
  fail 'verify accepted an inactive role resource'
fi
make_spec '.revision = 3 | .active_roles = ["web"] | del(.roles.node, .certificate_targets.node) | .roles.web.web_upstream = "127.0.0.1:9998"' "$tmp/drift"
FAKE_IDENTITY_DRIFT=1 expect_fail entry_v2_reconcile "$tmp/drift" "$ENTRY_STATE_ROOT"
[[ "$(jq -r '.code' <"$tmp/out")" == ownership_conflict ]] || fail 'resource identity drift was accepted'
make_spec '.revision = 4 | .active_roles = ["web"] | del(.roles.node, .certificate_targets.node) | .roles.web.web_upstream = "127.0.0.1:9999"' "$tmp/web-failure"
: >"$trace"
for fail_at in prepare activate verify; do
  : >"$trace"
  FAIL_AT="$fail_at" expect_fail entry_v2_reconcile "$tmp/web-failure" "$ENTRY_STATE_ROOT"
  [[ "$(jq -r '.active_roles | length' <"$ENTRY_STATE_ROOT/trojanpanelnext-combined.json")" == 1 ]] || fail "failed $fail_at changed committed roles"
  grep -Fqx rollback "$trace" || fail "failed $fail_at skipped rollback"
  [[ "$(jq -r '.resource' <"$tmp/out")" == trojanpanelnext-combined ]] || fail "failed $fail_at omitted resource"
  [[ "$(jq -r '.retryable' <"$tmp/out")" == true ]] || fail "failed $fail_at was not retryable"
  [[ "$(jq -r '.rollback_status' <"$tmp/out")" == succeeded ]] || fail "failed $fail_at reported wrong rollback status"
done
make_spec '.revision = 4 | .active_roles = ["web"] | del(.roles.node, .certificate_targets.node) | .roles.web.web_upstream = "127.0.0.1:10000"' "$tmp/failed-conflict"
: >"$trace"
expect_fail entry_v2_reconcile "$tmp/failed-conflict" "$ENTRY_STATE_ROOT"
[[ "$(jq -r '.code' <"$tmp/out")" == invalid_spec && ! -s "$trace" ]] || fail 'failed revision was reused for different target'
: >"$trace"
FAIL_AT=verify FAIL_ROLLBACK=1 expect_fail entry_v2_reconcile "$tmp/web-failure" "$ENTRY_STATE_ROOT"
[[ "$(jq -r '.phase' <"$ENTRY_STATE_ROOT/trojanpanelnext-combined.json")" == failed ]] || fail 'rollback failure was not durable'
[[ "$(jq -r '.rollback_status' <"$tmp/out")" == failed ]] || fail 'rollback failure status was not reported'
unset FAIL_AT
: >"$trace"
FAKE_PARTIAL=1 entry_v2_reconcile "$tmp/web-failure" "$ENTRY_STATE_ROOT" >/dev/null
[[ "$(head -n 2 "$trace" | tr '\n' ' ')" == 'probe rollback ' ]] || fail 'crash recovery did not precede prepare'
[[ "$(jq -r '.active_roles[0]' <"$ENTRY_STATE_ROOT/trojanpanelnext-combined.json")" == web ]] || fail 'node removal failed'
make_spec '.revision = 5' "$tmp/restore"
digest="$(jq -r '.committed_target.digest' <"$ENTRY_STATE_ROOT/trojanpanelnext-combined.json")"
jq --arg digest "$digest" '.restore_intent = {roles:["node"],expected_committed_digest:$digest}' "$tmp/restore" >"$tmp/explicit"
chmod 0600 "$tmp/explicit"
FAIL_AT=prepare expect_fail entry_v2_reconcile "$tmp/explicit" "$ENTRY_STATE_ROOT"
[[ "$(jq -r '.active_roles | join(",")' <"$ENTRY_STATE_ROOT/trojanpanelnext-combined.json")" == web ]] || fail 'failed restore changed active roles'
[[ "$(jq -r '.committed_target.spec.active_roles | join(",")' <"$ENTRY_STATE_ROOT/trojanpanelnext-combined.json")" == web ]] || fail 'failed restore changed committed roles'
make_spec '.revision = 6' "$tmp/implicit-restore"
: >"$trace"
expect_fail entry_v2_reconcile "$tmp/implicit-restore" "$ENTRY_STATE_ROOT"
[[ "$(jq -r '.code' <"$tmp/out")" == invalid_spec && ! -s "$trace" ]] || fail 'failed restore allowed implicit role restore'
unset FAIL_AT
entry_v2_reconcile "$tmp/explicit" "$ENTRY_STATE_ROOT" >/dev/null
[[ "$(jq -r '.active_roles | length' <"$ENTRY_STATE_ROOT/trojanpanelnext-combined.json")" == 2 ]] || fail 'explicit restoration failed'

# Second removal order: Web leaves first, then Node leaves through remove.
make_spec '.revision = 7 | .active_roles = ["node"] | del(.roles.web, .certificate_targets.web)' "$tmp/node"
entry_v2_reconcile "$tmp/node" "$ENTRY_STATE_ROOT" >/dev/null
[[ "$(jq -r '.active_roles[0]' <"$ENTRY_STATE_ROOT/trojanpanelnext-combined.json")" == node ]] || fail 'web removal failed'
FAIL_REMOVE=1 expect_fail entry_v2_remove "$tmp/node" "$ENTRY_STATE_ROOT" 0
[[ "$(jq -r '.phase' <"$ENTRY_STATE_ROOT/trojanpanelnext-combined.json")" == failed ]] || fail 'remove failure lost journal'
: >"$trace"
entry_v2_reconcile "$tmp/node" "$ENTRY_STATE_ROOT" >/dev/null
[[ "$(head -n 2 "$trace" | tr '\n' ' ')" == 'probe rollback ' ]] || fail 'remove crash recovery did not rollback'
entry_v2_remove "$tmp/node" "$ENTRY_STATE_ROOT" 0 >/dev/null
[[ ! -e "$ENTRY_STATE_ROOT/trojanpanelnext-combined.json" ]] || fail 'final removal did not clear journal'

# A modified committed snapshot cannot pass status validation.
snapshot="$tmp/first-failure/trojanpanelnext-combined.json"
cp "$snapshot" "$tmp/valid-state"
jq '.certificates.web.unexpected = true' "$tmp/valid-state" >"$snapshot"
chmod 0600 "$snapshot"
expect_fail main status --deployment trojanpanelnext-combined --state-root "$tmp/first-failure"
[[ "$(jq -r '.code' <"$tmp/out")" == state_not_found_or_invalid ]] || fail 'invalid certificate passed status'
jq '.capabilities = [42]' "$tmp/valid-state" >"$snapshot"
chmod 0600 "$snapshot"
expect_fail main status --deployment trojanpanelnext-combined --state-root "$tmp/first-failure"
[[ "$(jq -r '.code' <"$tmp/out")" == state_not_found_or_invalid ]] || fail 'non-string capability passed status'
jq '.committed_target.spec.unexpected = "field"' "$tmp/valid-state" >"$tmp/schema-tampered"
schema_digest="$(jq -S -c '.committed_target.spec | del(.revision, .restore_intent) | .active_roles |= sort' "$tmp/schema-tampered" | sha256sum | awk '{print $1}')"
jq --arg digest "$schema_digest" '.committed_target.digest = $digest | .desired_digest = $digest' "$tmp/schema-tampered" >"$tmp/schema-tampered-final"
chmod 0600 "$tmp/schema-tampered-final"
mv "$tmp/schema-tampered-final" "$snapshot"
expect_fail main status --deployment trojanpanelnext-combined --state-root "$tmp/first-failure"
[[ "$(jq -r '.code' <"$tmp/out")" == state_not_found_or_invalid ]] || fail 'schema-invalid committed spec passed status'
jq '.candidate_target = .committed_target | .phase = "preparing" | .health = "unknown"' "$tmp/valid-state" >"$tmp/candidate-tampered"
candidate_digest="$(jq -S -c '.candidate_target.spec | del(.revision, .restore_intent) | .active_roles |= sort' "$tmp/candidate-tampered" | sha256sum | awk '{print $1}')"
jq --arg digest "$candidate_digest" '.candidate_target.spec.unexpected = "field" | .candidate_target.digest = $digest | .desired_digest = $digest' "$tmp/candidate-tampered" >"$tmp/candidate-tampered-final"
chmod 0600 "$tmp/candidate-tampered-final"
mv "$tmp/candidate-tampered-final" "$snapshot"
expect_fail main status --deployment trojanpanelnext-combined --state-root "$tmp/first-failure"
[[ "$(jq -r '.code' <"$tmp/out")" == state_not_found_or_invalid ]] || fail 'schema-invalid candidate spec passed status'
jq '.committed_target.spec.roles.web.web_upstream = "127.0.0.1:9999"' "$snapshot" >"$tmp/tampered"
chmod 0600 "$tmp/tampered"
mv "$tmp/tampered" "$snapshot"
expect_fail main status --deployment trojanpanelnext-combined --state-root "$tmp/first-failure"
[[ "$(jq -r '.code' <"$tmp/out")" == state_not_found_or_invalid ]] || fail 'tampered snapshot passed status'

# v1 journal cannot be silently adopted, even with the same deployment ID.
jq '.deployment_id = "trojanpanelnext-combined"' "$repo/docs/entry-controller/examples/observed-state.json" >"$ENTRY_STATE_ROOT/trojanpanelnext-combined.json"
chmod 0600 "$ENTRY_STATE_ROOT/trojanpanelnext-combined.json"
: >"$trace"
expect_fail entry_v2_reconcile "$tmp/spec" "$ENTRY_STATE_ROOT"
[[ "$(jq -r '.code' <"$tmp/out")" == ownership_conflict && ! -s "$trace" ]] || fail 'v1 journal was adopted'

printf 'combined v2 contract: PASS\n'
