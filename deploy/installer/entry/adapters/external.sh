#!/usr/bin/env bash

# Local client contract for an external Entry Adapter. It does not know nginx,
# certd, or VPS Factory resource names and cannot mutate them directly.

external_driver_validate() {
  local driver="$1"
  local expected_uid="${ENTRY_DRIVER_OWNER_UID:-0}"
  [[ "${driver}" == /* && -f "${driver}" && ! -L "${driver}" && -x "${driver}" ]] || return 2

  local owner mode mode_value
  owner="$(stat -c %u "${driver}")" || return 2
  mode="$(stat -c %a "${driver}")" || return 2
  [[ "${owner}" == "${expected_uid}" ]] || return 3
  mode_value=$((8#${mode}))
  (( (mode_value & 0022) == 0 )) || return 3
}

external_driver_sha256() {
  local driver="$1"
  external_driver_validate "${driver}" || return $?
  command -v sha256sum >/dev/null 2>&1 || return 10
  sha256sum "${driver}" | awk '{print $1}'
}

external_driver_validate_result() {
  local output="$1"
  local action="$2"
  local deployment_id="$3"
  jq -e --arg action "${action}" --arg deployment_id "${deployment_id}" '
    def valid_resource:
      (type == "object") and
      ((keys - ["kind", "id", "owner", "retention", "digest"]) | length == 0) and
      (.kind as $v | ["file", "directory", "container", "process", "listener", "timer", "certificate", "driver-resource"] | index($v) != null) and
      (.id | type == "string" and length > 0) and
      (.owner as $v | ["controller", "provider", "external"] | index($v) != null) and
      (.retention as $v | ["managed", "preserve"] | index($v) != null) and
      ((.digest // "") | type == "string");
    def valid_listener:
      (type == "object") and
      ((keys - ["transport", "address", "port", "purpose", "owner"]) | length == 0) and
      (.transport == "tcp" or .transport == "udp") and
      (.address | type == "string") and
      (.port | type == "number" and floor == . and . >= 1 and . <= 65535) and
      (.purpose | type == "string" and length > 0) and
      (.owner as $v | ["controller", "provider", "external", "kernel"] | index($v) != null);
    def valid_certificate:
      (type == "object") and
      ((keys - ["domain", "cert_path", "key_path", "fingerprint", "not_after", "generation", "renewal_owner", "last_hook_status"]) | length == 0) and
      (.domain | type == "string" and length > 0) and
      (.cert_path | type == "string" and startswith("/")) and
      (.key_path | type == "string" and startswith("/")) and
      (.fingerprint | type == "string" and length > 0) and
      (.generation | type == "number" and floor == . and . >= 0) and
      (.renewal_owner == "external") and
      (if has("not_after") then (.not_after | type == "string" and length > 0) else true end) and
      ((.last_hook_status // "unknown") as $v | ["unchanged", "ok", "failed", "unknown"] | index($v) != null);
    def valid_status($a):
      if $a == "probe" then . == "ok" or . == "unchanged"
      elif $a == "prepare" then . == "prepared" or . == "unchanged"
      elif $a == "activate" then . == "active" or . == "unchanged"
      elif $a == "verify" then . == "healthy"
      elif $a == "rollback" then . == "rolled_back" or . == "unchanged"
      elif $a == "remove" then . == "removed" or . == "unchanged"
      else false end;
    .schema_version == 1 and
    .driver_protocol_version == 1 and
    .provider == "external" and
    .action == $action and
    .deployment_id == $deployment_id and
    (.status | valid_status($action)) and
    (.capabilities | type == "array" and length == (unique | length) and all(.[]; type == "string" and test("^[a-z][a-z0-9_.-]+$"))) and
    (.resources | type == "array" and all(.[]; valid_resource)) and
    (.listeners | type == "array" and all(.[]; valid_listener)) and
    (if has("certificate") then (.certificate | valid_certificate) else true end) and
    (if has("message") then (.message | type == "string") else true end) and
    ((keys - ["schema_version", "driver_protocol_version", "provider", "action", "deployment_id", "status", "capabilities", "resources", "listeners", "certificate", "message"]) | length == 0)
  ' <<<"${output}" >/dev/null 2>&1
}

external_driver_call() {
  local driver="$1"
  local action="$2"
  local spec="$3"
  case "${action}" in
  probe | prepare | activate | verify | rollback | remove) ;;
  *) return 2 ;;
  esac
  external_driver_validate "${driver}" || return $?
  entry_validate_spec "${spec}" || return 4

  local output status deployment_id
  deployment_id="$(jq -r '.deployment_id' "${spec}")"
  status=0
  output="$("${driver}" provider "${action}" --spec "${spec}")" || status=$?
  if ((status != 0)); then
    case "${status}" in
    64 | 65 | 66 | 67 | 68 | 69 | 70 | 71) return "${status}" ;;
    *) return 5 ;;
    esac
  fi
  external_driver_validate_result "${output}" "${action}" "${deployment_id}" || return 6
  printf '%s\n' "${output}"
}
