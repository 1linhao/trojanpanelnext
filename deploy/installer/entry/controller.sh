#!/usr/bin/env bash

# Pure EntryController planning and transition helpers. Host mutations stay in
# adapters; this file is intentionally sourceable by hermetic contract tests.
# shellcheck source=v2.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/v2.sh"

entry_normalize_provider() {
  local requested="${1:-}"
  local legacy_tls_mode="${2:-acme}"
  if [[ -z "${requested}" ]]; then
    if [[ "${legacy_tls_mode}" == "external" ]]; then
      requested="external"
    else
      requested="caddy-legacy"
    fi
  fi
  case "${requested}" in
  caddy | acme) requested="caddy-legacy" ;;
  nginx) requested="nginx-certbot" ;;
  esac
  case "${requested}" in
  caddy-legacy | nginx-certbot | external) printf '%s\n' "${requested}" ;;
  *) return 2 ;;
  esac
}

entry_provider_capabilities() {
  local provider="$1"
  case "${provider}" in
  caddy-legacy)
    printf '%s\n' \
      certificate.material certificate.issue certificate.renew \
      ingress.acme_http01 ingress.web_https ingress.plain_fallback \
      lifecycle.prepare lifecycle.rollback
    ;;
  nginx-certbot)
    printf '%s\n' \
      certificate.material certificate.issue certificate.renew certificate.notify \
      ingress.acme_http01 ingress.web_https ingress.plain_fallback \
      lifecycle.prepare lifecycle.rollback ownership.adopt
    ;;
  external)
    # External capabilities must come from its probe result; declaring none is
    # fail closed and prevents a contract-only driver from claiming reconcile.
    if [[ -n "${2:-}" ]]; then
      tr ',' '\n' <<<"$2" | sed '/^$/d'
    fi
    ;;
  *) return 2 ;;
  esac
}

entry_required_capabilities() {
  local purpose="$1"
  local fallback_required="${2:-0}"
  local provider="${3:-external}"
  printf '%s\n' certificate.material certificate.renew lifecycle.prepare lifecycle.rollback
  case "${provider}" in
  caddy-legacy | nginx-certbot)
    printf '%s\n' certificate.issue ingress.acme_http01
    ;;
  external) ;;
  *) return 2 ;;
  esac
  case "${purpose}" in
  web) printf '%s\n' ingress.web_https ;;
  node)
    printf '%s\n' certificate.notify
    if [[ "${fallback_required}" == "1" ]]; then
      printf '%s\n' ingress.plain_fallback
    fi
    ;;
  *) return 2 ;;
  esac
}

entry_missing_capabilities() {
  local required="$1"
  local available="$2"
  local capability
  while IFS= read -r capability; do
    [[ -z "${capability}" ]] && continue
    if ! grep -Fqx -- "${capability}" <<<"${available}"; then
      printf '%s\n' "${capability}"
    fi
  done <<<"${required}"
}

# Validate the stable, provider-neutral part of EntrySpec. Adapter-specific
# probes may add stricter checks, but no plan is produced for malformed input.
entry_validate_spec() {
  local spec="$1"
  command -v jq >/dev/null 2>&1 || return 10
  [[ -r "${spec}" ]] || return 11
  if [[ "$(jq -r '.schema_version // empty' "${spec}" 2>/dev/null)" == 2 ]]; then
    entry_v2_validate_spec "${spec}"
    return $?
  fi
  jq -e '
    .schema_version == 1 and
    (.revision | type == "number" and . >= 1 and floor == .) and
    (.deployment_id | type == "string" and test("^[a-z][a-z0-9-]{0,62}$")) and
    (.provider == "caddy-legacy" or .provider == "nginx-certbot" or .provider == "external") and
    (.purpose == "web" or .purpose == "node") and
    (.domain | type == "string" and test("^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$")) and
    (.certificate.managed_dir | type == "string" and startswith("/")) and
    (if .purpose == "web" then
       (.ingress.web_upstream | type == "string" and test("^(127\\.0\\.0\\.1|\\[::1\\]):[0-9]{1,5}$"))
     else
       .ingress.node_exposure == "direct" and
       (.ingress.route_manifest | type == "string" and startswith("/"))
     end) and
    (if .provider == "external" then
       .external_driver.protocol_version == 1 and
       (.external_driver.path | type == "string" and startswith("/")) and
       (.certificate.source_dir | type == "string" and startswith("/"))
     else true end)
  ' "${spec}" >/dev/null 2>&1
}

