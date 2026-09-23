#!/usr/bin/env bash

# Caddy legacy Adapter for combined EntrySpec v2 deployments.  All host
# operations are behind small functions so contract tests can replace Docker,
# socket and certificate observations without touching a real VPS.

caddy_adapter_safe_path() {
  [[ "${1:-}" == /* && "${1:-}" != *[$'\n\r\t;{}']* ]]
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
    facts="$(stat -c '%d:%i:%u:%g:%a' "$id")"
    while IFS= read -r -d '' child; do
      if [[ -f "$child" ]]; then
        facts+="$(printf '%s:' "${child#"$id"/}")$(sha256sum "$child" | awk '{print $1}')"
      else
        facts+="$(printf '%s:' "${child#"$id"/}")$(stat -c '%F:%a:%u:%g' "$child")"
      fi
    done < <(find "$id" -mindepth 1 -maxdepth 2 -print0 2>/dev/null | sort -z)
    ;;
  container)
    if [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]]; then
      facts="fake:${id}:$(cat "$(caddy_adapter_marker "$root")" 2>/dev/null || true)"
    else
      local docker="$(caddy_adapter_docker)" image label running
      command -v "$docker" >/dev/null 2>&1 || return 1
      "$docker" inspect "$id" >/dev/null 2>&1 || return 1
      label="$($docker inspect -f '{{ index .Config.Labels "io.trojanpanelnext.deployment" }}' "$id" 2>/dev/null)" || return 1
      image="$($docker inspect -f '{{.Config.Image}}' "$id" 2>/dev/null)" || return 1
      running="$($docker inspect -f '{{.State.Running}}' "$id" 2>/dev/null)" || return 1
      facts="${id}:${label}:${image}:${running}"
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
  container="$(caddy_adapter_container)"
  if [[ "${include_files}" == 1 ]]; then
    [[ -f "${root%/}/Caddyfile" ]] && caddy_adapter_resource_json "$deployment" file "${root%/}/Caddyfile" shared managed '' "$root"
    [[ -d "${root%/}/data" ]] && caddy_adapter_resource_json "$deployment" directory "${root%/}/data" shared managed '' "$root"
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
    [[ -f "$cert_path" ]] && caddy_adapter_resource_json "$deployment" certificate "$cert_path" role managed "$role" "$root"
    [[ -f "$key_path" ]] && caddy_adapter_resource_json "$deployment" certificate "$key_path" role managed "$role" "$root"
  done <<<"$roles"
}

caddy_adapter_render() {
  local spec="$1" role domain upstream
  local deployment
  deployment="$(jq -r '.deployment_id' "$spec")"
  while IFS= read -r role; do
    [[ -n "$role" ]] || continue
    domain="$(jq -r --arg role "$role" '.domains[$role]' "$spec")"
    caddy_adapter_safe_domain "$domain" || return 2
    case "$role" in
    web)
      upstream="$(jq -r '.roles.web.web_upstream' "$spec")"
      caddy_adapter_safe_upstream "$upstream" || return 2
      printf '%s {\n    reverse_proxy %s\n}\n\n' "$domain" "$upstream"
      ;;
    node)
      printf '%s {\n    root * /srv\n    file_server\n}\n\n' "$domain"
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
  elif command -v caddy >/dev/null 2>&1; then
    local temp
    temp="$(mktemp)" || return 2
    printf '%s' "$content" >"$temp"
    caddy validate --config "$temp" --adapter caddyfile >/dev/null 2>&1
    local status=$?
    rm -f "$temp"
    return "$status"
  elif [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]]; then
    return 0
  else
    return 1
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
  [[ -d "$root" ]] || return 2
  if [[ ! -f "$marker" ]]; then
    find "$root" -mindepth 1 -maxdepth 1 -print -quit | grep -q . && return 3
    printf 'empty\n'
    return 0
  fi
  [[ "$(cat "$marker" 2>/dev/null)" == "deployment=${deployment}" ]] || return 3
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
  [[ -n "${CADDY_ADAPTER_PORT_CHECK_CMD:-}" ]] && bash -c "$CADDY_ADAPTER_PORT_CHECK_CMD" && return
  command -v ss >/dev/null 2>&1 || return 1
  local out
  out="$(ss -Hlnpt '( sport = :80 or sport = :443 )' 2>/dev/null || true)"
  [[ -z "$out" ]] || [[ "$out" == *"$(caddy_adapter_container)"* ]]
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
  [[ "$label" == "$deployment" ]]
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
  openssl verify -purpose sslserver -CAfile "$cert" "$cert" >/dev/null 2>&1 || return 1
  openssl x509 -in "$cert" -noout -text 2>/dev/null | grep -Eq "DNS:${domain}([,[:space:]]|$)" || return 1
}

caddy_adapter_certificate_json() {
  local spec="$1" role="$2" generation="${3:-1}" cert key domain fingerprint not_after
  domain="$(jq -r --arg role "$role" '.domains[$role]' "$spec")"
  cert="$(jq -r --arg role "$role" '.certificate_targets[$role].cert_path' "$spec")"
  key="$(jq -r --arg role "$role" '.certificate_targets[$role].key_path' "$spec")"
  caddy_adapter_probe_cert "$domain" "$cert" "$key" || return 1
  fingerprint="$(openssl x509 -in "$cert" -noout -fingerprint -sha256 | sed 's/.*=//; s/://g' | tr '[:upper:]' '[:lower:]')"
  not_after="$(openssl x509 -in "$cert" -noout -enddate | sed 's/^notAfter=//')"
  not_after="$(date -u -d "$not_after" '+%Y-%m-%dT%H:%M:%SZ')" || return 1
  jq -cn --arg domain "$domain" --arg cert "$cert" --arg key "$key" --arg fp "$fingerprint" --arg expiry "$not_after" \
    --argjson generation "$generation" '{domain:$domain,cert_path:$cert,key_path:$key,fingerprint:$fp,generation:$generation,renewal_owner:"caddy-legacy",last_hook_status:"ok",not_after:$expiry}'
}

caddy_adapter_sync_certificates() {
  local spec="$1" root="$2" role domain target_cert target_key source_dir source_cert source_key
  [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]] && return 0
  while IFS= read -r role; do
    [[ -n "$role" ]] || continue
    domain="$(jq -r --arg role "$role" '.domains[$role]' "$spec")"
    target_cert="$(jq -r --arg role "$role" '.certificate_targets[$role].cert_path' "$spec")"
    target_key="$(jq -r --arg role "$role" '.certificate_targets[$role].key_path' "$spec")"
    if [[ -s "$target_cert" && -s "$target_key" ]]; then continue; fi
    source_dir="${root%/}/data/caddy/certificates/acme-v02.api.letsencrypt.org-directory/${domain}"
    source_cert="${source_dir}/${domain}.crt"
    source_key="${source_dir}/${domain}.key"
    [[ -s "$source_cert" && -s "$source_key" ]] || return 1
    mkdir -p "$(dirname "$target_cert")" "$(dirname "$target_key")" || return 1
    install -m 0644 "$source_cert" "$target_cert" || return 1
    install -m 0600 "$source_key" "$target_key" || return 1
  done < <(jq -r '.active_roles[]' "$spec")
}

caddy_adapter_ensure_container() {
  local spec="$1" root="$2" docker container config
  docker="$(caddy_adapter_docker)"; container="$(caddy_adapter_container)"; config="$(caddy_adapter_config "$root")"
  if [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]]; then
    : >"${root}/.active"
    return 0
  fi
  command -v "$docker" >/dev/null 2>&1 || return 1
  if "$docker" inspect "$container" >/dev/null 2>&1; then
    caddy_adapter_check_container_owner "$spec"
    return $?
  fi
  "$docker" create --name "$container" --restart always --network host \
    --label "io.trojanpanelnext.deployment=$(jq -r '.deployment_id' "$spec")" \
    -v "${config}:/etc/caddy/Caddyfile:ro" -v "${root}/data:/data" -v "${root}:/config" \
    -v "${CADDY_ADAPTER_WEB_ROOT:-/tpdata/web}:/srv:ro" "${CADDY_ADAPTER_IMAGE:-caddy:2.8.4}" >/dev/null
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
    local port count
    for port in 80 443; do
      count="$(ss -Hlnpt "( sport = :${port} )" 2>/dev/null | wc -l)"
      [[ "$count" == 1 ]] || return 1
    done
  fi
  printf '%s\n' \
    '{"transport":"tcp","address":"0.0.0.0","port":80,"purpose":"acme-http01","owner":"provider","scope":"shared"}' \
    '{"transport":"tcp","address":"0.0.0.0","port":443,"purpose":"web-https","owner":"provider","scope":"shared"}'
  if jq -e '.active_roles | index("node") != null' "$1" >/dev/null; then
    local manifest port
    manifest="$(jq -r '.roles.node.route_manifest' "$1")"
    if [[ -s "$manifest" ]] && command -v jq >/dev/null 2>&1; then
      jq -c '.routes[]? | select(.transport == "tcp" or .transport == "udp") | {transport,address:(.address // "0.0.0.0"),port,purpose:"node-direct",owner:"kernel",scope:"role",role:"node"}' "$manifest" 2>/dev/null || true
    fi
    port="${CADDY_ADAPTER_NODE_PORT:-8443}"
    [[ "$port" =~ ^[1-9][0-9]{0,4}$ ]] && ((10#$port <= 65535)) && printf '{"transport":"tcp","address":"127.0.0.1","port":%s,"purpose":"node-direct","owner":"kernel","scope":"role","role":"node"}\n' "$port"
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
  local spec="$1" state="${2:-null}" root status observation_spec="$1" temporary=""
  root="$(caddy_adapter_root "$spec")"
  caddy_adapter_safe_path "$root" || return 1
  caddy_adapter_owner_status "$spec" "$root" >/dev/null || return 1
  caddy_adapter_check_container_owner "$spec" || return 1
  caddy_adapter_check_dns "$spec" || return 1
  caddy_adapter_check_ports || return 1
  status="$(caddy_adapter_owner_status "$spec" "$root")"
  # A role removal probes the last committed target so the controller can
  # compare every currently owned resource before staging the smaller target.
  if [[ "$state" != null && "$(jq -r '.committed_target == null' <<<"$state")" == false ]]; then
    temporary="$(mktemp)" || return 1
    jq -c '.committed_target.spec' <<<"$state" >"$temporary" || { rm -f "$temporary"; return 1; }
    observation_spec="$temporary"
  fi
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
  local spec="$1" state="$2" root candidate content marker
  root="$(caddy_adapter_root "$spec")"
  candidate="$(caddy_adapter_candidate "$root")"
  content="$(caddy_adapter_render "$spec")" || return 1
  caddy_adapter_validate_render "$content" || return 1
  mkdir -p "$root" "$root/data" || return 1
  marker="$(caddy_adapter_marker "$root")"
  printf 'deployment=%s\n' "$(jq -r '.deployment_id' "$spec")" >"$marker" || return 1
  printf '%s' "$content" >"$candidate" || return 1
  mv -f "$candidate" "$(caddy_adapter_config "$root")" || return 1
  caddy_adapter_ensure_container "$spec" "$root" || return 1
  chmod 0600 "$candidate" "$marker" 2>/dev/null || true
  caddy_adapter_observation "$spec" "$root" 1 | jq --argjson resources "$(caddy_adapter_resource_list "$spec" "$root" "$(jq -r '.deployment_id' "$spec")" "$(jq -r '.active_roles[]' "$spec")" 1 | jq -s '.')" '.candidate_resources=$resources'
}

entry_v2_adapter_activate() {
  local spec="$1" state="$2" root config candidate docker container
  root="$(caddy_adapter_root "$spec")"
  config="$(caddy_adapter_config "$root")"
  candidate="$(caddy_adapter_candidate "$root")"
  [[ -s "$config" ]] || return 1
  docker="$(caddy_adapter_docker)"
  container="$(caddy_adapter_container)"
  if [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]]; then
    : >"${root}/.active"
    return 0
  fi
  command -v "$docker" >/dev/null 2>&1 || return 1
  if "$docker" inspect "$container" >/dev/null 2>&1; then
    if [[ "$($docker inspect -f '{{.State.Running}}' "$container" 2>/dev/null)" == true ]]; then
      "$docker" restart "$container" >/dev/null
    else
      "$docker" start "$container" >/dev/null
    fi
  else
    "$docker" run -d --name "$container" --restart always --network host \
      --label "io.trojanpanelnext.deployment=$(jq -r '.deployment_id' "$spec")" \
      -v "${config}:/etc/caddy/Caddyfile:ro" -v "${root}/data:/data" -v "${root}:/config" -v "${CADDY_ADAPTER_WEB_ROOT:-/tpdata/web}:/srv:ro" \
      "${CADDY_ADAPTER_IMAGE:-caddy:2.8.4}" >/dev/null || return 1
  fi
  caddy_adapter_wait_for_certificates "$spec" "$root" || return 1
}

caddy_adapter_verify_runtime() {
  local spec="$1" docker container running health domain role upstream
  [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]] && return 0
  caddy_adapter_check_container_owner "$spec" || return 1
  docker="$(caddy_adapter_docker)"; container="$(caddy_adapter_container)"
  running="$($docker inspect -f '{{.State.Running}}' "$container" 2>/dev/null)" || return 1
  [[ "$running" == true ]] || return 1
  health="$($docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$container" 2>/dev/null)" || return 1
  [[ "$health" != unhealthy ]] || return 1
  if jq -e '.active_roles | index("web") != null' "$spec" >/dev/null; then
    upstream="$(jq -r '.roles.web.web_upstream' "$spec")"
    grep -Fqx "    reverse_proxy ${upstream}" "$(caddy_adapter_config "$(caddy_adapter_root "$spec")")" || return 1
  fi
  command -v curl >/dev/null 2>&1 || return 1
  while IFS= read -r role; do
    [[ -n "$role" ]] || continue
    domain="$(jq -r --arg role "$role" '.domains[$role]' "$spec")"
    curl --fail --silent --show-error --insecure --max-time "${CADDY_ADAPTER_CURL_TIMEOUT:-5}" \
      --resolve "${domain}:443:127.0.0.1" "https://${domain}/" -o /dev/null || return 1
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
  caddy_adapter_observation "$spec" "$root" 1 | jq --argjson resources "$(caddy_adapter_resource_list "$spec" "$root" "$(jq -r '.deployment_id' "$spec")" "$(jq -r '.active_roles[]' "$spec")" 1 | jq -s '.')" '.resources=$resources | .candidate_resources=$resources'
}

entry_v2_adapter_rollback() {
  local spec="$1" state="$2" root config candidate docker container
  root="$(caddy_adapter_root "$spec")"
  config="$(caddy_adapter_config "$root")"
  candidate="$(caddy_adapter_candidate "$root")"
  rm -f "$candidate"
  if [[ "$(jq -r '.committed_target == null' <<<"$state")" == true ]]; then
    rm -f "$config" "$(caddy_adapter_marker "$root")"
    rmdir "${root}/data" 2>/dev/null || true
    rmdir "$root" 2>/dev/null || true
    if [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]]; then
      rm -f "${root}/.active"
    else
      docker="$(caddy_adapter_docker)"; container="$(caddy_adapter_container)"
      caddy_adapter_check_container_owner "$spec" || return 1
      "$docker" rm -f "$container" >/dev/null 2>&1 || return 1
    fi
    return 0
  fi
  local old_spec
  old_spec="$(mktemp)" || return 1
  jq -c '.committed_target.spec' <<<"$state" >"$old_spec" || { rm -f "$old_spec"; return 1; }
  local old_content
  old_content="$(caddy_adapter_render "$old_spec")" || { rm -f "$old_spec"; return 1; }
  printf '%s' "$old_content" >"$config" || { rm -f "$old_spec"; return 1; }
  rm -f "$old_spec"
  if [[ "${CADDY_ADAPTER_FAKE:-0}" == 1 ]]; then return 0; fi
  docker="$(caddy_adapter_docker)"; container="$(caddy_adapter_container)"
  command -v "$docker" >/dev/null 2>&1 || return 1
  "$docker" restart "$container" >/dev/null 2>&1
}

entry_v2_adapter_remove() {
  local spec="$1" state="$2" purge="${3:-0}" root config marker docker container
  root="$(caddy_adapter_root "$spec")"; config="$(caddy_adapter_config "$root")"; marker="$(caddy_adapter_marker "$root")"
  caddy_adapter_validate_remove_ownership "$spec" "$state" || return 1
  if [[ "${CADDY_ADAPTER_FAKE:-0}" != 1 ]]; then
    docker="$(caddy_adapter_docker)"; container="$(caddy_adapter_container)"
    command -v "$docker" >/dev/null 2>&1 || return 1
    "$docker" rm -f "$container" >/dev/null 2>&1 || true
  fi
  rm -f "$config" "$marker" || return 1
  if [[ "$purge" == 1 ]]; then rm -rf "${root}/data"; fi
  if [[ "$purge" == 1 ]]; then
    while IFS= read -r role; do
      [[ -n "$role" ]] || continue
      rm -f "$(jq -r --arg role "$role" '.certificate_targets[$role].cert_path' "$spec")" \
        "$(jq -r --arg role "$role" '.certificate_targets[$role].key_path' "$spec")"
    done < <(jq -r '.active_roles[]' "$spec")
  fi
  rmdir "$root" 2>/dev/null || true
}
