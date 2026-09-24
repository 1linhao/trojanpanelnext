#!/usr/bin/env bash

# Caddy legacy Adapter for combined EntrySpec v2 deployments.  All host
# operations are behind small functions so contract tests can replace Docker,
# socket and certificate observations without touching a real VPS.

caddy_adapter_safe_path() {
  [[ "${1:-}" == /* && "${1:-}" != / && "${1:-}" != *'//'* &&
     "${1:-}" != */../* && "${1:-}" != */./* &&
     "${1:-}" != */.. && "${1:-}" != */. &&
     "${1:-}" != *[$'\n\r\t;{}']* ]] &&
    [[ "$(realpath -m -- "$1")" == "$1" ]]
}

caddy_adapter_safe_domain() {
  [[ "${1:-}" =~ ^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$ ]]
}

caddy_adapter_safe_upstream() {
  [[ "${1:-}" =~ ^(127\.0\.0\.1|\[::1\]):([1-9][0-9]{0,4})$ ]] || return 1
  ((10#${BASH_REMATCH[2]} <= 65535))
}

caddy_adapter_root() {
  local spec="$1"
  printf '%s\n' "${CADDY_ADAPTER_ROOT:-$(jq -r '.certificate_targets.web.managed_dir // .certificate_targets.node.managed_dir' "$spec")}"
}

caddy_adapter_container() {
  printf '%s\n' "${CADDY_ADAPTER_CONTAINER:-trojanpanelnext-entry-caddy}"
}

caddy_adapter_docker() {
  printf '%s\n' "${CADDY_ADAPTER_DOCKER:-docker}"
}

caddy_adapter_image() {
  local image="${CADDY_ADAPTER_IMAGE:-}"
  if [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]]; then
    printf '%s\n' "${image:-test.invalid/caddy@sha256:$(printf '0%.0s' {1..64})}"
    return
  fi
  [[ "$image" =~ ^[a-zA-Z0-9./:_-]+@sha256:[a-f0-9]{64}$ ]] || return 1
  printf '%s\n' "$image"
}

caddy_adapter_ca_file() {
  if [[ -n "${CADDY_ADAPTER_CA_FILE:-}" ]]; then
    printf '%s\n' "$CADDY_ADAPTER_CA_FILE"
  elif [[ "${CADDY_ADAPTER_TEST_INTERNAL_TLS:-0}" == 1 ]]; then
    printf '%s/data/caddy/pki/authorities/local/root.crt\n' "$(caddy_adapter_root "$1")"
  fi
}

caddy_adapter_test_internal_allowed() {
  [[ "${CADDY_ADAPTER_TEST_INTERNAL_TLS:-0}" != 1 ]] && return 0
  [[ "${CADDY_ADAPTER_ROOT:-}" == /tmp/* &&
     "$(caddy_adapter_container)" == tpn-entry-smoke-* ]]
}

caddy_adapter_marker() {
  local root="$1"
  printf '%s/.trojanpanelnext-owner\n' "${root%/}"
}

caddy_adapter_config() {
  local root="$1"
  printf '%s/Caddyfile\n' "${root%/}"
}

caddy_adapter_candidate() {
  local root="$1"
  printf '%s/.Caddyfile.candidate\n' "${root%/}"
}

caddy_adapter_consumer_registry() { printf '%s/.consumer-registry.json\n' "${1%/}"; }
caddy_adapter_renewal_spec() { printf '%s/.entry-spec.json\n' "${1%/}"; }
caddy_adapter_renewal_hook() { printf '%s/.entry-renew-hook\n' "${1%/}"; }
caddy_adapter_timer_dir() { printf '%s\n' "${CADDY_ADAPTER_TIMER_DIR:-/etc/systemd/system}"; }
caddy_adapter_timer_service() { printf '%s/trojanpanelnext-entry-renewal.service\n' "$(caddy_adapter_timer_dir)"; }
caddy_adapter_timer_unit() { printf '%s/trojanpanelnext-entry-renewal.timer\n' "$(caddy_adapter_timer_dir)"; }

caddy_adapter_validate_timer_dir() {
  local dir="$(caddy_adapter_timer_dir)" probe
  [[ "$dir" == /* && "$dir" != / && "$dir" != *'//' && "$dir" != */./* && "$dir" != */../* && "$dir" != */.. ]] || return 1
  case "$dir" in
    /etc/systemd/system|/run/systemd/system|/tmp/*) ;;
    *) return 1 ;;
  esac
  [[ "$(realpath -m -- "$dir")" == "$dir" ]] || return 1
  probe="$dir"
  while [[ "$probe" != / ]]; do
    [[ ! -L "$probe" ]] || return 1
    probe="$(dirname -- "$probe")"
  done
}

caddy_adapter_atomic_write() {
  local path="$1" mode="$2" tmp
  tmp="$(mktemp "${path}.tmp.XXXXXXXX")" || return 1
  if ! cat >"$tmp" || ! chmod "$mode" "$tmp" || ! mv -f -- "$tmp" "$path"; then
    rm -f -- "$tmp"
    return 1
  fi
}

caddy_adapter_validate_consumer_registry() {
  local registry="$1"
  jq -e '
    type == "object" and (keys | sort) == ["consumers","domains"] and
    (.consumers | type == "array" and all(.[];
      type == "object" and (keys | sort) == ["active","deployment_id","owner","path","removed","role"] and
      (.deployment_id | type == "string") and (.role | type == "string") and
      (.path | type == "string" and startswith("/")) and (.owner == "provider") and
      (.active | type == "boolean") and (.removed | type == "boolean"))) and
    (.domains | type == "array" and all(.[];
      type == "object" and (keys | sort) == ["active","deployment_id","domain","owner","role"] and
      (.deployment_id | type == "string") and (.role | type == "string") and
      (.domain | type == "string") and (.owner == "provider") and (.active | type == "boolean")))
  ' "$registry" >/dev/null
}

caddy_adapter_backup_renewal_trigger() {
  local root="$1" timer_dir backup path name
  caddy_adapter_validate_timer_dir || return 1
  timer_dir="$(caddy_adapter_timer_dir)"
  backup="${root}/.rollback-renewal"
  rm -rf -- "$backup" || return 1
  mkdir -m 0700 "$backup" || return 1
  local -a paths=(
    "$(caddy_adapter_renewal_spec "$root")"
    "$(caddy_adapter_renewal_hook "$root")"
    "$(caddy_adapter_consumer_registry "$root")"
    "$(caddy_adapter_timer_service)"
    "$(caddy_adapter_timer_unit)"
    "${timer_dir}/.tpn-renewal-owner"
  )
  for path in "${paths[@]}"; do
    name="$(basename -- "$path")"
    if [[ -e "$path" || -L "$path" ]]; then
      [[ ! -L "$path" ]] || return 1
      cp -p -- "$path" "${backup}/${name}" || return 1
    else
      : >"${backup}/.absent-${name}" || return 1
    fi
  done
}

caddy_adapter_restore_renewal_trigger() {
  local root="$1" spec="$2" timer_dir backup path name
  backup="${root}/.rollback-renewal"
  [[ -d "$backup" && ! -L "$backup" ]] || return 1
  caddy_adapter_validate_timer_dir || return 1
  timer_dir="$(caddy_adapter_timer_dir)"
  local deployment="$(jq -r '.deployment_id' "$spec")"
  if [[ -e "${timer_dir}/.tpn-renewal-owner" ]]; then
    [[ "$(cat "${timer_dir}/.tpn-renewal-owner" 2>/dev/null)" == "deployment=$deployment" ]] || return 1
  fi
  local -a paths=(
    "$(caddy_adapter_renewal_spec "$root")"
    "$(caddy_adapter_renewal_hook "$root")"
    "$(caddy_adapter_consumer_registry "$root")"
    "$(caddy_adapter_timer_service)"
    "$(caddy_adapter_timer_unit)"
    "${timer_dir}/.tpn-renewal-owner"
  )
  for path in "${paths[@]}"; do
    [[ ! -L "$path" ]] || return 1
    rm -f -- "$path" || return 1
  done
  for path in "${paths[@]}"; do
    name="$(basename -- "$path")"
    if [[ -e "${backup}/${name}" ]]; then
      mv -f -- "${backup}/${name}" "$path" || return 1
    elif [[ ! -e "${backup}/.absent-${name}" ]]; then
      return 1
    fi
  done
  caddy_adapter_validate_consumer_registry "$(caddy_adapter_consumer_registry "$root")" 2>/dev/null || {
    [[ ! -e "$(caddy_adapter_consumer_registry "$root")" ]] || return 1
  }
  rm -rf -- "$backup"
}

caddy_adapter_commit_renewal_trigger() {
  local root="$1"
  [[ ! -e "${root}/.rollback-renewal" ]] || rm -rf -- "${root}/.rollback-renewal"
}

caddy_adapter_write_renewal_trigger() {
  local spec="$1" root="$2" deployment entryctl state_root timer_dir service unit registry consumer image
  [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]] && return 0
  deployment="$(jq -r '.deployment_id' "$spec")"
  entryctl="${CADDY_ADAPTER_ENTRYCTL_PATH:-${ENTRYCTL_PATH:-/usr/local/lib/trojanpanelnext/entry/entryctl.sh}}"
  state_root="${CADDY_ADAPTER_ENTRY_STATE_ROOT:-/tpdata/trojanpanelnext-entry/state}"
  image="$(caddy_adapter_image)" || return 1
  [[ -x "$entryctl" && ! -L "$entryctl" ]] || return 1
  timer_dir="$(caddy_adapter_timer_dir)"; service="$(caddy_adapter_timer_service)"; unit="$(caddy_adapter_timer_unit)"
  caddy_adapter_validate_timer_dir || return 1
  mkdir -p "$root" "$timer_dir" || return 1
  if [[ -e "$service" || -e "$unit" || -e "${timer_dir}/.tpn-renewal-owner" ]]; then
    [[ -f "${timer_dir}/.tpn-renewal-owner" && "$(cat "${timer_dir}/.tpn-renewal-owner" 2>/dev/null)" == "deployment=$deployment" ]] || return 1
  fi
  [[ ! -L "$service" && ! -L "$unit" && ! -L "${timer_dir}/.tpn-renewal-owner" ]] || return 1
  local spec_tmp
  spec_tmp="$(mktemp)" || return 1
  jq -S . "$spec" >"$spec_tmp" || { rm -f "$spec_tmp"; return 1; }
  install -m 0600 "$spec_tmp" "$(caddy_adapter_renewal_spec "$root")" || { rm -f "$spec_tmp"; return 1; }
  rm -f "$spec_tmp"
  {
    printf '#!/bin/bash\nset -Eeuo pipefail\n'
    printf 'export CADDY_ADAPTER_IMAGE=%q\n' "$image"
    printf 'export CADDY_ADAPTER_ENTRYCTL_PATH=%q\n' "$entryctl"
    printf 'export CADDY_ADAPTER_TIMER_DIR=%q\n' "$timer_dir"
    for var in CADDY_ADAPTER_ROOT CADDY_ADAPTER_DOCKER CADDY_ADAPTER_CONTAINER CADDY_ADAPTER_NODE_CONTAINER CADDY_ADAPTER_WEB_ROOT CADDY_ADAPTER_CA_FILE CADDY_ADAPTER_TEST_INTERNAL_TLS CADDY_ADAPTER_SKIP_DNS_CHECK CADDY_ADAPTER_CERT_WAIT_ATTEMPTS CADDY_ADAPTER_CERT_WAIT_SECONDS; do
      [[ -n "${!var+x}" ]] && printf 'export %s=%q\n' "$var" "${!var}"
    done
    printf 'exec %q reconcile --spec %q --state-root %q\n' "$entryctl" "$(caddy_adapter_renewal_spec "$root")" "$state_root"
  } | caddy_adapter_atomic_write "$(caddy_adapter_renewal_hook "$root")" 0700 || return 1
  printf 'deployment=%s\n' "$deployment" | caddy_adapter_atomic_write "${timer_dir}/.tpn-renewal-owner" 0600 || return 1
  cat <<EOF | caddy_adapter_atomic_write "$service" 0644
[Unit]
Description=TrojanPanelNext Caddy certificate reconcile
[Service]
Type=oneshot
ExecStart=$(caddy_adapter_renewal_hook "$root")
EOF
  cat <<EOF | caddy_adapter_atomic_write "$unit" 0644
[Unit]
Description=TrojanPanelNext Caddy certificate reconcile timer
[Timer]
OnBootSec=5min
OnUnitActiveSec=15min
Persistent=true
[Install]
WantedBy=timers.target
EOF
  registry="$(caddy_adapter_consumer_registry "$root")"
  if [[ -e "$registry" ]]; then
    jq -c '.consumers |= map(. + {removed:(.removed // false)}) | .domains |= map(.)' "$registry" >"${registry}.tmp" && mv -f "${registry}.tmp" "$registry" || return 1
    caddy_adapter_validate_consumer_registry "$registry" || return 1
  else
    printf '{"consumers":[],"domains":[]}\n' | caddy_adapter_atomic_write "$registry" 0600
  fi
  jq -c --arg d "$deployment" --argjson domains "$(jq -c '.domains' "$spec")" --argjson roles "$(jq -c '.active_roles' "$spec")" '
    .domains |= ((. // []) | map(if .deployment_id == $d then .active = false else . end) +
      [$domains | to_entries[] | select(.key as $role | $roles | index($role) != null) |
       {deployment_id:$d,role:.key,domain:.value,owner:"provider",active:true}] |
      group_by(.deployment_id,.role,.domain) | map(last))
  ' "$registry" >"${registry}.tmp" && mv -f "${registry}.tmp" "$registry" || return 1
  if jq -e '.active_roles | index("node") != null' "$spec" >/dev/null; then
    consumer="$(jq -r '.roles.node.certificate_consumer' "$spec")"
    jq -c --arg d "$deployment" --arg p "$consumer" ' .consumers |= (map(select(.deployment_id != $d or .role != "node")) + [{deployment_id:$d,role:"node",path:$p,owner:"provider",active:true,removed:false}]) ' "$registry" >"${registry}.tmp" && mv -f "${registry}.tmp" "$registry" || return 1
  else
    jq -c --arg d "$deployment" ' .consumers |= map(if .deployment_id == $d and .role == "node" then .active=false | .removed=true else . end) ' "$registry" >"${registry}.tmp" && mv -f "${registry}.tmp" "$registry" || return 1
  fi
  caddy_adapter_validate_consumer_registry "$registry" || return 1
  chmod 0600 "$registry"
}

caddy_adapter_enable_renewal_trigger() {
  local timer_dir service
  [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]] && return 0
  timer_dir="$(caddy_adapter_timer_dir)"; service="$(caddy_adapter_timer_service)"
  caddy_adapter_validate_timer_dir || return 1
  if [[ -n "${CADDY_ADAPTER_TIMER_ENABLE_CMD:-}" ]]; then
    CADDY_ENTRY_TIMER_SERVICE="$service" bash -c "$CADDY_ADAPTER_TIMER_ENABLE_CMD"
  elif [[ "$timer_dir" == /etc/systemd/system ]] && command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload && systemctl enable --now trojanpanelnext-entry-renewal.timer >/dev/null
  else
    return 1
  fi
}

caddy_adapter_remove_renewal_trigger() {
  local root="$1" spec="$2" timer_dir service unit owner deployment
  [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]] && return 0
  deployment="$(jq -r '.deployment_id' "$spec")"
  timer_dir="$(caddy_adapter_timer_dir)"; service="$(caddy_adapter_timer_service)"; unit="$(caddy_adapter_timer_unit)"
  caddy_adapter_validate_timer_dir || return 1
  owner="${timer_dir}/.tpn-renewal-owner"
  if [[ -e "$service" || -e "$unit" || -e "$owner" ]]; then
    [[ -f "$owner" && ! -L "$owner" && "$(cat "$owner" 2>/dev/null)" == "deployment=$deployment" ]] || return 1
  fi
  if [[ -n "${CADDY_ADAPTER_TIMER_ENABLE_CMD:-}" ]]; then
    CADDY_ENTRY_TIMER_SERVICE="$service" bash -c "${CADDY_ADAPTER_TIMER_DISABLE_CMD:-true}" || return 1
  elif [[ "$timer_dir" == /etc/systemd/system ]] && command -v systemctl >/dev/null 2>&1; then
    systemctl disable --now trojanpanelnext-entry-renewal.timer >/dev/null 2>&1 || true
    systemctl daemon-reload || return 1
  fi
  rm -f "$service" "$unit" "$owner" "$(caddy_adapter_renewal_hook "$root")" "$(caddy_adapter_renewal_spec "$root")"
}

caddy_adapter_install_config() {
  local root="$1" content="$2" temp
  temp="$(mktemp "${root}/.Caddyfile.XXXXXXXX")" || return 1
  printf '%s' "$content" >"$temp" || { rm -f "$temp"; return 1; }
  chmod 0600 "$temp" || { rm -f "$temp"; return 1; }
  mv -f -- "$temp" "$(caddy_adapter_config "$root")"
}

caddy_adapter_sha256_text() {
  printf '%s' "$1" | sha256sum | awk '{print $1}'
}

caddy_adapter_fact_digest() {
  local kind="$1" id="$2" root="${3:-}" facts
  case "$kind" in
  file|certificate)
    [[ -f "$id" && ! -L "$id" ]] || return 1
    facts="$(sha256sum "$id" | awk '{print $1}')"
    ;;
  directory)
    [[ -d "$id" && ! -L "$id" ]] || return 1
    # Caddy owns /data and writes ACME locks, accounts and renewed certificates
    # asynchronously. Its directory identity must not depend on those contents.
    facts="$(stat -c '%d:%i:%u:%g:%a' "$id")"
    ;;
  container)
    if [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]]; then
      facts="fake:${id}:$(cat "$(caddy_adapter_marker "$root")" 2>/dev/null || true)"
    else
      local docker="$(caddy_adapter_docker)" image label container_id mounts
      command -v "$docker" >/dev/null 2>&1 || return 1
      "$docker" inspect "$id" >/dev/null 2>&1 || return 1
      label="$($docker inspect -f '{{ index .Config.Labels "io.trojanpanelnext.deployment" }}' "$id" 2>/dev/null)" || return 1
      image="$($docker inspect -f '{{.Config.Image}}' "$id" 2>/dev/null)" || return 1
      container_id="$($docker inspect -f '{{.Id}}' "$id" 2>/dev/null)" || return 1
      # Docker does not promise Mounts iteration order across daemon versions.
      # Canonicalize it before hashing so a restart cannot look like a new owner.
      mounts="$($docker inspect -f '{{range .Mounts}}{{.Source}}:{{.Destination}}:{{.RW}}{{println}}{{end}}' "$id" 2>/dev/null | sort | tr '\n' ';')" || return 1
      facts="${container_id}:${label}:${image}:${mounts}"
    fi
    ;;
  *) return 1 ;;
  esac
  caddy_adapter_sha256_text "$facts"
}

caddy_adapter_resource_json() {
  local deployment="$1" kind="$2" id="$3" scope="$4" retention="$5" role="${6:-}" root="${7:-}" digest
  local marker="owned:${deployment}:${kind}:${id}"
  digest="$(caddy_adapter_fact_digest "$kind" "$id" "$root" 2>/dev/null)" || return 1
  jq -cn --arg kind "$kind" --arg id "$id" --arg owner provider --arg deployment "$deployment" \
    --arg scope "$scope" --arg retention "$retention" --arg marker "$marker" --arg digest "$digest" --arg role "$role" \
    '{kind:$kind,id:$id,owner:$owner,deployment_id:$deployment,scope:$scope,retention:$retention,identity:{marker:$marker,digest:$digest}} + (if $role != "" then {role:$role} else {} end)'
}

caddy_adapter_resource_list() {
  local spec="$1" root="$2" deployment="$3" roles="$4" include_files="${5:-1}" container
  caddy_adapter_validate_timer_dir || return 1
  container="$(caddy_adapter_container)"
  if [[ "${include_files}" == 1 ]]; then
    [[ -f "${root%/}/Caddyfile" ]] && caddy_adapter_resource_json "$deployment" file "${root%/}/Caddyfile" shared managed '' "$root"
    [[ -d "${root%/}/data" ]] && caddy_adapter_resource_json "$deployment" directory "${root%/}/data" shared managed '' "$root"
    [[ -f "$(caddy_adapter_renewal_spec "$root")" ]] && caddy_adapter_resource_json "$deployment" file "$(caddy_adapter_renewal_spec "$root")" shared managed '' "$root"
    [[ -f "$(caddy_adapter_renewal_hook "$root")" ]] && caddy_adapter_resource_json "$deployment" file "$(caddy_adapter_renewal_hook "$root")" shared managed '' "$root"
    [[ -f "$(caddy_adapter_consumer_registry "$root")" ]] && caddy_adapter_resource_json "$deployment" file "$(caddy_adapter_consumer_registry "$root")" shared managed '' "$root"
    [[ -f "$(caddy_adapter_timer_service)" ]] && caddy_adapter_resource_json "$deployment" file "$(caddy_adapter_timer_service)" shared managed '' "$root"
    [[ -f "$(caddy_adapter_timer_unit)" ]] && caddy_adapter_resource_json "$deployment" file "$(caddy_adapter_timer_unit)" shared managed '' "$root"
    [[ -f "$(caddy_adapter_timer_dir)/.tpn-renewal-owner" ]] &&
      caddy_adapter_resource_json "$deployment" file "$(caddy_adapter_timer_dir)/.tpn-renewal-owner" shared managed '' "$root"
  fi
  if [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]]; then
    [[ -f "${root}/.active" ]] && caddy_adapter_resource_json "$deployment" container "$container" shared managed '' "$root"
  else
    local docker="$(caddy_adapter_docker)"
    if command -v "$docker" >/dev/null 2>&1 && "$docker" inspect "$container" >/dev/null 2>&1; then
      caddy_adapter_resource_json "$deployment" container "$container" shared managed '' "$root"
    fi
  fi
  while IFS= read -r role; do
    [[ -n "$role" ]] || continue
    local cert_path key_path
    cert_path="$(jq -r --arg role "$role" '.certificate_targets[$role].cert_path' "$spec")"
    key_path="$(jq -r --arg role "$role" '.certificate_targets[$role].key_path' "$spec")"
    [[ -f "$cert_path" ]] && caddy_adapter_resource_json "$deployment" certificate "$cert_path" role preserve "$role" "$root"
    [[ -f "$key_path" ]] && caddy_adapter_resource_json "$deployment" certificate "$key_path" role preserve "$role" "$root"
  done <<<"$roles"
  return 0
}

caddy_adapter_render() {
  local spec="$1" role domain upstream
  local deployment
  caddy_adapter_test_internal_allowed || return 2
  deployment="$(jq -r '.deployment_id' "$spec")"
  while IFS= read -r role; do
    [[ -n "$role" ]] || continue
    domain="$(jq -r --arg role "$role" '.domains[$role]' "$spec")"
    caddy_adapter_safe_domain "$domain" || return 2
    case "$role" in
    web)
      upstream="$(jq -r '.roles.web.web_upstream' "$spec")"
      caddy_adapter_safe_upstream "$upstream" || return 2
      printf '%s {\n    reverse_proxy %s\n' "$domain" "$upstream"
      [[ "${CADDY_ADAPTER_TEST_INTERNAL_TLS:-0}" != 1 ]] || printf '    tls internal\n'
      printf '}\n\n'
      ;;
    node)
      printf '%s {\n    root * /srv\n    file_server\n' "$domain"
      [[ "${CADDY_ADAPTER_TEST_INTERNAL_TLS:-0}" != 1 ]] || printf '    tls internal\n'
      printf '}\n\n'
      ;;
    *) return 2 ;;
    esac
  done < <(jq -r '.active_roles[]' "$spec")
  [[ -n "$deployment" ]] || return 2
}

caddy_adapter_validate_render() {
  local content="$1"
  [[ -n "$content" && "$content" != *$'\n'*'{'*'\n'* ]] || return 2
  if [[ -n "${CADDY_ADAPTER_VALIDATE_CMD:-}" ]]; then
    CADDYFILE_CONTENT="$content" bash -c "$CADDY_ADAPTER_VALIDATE_CMD"
  elif [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]]; then
    return 0
  else
    local docker image temp
    docker="$(caddy_adapter_docker)"
    image="$(caddy_adapter_image)" || return 1
    temp="$(mktemp -d)" || return 1
    printf '%s' "$content" >"$temp/Caddyfile"
    "$docker" run --rm --network none -v "$temp/Caddyfile:/etc/caddy/Caddyfile:ro" \
      "$image" caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile >/dev/null 2>&1
    local status=$?
    rm -rf "$temp"
    return "$status"
  fi
}

caddy_adapter_owner_status() {
  local spec="$1" root="$2" marker deployment
  deployment="$(jq -r '.deployment_id' "$spec")"
  marker="$(caddy_adapter_marker "$root")"
  if [[ ! -e "$root" ]]; then
    printf 'absent\n'
    return 0
  fi
  [[ -d "$root" && ! -L "$root" && "$(stat -c %u "$root")" == "$(id -u)" ]] || return 2
  if [[ ! -f "$marker" ]]; then
    find "$root" -mindepth 1 -maxdepth 1 -print -quit | grep -q . && return 3
    printf 'empty\n'
    return 0
  fi
  [[ ! -L "$marker" && "$(stat -c %a "$marker")" == 600 &&
     "$(cat "$marker" 2>/dev/null)" == "deployment=${deployment}" ]] || return 3
  printf 'owned\n'
}

caddy_adapter_validate_active_config() {
  local spec="$1" root="$2" expected
  [[ -s "$(caddy_adapter_config "$root")" ]] || return 1
  expected="$(caddy_adapter_render "$spec")" || return 1
  cmp -s <(printf '%s' "$expected") "$(caddy_adapter_config "$root")"
}

caddy_adapter_check_ports() {
  [[ "${CADDY_ADAPTER_SKIP_PORT_CHECK:-0}" == 1 || "${CADDY_ADAPTER_FAKE:-0}" == 1 ]] && return 0
  if [[ -n "${CADDY_ADAPTER_PORT_CHECK_CMD:-}" ]]; then bash -c "$CADDY_ADAPTER_PORT_CHECK_CMD"; return; fi
  command -v ss >/dev/null 2>&1 || return 1
  local docker container pid port out line
  docker="$(caddy_adapter_docker)"; container="$(caddy_adapter_container)"
  pid="$("$docker" inspect -f '{{.State.Pid}}' "$container" 2>/dev/null || true)"
  [[ "$pid" =~ ^[0-9]+$ ]] || pid=0
  for port in 80 443; do
    out="$(ss -Hlnpt "( sport = :${port} )" 2>/dev/null)" || return 1
    if [[ "$pid" == 0 ]]; then
      [[ -z "$out" ]] || return 1
    else
      [[ -n "$out" ]] || return 1
      while IFS= read -r line; do
        [[ "$line" == *"pid=${pid},"* ]] || return 1
      done <<<"$out"
    fi
  done
}

caddy_adapter_check_dns() {
  local spec="$1" role domain
  [[ "${CADDY_ADAPTER_SKIP_DNS_CHECK:-0}" == 1 || "${CADDY_ADAPTER_FAKE:-0}" == 1 ]] && return 0
  [[ -n "${CADDY_ADAPTER_DNS_CHECK_CMD:-}" ]] && CADDY_ENTRY_SPEC="$spec" bash -c "$CADDY_ADAPTER_DNS_CHECK_CMD" && return
  command -v getent >/dev/null 2>&1 || return 1
  while IFS= read -r role; do
    [[ -n "$role" ]] || continue
    domain="$(jq -r --arg role "$role" '.domains[$role]' "$spec")"
    getent ahosts "$domain" >/dev/null 2>&1 || return 1
  done < <(jq -r '.active_roles[]' "$spec")
}

caddy_adapter_check_manifest() {
  local spec="$1" manifest upstream_port
  [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]] && return 0
  jq -e '.active_roles | index("node") != null' "$spec" >/dev/null || return 0
  manifest="$(jq -r '.roles.node.route_manifest' "$spec")"
  caddy_adapter_safe_path "$manifest" && [[ -s "$manifest" && ! -L "$manifest" ]] || return 1
  upstream_port="$(jq -r '.roles.web.web_upstream // ""' "$spec" | sed 's/.*://')"
  jq -e --argjson upstream_port "${upstream_port:-0}" '
    .routes | type == "array" and
    length >= 1 and
    all(.[]; (.network == "tcp" or .network == "udp") and
      (.port | type == "number" and floor == . and . >= 1 and . <= 65535) and
      (.port != 80 and .port != 443 and .port != $upstream_port)) and
    ([.[] | .network + ":" + (.port | tostring)] | length == (unique | length))
  ' "$manifest" >/dev/null 2>&1
}

caddy_adapter_check_node_runtime() {
  local spec="$1" require_all="${2:-1}" docker core pids network port out line pid found consumer envs mount running
  [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]] && return 0
  jq -e '.active_roles | index("node") != null' "$spec" >/dev/null || return 0
  docker="$(caddy_adapter_docker)"; core="${CADDY_ADAPTER_NODE_CONTAINER:-trojan-panel-core}"
  running="$($docker inspect -f '{{.State.Running}}' "$core" 2>/dev/null || true)"
  if [[ "$running" == true ]]; then
    consumer="$(jq -r '.roles.node.certificate_consumer' "$spec")"
    caddy_adapter_safe_path "$consumer" || return 1
    envs="$($docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$core" 2>/dev/null)" || return 1
    grep -Fxq "crt_path=$consumer/fullchain.pem" <<<"$envs" || return 1
    grep -Fxq "key_path=$consumer/privkey.pem" <<<"$envs" || return 1
    mount="$($docker inspect -f '{{range .Mounts}}{{if eq .Destination "'"$consumer"'"}}{{.Source}}{{end}}{{end}}' "$core" 2>/dev/null)" || return 1
    [[ "$mount" == "$consumer" ]] || return 1
    pids="$("$docker" top "$core" -eo pid 2>/dev/null | tail -n +2 | tr '\n' ' ')" || return 1
    [[ -n "$pids" ]] || return 1
  else
    [[ "$require_all" == 0 ]] || return 1
    pids=''
  fi
  while IFS=$'\t' read -r network port; do
    [[ -n "$network" ]] || continue
    case "$network" in tcp) out="$(ss -Hlnpt "( sport = :${port} )" 2>/dev/null)" ;; udp) out="$(ss -Hlnpu "( sport = :${port} )" 2>/dev/null)" ;; *) return 1 ;; esac
    if [[ -z "$out" ]]; then [[ "$require_all" == 0 ]] && continue; return 1; fi
    [[ -n "$pids" ]] || return 1
    while IFS= read -r line; do
      found=0
      while [[ "$line" =~ pid=([0-9]+), ]]; do
        pid="${BASH_REMATCH[1]}"
        [[ " $pids " == *" $pid "* ]] || return 1
        line="${line#*pid=$pid,}"
        found=1
      done
      [[ "$found" == 1 ]] || return 1
    done <<<"$out"
  done < <(jq -r '.routes[] | [.network,(.port | tostring)] | @tsv' "$(jq -r '.roles.node.route_manifest' "$spec")")
}

caddy_adapter_wait_node_runtime() {
  local spec="$1" attempts="${CADDY_ADAPTER_NODE_WAIT_ATTEMPTS:-20}" delay="${CADDY_ADAPTER_NODE_WAIT_SECONDS:-1}" i
  for ((i = 0; i < attempts; i++)); do
    caddy_adapter_check_node_runtime "$spec" && return 0
    sleep "$delay"
  done
  return 1
}

entry_v2_adapter_plan_ready() {
  caddy_adapter_check_node_runtime "$1"
}

caddy_adapter_check_container_owner() {
  local spec="$1" docker container deployment label
  [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]] && return 0
  docker="$(caddy_adapter_docker)"
  container="$(caddy_adapter_container)"
  deployment="$(jq -r '.deployment_id' "$spec")"
  command -v "$docker" >/dev/null 2>&1 || return 1
  if ! "$docker" inspect "$container" >/dev/null 2>&1; then
    return 0
  fi
  label="$($docker inspect -f '{{ index .Config.Labels "io.trojanpanelnext.deployment" }}' "$container" 2>/dev/null || true)"
  [[ "$label" == "$deployment" ]] || return 1
  local image config_mount data_mount web_mount
  image="$(caddy_adapter_image)" || return 1
  [[ "$($docker inspect -f '{{.Config.Image}}' "$container" 2>/dev/null)" == "$image" ]] || return 1
  config_mount="$($docker inspect -f '{{range .Mounts}}{{if eq .Destination "/etc/caddy"}}{{.Source}}{{end}}{{end}}' "$container" 2>/dev/null)" || return 1
  data_mount="$($docker inspect -f '{{range .Mounts}}{{if eq .Destination "/data"}}{{.Source}}{{end}}{{end}}' "$container" 2>/dev/null)" || return 1
  web_mount="$($docker inspect -f '{{range .Mounts}}{{if eq .Destination "/srv"}}{{.Source}}{{end}}{{end}}' "$container" 2>/dev/null)" || return 1
  [[ "$config_mount" == "$(caddy_adapter_root "$spec")" &&
     "$data_mount" == "$(caddy_adapter_root "$spec")/data" &&
     "$web_mount" == "${CADDY_ADAPTER_WEB_ROOT:-/tpdata/web}" ]]
}

caddy_adapter_check_certificate_ownership() {
  local spec="$1" role cert key marker deployment
  [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]] && return 0
  deployment="$(jq -r '.deployment_id' "$spec")"
  while IFS= read -r role; do
    cert="$(jq -r --arg role "$role" '.certificate_targets[$role].cert_path' "$spec")"
    key="$(jq -r --arg role "$role" '.certificate_targets[$role].key_path' "$spec")"
    caddy_adapter_safe_path "$cert" && caddy_adapter_safe_path "$key" || return 1
    [[ ! -L "$cert" && ! -L "$key" ]] || return 1
    marker="$(dirname "$cert")/.tpn-${deployment}-${role}.owner"
    if [[ -e "$cert" || -e "$key" || -e "$marker" || -L "$marker" ]]; then
      [[ -f "$marker" && ! -L "$marker" &&
         "$(cat "$marker")" == "deployment=${deployment};role=${role}" ]] || return 1
    fi
  done < <(jq -r '.active_roles[]' "$spec")
}

caddy_adapter_probe_cert() {
  local domain="$1" cert="$2" key="$3"
  [[ -s "$cert" && -s "$key" ]] || return 1
  command -v openssl >/dev/null 2>&1 || return 1
  openssl x509 -in "$cert" -noout >/dev/null 2>&1 || return 1
  local key_pub cert_pub
  key_pub="$(openssl pkey -in "$key" -pubout 2>/dev/null | sha256sum | awk '{print $1}')" || return 1
  cert_pub="$(openssl x509 -in "$cert" -pubkey -noout 2>/dev/null | openssl pkey -pubin -outform pem -pubout 2>/dev/null | sha256sum | awk '{print $1}')" || return 1
  [[ -n "$key_pub" && "$key_pub" == "$cert_pub" ]] || return 1
  openssl x509 -in "$cert" -checkend 0 -noout >/dev/null 2>&1 || return 1
  local ca_file
  ca_file="$(caddy_adapter_ca_file "${4:-}")"
  if [[ -n "$ca_file" ]]; then
    openssl verify -purpose sslserver -CAfile "$ca_file" -untrusted "$cert" "$cert" >/dev/null 2>&1 || return 1
  else
    openssl verify -purpose sslserver -untrusted "$cert" "$cert" >/dev/null 2>&1 || return 1
  fi
  openssl x509 -in "$cert" -noout -text 2>/dev/null | grep -Eq "DNS:${domain}([,[:space:]]|$)" || return 1
}

caddy_adapter_certificate_json() {
  local spec="$1" role="$2" generation="${3:-1}" cert key domain fingerprint not_after
  domain="$(jq -r --arg role "$role" '.domains[$role]' "$spec")"
  cert="$(jq -r --arg role "$role" '.certificate_targets[$role].cert_path' "$spec")"
  key="$(jq -r --arg role "$role" '.certificate_targets[$role].key_path' "$spec")"
  caddy_adapter_probe_cert "$domain" "$cert" "$key" "$spec" || return 1
  fingerprint="$(openssl x509 -in "$cert" -noout -fingerprint -sha256 | sed 's/.*=//; s/://g' | tr '[:upper:]' '[:lower:]')"
  not_after="$(openssl x509 -in "$cert" -noout -enddate | sed 's/^notAfter=//')"
  not_after="$(date -u -d "$not_after" '+%Y-%m-%dT%H:%M:%SZ')" || return 1
  jq -cn --arg domain "$domain" --arg cert "$cert" --arg key "$key" --arg fp "$fingerprint" --arg expiry "$not_after" \
    --argjson generation "$generation" '{domain:$domain,cert_path:$cert,key_path:$key,fingerprint:$fp,generation:$generation,renewal_owner:"caddy-legacy",last_hook_status:"ok",not_after:$expiry}'
}

caddy_adapter_apply_generations() {
  local observation="$1" state="$2"
  jq -c --argjson old "$state" '
    .certificates |= with_entries(
      .key as $role | .value as $current |
      .value.generation = (if $old.certificates[$role].fingerprint == $current.fingerprint
        then $old.certificates[$role].generation else (($old.certificates[$role].generation // 0) + 1) end))' <<<"$observation"
}

caddy_adapter_backup_certificates() {
  local spec="$1" root="$2" backup="${root}/.rollback-certs" role cert key consumer
  [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]] && return 0
  caddy_adapter_owner_status "$spec" "$root" | grep -qx owned || return 1
  rm -rf -- "$backup" || return 1
  mkdir -m 0700 "$backup" || return 1
  while IFS= read -r role; do
    cert="$(jq -r --arg role "$role" '.certificate_targets[$role].cert_path' "$spec")"
    key="$(jq -r --arg role "$role" '.certificate_targets[$role].key_path' "$spec")"
    if [[ -f "$cert" && -f "$key" ]]; then
      cp -p -- "$cert" "$backup/${role}.crt" && cp -p -- "$key" "$backup/${role}.key" || return 1
    else
      : >"$backup/.absent-${role}.crt" && : >"$backup/.absent-${role}.key" || return 1
    fi
  done < <(jq -r '.active_roles[]' "$spec")
  if jq -e '.active_roles | index("node") != null' "$spec" >/dev/null; then
    consumer="$(jq -r '.roles.node.certificate_consumer' "$spec")"
    if [[ -f "$consumer/fullchain.pem" && -f "$consumer/privkey.pem" ]]; then
      cp -p -- "$consumer/fullchain.pem" "$backup/consumer.crt" &&
        cp -p -- "$consumer/privkey.pem" "$backup/consumer.key" || return 1
    else
      : >"$backup/.absent-consumer.crt" && : >"$backup/.absent-consumer.key" || return 1
    fi
  fi
  printf 'deployment=%s\n' "$(jq -r '.deployment_id' "$spec")" >"$backup/owner"
}

caddy_adapter_restore_certificates() {
  local spec="$1" root="$2" backup="${root}/.rollback-certs" role cert key consumer docker core envs mount
  [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 || ! -e "$backup" ]] && return 0
  [[ "$(cat "$backup/owner" 2>/dev/null)" == "deployment=$(jq -r '.deployment_id' "$spec")" ]] || return 1
  while IFS= read -r role; do
    cert="$(jq -r --arg role "$role" '.certificate_targets[$role].cert_path' "$spec")"
    key="$(jq -r --arg role "$role" '.certificate_targets[$role].key_path' "$spec")"
    if [[ -f "$backup/${role}.crt" && -f "$backup/${role}.key" ]]; then
      cp -p -- "$backup/${role}.crt" "$cert" && cp -p -- "$backup/${role}.key" "$key" || return 1
    elif [[ -e "$backup/.absent-${role}.crt" && -e "$backup/.absent-${role}.key" ]]; then
      rm -f -- "$cert" "$key" "$(dirname "$cert")/.tpn-$(jq -r '.deployment_id' "$spec")-${role}.owner" || return 1
    fi
  done < <(jq -r '.active_roles[]' "$spec")
  if [[ -f "$backup/consumer.crt" && -f "$backup/consumer.key" ]]; then
    consumer="$(jq -r '.roles.node.certificate_consumer' "$spec")"
    if ! cmp -s "$backup/consumer.crt" "$consumer/fullchain.pem" ||
       ! cmp -s "$backup/consumer.key" "$consumer/privkey.pem"; then
      cp -p -- "$backup/consumer.crt" "$consumer/fullchain.pem" &&
        cp -p -- "$backup/consumer.key" "$consumer/privkey.pem" || return 1
      docker="$(caddy_adapter_docker)"; core="${CADDY_ADAPTER_NODE_CONTAINER:-trojan-panel-core}"
      if "$docker" inspect "$core" >/dev/null 2>&1; then
        envs="$($docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$core" 2>/dev/null)" || return 1
        grep -Fxq "crt_path=$consumer/fullchain.pem" <<<"$envs" || return 1
        grep -Fxq "key_path=$consumer/privkey.pem" <<<"$envs" || return 1
        mount="$($docker inspect -f '{{range .Mounts}}{{if eq .Destination "'"$consumer"'"}}{{.Source}}{{end}}{{end}}' "$core" 2>/dev/null)" || return 1
        [[ "$mount" == "$consumer" ]] || return 1
        if [[ "$($docker inspect -f '{{.State.Running}}' "$core" 2>/dev/null)" == true ]]; then
          "$docker" restart "$core" >/dev/null || return 1
        fi
      fi
    fi
  elif [[ -e "$backup/.absent-consumer.crt" && -e "$backup/.absent-consumer.key" ]]; then
    consumer="$(jq -r '.roles.node.certificate_consumer // ""' "$spec")"
    if [[ -n "$consumer" ]]; then
      rm -f -- "$consumer/fullchain.pem" "$consumer/privkey.pem" "$consumer/.tpn-$(jq -r '.deployment_id' "$spec")-consumer.owner" || return 1
    fi
  fi
  rm -rf -- "$backup"
}

caddy_adapter_retire_node_consumer() {
  local old_spec="$1" new_spec="$2" consumer marker
  [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]] && return 0
  jq -e '.active_roles | index("node") != null' "$old_spec" >/dev/null || return 0
  jq -e '.active_roles | index("node") == null' "$new_spec" >/dev/null || return 0
  consumer="$(jq -r '.roles.node.certificate_consumer' "$old_spec")"
  caddy_adapter_safe_path "$consumer" || return 1
  marker="$consumer/.tpn-$(jq -r '.deployment_id' "$old_spec")-consumer.owner"
  if [[ -e "$consumer/fullchain.pem" || -e "$consumer/privkey.pem" || -e "$marker" ]]; then
    [[ -f "$marker" && "$(cat "$marker" 2>/dev/null)" == "deployment=$(jq -r '.deployment_id' "$old_spec")" ]] || return 1
    fuser -s "$consumer/fullchain.pem" "$consumer/privkey.pem" && return 1
    rm -f -- "$consumer/fullchain.pem" "$consumer/privkey.pem" "$marker" || return 1
  fi
}

caddy_adapter_refresh_node_consumer() {
  local spec="$1" node_cert node_key consumer marker docker core envs mount temp_cert temp_key changed=0
  [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]] && return 0
  jq -e '.active_roles | index("node") != null' "$spec" >/dev/null || return 0
  node_cert="$(jq -r '.certificate_targets.node.cert_path' "$spec")"
  node_key="$(jq -r '.certificate_targets.node.key_path' "$spec")"
  consumer="$(jq -r '.roles.node.certificate_consumer' "$spec")"
  caddy_adapter_safe_path "$consumer" || return 1
  [[ ! -L "$consumer" ]] || return 1
  marker="${consumer}/.tpn-$(jq -r '.deployment_id' "$spec")-consumer.owner"
  if [[ -d "$consumer" ]]; then
    if [[ -e "$consumer/fullchain.pem" || -e "$consumer/privkey.pem" ]]; then
      [[ -f "$marker" && "$(cat "$marker")" == "deployment=$(jq -r '.deployment_id' "$spec")" ]] || return 1
    fi
  fi
  mkdir -p "$consumer" || return 1
  if [[ ! -f "$consumer/fullchain.pem" || ! -f "$consumer/privkey.pem" ]] ||
     ! cmp -s "$node_cert" "$consumer/fullchain.pem" || ! cmp -s "$node_key" "$consumer/privkey.pem"; then
    changed=1
    temp_cert="$(mktemp "$consumer/.fullchain.XXXXXXXX")" || return 1
    temp_key="$(mktemp "$consumer/.privkey.XXXXXXXX")" || { rm -f "$temp_cert"; return 1; }
    install -m 0644 "$node_cert" "$temp_cert" && install -m 0600 "$node_key" "$temp_key" &&
      mv -f "$temp_cert" "$consumer/fullchain.pem" && mv -f "$temp_key" "$consumer/privkey.pem" || {
        rm -f "$temp_cert" "$temp_key"; return 1;
      }
    printf 'deployment=%s\n' "$(jq -r '.deployment_id' "$spec")" >"$marker" || return 1
    chmod 0600 "$marker" || return 1
  fi
  docker="$(caddy_adapter_docker)"; core="${CADDY_ADAPTER_NODE_CONTAINER:-trojan-panel-core}"
  if "$docker" inspect "$core" >/dev/null 2>&1; then
    envs="$($docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$core" 2>/dev/null)" || return 1
    grep -Fxq "crt_path=$consumer/fullchain.pem" <<<"$envs" || return 1
    grep -Fxq "key_path=$consumer/privkey.pem" <<<"$envs" || return 1
    mount="$($docker inspect -f '{{range .Mounts}}{{if eq .Destination "'"$consumer"'"}}{{.Source}}{{end}}{{end}}' "$core" 2>/dev/null)" || return 1
    [[ "$mount" == "$consumer" ]] || return 1
    [[ "$($docker inspect -f '{{.State.Running}}' "$core" 2>/dev/null)" == true ]] || return 1
    if [[ "$changed" == 1 ]]; then
      "$docker" restart "$core" >/dev/null || return 1
      # Docker restart returns before the Node listener is necessarily bound.
      caddy_adapter_wait_node_runtime "$spec" || return 1
    fi
  else
    return 1
  fi
}

caddy_adapter_sync_certificates() {
  local spec="$1" root="$2" role domain target_cert target_key source_cert source_key source_dir marker deployment temp_cert temp_key
  [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]] && return 0
  deployment="$(jq -r '.deployment_id' "$spec")"
  while IFS= read -r role; do
    [[ -n "$role" ]] || continue
    domain="$(jq -r --arg role "$role" '.domains[$role]' "$spec")"
    target_cert="$(jq -r --arg role "$role" '.certificate_targets[$role].cert_path' "$spec")"
    target_key="$(jq -r --arg role "$role" '.certificate_targets[$role].key_path' "$spec")"
    caddy_adapter_safe_path "$target_cert" && caddy_adapter_safe_path "$target_key" || return 1
    marker="$(dirname "$target_cert")/.tpn-${deployment}-${role}.owner"
    if [[ -e "$target_cert" || -e "$target_key" ]]; then
      [[ -f "$marker" && "$(cat "$marker")" == "deployment=${deployment};role=${role}" ]] || return 1
    fi
    [[ ! -L "$target_cert" && ! -L "$target_key" ]] || return 1
    source_cert=''; source_key=''
    for source_dir in "${root%/}/data/caddy/certificates"/*/"${domain}"; do
      if [[ -f "${source_dir}/${domain}.crt" && -f "${source_dir}/${domain}.key" &&
            ! -L "${source_dir}/${domain}.crt" && ! -L "${source_dir}/${domain}.key" ]]; then
        source_cert="${source_dir}/${domain}.crt"; source_key="${source_dir}/${domain}.key"; break
      fi
    done
    [[ -s "$source_cert" && -s "$source_key" ]] || return 1
    caddy_adapter_probe_cert "$domain" "$source_cert" "$source_key" "$spec" || return 1
    if [[ -s "$target_cert" && -s "$target_key" ]] && cmp -s "$source_cert" "$target_cert" && cmp -s "$source_key" "$target_key"; then continue; fi
    mkdir -p "$(dirname "$target_cert")" "$(dirname "$target_key")" || return 1
    temp_cert="$(mktemp "${target_cert}.XXXXXXXX")" || return 1
    temp_key="$(mktemp "${target_key}.XXXXXXXX")" || { rm -f "$temp_cert"; return 1; }
    printf 'deployment=%s;role=%s\n' "$deployment" "$role" >"$marker" || return 1
    chmod 0600 "$marker" || return 1
    install -m 0644 "$source_cert" "$temp_cert" && install -m 0600 "$source_key" "$temp_key" &&
      mv -f "$temp_cert" "$target_cert" && mv -f "$temp_key" "$target_key" || { rm -f "$temp_cert" "$temp_key"; return 1; }
  done < <(jq -r '.active_roles[]' "$spec")
}

caddy_adapter_ensure_container() {
  local spec="$1" root="$2" docker container image
  docker="$(caddy_adapter_docker)"; container="$(caddy_adapter_container)"
  if [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]]; then
    : >"${root}/.active"
    return 0
  fi
  command -v "$docker" >/dev/null 2>&1 || return 1
  image="$(caddy_adapter_image)" || return 1
  if "$docker" inspect "$container" >/dev/null 2>&1; then
    caddy_adapter_check_container_owner "$spec"
    return $?
  fi
  "$docker" create --name "$container" --restart always --network host \
    --label "io.trojanpanelnext.deployment=$(jq -r '.deployment_id' "$spec")" \
    -v "${root}:/etc/caddy:ro" -v "${root}/data:/data" \
    -v "${CADDY_ADAPTER_WEB_ROOT:-/tpdata/web}:/srv:ro" "$image" >/dev/null
}

