#!/usr/bin/env bash
set -Eeuo pipefail

ENTRY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../entry" && pwd)"
REPO_DIR="$(cd "${ENTRY_DIR}/../../.." && pwd)"
source "${ENTRY_DIR}/controller.sh"
source "${ENTRY_DIR}/adapters/nginx_certbot.sh"
source "${ENTRY_DIR}/adapters/external.sh"
source "${REPO_DIR}/deploy/installer/tests/fixtures/fake_nginx_certbot_system.sh"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

assert_eq() {
  [[ "$1" == "$2" ]] || fail "got '$1', want '$2'"
}

assert_fails() {
  if "$@" >/dev/null 2>&1; then
    fail "command unexpectedly succeeded: $*"
  fi
}

# Legacy schema inputs preserve today's provider selection.
assert_eq "$(entry_normalize_provider '' acme)" caddy-legacy
assert_eq "$(entry_normalize_provider '' external)" external
assert_eq "$(entry_normalize_provider nginx acme)" nginx-certbot
assert_fails entry_normalize_provider unknown acme

# Purpose capabilities do not make Node depend on dynamic L4 forwarding.
node_required="$(entry_required_capabilities node 1 external)"
grep -Fqx certificate.material <<<"${node_required}"
grep -Fqx certificate.renew <<<"${node_required}"
grep -Fqx certificate.notify <<<"${node_required}"
grep -Fqx ingress.plain_fallback <<<"${node_required}"
if grep -Fq stream. <<<"${node_required}"; then
  fail "direct Node unexpectedly requires stream capabilities"
fi

external_available='certificate.material
lifecycle.prepare
lifecycle.rollback'
missing="$(entry_missing_capabilities "${node_required}" "${external_available}")"
grep -Fqx certificate.renew <<<"${missing}"
grep -Fqx certificate.notify <<<"${missing}"
grep -Fqx ingress.plain_fallback <<<"${missing}"

# Plans distinguish create, reconcile and provider migration, and preserve the
# recovery action when a previous process died after deactivation.
create_plan="$(entry_transition_plan '' nginx-certbot stable)"
grep -Fqx prepare_target <<<"${create_plan}"
same_plan="$(entry_transition_plan caddy-legacy caddy-legacy stable)"
grep -Fqx reconcile_certificate <<<"${same_plan}"
switch_plan="$(entry_transition_plan caddy-legacy external stable)"
grep -Fqx deactivate_previous <<<"${switch_plan}"
recovery_plan="$(entry_transition_plan caddy-legacy external activating)"
assert_eq "$(head -n 1 <<<"${recovery_plan}")" recover_previous_provider

# Invalid state transitions fail instead of silently resetting the journal.
phase=stable
phase="$(entry_next_phase "${phase}" begin_prepare)"
phase="$(entry_next_phase "${phase}" prepared)"
phase="$(entry_next_phase "${phase}" begin_deactivate)"
phase="$(entry_next_phase "${phase}" begin_activate)"
phase="$(entry_next_phase "${phase}" begin_verify)"
phase="$(entry_next_phase "${phase}" commit)"
assert_eq "${phase}" stable
assert_fails entry_next_phase stable commit
assert_eq "$(entry_next_phase verifying fail)" rolling_back
assert_eq "$(entry_next_phase rolling_back rollback_failed)" failed

# Remove filters ownership and retention before any Adapter sees resources.
resources='controller|managed|file|/managed.conf
provider|preserve|certificate|example.com
external|managed|file|/etc/nginx/external.conf'
default_remove="$(entry_removable_resources 0 "${resources}")"
assert_eq "${default_remove}" 'controller|managed|file|/managed.conf'
purge_remove="$(entry_removable_resources 1 "${resources}")"
grep -Fqx 'provider|preserve|certificate|example.com' <<<"${purge_remove}"
if grep -Fq '/etc/nginx/external.conf' <<<"${purge_remove}"; then
  fail "external resource crossed the ownership seam"
