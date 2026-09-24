#!/usr/bin/env bash
set -Eeuo pipefail

ENTRY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=controller.sh
source "${ENTRY_DIR}/controller.sh"
# shellcheck source=adapters/caddy.sh
source "${ENTRY_DIR}/adapters/caddy.sh"
# shellcheck source=adapters/external.sh
source "${ENTRY_DIR}/adapters/external.sh"

usage() {
  cat <<'EOF'
Usage:
  entryctl.sh plan      --spec <file>
  entryctl.sh reconcile --spec <file>
  entryctl.sh status    --deployment <id> [--state-root <directory>]
  entryctl.sh remove    --spec <file> [--purge]

The versioned EntrySpec and ObservedState contracts are documented under
docs/entry-controller/schema/. Caddy v2 uses the journaled combined Adapter;
the external adapter is executable when its root-owned driver passes capability
negotiation.
EOF
}

entryctl_error() {
  local code="$1"
  local phase="$2"
  local resource="$3"
  local retryable="$4"
  local rollback_status="$5"
  local message="$6"
  jq -cn --arg code "${code}" --arg phase "${phase}" --arg resource "${resource}" \
    --argjson retryable "${retryable}" --arg rollback_status "${rollback_status}" \
    --arg message "${message}" \
    '{schema_version:1,code:$code,phase:$phase,resource:$resource,retryable:$retryable,rollback_status:$rollback_status,message:$message}'
}

entryctl_state_json() {
  local spec="$1"
  local phase="$2"
  local health="$3"
  local observed_revision="$4"
  local generation="$5"
  local observation="$6"
  local previous_provider="${7:-}"
  local error_json="${8:-null}"
  local driver_path driver_sha256
  driver_path="$(jq -r '.external_driver.path // ""' "${spec}")"
  driver_sha256="$(external_driver_sha256 "${driver_path}")"
  jq -cn \
    --argjson spec "$(jq -c . "${spec}")" \
    --arg phase "${phase}" \
    --arg health "${health}" \
    --argjson observed_revision "${observed_revision}" \
    --argjson generation "${generation}" \
    --argjson observation "${observation}" \
    --arg previous_provider "${previous_provider}" \
    --arg driver_path "${driver_path}" \
    --arg driver_sha256 "${driver_sha256}" \
    --argjson last_error "${error_json}" '
      {
        schema_version: 1,
        deployment_id: $spec.deployment_id,
        desired_revision: $spec.revision,
        observed_revision: $observed_revision,
        generation: $generation,
        active_provider: $spec.provider,
        driver: {path: $driver_path, sha256: $driver_sha256},
        phase: $phase,
        health: $health,
        capabilities: ($observation.capabilities // [] | unique),
        resources: ($observation.resources // []),
        listeners: ($observation.listeners // [])
      }
      + (if ($previous_provider | length) > 0 and $previous_provider != $spec.provider
         then {previous_provider: $previous_provider} else {} end)
      + (if ($observation.certificate // null) != null
         then {certificate: $observation.certificate} else {} end)
      + (if $last_error != null then {last_error: $last_error} else {} end)'
}

entryctl_persist_state() {
  local state_root="$1"
  local state_json="$2"
  local temporary
  temporary="$(mktemp)"
  chmod 0600 "${temporary}"
  printf '%s\n' "${state_json}" >"${temporary}"
  if ! entry_write_state "${state_root}" "${temporary}" >/dev/null; then
    rm -f "${temporary}"
    return 1
  fi
  rm -f "${temporary}"
}

entryctl_observation_from_state() {
  jq -c '{capabilities,resources,listeners} + (if has("certificate") then {certificate} else {} end)' <<<"$1"
}

entryctl_driver_error_code() {
  case "$1" in
  64) printf 'invalid_spec\n' ;;
  65) printf 'unsupported_capability\n' ;;
  66) printf 'ownership_conflict\n' ;;
  67) printf 'dependency_missing\n' ;;
  69) printf 'verification_failed\n' ;;
  70) printf 'rollback_failed\n' ;;
  *) printf 'driver_failed\n' ;;
  esac
}