caddy_adapter_candidate_resources() {
  local spec="$1" root="$2" content="$3" deployment roles config list digest
  deployment="$(jq -r '.deployment_id' "$spec")"
  roles="$(jq -r '.active_roles[]' "$spec")"
  config="$(caddy_adapter_config "$root")"
  list="$(caddy_adapter_resource_list "$spec" "$root" "$deployment" "$roles" 1 | jq -s -c --arg config "$config" 'map(select(.id != $config))')" || return 1
  digest="$(caddy_adapter_sha256_text "$(caddy_adapter_sha256_text "$content")")"
  jq -cn --argjson list "$list" --arg config "$config" --arg deployment "$deployment" --arg digest "$digest" '
    $list + [{kind:"file",id:$config,owner:"provider",deployment_id:$deployment,scope:"shared",retention:"managed",identity:{marker:("owned:"+$deployment+":file:"+$config),digest:$digest}}]'
}

caddy_adapter_wait_for_certificates() {
  local spec="$1" root="$2" attempts="${CADDY_ADAPTER_CERT_WAIT_ATTEMPTS:-60}" delay="${CADDY_ADAPTER_CERT_WAIT_SECONDS:-3}" i
  [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]] && return 0
  for ((i = 0; i < attempts; i++)); do
    caddy_adapter_sync_certificates "$spec" "$root" && return 0
    sleep "$delay"
  done
  return 1
}