# Mutating commands require a regular, non-symlink spec owned by the trusted
# caller and inaccessible to group/other because deployment paths and driver
# selection are security-sensitive even though secrets should not be present.
entry_validate_mutating_spec_file() {
  local spec="$1"
  local expected_uid="${ENTRY_SPEC_OWNER_UID:-0}"
  [[ "${spec}" == /* && -f "${spec}" && ! -L "${spec}" ]] || return 2
  local owner mode mode_value
  owner="$(stat -c %u "${spec}")" || return 2
  mode="$(stat -c %a "${spec}")" || return 2
  [[ "${owner}" == "${expected_uid}" ]] || return 3
  mode_value=$((8#${mode}))
  (( (mode_value & 0077) == 0 )) || return 3
}

entry_validate_observed_state() {
  local state="$1"
  command -v jq >/dev/null 2>&1 || return 10
  [[ -r "${state}" ]] || return 11
  if [[ "$(jq -r '.schema_version // empty' "${state}" 2>/dev/null)" == 2 ]]; then
    entry_v2_validate_state "${state}" && entry_v2_verify_snapshot_file "${state}"
    return $?
  fi
  jq -e '
    .schema_version == 1 and
    (.deployment_id | type == "string" and test("^[a-z][a-z0-9-]{0,62}$")) and
    (.desired_revision | type == "number" and . >= 1 and floor == .) and
    (.observed_revision | type == "number" and . >= 0 and floor == .) and
    (.generation | type == "number" and . >= 0 and floor == .) and
    (.active_provider == "caddy-legacy" or .active_provider == "nginx-certbot" or .active_provider == "external") and
    (if .active_provider == "external" then
       (.driver.path | type == "string" and startswith("/")) and
       (.driver.sha256 | type == "string" and test("^[a-f0-9]{64}$"))
     else true end) and
    (.phase as $phase | (["stable", "preparing", "prepared", "deactivating", "activating", "verifying", "rolling_back", "failed"] | index($phase)) != null) and
    (.health as $health | (["healthy", "degraded", "unhealthy", "unknown"] | index($health)) != null) and
    (.capabilities | type == "array") and
    (.resources | type == "array") and
    (.listeners | type == "array")
  ' "${state}" >/dev/null 2>&1
}

entry_plan_spec() {
  local spec="$1"
  entry_validate_spec "${spec}" || return $?
  if [[ "$(jq -r '.schema_version' "${spec}")" == 2 ]]; then
    entry_v2_plan "${spec}" "${ENTRY_STATE_ROOT:-/tpdata/trojanpanelnext-entry/state}"
    return $?
  fi

  local provider purpose fallback_required available required missing actions
  provider="$(jq -r '.provider' "${spec}")"
  purpose="$(jq -r '.purpose' "${spec}")"
  fallback_required="$(jq -r 'if ((.ingress.fallbacks // []) | length) > 0 then 1 else 0 end' "${spec}")"
  required="$(entry_required_capabilities "${purpose}" "${fallback_required}" "${provider}")"
  available="$(entry_provider_capabilities "${provider}" "${ENTRY_EXTERNAL_CAPABILITIES:-}")"
  missing="$(entry_missing_capabilities "${required}" "${available}")"
  actions="$(entry_transition_plan "${ENTRY_CURRENT_PROVIDER:-}" "${provider}" "${ENTRY_CURRENT_PHASE:-stable}")"

  jq -n \
    --arg deployment_id "$(jq -r '.deployment_id' "${spec}")" \
    --argjson desired_revision "$(jq -r '.revision' "${spec}")" \
    --arg provider "${provider}" \
    --arg purpose "${purpose}" \
    --arg required "${required}" \
    --arg available "${available}" \
    --arg missing "${missing}" \
    --arg actions "${actions}" '
      {
        schema_version: 1,
        deployment_id: $deployment_id,
        desired_revision: $desired_revision,
        provider: $provider,
        purpose: $purpose,
        capabilities: {
          required: ($required | split("\n") | map(select(length > 0))),
          available: ($available | split("\n") | map(select(length > 0))),
          missing: ($missing | split("\n") | map(select(length > 0))),
          satisfied: (($missing | length) == 0)
        },
        actions: ($actions | split("\n") | map(select(length > 0))),
        mutation_enabled: ($provider == "external" and (($missing | length) == 0)),
        executable: ($provider == "external" and (($missing | length) == 0))
      }'
}

entry_state_path() {
  local root="$1"
  local deployment_id="$2"
  [[ "${root}" == /* ]] || return 2
  [[ "${deployment_id}" =~ ^[a-z][a-z0-9-]{0,62}$ ]] || return 2
  printf '%s/%s.json\n' "${root%/}" "${deployment_id}"
}

entry_write_state() {
  local root="$1"
  local source="$2"
  entry_validate_observed_state "${source}" || return $?
  local deployment_id target temporary
  deployment_id="$(jq -r '.deployment_id' "${source}")"
  target="$(entry_state_path "${root}" "${deployment_id}")" || return 2
  mkdir -p "${root}" || return 12
  chmod 0700 "${root}" || return 12
  temporary="$(mktemp "${root%/}/.state.XXXXXX")" || return 12
  if ! install -m 0600 "${source}" "${temporary}"; then
    rm -f "${temporary}"
    return 12
  fi
  if ! mv -f "${temporary}" "${target}"; then
    rm -f "${temporary}"
    return 12
  fi
  sync -f "${target}" 2>/dev/null || sync 2>/dev/null || true
  printf '%s\n' "${target}"
}

entry_read_state() {
  local root="$1"
  local deployment_id="$2"
  local path
  path="$(entry_state_path "${root}" "${deployment_id}")" || return 2
  entry_validate_observed_state "${path}" || return $?
  jq -c . "${path}"
}

# Print the ordered, side-effect-free action plan. Recovery is deliberately an
# action instead of an implicit reset: a caller must never overwrite a journal
# left after the old provider was deactivated.
entry_transition_plan() {
  local current_provider="${1:-}"
  local desired_provider="$2"
  local phase="${3:-stable}"

  if [[ "${phase}" != "stable" ]]; then
    case "${phase}" in
    preparing | prepared)
      printf '%s\n' abandon_prepared_candidate observe
      ;;
    deactivating | activating | verifying | rolling_back | failed)
      printf '%s\n' recover_previous_provider observe
      ;;
    *) return 2 ;;
    esac
  else
    printf '%s\n' observe
  fi

  if [[ -z "${current_provider}" ]]; then
    printf '%s\n' prepare_target activate_target verify_target commit
  elif [[ "${current_provider}" == "${desired_provider}" ]]; then
    printf '%s\n' reconcile_certificate reconcile_ingress verify_target commit
  else
    printf '%s\n' prepare_target snapshot_previous deactivate_previous activate_target verify_target commit retain_previous_data
  fi
}

# Validate one state-machine event and print the next phase.
entry_next_phase() {
  local phase="$1"
  local event="$2"
  case "${phase}:${event}" in
  stable:begin_prepare | failed:begin_prepare) printf 'preparing\n' ;;
  preparing:prepared) printf 'prepared\n' ;;
  prepared:begin_deactivate) printf 'deactivating\n' ;;
  deactivating:begin_activate) printf 'activating\n' ;;
  activating:begin_verify) printf 'verifying\n' ;;
  verifying:commit) printf 'stable\n' ;;
  preparing:fail | prepared:fail) printf 'stable\n' ;;
  deactivating:fail | activating:fail | verifying:fail) printf 'rolling_back\n' ;;
  rolling_back:rollback_succeeded) printf 'stable\n' ;;
  rolling_back:rollback_failed) printf 'failed\n' ;;
  *) return 2 ;;
  esac
}

# Resource records are owner|retention|kind|id. Only controller/provider-owned
# managed records are removable by default; external records are always
# delegated and preserve records require an explicit purge.
entry_removable_resources() {
  local purge="${1:-0}"
  local records="${2:-}"
  local owner retention kind id
  while IFS='|' read -r owner retention kind id; do
    [[ -z "${owner}" ]] && continue
    [[ "${owner}" == "external" ]] && continue
    [[ "${retention}" == "preserve" && "${purge}" != "1" ]] && continue
    printf '%s|%s|%s|%s\n' "${owner}" "${retention}" "${kind}" "${id}"
  done <<<"${records}"
}

entry_call_provider() {
  local layer="$1"
  local adapter="$2"
  local action="$3"
  local function_name="${adapter}_${layer}_${action}"
  [[ "${function_name}" =~ ^[a-zA-Z_][a-zA-Z0-9_]*$ ]] || return 2
  declare -F "${function_name}" >/dev/null || return 2
  "${function_name}"
}

# Execute the mutating half of a provider switch through injected adapters.
# It exists to test ordering and rollback without Docker, nginx or systemd.
entry_execute_switch() {
  local previous_adapter="$1"
  local target_adapter="$2"

  entry_call_provider certificate "${target_adapter}" prepare || return 20
  if ! entry_call_provider ingress "${target_adapter}" prepare; then
    entry_call_provider certificate "${target_adapter}" rollback || true
    return 20
  fi
  entry_call_provider ingress "${previous_adapter}" deactivate || return 21
  if ! entry_call_provider ingress "${target_adapter}" activate; then
    entry_call_provider ingress "${target_adapter}" rollback || true
    entry_call_provider certificate "${target_adapter}" rollback || true
    entry_call_provider ingress "${previous_adapter}" activate || return 31
    return 22
  fi
  if ! entry_call_provider certificate "${target_adapter}" verify ||
    ! entry_call_provider ingress "${target_adapter}" verify; then
    entry_call_provider ingress "${target_adapter}" rollback || true
    entry_call_provider certificate "${target_adapter}" rollback || true
    entry_call_provider ingress "${previous_adapter}" activate || return 31
    return 23
  fi
  entry_call_provider ingress "${target_adapter}" commit || return 24
  entry_call_provider certificate "${target_adapter}" commit || return 24
}