entryctl_external_reconcile() {
  local spec="$1"
  local state_root="$2"
  entry_validate_mutating_spec_file "${spec}" && entry_validate_spec "${spec}" || {
    entryctl_error invalid_spec preparing "${spec}" false not-needed "EntrySpec validation failed"
    return 2
  }
  [[ "$(jq -r '.provider' "${spec}")" == external ]] || {
    entryctl_error unsupported_capability preparing "$(jq -r '.provider' "${spec}")" false not-needed "Only the external mutating adapter is enabled"
    return 3
  }

  local driver deployment desired_revision existing="" previous_provider="" observed_revision=0 generation=0
  local probe required available missing purpose fallback_required observation state error rollback_status
  local call_status=0 code retryable=true current_driver_sha existing_driver_path existing_driver_sha existing_phase=stable known_desired=0
  driver="$(jq -r '.external_driver.path' "${spec}")"
  deployment="$(jq -r '.deployment_id' "${spec}")"
  desired_revision="$(jq -r '.revision' "${spec}")"
  if ! current_driver_sha="$(external_driver_sha256 "${driver}")"; then
    entryctl_error ownership_conflict preparing "${driver}" false not-needed "External driver ownership, mode or digest validation failed"
    return 4
  fi
  if existing="$(entry_read_state "${state_root}" "${deployment}" 2>/dev/null)"; then
    previous_provider="$(jq -r '.active_provider' <<<"${existing}")"
    observed_revision="$(jq -r '.observed_revision' <<<"${existing}")"
    known_desired="$(jq -r '.desired_revision' <<<"${existing}")"
    generation="$(jq -r '.generation' <<<"${existing}")"
    existing_phase="$(jq -r '.phase' <<<"${existing}")"
    if ((desired_revision < observed_revision || desired_revision < known_desired)); then
      entryctl_error invalid_spec "${existing_phase}" "${spec}" false not-needed "Spec revision is older than the journaled desired or observed revision"
      return 2
    fi
    if [[ "${previous_provider}" != external ]]; then
      entryctl_error unsupported_capability preparing "${previous_provider}" false not-needed "Cross-provider mutation is not enabled"
      return 3
    fi
    if [[ "${existing_phase}" != stable ]]; then
      existing_driver_path="$(jq -r '.driver.path' <<<"${existing}")"
      existing_driver_sha="$(jq -r '.driver.sha256' <<<"${existing}")"
      if [[ "${existing_driver_path}" != "${driver}" || "${existing_driver_sha}" != "${current_driver_sha}" ]]; then
        entryctl_error ownership_conflict "${existing_phase}" "${driver}" false pending "Unfinished journal belongs to a different external driver identity"
        return 4
      fi
    fi
  fi

  probe="$(external_driver_call "${driver}" probe "${spec}")" || call_status=$?
  if ((call_status != 0)); then
    code="$(entryctl_driver_error_code "${call_status}")"
    case "${code}" in invalid_spec | unsupported_capability | ownership_conflict | dependency_missing) retryable=false ;; esac
    entryctl_error "${code}" preparing "${driver}" "${retryable}" not-needed "External driver probe failed"
    return 4
  fi
  purpose="$(jq -r '.purpose' "${spec}")"
  fallback_required="$(jq -r 'if ((.ingress.fallbacks // []) | length) > 0 then 1 else 0 end' "${spec}")"
  required="$(entry_required_capabilities "${purpose}" "${fallback_required}" external)"
  available="$(jq -r '.capabilities[]' <<<"${probe}")"
  missing="$(entry_missing_capabilities "${required}" "${available}")"
  if [[ -n "${missing}" ]]; then
    entryctl_error unsupported_capability preparing "${driver}" false not-needed "External driver is missing: $(tr '\n' ',' <<<"${missing}" | sed 's/,$//')"
    return 3
  fi

  # A non-stable journal means the previous process may have staged or
  # activated resources. Recovery uses exactly the recorded driver identity;
  # a replaced executable cannot inherit an unfinished rollback journal.
  if [[ -n "${existing}" && "${existing_phase}" != stable ]]; then
    observation="$(entryctl_observation_from_state "${existing}")"
    state="$(entryctl_state_json "${spec}" rolling_back unknown "${observed_revision}" "${generation}" "${observation}" "${previous_provider}")"
    entryctl_persist_state "${state_root}" "${state}"
    call_status=0
    external_driver_call "${driver}" rollback "${spec}" >/dev/null || call_status=$?
    if ((call_status != 0)); then
      error='{"code":"rollback_failed","phase":"rolling_back","retryable":true,"rollback_status":"failed","message":"Crash recovery rollback failed"}'
      state="$(entryctl_state_json "${spec}" failed unhealthy "${observed_revision}" "${generation}" "${observation}" "${previous_provider}" "${error}")"
      entryctl_persist_state "${state_root}" "${state}"
      entryctl_error rollback_failed rolling_back "${driver}" true failed "Crash recovery rollback failed"
      return 8
    fi
  fi

  observation="${probe}"
  state="$(entryctl_state_json "${spec}" preparing unknown "${observed_revision}" "${generation}" "${observation}" "${previous_provider}")"
  entryctl_persist_state "${state_root}" "${state}"
  call_status=0
  observation="$(external_driver_call "${driver}" prepare "${spec}")" || call_status=$?
  if ((call_status != 0)); then
    code="$(entryctl_driver_error_code "${call_status}")"
    [[ "${code}" != driver_failed ]] || code=prepare_failed
    retryable=true
    case "${code}" in invalid_spec | unsupported_capability | ownership_conflict | dependency_missing) retryable=false ;; esac
    error="$(jq -cn --arg code "${code}" --argjson retryable "${retryable}" '{code:$code,phase:"preparing",retryable:$retryable,rollback_status:"not-needed",message:"External driver prepare failed"}')"
    state="$(entryctl_state_json "${spec}" stable degraded "${observed_revision}" "${generation}" "${probe}" "${previous_provider}" "${error}")"
    entryctl_persist_state "${state_root}" "${state}"
    entryctl_error "${code}" preparing "${driver}" "${retryable}" not-needed "External driver prepare failed"
    return 5
  fi

  state="$(entryctl_state_json "${spec}" prepared unknown "${observed_revision}" "${generation}" "${observation}" "${previous_provider}")"
  entryctl_persist_state "${state_root}" "${state}"
  state="$(entryctl_state_json "${spec}" activating unknown "${observed_revision}" "${generation}" "${observation}" "${previous_provider}")"
  entryctl_persist_state "${state_root}" "${state}"
  call_status=0
  observation="$(external_driver_call "${driver}" activate "${spec}")" || call_status=$?
  if ((call_status != 0)); then
    code="$(entryctl_driver_error_code "${call_status}")"
    [[ "${code}" != driver_failed ]] || code=activation_failed
    rollback_status=succeeded
    state="$(entryctl_state_json "${spec}" rolling_back unknown "${observed_revision}" "${generation}" "${probe}" "${previous_provider}")"
    entryctl_persist_state "${state_root}" "${state}"
    external_driver_call "${driver}" rollback "${spec}" >/dev/null || rollback_status=failed
    error="$(jq -cn --arg code "${code}" --arg status "${rollback_status}" '{code:$code,phase:"activating",retryable:true,rollback_status:$status,message:"External driver activation failed"}')"
    if [[ "${rollback_status}" == succeeded ]]; then
      state="$(entryctl_state_json "${spec}" stable degraded "${observed_revision}" "${generation}" "${probe}" "${previous_provider}" "${error}")"
    else
      state="$(entryctl_state_json "${spec}" failed unhealthy "${observed_revision}" "${generation}" "${probe}" "${previous_provider}" "${error}")"
    fi
    entryctl_persist_state "${state_root}" "${state}"
    entryctl_error "${code}" activating "${driver}" true "${rollback_status}" "External driver activation failed"
    return 6
  fi

  state="$(entryctl_state_json "${spec}" verifying unknown "${observed_revision}" "${generation}" "${observation}" "${previous_provider}")"
  entryctl_persist_state "${state_root}" "${state}"
  call_status=0
  observation="$(external_driver_call "${driver}" verify "${spec}")" || call_status=$?
  if ((call_status != 0)); then
    code="$(entryctl_driver_error_code "${call_status}")"
    [[ "${code}" != driver_failed ]] || code=verification_failed
    rollback_status=succeeded
    state="$(entryctl_state_json "${spec}" rolling_back unknown "${observed_revision}" "${generation}" "${probe}" "${previous_provider}")"
    entryctl_persist_state "${state_root}" "${state}"
    external_driver_call "${driver}" rollback "${spec}" >/dev/null || rollback_status=failed
    error="$(jq -cn --arg code "${code}" --arg status "${rollback_status}" '{code:$code,phase:"verifying",retryable:true,rollback_status:$status,message:"External driver verification failed"}')"
    if [[ "${rollback_status}" == succeeded ]]; then
      state="$(entryctl_state_json "${spec}" stable degraded "${observed_revision}" "${generation}" "${probe}" "${previous_provider}" "${error}")"
    else
      state="$(entryctl_state_json "${spec}" failed unhealthy "${observed_revision}" "${generation}" "${probe}" "${previous_provider}" "${error}")"
    fi
    entryctl_persist_state "${state_root}" "${state}"
    entryctl_error "${code}" verifying "${driver}" true "${rollback_status}" "External driver verification failed"
    return 7
  fi

  local old_fingerprint="" new_fingerprint=""
  [[ -z "${existing}" ]] || old_fingerprint="$(jq -r '.certificate.fingerprint // ""' <<<"${existing}")"
  new_fingerprint="$(jq -r '.certificate.fingerprint // ""' <<<"${observation}")"
  if [[ -z "${existing}" || "${old_fingerprint}" != "${new_fingerprint}" ]]; then
    generation=$((generation + 1))
  fi
  state="$(entryctl_state_json "${spec}" stable healthy "${desired_revision}" "${generation}" "${observation}" "${previous_provider}")"
  entryctl_persist_state "${state_root}" "${state}"
  printf '%s\n' "${state}"
}