caddy_adapter_listeners() {
  local require_runtime="${2:-0}"
  if [[ "$require_runtime" == 1 && "${CADDY_ADAPTER_FAKE:-0}" != 1 ]]; then
    command -v ss >/dev/null 2>&1 || return 1
    caddy_adapter_check_ports || return 1
  fi
  printf '%s\n' \
    '{"transport":"tcp","address":"0.0.0.0","port":80,"purpose":"acme-http01","owner":"provider","scope":"shared"}' \
    '{"transport":"tcp","address":"0.0.0.0","port":443,"purpose":"web-https","owner":"provider","scope":"shared"}'
  if jq -e '.active_roles | index("node") != null' "$1" >/dev/null; then
    local manifest
    manifest="$(jq -r '.roles.node.route_manifest' "$1")"
    if [[ -s "$manifest" ]] && command -v jq >/dev/null 2>&1; then
      jq -c '.routes[]? | {transport:.network,address:"0.0.0.0",port,purpose:"node-direct",owner:"kernel",scope:"role",role:"node"}' "$manifest" 2>/dev/null || return 1
    fi
  fi
}

caddy_adapter_observation() {
  local spec="$1" root="$2" include_current="$3" deployment roles resources certificates listeners
  deployment="$(jq -r '.deployment_id' "$spec")"
  roles="$(jq -r '.active_roles[]' "$spec")"
  resources='[]'
  if [[ "$include_current" == 1 ]]; then
    resources="$(caddy_adapter_resource_list "$spec" "$root" "$deployment" "$roles" 1 | jq -s '.')"
  fi
  certificates='{}'
  while IFS= read -r role; do
    [[ -n "$role" ]] || continue
    local cert_json
    cert_json="$(caddy_adapter_certificate_json "$spec" "$role" "${CADDY_ADAPTER_CERT_GENERATION:-1}" 2>/dev/null || true)"
    [[ -n "$cert_json" ]] && certificates="$(jq -c --arg role "$role" --argjson cert "$cert_json" '. + {($role):$cert}' <<<"$certificates")"
  done <<<"$roles"
  listeners="$(caddy_adapter_listeners "$spec" "$include_current" | jq -s '.')" || return 1
  jq -cn --arg deployment "$deployment" --argjson resources "$resources" --argjson certificates "$certificates" --argjson listeners "$listeners" \
    '{schema_version:2,topology:"combined",deployment_id:$deployment,provider:"caddy-legacy",ownership_verified:true,resources:$resources,candidate_resources:$resources,certificates:$certificates,listeners:$listeners,capabilities:["certificate.material","certificate.issue","ingress.acme_http01","ingress.web_https","ingress.plain_fallback","lifecycle.prepare","lifecycle.rollback"]}'
}