fi

# Fake certificate and ingress providers prove the split seam, ordering and
# rollback without host mutations.
TRACE=''
trace() { TRACE+="$1 "; }
old_ingress_deactivate() { trace old.ingress.deactivate; }
old_ingress_activate() { trace old.ingress.activate; }
new_certificate_prepare() { trace new.certificate.prepare; }
new_certificate_verify() { trace new.certificate.verify; }
new_certificate_commit() { trace new.certificate.commit; }
new_certificate_rollback() { trace new.certificate.rollback; }
new_ingress_prepare() { trace new.ingress.prepare; }
new_ingress_activate() { trace new.ingress.activate; [[ "${FAIL_AT:-}" != activate ]]; }
new_ingress_verify() { trace new.ingress.verify; [[ "${FAIL_AT:-}" != verify ]]; }
new_ingress_commit() { trace new.ingress.commit; }
new_ingress_rollback() { trace new.ingress.rollback; }

entry_execute_switch old new
assert_eq "${TRACE}" 'new.certificate.prepare new.ingress.prepare old.ingress.deactivate new.ingress.activate new.certificate.verify new.ingress.verify new.ingress.commit new.certificate.commit '
TRACE=''
FAIL_AT=verify
assert_fails entry_execute_switch old new
assert_eq "${TRACE}" 'new.certificate.prepare new.ingress.prepare old.ingress.deactivate new.ingress.activate new.certificate.verify new.ingress.verify new.ingress.rollback new.certificate.rollback old.ingress.activate '