entryctl_external_remove() {
  local spec="$1"
  local state_root="$2"
  local purge="$3"
  entry_validate_mutating_spec_file "${spec}" && entry_validate_spec "${spec}" || {
    entryctl_error invalid_spec stable "${spec}" false not-needed "EntrySpec validation failed"
    return 2
  }
  [[ "$(jq -r '.provider' "${spec}")" == external ]] || {
    entryctl_error unsupported_capability stable "$(jq -r '.provider' "${spec}")" false not-needed "Only the external mutating adapter is enabled"
    return 3
  }
  local driver deployment output state_path existing current_sha recorded_path recorded_sha desired_revision known_desired
  driver="$(jq -r '.external_driver.path' "${spec}")"
  deployment="$(jq -r '.deployment_id' "${spec}")"
  desired_revision="$(jq -r '.revision' "${spec}")"
  current_sha="$(external_driver_sha256 "${driver}")" || {
    entryctl_error ownership_conflict stable "${driver}" false not-needed "External driver ownership, mode or digest validation failed"
    return 4
  }
  if existing="$(entry_read_state "${state_root}" "${deployment}" 2>/dev/null)"; then
    [[ "$(jq -r '.active_provider' <<<"${existing}")" == external ]] || {
      entryctl_error ownership_conflict stable "${deployment}" false not-needed "Journal belongs to another provider"
      return 4
    }
    known_desired="$(jq -r '.desired_revision' <<<"${existing}")"
    if ((desired_revision < known_desired)); then
      entryctl_error invalid_spec stable "${spec}" false not-needed "Remove spec revision is older than the journaled desired revision"
      return 2
    fi
    recorded_path="$(jq -r '.driver.path' <<<"${existing}")"
    recorded_sha="$(jq -r '.driver.sha256' <<<"${existing}")"
    if [[ "${recorded_path}" != "${driver}" || "${recorded_sha}" != "${current_sha}" ]]; then
      entryctl_error ownership_conflict stable "${driver}" false not-needed "Remove driver identity differs from the journal owner"
      return 4
    fi
  fi
  if ! output="$(ENTRY_PURGE="${purge}" external_driver_call "${driver}" remove "${spec}")"; then
    entryctl_error driver_failed stable "${driver}" true not-needed "External driver remove failed"
    return 4
  fi
  state_path="$(entry_state_path "${state_root}" "${deployment}")"
  if [[ -f "${state_path}" ]]; then
    rm -f -- "${state_path}"
  fi
  printf '%s\n' "${output}"
}