entry_v2_adapter_probe() {
  local spec="$1" state="${2:-null}" root status timer_owner observation_spec="$1" temporary=""
  export CADDY_ADAPTER_REAL_CONNECTED=1
  root="$(caddy_adapter_root "$spec")"
  caddy_adapter_safe_path "$root" || return 1
  caddy_adapter_owner_status "$spec" "$root" >/dev/null || return 1
  caddy_adapter_check_container_owner "$spec" || return 1
  caddy_adapter_check_dns "$spec" || return 1
  caddy_adapter_check_manifest "$spec" || return 1
  caddy_adapter_check_node_runtime "$spec" 0 || return 1
  caddy_adapter_check_ports || return 1
  status="$(caddy_adapter_owner_status "$spec" "$root")"
  if [[ "$status" == owned && "${CADDY_ADAPTER_FAKE:-0}" != 1 ]]; then
    timer_owner="$(caddy_adapter_timer_dir)/.tpn-renewal-owner"
    [[ -f "$timer_owner" && ! -L "$timer_owner" &&
       "$(cat "$timer_owner" 2>/dev/null)" == "deployment=$(jq -r '.deployment_id' "$spec")" ]] || {
      [[ -z "$temporary" ]] || rm -f "$temporary"
      return 1
    }
    local registry="$(caddy_adapter_consumer_registry "$root")"
    [[ -f "$registry" && ! -L "$registry" ]] || { [[ -z "$temporary" ]] || rm -f "$temporary"; return 1; }
    caddy_adapter_validate_consumer_registry "$registry" || { [[ -z "$temporary" ]] || rm -f "$temporary"; return 1; }
  fi
  # A role removal probes the last committed target so the controller can
  # compare every currently owned resource before staging the smaller target.
  if [[ "$state" != null && "$(jq -r '.committed_target == null' <<<"$state")" == false ]]; then
    temporary="$(mktemp)" || return 1
    jq -c '.committed_target.spec' <<<"$state" >"$temporary" || { rm -f "$temporary"; return 1; }
    observation_spec="$temporary"
  fi
  caddy_adapter_check_certificate_ownership "$observation_spec" || { [[ -z "$temporary" ]] || rm -f "$temporary"; return 1; }
  if [[ "$status" == owned ]]; then
    if [[ "$state" == null || "$(jq -r '.phase' <<<"$state")" == stable ]]; then
      caddy_adapter_validate_active_config "$observation_spec" "$root" || return 1
    fi
    caddy_adapter_observation "$observation_spec" "$root" 1
  else
    caddy_adapter_observation "$observation_spec" "$root" 0
  fi
  local result=$?
  [[ -z "$temporary" ]] || rm -f "$temporary"
  return "$result"
}

