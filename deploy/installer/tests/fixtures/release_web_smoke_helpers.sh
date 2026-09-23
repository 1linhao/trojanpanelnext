#!/usr/bin/env bash

smoke_config_secret() {
  local config="$1"
  local key="$2"
  awk -F: -v key="${key}" '
    $1 == "  " key {
      sub(/^[^:]*:[[:space:]]*/, "")
      sub(/[[:space:]]+#.*$/, "")
      gsub(/^"|"$/, "")
      print
      exit
    }
  ' "${config}"
}

# Replace secrets as literal strings inside the current Bash process, keeping
# them out of sed/awk argv and ensuring failure diagnostics cannot echo them.
smoke_sanitize_diagnostic_file() {
  local config="$1"
  local input="$2"
  local known_admin_password="${3:-}"
  local line secret
  local -a secrets=(
    "${known_admin_password}"
    "$(smoke_config_secret "${config}" sysadmin_password)"
    "$(smoke_config_secret "${config}" mariadb_password)"
    "$(smoke_config_secret "${config}" redis_password)"
  )

  while IFS= read -r line || [[ -n "${line}" ]]; do
    for secret in "${secrets[@]}"; do
      [[ -n "${secret}" ]] || continue
      line="${line//"${secret}"/[REDACTED]}"
    done
    printf '%s\n' "${line}"
  done <"${input}"
}

smoke_print_install_failure_diagnostics() {
  local config="$1"
  local stdout_file="$2"
  local stderr_file="$3"
  local api_log_file="${4:-}"

  printf '%s\n' '--- sanitized installer stdout ---' >&2
  smoke_sanitize_diagnostic_file "${config}" "${stdout_file}" |
    sed -n '1,160p' >&2
  printf '%s\n' '--- sanitized installer stderr ---' >&2
  smoke_sanitize_diagnostic_file "${config}" "${stderr_file}" |
    sed -n '1,160p' >&2
  if [[ -n "${api_log_file}" && -f "${api_log_file}" ]]; then
    printf '%s\n' '--- sanitized API container logs ---' >&2
    smoke_sanitize_diagnostic_file "${config}" "${api_log_file}" |
      sed -n '1,160p' >&2
  fi
}

smoke_assert_diagnostics_sanitized() {
  local config="$1"
  local input="$2"
  local sanitized
  sanitized="$(mktemp)"
  smoke_sanitize_diagnostic_file "${config}" "${input}" >"${sanitized}"
  if ! cmp -s "${input}" "${sanitized}"; then
    rm -f -- "${sanitized}"
    return 1
  fi
  rm -f -- "${sanitized}"
}

smoke_report_admin_credential_state() {
  local config="$1"
  local host_password_file="$2"
  local container="$3"
  local expected_password host_password container_password verifier_status
  local host_matches=0 container_matches=0

  expected_password="$(smoke_config_secret "${config}" sysadmin_password)"
  host_password="$(<"${host_password_file}")"
  container_password="$(/usr/bin/docker exec "${container}" sh -c 'cat "$TP_INITIAL_SYSADMIN_PASSWORD_FILE"')"
  [[ "${host_password}" == "${expected_password}" ]] && host_matches=1
  [[ "${container_password}" == "${expected_password}" ]] && container_matches=1
  if /usr/bin/docker exec -e TP_VERIFY_SYSADMIN_CREDENTIAL=1 "${container}" ./trojan-panel >/dev/null 2>&1; then
    verifier_status=0
  else
    verifier_status=$?
  fi
  printf 'TRACE admin_file_matches_config=%s container_file_matches_config=%s verifier_exit=%s\n' \
    "${host_matches}" "${container_matches}" "${verifier_status}"
}