# Schema fixtures are valid JSON and the pure plan exposes no stream dependency
# for Node direct mode.
for schema in "${REPO_DIR}"/docs/entry-controller/schema/*.json; do
  jq empty "${schema}"
done
for fixture in "${REPO_DIR}"/docs/entry-controller/examples/*.json; do
  jq empty "${fixture}"
done
entry_validate_spec "${REPO_DIR}/docs/entry-controller/examples/web-caddy.json"
entry_validate_spec "${REPO_DIR}/docs/entry-controller/examples/node-external.json"
node_spec="${REPO_DIR}/docs/entry-controller/examples/node-nginx-certbot.json"
entry_validate_spec "${node_spec}"
plan="$("${ENTRY_DIR}/entryctl.sh" plan --spec "${node_spec}")"
assert_eq "$(jq -r '.provider' <<<"${plan}")" nginx-certbot
assert_eq "$(jq -r '.capabilities.satisfied' <<<"${plan}")" true
assert_eq "$(jq -r '.mutation_enabled' <<<"${plan}")" false
assert_eq "$(jq -r '.executable' <<<"${plan}")" false
assert_eq "$(jq '[.capabilities.required[] | select(startswith("stream."))] | length' <<<"${plan}")" 0
external_plan="$("${ENTRY_DIR}/entryctl.sh" plan --spec "${REPO_DIR}/docs/entry-controller/examples/node-external.json")"
assert_eq "$(jq -r '.executable' <<<"${external_plan}")" false
grep -Fqx certificate.notify < <(jq -r '.capabilities.missing[]' <<<"${external_plan}")
assert_fails entry_validate_spec "${REPO_DIR}/docs/entry-controller/examples/observed-state.json"
entry_validate_observed_state "${REPO_DIR}/docs/entry-controller/examples/observed-state.json"

# ObservedState is an atomic 0600 journal and powers the read-only status
# command without any provider or host interaction.
state_root="$(mktemp -d)"
state_path="$(entry_write_state "${state_root}" "${REPO_DIR}/docs/entry-controller/examples/observed-state.json")"
assert_eq "$(stat -c %a "${state_root}")" 700
assert_eq "$(stat -c %a "${state_path}")" 600
assert_eq "$(jq -r '.phase' <<<"$(entry_read_state "${state_root}" trojanpanelnext-node)")" stable
status_json="$("${ENTRY_DIR}/entryctl.sh" status --deployment trojanpanelnext-node --state-root "${state_root}")"
assert_eq "$(jq -r '.health' <<<"${status_json}")" healthy
assert_fails "${ENTRY_DIR}/entryctl.sh" status --deployment missing --state-root "${state_root}"
rm -rf "${state_root}"

# nginx-certbot rendering is side-effect free. Node output contains ACME and
# required plain fallback only, never stream forwarding or TLS termination.
web_candidate="$(nginx_certbot_render_web panel.example.com 127.0.0.1:8888 /var/lib/tpn-acme /etc/letsencrypt/live/panel/fullchain.pem /etc/letsencrypt/live/panel/privkey.pem)"
grep -Fq 'listen 443 ssl;' <<<"${web_candidate}"
grep -Fq 'proxy_pass http://127.0.0.1:8888;' <<<"${web_candidate}"
node_candidate="$(nginx_certbot_render_node node.example.com /var/lib/tpn-acme '127.0.0.1:8443|/tpdata/web')"
grep -Fq 'listen 80;' <<<"${node_candidate}"
grep -Fq 'listen 127.0.0.1:8443;' <<<"${node_candidate}"
if grep -Eq 'stream|listen 443|ssl_certificate' <<<"${node_candidate}"; then
  fail "Node direct candidate unexpectedly configures L4 or TLS ingress"
fi
assert_fails nginx_certbot_render_node 'bad;domain' /var/lib/tpn-acme ''
assert_fails nginx_certbot_render_node node.example.com /var/lib/tpn-acme '127.0.0.1:65536|/tpdata/web'

# nginx-certbot remains a pure local candidate plan: exact argv vectors are
# data, executable=false prevents accidental host use, and a temp-only fake
# proves activation rollback preserves the previous active config.
nginx_plan_a="$(nginx_certbot_plan_spec "${node_spec}")"
nginx_plan_b="$(nginx_certbot_plan_spec "${node_spec}")"
assert_eq "${nginx_plan_a}" "${nginx_plan_b}"
nginx_certbot_validate_plan "${nginx_plan_a}"
assert_eq "$(jq -r '.executable' <<<"${nginx_plan_a}")" false
assert_eq "$(jq -r '.commands.issue[0]' <<<"${nginx_plan_a}")" certbot
if jq -r '.candidate.content' <<<"${nginx_plan_a}" | grep -Eq 'stream|listen 443|ssl_certificate'; then
  fail "Node nginx-certbot plan unexpectedly owns stream or TLS ingress"
fi
web_nginx_spec="$(mktemp)"
jq '.provider = "nginx-certbot" | .certificate.email = "admin@example.com"' \
  "${REPO_DIR}/docs/entry-controller/examples/web-caddy.json" >"${web_nginx_spec}"
web_nginx_plan="$(nginx_certbot_plan_spec "${web_nginx_spec}")"
nginx_certbot_validate_plan "${web_nginx_plan}"
grep -Fq 'listen 443 ssl;' <<<"$(jq -r '.candidate.content' <<<"${web_nginx_plan}")"
fake_nginx_root="$(mktemp -d)"
printf '%s\n' legacy-active >"${fake_nginx_root}/active.conf"
NGINX_CERTBOT_FAKE_FAIL_AT=activate assert_fails nginx_certbot_fake_reconcile "${nginx_plan_a}" "${fake_nginx_root}"
assert_eq "$(cat "${fake_nginx_root}/active.conf")" legacy-active
fake_nginx_state="$(nginx_certbot_fake_reconcile "${nginx_plan_a}" "${fake_nginx_root}")"
assert_eq "$(jq -r '.phase' <<<"${fake_nginx_state}")" stable
assert_eq "$(jq -r '.health' <<<"${fake_nginx_state}")" healthy
rm -rf "${fake_nginx_root}"
rm -f "${web_nginx_spec}"

# External resources stay behind an executable protocol. The client rejects
# unsafe ownership/mode and validates structured output from a local fake.
fake_driver="$(mktemp)"
cp "${REPO_DIR}/deploy/installer/tests/fixtures/fake_external_driver.sh" "${fake_driver}"
chmod 0755 "${fake_driver}"
ENTRY_DRIVER_OWNER_UID="$(id -u)"
external_probe="$(external_driver_call "${fake_driver}" probe "${REPO_DIR}/docs/entry-controller/examples/node-external.json")"
assert_eq "$(jq -r '.provider' <<<"${external_probe}")" external
assert_fails env ENTRY_DRIVER_OWNER_UID="$(id -u)" EXTERNAL_DRIVER_ACTION_OVERRIDE=verify \
  bash -c 'source "$1"; source "$2"; external_driver_call "$3" probe "$4"' _ \
  "${ENTRY_DIR}/controller.sh" "${ENTRY_DIR}/adapters/external.sh" "${fake_driver}" \
  "${REPO_DIR}/docs/entry-controller/examples/node-external.json"
assert_fails env ENTRY_DRIVER_OWNER_UID="$(id -u)" EXTERNAL_DRIVER_DEPLOYMENT_OVERRIDE=wrong-deployment \
  bash -c 'source "$1"; source "$2"; external_driver_call "$3" probe "$4"' _ \
  "${ENTRY_DIR}/controller.sh" "${ENTRY_DIR}/adapters/external.sh" "${fake_driver}" \
  "${REPO_DIR}/docs/entry-controller/examples/node-external.json"
assert_fails env ENTRY_DRIVER_OWNER_UID="$(id -u)" EXTERNAL_DRIVER_STATUS_OVERRIDE=healthy \
  bash -c 'source "$1"; source "$2"; external_driver_call "$3" probe "$4"' _ \
  "${ENTRY_DIR}/controller.sh" "${ENTRY_DIR}/adapters/external.sh" "${fake_driver}" \
  "${REPO_DIR}/docs/entry-controller/examples/node-external.json"
assert_fails env ENTRY_DRIVER_OWNER_UID="$(id -u)" EXTERNAL_DRIVER_RESOURCES_JSON='[{"bad":"shape"}]' \
  bash -c 'source "$1"; source "$2"; external_driver_call "$3" probe "$4"' _ \
  "${ENTRY_DIR}/controller.sh" "${ENTRY_DIR}/adapters/external.sh" "${fake_driver}" \
  "${REPO_DIR}/docs/entry-controller/examples/node-external.json"
fake_spec="$(mktemp)"
jq --arg path "${fake_driver}" '.external_driver.path = $path' \
  "${REPO_DIR}/docs/entry-controller/examples/node-external.json" >"${fake_spec}"
chmod 0600 "${fake_spec}"
fake_plan="$(ENTRY_DRIVER_OWNER_UID="$(id -u)" "${ENTRY_DIR}/entryctl.sh" plan --spec "${fake_spec}")"
assert_eq "$(jq -r '.executable' <<<"${fake_plan}")" true

# The external adapter now drives a journaled reconcile. Reconcile observes,
# negotiates, prepares, activates and verifies in order; remove delegates
# provider resources before deleting only the controller journal.
external_state_root="$(mktemp -d)"
external_trace="$(mktemp)"
reconciled="$(ENTRY_DRIVER_OWNER_UID="$(id -u)" ENTRY_SPEC_OWNER_UID="$(id -u)" EXTERNAL_DRIVER_TRACE="${external_trace}" \
  "${ENTRY_DIR}/entryctl.sh" reconcile --spec "${fake_spec}" --state-root "${external_state_root}")"
assert_eq "$(jq -r '.phase' <<<"${reconciled}")" stable
assert_eq "$(jq -r '.health' <<<"${reconciled}")" healthy
assert_eq "$(jq -r '.observed_revision' <<<"${reconciled}")" 1
assert_eq "$(jq -r '.generation' <<<"${reconciled}")" 1
assert_eq "$(tr '\n' ' ' <"${external_trace}")" 'probe prepare activate verify '
status_json="$("${ENTRY_DIR}/entryctl.sh" status --deployment trojanpanelnext-node --state-root "${external_state_root}")"
assert_eq "$(jq -r '.active_provider' <<<"${status_json}")" external
assert_eq "$(jq -r '.driver.path' <<<"${status_json}")" "${fake_driver}"
assert_eq "$(jq -r '.driver.sha256' <<<"${status_json}")" "$(sha256sum "${fake_driver}" | awk '{print $1}')"
chmod 0644 "${fake_spec}"
assert_fails env ENTRY_DRIVER_OWNER_UID="$(id -u)" ENTRY_SPEC_OWNER_UID="$(id -u)" \
  "${ENTRY_DIR}/entryctl.sh" reconcile --spec "${fake_spec}" --state-root "${external_state_root}"
chmod 0600 "${fake_spec}"
fake_spec_link="$(mktemp -u)"
ln -s "${fake_spec}" "${fake_spec_link}"
assert_fails env ENTRY_DRIVER_OWNER_UID="$(id -u)" ENTRY_SPEC_OWNER_UID="$(id -u)" \
  "${ENTRY_DIR}/entryctl.sh" reconcile --spec "${fake_spec_link}" --state-root "${external_state_root}"
rm -f "${fake_spec_link}"

: >"${external_trace}"
assert_fails env ENTRY_DRIVER_OWNER_UID="$(id -u)" ENTRY_SPEC_OWNER_UID="$(id -u)" EXTERNAL_DRIVER_TRACE="${external_trace}" \
  EXTERNAL_DRIVER_FAIL_AT=verify "${ENTRY_DIR}/entryctl.sh" reconcile --spec "${fake_spec}" --state-root "${external_state_root}"
assert_eq "$(tr '\n' ' ' <"${external_trace}")" 'probe prepare activate verify rollback '
failed_status="$("${ENTRY_DIR}/entryctl.sh" status --deployment trojanpanelnext-node --state-root "${external_state_root}")"
assert_eq "$(jq -r '.phase' <<<"${failed_status}")" stable
assert_eq "$(jq -r '.health' <<<"${failed_status}")" degraded
assert_eq "$(jq -r '.last_error.rollback_status' <<<"${failed_status}")" succeeded

# Driver-defined error codes survive the controller boundary. A prepare-time
# ownership conflict needs no rollback and leaves the last stable deployment
# journaled as degraded.
: >"${external_trace}"
prepare_error="$(ENTRY_DRIVER_OWNER_UID="$(id -u)" ENTRY_SPEC_OWNER_UID="$(id -u)" EXTERNAL_DRIVER_TRACE="${external_trace}" \
  EXTERNAL_DRIVER_FAIL_AT=prepare EXTERNAL_DRIVER_EXIT_CODE=66 \
  "${ENTRY_DIR}/entryctl.sh" reconcile --spec "${fake_spec}" --state-root "${external_state_root}" || true)"
assert_eq "$(jq -r '.code' <<<"${prepare_error}")" ownership_conflict
assert_eq "$(tr '\n' ' ' <"${external_trace}")" 'probe prepare '
assert_eq "$(jq -r '.phase' <"${external_state_root}/trojanpanelnext-node.json")" stable

# Revision monotonicity considers both committed and in-flight desired
# revisions. A stale spec is rejected before the driver is called.
fake_spec_v2="$(mktemp)"
jq '.revision = 2' "${fake_spec}" >"${fake_spec_v2}"
chmod 0600 "${fake_spec_v2}"
: >"${external_trace}"
ENTRY_DRIVER_OWNER_UID="$(id -u)" ENTRY_SPEC_OWNER_UID="$(id -u)" EXTERNAL_DRIVER_TRACE="${external_trace}" \
  "${ENTRY_DIR}/entryctl.sh" reconcile --spec "${fake_spec_v2}" --state-root "${external_state_root}" >/dev/null
: >"${external_trace}"
assert_fails env ENTRY_DRIVER_OWNER_UID="$(id -u)" ENTRY_SPEC_OWNER_UID="$(id -u)" EXTERNAL_DRIVER_TRACE="${external_trace}" \
  "${ENTRY_DIR}/entryctl.sh" reconcile --spec "${fake_spec}" --state-root "${external_state_root}"
assert_eq "$(wc -c <"${external_trace}" | tr -d ' ')" 0

# A crash journal is rolled back with the exact recorded executable before a
# retry. Rollback failure is durable; a later retry resumes safely.
journal="${external_state_root}/trojanpanelnext-node.json"
journal_tmp="$(mktemp)"
jq '.phase = "activating" | .health = "unknown"' "${journal}" >"${journal_tmp}"
chmod 0600 "${journal_tmp}"
mv "${journal_tmp}" "${journal}"
: >"${external_trace}"
ENTRY_DRIVER_OWNER_UID="$(id -u)" ENTRY_SPEC_OWNER_UID="$(id -u)" EXTERNAL_DRIVER_TRACE="${external_trace}" \
  "${ENTRY_DIR}/entryctl.sh" reconcile --spec "${fake_spec_v2}" --state-root "${external_state_root}" >/dev/null
assert_eq "$(tr '\n' ' ' <"${external_trace}")" 'probe rollback prepare activate verify '

journal_tmp="$(mktemp)"
jq '.phase = "activating" | .health = "unknown"' "${journal}" >"${journal_tmp}"
chmod 0600 "${journal_tmp}"
mv "${journal_tmp}" "${journal}"
: >"${external_trace}"
assert_fails env ENTRY_DRIVER_OWNER_UID="$(id -u)" ENTRY_SPEC_OWNER_UID="$(id -u)" EXTERNAL_DRIVER_TRACE="${external_trace}" \
  EXTERNAL_DRIVER_FAIL_AT=rollback "${ENTRY_DIR}/entryctl.sh" reconcile --spec "${fake_spec_v2}" --state-root "${external_state_root}"
assert_eq "$(tr '\n' ' ' <"${external_trace}")" 'probe rollback '
assert_eq "$(jq -r '.phase' <"${journal}")" failed
assert_eq "$(jq -r '.health' <"${journal}")" unhealthy
: >"${external_trace}"
ENTRY_DRIVER_OWNER_UID="$(id -u)" ENTRY_SPEC_OWNER_UID="$(id -u)" EXTERNAL_DRIVER_TRACE="${external_trace}" \
  "${ENTRY_DIR}/entryctl.sh" reconcile --spec "${fake_spec_v2}" --state-root "${external_state_root}" >/dev/null
assert_eq "$(tr '\n' ' ' <"${external_trace}")" 'probe rollback prepare activate verify '

# An unfinished journal cannot be inherited by changed executable contents,
# even at the same path. The refusal happens before probe.
journal_tmp="$(mktemp)"
jq '.phase = "activating" | .health = "unknown"' "${journal}" >"${journal_tmp}"
chmod 0600 "${journal_tmp}"
mv "${journal_tmp}" "${journal}"
driver_backup="$(mktemp)"
cp "${fake_driver}" "${driver_backup}"
printf '\n' >>"${fake_driver}"
: >"${external_trace}"
assert_fails env ENTRY_DRIVER_OWNER_UID="$(id -u)" ENTRY_SPEC_OWNER_UID="$(id -u)" EXTERNAL_DRIVER_TRACE="${external_trace}" \
  "${ENTRY_DIR}/entryctl.sh" reconcile --spec "${fake_spec_v2}" --state-root "${external_state_root}"
assert_eq "$(wc -c <"${external_trace}" | tr -d ' ')" 0
cp "${driver_backup}" "${fake_driver}"
chmod 0755 "${fake_driver}"
rm -f "${driver_backup}"
: >"${external_trace}"
ENTRY_DRIVER_OWNER_UID="$(id -u)" ENTRY_SPEC_OWNER_UID="$(id -u)" EXTERNAL_DRIVER_TRACE="${external_trace}" \
  "${ENTRY_DIR}/entryctl.sh" reconcile --spec "${fake_spec_v2}" --state-root "${external_state_root}" >/dev/null

# Removal is bound to the driver identity that owns the stable journal.
driver_backup="$(mktemp)"
cp "${fake_driver}" "${driver_backup}"
printf '\n' >>"${fake_driver}"
: >"${external_trace}"
assert_fails env ENTRY_DRIVER_OWNER_UID="$(id -u)" ENTRY_SPEC_OWNER_UID="$(id -u)" EXTERNAL_DRIVER_TRACE="${external_trace}" \
  "${ENTRY_DIR}/entryctl.sh" remove --spec "${fake_spec_v2}" --state-root "${external_state_root}"
assert_eq "$(wc -c <"${external_trace}" | tr -d ' ')" 0
cp "${driver_backup}" "${fake_driver}"
chmod 0755 "${fake_driver}"
rm -f "${driver_backup}"

: >"${external_trace}"
ENTRY_DRIVER_OWNER_UID="$(id -u)" ENTRY_SPEC_OWNER_UID="$(id -u)" EXTERNAL_DRIVER_TRACE="${external_trace}" \
  "${ENTRY_DIR}/entryctl.sh" remove --spec "${fake_spec_v2}" --state-root "${external_state_root}" >/dev/null
assert_eq "$(cat "${external_trace}")" remove
assert_fails "${ENTRY_DIR}/entryctl.sh" status --deployment trojanpanelnext-node --state-root "${external_state_root}"
# Remove is retry-safe even when the controller journal is already absent.
: >"${external_trace}"
ENTRY_DRIVER_OWNER_UID="$(id -u)" ENTRY_SPEC_OWNER_UID="$(id -u)" EXTERNAL_DRIVER_TRACE="${external_trace}" \
  "${ENTRY_DIR}/entryctl.sh" remove --spec "${fake_spec_v2}" --state-root "${external_state_root}" >/dev/null
assert_eq "$(cat "${external_trace}")" remove
rm -rf "${external_state_root}"
rm -f "${external_trace}"

fake_driver_link="$(mktemp -u)"
ln -s "${fake_driver}" "${fake_driver_link}"
assert_fails external_driver_validate "${fake_driver_link}"
rm -f "${fake_driver_link}"
chmod 0775 "${fake_driver}"
assert_fails external_driver_validate "${fake_driver}"
rm -f "${fake_driver}" "${fake_spec}" "${fake_spec_v2}"

"${ENTRY_DIR}/entryctl.sh" --help | grep -Fq 'entryctl.sh plan'
disabled_spec="$(mktemp)"
cp "${REPO_DIR}/docs/entry-controller/examples/node-nginx-certbot.json" "${disabled_spec}"
chmod 0600 "${disabled_spec}"
if output="$(ENTRY_SPEC_OWNER_UID="$(id -u)" ${ENTRY_DIR}/entryctl.sh reconcile --spec "${disabled_spec}" 2>/dev/null)"; then
  fail "disabled nginx-certbot mutating Adapter unexpectedly succeeded"
fi
grep -Fq 'unsupported_capability' <<<"${output}"
rm -f "${disabled_spec}"
assert_fails "${ENTRY_DIR}/entryctl.sh" unknown

printf 'PASS EntryController pure contract and fake Adapter tests\n'