entry_v2_adapter_prepare() {
  local spec="$1" state="$2" root candidate content marker observation resources role cert key
  root="$(caddy_adapter_root "$spec")"
  candidate="$(caddy_adapter_candidate "$root")"
  content="$(caddy_adapter_render "$spec")" || return 1
  caddy_adapter_validate_render "$content" || return 1
  if [[ "${CADDY_ADAPTER_FAKE:-0}" != 1 && "$(jq -r '.committed_target == null' <<<"$state")" == true ]]; then
    while IFS= read -r role; do
      cert="$(jq -r --arg role "$role" '.certificate_targets[$role].cert_path' "$spec")"
      key="$(jq -r --arg role "$role" '.certificate_targets[$role].key_path' "$spec")"
      [[ ! -e "$cert" && ! -e "$key" && ! -L "$cert" && ! -L "$key" ]] || return 1
    done < <(jq -r '.active_roles[]' "$spec")
  fi
  mkdir -p "$root" "$root/data" || return 1
  chmod 0700 "$root" "$root/data" || return 1
  if [[ "$(jq -r '.committed_target == null' <<<"$state")" == false ]]; then
    caddy_adapter_backup_renewal_trigger "$root" || return 1
  fi
  caddy_adapter_write_renewal_trigger "$spec" "$root" || return 1
  marker="$(caddy_adapter_marker "$root")"
  printf 'deployment=%s\n' "$(jq -r '.deployment_id' "$spec")" >"$marker" || return 1
  chmod 0600 "$marker" || return 1
  if [[ "$(jq -r '.committed_target == null' <<<"$state")" == false ]]; then
    local old_spec
    old_spec="$(mktemp)" || return 1
    jq -c '.committed_target.spec' <<<"$state" >"$old_spec" || return 1
    caddy_adapter_backup_certificates "$old_spec" "$root" || { rm -f "$old_spec"; return 1; }
    rm -f "$old_spec"
  fi
  printf '%s' "$content" >"$candidate" || return 1
  caddy_adapter_ensure_container "$spec" "$root" || return 1
  chmod 0600 "$candidate" || return 1
  resources="$(caddy_adapter_candidate_resources "$spec" "$root" "$content")" || return 1
  observation="$(caddy_adapter_observation "$spec" "$root" 0)" || return 1
  jq -c --argjson resources "$resources" '.candidate_resources=$resources' <<<"$observation"
}