main() {
  local command="${1:-}"
  case "${command}" in
  -h | --help | help | "") usage ;;
  plan)
    shift
    local spec=""
    local state_root="${ENTRY_STATE_ROOT:-/tpdata/trojanpanelnext-entry/state}"
    while (($# > 0)); do
      case "$1" in
      --spec)
        [[ $# -ge 2 ]] || { printf '{"schema_version":1,"code":"invalid_spec","message":"--spec requires a file"}\n'; return 2; }
        spec="$2"
        shift 2
        ;;
      --state-root)
        [[ $# -ge 2 ]] || return 2
        state_root="$2"
        shift 2
        ;;
      *) printf '{"schema_version":1,"code":"invalid_argument","message":"unknown plan argument"}\n'; return 2 ;;
      esac
    done
    if [[ -z "${spec}" ]]; then
      printf '{"schema_version":1,"code":"invalid_spec","message":"--spec is required"}\n'
      return 2
    fi
    if [[ "$(jq -r '.schema_version // empty' "${spec}" 2>/dev/null)" == 2 ]]; then
      entry_validate_spec "${spec}" || { entry_v2_error invalid_spec stable 'Invalid v2 EntrySpec'; return 2; }
      entry_v2_plan "${spec}" "${state_root}"
      return $?
    fi
    local status=0
    if entry_validate_spec "${spec}"; then
      local deployment current
      deployment="$(jq -r '.deployment_id' "${spec}")"
      ENTRY_STATE_ROOT="${state_root}"
      export ENTRY_STATE_ROOT
      if current="$(entry_read_state "${state_root}" "${deployment}" 2>/dev/null)"; then
        ENTRY_CURRENT_PROVIDER="$(jq -r '.active_provider' <<<"${current}")"
        ENTRY_CURRENT_PHASE="$(jq -r '.phase' <<<"${current}")"
        export ENTRY_CURRENT_PROVIDER ENTRY_CURRENT_PHASE
      fi
    fi
    if entry_validate_spec "${spec}" && [[ "$(jq -r '.provider' "${spec}")" == "external" ]]; then
      local driver probe
      driver="$(jq -r '.external_driver.path' "${spec}")"
      if probe="$(external_driver_call "${driver}" probe "${spec}")"; then
        ENTRY_EXTERNAL_CAPABILITIES="$(jq -r '.capabilities | join(",")' <<<"${probe}")"
        export ENTRY_EXTERNAL_CAPABILITIES
      fi
    fi
    entry_plan_spec "${spec}" || status=$?
    case "${status}" in
    0) ;;
    10) printf '{"schema_version":1,"code":"dependency_missing","resource":"jq","retryable":false}\n'; return 3 ;;
    *) jq -cn --arg resource "${spec}" '{schema_version:1,code:"invalid_spec",resource:$resource,retryable:false}'; return 2 ;;
    esac
    ;;
  status)
    shift
    local deployment=""
    local state_root="${ENTRY_STATE_ROOT:-/tpdata/trojanpanelnext-entry/state}"
    while (($# > 0)); do
      case "$1" in
      --deployment)
        [[ $# -ge 2 ]] || return 2
        deployment="$2"
        shift 2
        ;;
      --state-root)
        [[ $# -ge 2 ]] || return 2
        state_root="$2"
        shift 2
        ;;
      *) return 2 ;;
      esac
    done
    if [[ -z "${deployment}" ]] || ! entry_read_state "${state_root}" "${deployment}"; then
      jq -cn --arg deployment_id "${deployment}" '{schema_version:1,code:"state_not_found_or_invalid",deployment_id:$deployment_id,retryable:false}'
      return 3
    fi
    ;;
  reconcile)
    shift
    local spec=""
    local state_root="${ENTRY_STATE_ROOT:-/tpdata/trojanpanelnext-entry/state}"
    while (($# > 0)); do
      case "$1" in
      --spec) [[ $# -ge 2 ]] || return 2; spec="$2"; shift 2 ;;
      --state-root) [[ $# -ge 2 ]] || return 2; state_root="$2"; shift 2 ;;
      *) return 2 ;;
      esac
    done
    [[ -n "${spec}" ]] || { entryctl_error invalid_spec preparing "" false not-needed "--spec is required"; return 2; }
    if [[ "$(jq -r '.schema_version // empty' "${spec}" 2>/dev/null)" == 2 ]]; then
      entry_v2_reconcile "${spec}" "${state_root}"
    else
      entryctl_external_reconcile "${spec}" "${state_root}"
    fi
    ;;
  remove)
    shift
    local spec=""
    local state_root="${ENTRY_STATE_ROOT:-/tpdata/trojanpanelnext-entry/state}"
    local purge=0
    while (($# > 0)); do
      case "$1" in
      --spec) [[ $# -ge 2 ]] || return 2; spec="$2"; shift 2 ;;
      --state-root) [[ $# -ge 2 ]] || return 2; state_root="$2"; shift 2 ;;
      --purge) purge=1; shift ;;
      *) return 2 ;;
      esac
    done
    [[ -n "${spec}" ]] || { entryctl_error invalid_spec stable "" false not-needed "--spec is required"; return 2; }
    if [[ "$(jq -r '.schema_version // empty' "${spec}" 2>/dev/null)" == 2 ]]; then
      entry_v2_remove "${spec}" "${state_root}" "${purge}"
    else
      entryctl_external_remove "${spec}" "${state_root}" "${purge}"
    fi
    ;;
  *)
    printf 'Unknown command: %s\n' "${command}" >&2
    usage >&2
    return 2
    ;;
  esac
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