entry_v2_adapter_activate() {
  local spec="$1" state="$2" root config candidate docker container old_spec
  root="$(caddy_adapter_root "$spec")"
  config="$(caddy_adapter_config "$root")"
  candidate="$(caddy_adapter_candidate "$root")"
  [[ -s "$candidate" ]] || return 1
  caddy_adapter_check_container_owner "$spec" || return 1
  local content
  content="$(cat "$candidate")" || return 1
  caddy_adapter_install_config "$root" "$content" || return 1
  docker="$(caddy_adapter_docker)"
  container="$(caddy_adapter_container)"
  if [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]]; then
    : >"${root}/.active"
    return 0
  fi
  command -v "$docker" >/dev/null 2>&1 || return 1
  if [[ "$($docker inspect -f '{{.State.Running}}' "$container" 2>/dev/null)" == true ]]; then
    "$docker" restart "$container" >/dev/null || return 1
  else
    "$docker" start "$container" >/dev/null || return 1
  fi
  caddy_adapter_wait_for_certificates "$spec" "$root" || return 1
  if jq -e '.active_roles | index("node") != null' "$spec" >/dev/null; then
    caddy_adapter_refresh_node_consumer "$spec" || return 1
  else
    old_spec="$(mktemp)" || return 1
    jq -c '.committed_target.spec' <<<"$state" >"$old_spec" || { rm -f "$old_spec"; return 1; }
    caddy_adapter_retire_node_consumer "$old_spec" "$spec" || { rm -f "$old_spec"; return 1; }
    rm -f "$old_spec"
  fi
}

caddy_adapter_verify_runtime() {
  local spec="$1" docker container running health domain role upstream cert served_fp target_fp
  local -a curl_trust=()
  [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]] && return 0
  caddy_adapter_check_container_owner "$spec" || return 1
  docker="$(caddy_adapter_docker)"; container="$(caddy_adapter_container)"
  running="$($docker inspect -f '{{.State.Running}}' "$container" 2>/dev/null)" || return 1
  [[ "$running" == true ]] || return 1
  health="$($docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$container" 2>/dev/null)" || return 1
  [[ "$health" != unhealthy ]] || return 1
  caddy_adapter_check_node_runtime "$spec" || return 1
  if jq -e '.active_roles | index("web") != null' "$spec" >/dev/null; then
    upstream="$(jq -r '.roles.web.web_upstream' "$spec")"
    grep -Fqx "    reverse_proxy ${upstream}" "$(caddy_adapter_config "$(caddy_adapter_root "$spec")")" || return 1
  fi
  command -v curl >/dev/null 2>&1 || return 1
  local ca_file
  ca_file="$(caddy_adapter_ca_file "$spec")"
  [[ -z "$ca_file" ]] || curl_trust=(--cacert "$ca_file")
  while IFS= read -r role; do
    [[ -n "$role" ]] || continue
    domain="$(jq -r --arg role "$role" '.domains[$role]' "$spec")"
    curl --fail --silent --show-error --noproxy '*' --max-time "${CADDY_ADAPTER_CURL_TIMEOUT:-5}" \
      "${curl_trust[@]}" \
      --resolve "${domain}:443:127.0.0.1" "https://${domain}/" -o /dev/null || return 1
    cert="$(jq -r --arg role "$role" '.certificate_targets[$role].cert_path' "$spec")"
    target_fp="$(openssl x509 -in "$cert" -noout -fingerprint -sha256 | sed 's/.*=//; s/://g' | tr '[:upper:]' '[:lower:]')" || return 1
    served_fp="$(openssl s_client -connect 127.0.0.1:443 -servername "$domain" -showcerts </dev/null 2>/dev/null |
      openssl x509 -noout -fingerprint -sha256 2>/dev/null | sed 's/.*=//; s/://g' | tr '[:upper:]' '[:lower:]')" || return 1
    [[ -n "$served_fp" && "$served_fp" == "$target_fp" ]] || return 1
  done < <(jq -r '.active_roles[]' "$spec")
}

caddy_adapter_validate_remove_ownership() {
  local spec="$1" state="$2" root deployment recorded current
  root="$(caddy_adapter_root "$spec")"
  deployment="$(jq -r '.deployment_id' "$spec")"
  [[ "$(jq -r '.deployment_id' <<<"$state")" == "$deployment" ]] || return 1
  [[ "$(jq -r '.active_provider' <<<"$state")" == caddy-legacy ]] || return 1
  caddy_adapter_owner_status "$spec" "$root" >/dev/null || return 1
  caddy_adapter_check_container_owner "$spec" || return 1
  recorded="$(jq -cS '.resources' <<<"$state")" || return 1
  current="$(caddy_adapter_resource_list "$spec" "$root" "$deployment" "$(jq -r '.active_roles[]' "$spec")" 1 | jq -s -cS '.')" || return 1
  [[ "$recorded" == "$current" ]]
}

entry_v2_adapter_verify() {
  local spec="$1" state="$2" root
  root="$(caddy_adapter_root "$spec")"
  [[ -s "$(caddy_adapter_config "$root")" ]] || return 1
  caddy_adapter_check_ports || return 1
  caddy_adapter_verify_runtime "$spec" || return 1
  local observation
  observation="$(caddy_adapter_observation "$spec" "$root" 1)" || return 1
  rm -f -- "$(caddy_adapter_candidate "$root")" || return 1
  local result
  result="$(caddy_adapter_apply_generations "$observation" "$state")" || return 1
  caddy_adapter_commit_renewal_trigger "$root" || return 1
  printf '%s\n' "$result"
}

entry_v2_adapter_refresh() {
  local spec="$1" state="$2" root observation
  root="$(caddy_adapter_root "$spec")"
  caddy_adapter_backup_certificates "$spec" "$root" || return 1
  caddy_adapter_sync_certificates "$spec" "$root" || return 1
  caddy_adapter_refresh_node_consumer "$spec" || return 1
  caddy_adapter_verify_runtime "$spec" || return 1
  observation="$(caddy_adapter_observation "$spec" "$root" 1)" || return 1
  caddy_adapter_apply_generations "$observation" "$state"
}

entry_v2_adapter_rollback() {
  local spec="$1" state="$2" root config candidate docker container
  root="$(caddy_adapter_root "$spec")"
  config="$(caddy_adapter_config "$root")"
  candidate="$(caddy_adapter_candidate "$root")"
  if [[ "$(jq -r '.committed_target == null' <<<"$state")" == true && ! -e "$root" ]]; then
    caddy_adapter_check_container_owner "$spec" || return 1
    if [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]] ||
       ! "$(caddy_adapter_docker)" inspect "$(caddy_adapter_container)" >/dev/null 2>&1; then
      return 0
    fi
    return 1
  fi
  if [[ "$(jq -r '.committed_target == null' <<<"$state")" == true ]]; then
    caddy_adapter_check_container_owner "$spec" || return 1
  else
    caddy_adapter_owner_status "$spec" "$root" | grep -qx owned || return 1
    caddy_adapter_check_container_owner "$spec" || return 1
  fi
  rm -f "$candidate"
  if [[ "$(jq -r '.committed_target == null' <<<"$state")" == true ]]; then
    if [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]]; then
      rm -f "${root}/.active"
    else
      docker="$(caddy_adapter_docker)"; container="$(caddy_adapter_container)"
      if "$docker" inspect "$container" >/dev/null 2>&1; then
        "$docker" rm -f "$container" >/dev/null 2>&1 || return 1
      fi
    fi
    local role cert key cert_marker
    while IFS= read -r role; do
      cert="$(jq -r --arg role "$role" '.certificate_targets[$role].cert_path' "$spec")"
      key="$(jq -r --arg role "$role" '.certificate_targets[$role].key_path' "$spec")"
      cert_marker="$(dirname "$cert")/.tpn-$(jq -r '.deployment_id' "$spec")-${role}.owner"
      if [[ -e "$cert_marker" ]]; then
        [[ "$(cat "$cert_marker")" == "deployment=$(jq -r '.deployment_id' "$spec");role=${role}" ]] || return 1
        rm -f -- "$cert" "$key" "$cert_marker" || return 1
      fi
    done < <(jq -r '.active_roles[]' "$spec")
    if jq -e '.active_roles | index("node") != null' "$spec" >/dev/null; then
      local consumer consumer_marker
      consumer="$(jq -r '.roles.node.certificate_consumer' "$spec")"
      consumer_marker="$consumer/.tpn-$(jq -r '.deployment_id' "$spec")-consumer.owner"
      if [[ -e "$consumer_marker" ]]; then
        [[ "$(cat "$consumer_marker")" == "deployment=$(jq -r '.deployment_id' "$spec")" ]] || return 1
        rm -f -- "$consumer/fullchain.pem" "$consumer/privkey.pem" "$consumer_marker" || return 1
      fi
    fi
    caddy_adapter_remove_renewal_trigger "$root" "$spec" || return 1
    rm -f "$config" "$(caddy_adapter_marker "$root")" "$(caddy_adapter_consumer_registry "$root")"
    rm -rf -- "${root}/data" || return 1
    rmdir "$root" 2>/dev/null || true
    return 0
  fi
  local old_spec
  old_spec="$(mktemp)" || return 1
  jq -c '.committed_target.spec' <<<"$state" >"$old_spec" || { rm -f "$old_spec"; return 1; }
  local old_content
  old_content="$(caddy_adapter_render "$old_spec")" || { rm -f "$old_spec"; return 1; }
  if [[ -d "${root}/.rollback-renewal" ]]; then
    caddy_adapter_restore_renewal_trigger "$root" "$old_spec" || { rm -f "$old_spec"; return 1; }
  else
    caddy_adapter_write_renewal_trigger "$old_spec" "$root" || { rm -f "$old_spec"; return 1; }
  fi
  caddy_adapter_install_config "$root" "$old_content" || { rm -f "$old_spec"; return 1; }
  caddy_adapter_restore_certificates "$old_spec" "$root" || { rm -f "$old_spec"; return 1; }
  if [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]]; then rm -f "$old_spec"; return 0; fi
  docker="$(caddy_adapter_docker)"; container="$(caddy_adapter_container)"
  if ! command -v "$docker" >/dev/null 2>&1 ||
     ! "$docker" restart "$container" >/dev/null 2>&1 ||
     ! caddy_adapter_refresh_node_consumer "$old_spec"; then
    rm -f "$old_spec"
    return 1
  fi
  local attempt restored=0
  for ((attempt = 0; attempt < 10; attempt++)); do
    if caddy_adapter_check_ports && caddy_adapter_verify_runtime "$old_spec"; then restored=1; break; fi
    sleep 1
  done
  rm -f "$old_spec"
  [[ "$restored" == 1 ]]
}

entry_v2_adapter_remove() {
  local spec="$1" state="$2" purge="${3:-0}" root config marker docker container role cert key cert_marker consumer core
  root="$(caddy_adapter_root "$spec")"; config="$(caddy_adapter_config "$root")"; marker="$(caddy_adapter_marker "$root")"
  caddy_adapter_validate_remove_ownership "$spec" "$state" || return 1
  if [[ "$purge" == 1 ]]; then
    # A removed role may still have a CertificateRef consumer. The current
    # one-role spec cannot authorize deleting that role's Caddy storage.
    local storage_domain storage_dir registry
    if [[ "${CADDY_ADAPTER_FAKE:-0}" != 1 ]]; then
      registry="$(caddy_adapter_consumer_registry "$root")"
      [[ -f "$registry" && ! -L "$registry" ]] || return 1
      caddy_adapter_validate_consumer_registry "$registry" || return 1
      jq -e --arg deployment "$(jq -r '.deployment_id' "$spec")" --arg consumer "$(jq -r '.roles.node.certificate_consumer // ""' "$spec")" '
        all(.consumers[];
          .deployment_id == $deployment and .owner == "provider" and
          ((.active == true and .removed == false and .role == "node" and $consumer != "" and .path == $consumer) or
           (.active == false and .removed == true)))
      ' "$registry" >/dev/null || return 1
      while IFS=$'\t' read -r consumer_path consumer_active consumer_removed; do
        [[ "$consumer_active" == false && "$consumer_removed" == true ]] || continue
        caddy_adapter_safe_path "$consumer_path" || return 1
        [[ ! -e "$consumer_path/fullchain.pem" && ! -e "$consumer_path/privkey.pem" &&
           ! -e "$consumer_path/.tpn-$(jq -r '.deployment_id' "$spec")-consumer.owner" ]] || return 1
      done < <(jq -r --arg deployment "$(jq -r '.deployment_id' "$spec")" '.consumers[] | select(.deployment_id == $deployment) | [.path,.active,.removed] | @tsv' "$registry")
      for storage_dir in "${root}/data/caddy/certificates"/*/*; do
        [[ -d "$storage_dir" ]] || continue
        storage_domain="${storage_dir##*/}"
        jq -e --arg deployment "$(jq -r '.deployment_id' "$spec")" --arg domain "$storage_domain" \
          'any(.domains[]; .deployment_id == $deployment and .owner == "provider" and .domain == $domain)' "$registry" >/dev/null || return 1
      done
    fi
    command -v fuser >/dev/null 2>&1 || return 1
    while IFS= read -r role; do
      cert="$(jq -r --arg role "$role" '.certificate_targets[$role].cert_path' "$spec")"
      key="$(jq -r --arg role "$role" '.certificate_targets[$role].key_path' "$spec")"
      caddy_adapter_safe_path "$cert" && caddy_adapter_safe_path "$key" || return 1
      [[ ! -L "$cert" && ! -L "$key" ]] || return 1
      cert_marker="$(dirname "$cert")/.tpn-$(jq -r '.deployment_id' "$spec")-${role}.owner"
      if [[ "${CADDY_ADAPTER_FAKE:-0}" != 1 ]]; then
        [[ "$(cat "$cert_marker" 2>/dev/null)" == "deployment=$(jq -r '.deployment_id' "$spec");role=${role}" ]] || return 1
      fi
      if [[ -e "$cert" || -e "$key" ]]; then
        fuser -s "$cert" "$key" && return 1
      fi
    done < <(jq -r '.active_roles[]' "$spec")
    if jq -e '.active_roles | index("node") != null' "$spec" >/dev/null; then
      consumer="$(jq -r '.roles.node.certificate_consumer' "$spec")"
      core="${CADDY_ADAPTER_NODE_CONTAINER:-trojan-panel-core}"
      caddy_adapter_safe_path "$consumer" || return 1
      [[ ! -L "$consumer" && ! -L "$consumer/fullchain.pem" && ! -L "$consumer/privkey.pem" ]] || return 1
      if [[ "${CADDY_ADAPTER_FAKE:-0}" != 1 ]]; then
        docker="$(caddy_adapter_docker)"
        [[ "$($docker inspect -f '{{.State.Running}}' "$core" 2>/dev/null || true)" != true ]] || return 1
        [[ "$(cat "$consumer/.tpn-$(jq -r '.deployment_id' "$spec")-consumer.owner" 2>/dev/null)" == "deployment=$(jq -r '.deployment_id' "$spec")" ]] || return 1
      fi
      if [[ -e "$consumer/fullchain.pem" || -e "$consumer/privkey.pem" ]]; then
        fuser -s "$consumer/fullchain.pem" "$consumer/privkey.pem" && return 1
      fi
    fi
  fi
  local backup="${root}/.rollback-certs"
  if [[ -e "$backup" ]]; then
    [[ -d "$backup" && ! -L "$backup" &&
       "$(cat "$backup/owner" 2>/dev/null)" == "deployment=$(jq -r '.deployment_id' "$spec")" ]] || return 1
  fi
  if [[ "${CADDY_ADAPTER_FAKE:-0}" != 1 ]]; then
    docker="$(caddy_adapter_docker)"; container="$(caddy_adapter_container)"
    command -v "$docker" >/dev/null 2>&1 || return 1
    caddy_adapter_check_container_owner "$spec" || return 1
    "$docker" rm -f "$container" >/dev/null 2>&1 || return 1
  fi
  caddy_adapter_remove_renewal_trigger "$root" "$spec" || return 1
  rm -f "$config" "$(caddy_adapter_candidate "$root")" "$marker" || return 1
  [[ ! -e "$backup" ]] || rm -rf -- "$backup" || return 1
  if [[ "$purge" == 1 ]]; then rm -rf -- "${root}/data" || return 1; fi
  rm -f "$(caddy_adapter_consumer_registry "$root")"
  if [[ "$purge" == 1 ]]; then
    while IFS= read -r role; do
      [[ -n "$role" ]] || continue
      cert="$(jq -r --arg role "$role" '.certificate_targets[$role].cert_path' "$spec")"
      key="$(jq -r --arg role "$role" '.certificate_targets[$role].key_path' "$spec")"
      cert_marker="$(dirname "$cert")/.tpn-$(jq -r '.deployment_id' "$spec")-${role}.owner"
      rm -f -- "$cert" "$key" "$cert_marker" || return 1
    done < <(jq -r '.active_roles[]' "$spec")
    if [[ -n "${consumer:-}" ]]; then
      rm -f -- "$consumer/fullchain.pem" "$consumer/privkey.pem" "$consumer/.tpn-$(jq -r '.deployment_id' "$spec")-consumer.owner" || return 1
    fi
  fi
  rmdir "$root" 2>/dev/null || true
}
