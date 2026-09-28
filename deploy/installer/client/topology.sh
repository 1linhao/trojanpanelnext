#!/usr/bin/env bash
set -Eeuo pipefail

# A secret-free, read-only planning interface. Output columns are:
# host_id, transport, deployment_mode, node_key, node_state_path.
fail() { printf 'topology: %s\n' "$1" >&2; exit 2; }
usage() { printf 'Usage: %s --config FILE\n' "$0"; }

config=""
while (($#)); do
  case "$1" in
  --config) [[ $# -ge 2 && -z "${config}" ]] || fail 'one --config FILE is required'; config="$2"; shift 2 ;;
  -h | --help) usage; exit 0 ;;
  *) usage >&2; exit 2 ;;
  esac
done
[[ -n "${config}" && -f "${config}" && ! -L "${config}" ]] || fail 'configuration file is missing or unsafe'
command -v yq >/dev/null 2>&1 || fail 'mikefarah yq v4 is required; see https://github.com/mikefarah/yq#install'
[[ "$(yq --version 2>/dev/null)" =~ version[[:space:]]+v?4\. ]] || fail 'mikefarah yq v4 is required'

valid() { yq -e "$1" "${config}" >/dev/null 2>&1 || fail "$2"; }
raw() { yq -r "$1" "${config}" 2>/dev/null || fail 'invalid YAML'; }
scalar() {
  local value
  value="$(raw "$1")"
  [[ "${value}" != null && "${value}" != *$'\n'* && "${value}" != *$'\t'* ]] || fail "invalid value for $1"
  printf '%s' "${value}"
}
identifier() { [[ "$1" =~ ^[a-z][a-z0-9-]{0,62}$ ]] || fail "invalid $2"; }
domain() {
  local name="$1" label last
  local -a labels=()
  [[ "${name}" == *.* && "${name}" != .* && "${name}" != *. && ${#name} -le 253 && "${name}" =~ ^[a-z0-9.-]+$ ]] || fail "invalid $2"
  IFS=. read -r -a labels <<<"${name}"
  for label in "${labels[@]}"; do
    [[ ${#label} -le 63 && "${label}" =~ ^[a-z0-9]([a-z0-9-]*[a-z0-9])?$ ]] || fail "invalid $2"
  done
  last="${labels[${#labels[@]}-1]}"
  [[ "${last}" =~ ^[a-z]{2,}$ ]] || fail "invalid $2"
}
ssh_hostname() {
  local name="$1" label
  local -a labels=()
  if [[ "${name}" == *:* || "${name}" =~ ^[0-9.]+$ ]]; then public_ip "${name}" "$2"; return; fi
  [[ -n "${name}" && "${name}" != .* && "${name}" != *. && ${#name} -le 253 && "${name}" =~ ^[A-Za-z0-9.-]+$ ]] || fail "invalid $2"
  IFS=. read -r -a labels <<<"${name}"
  for label in "${labels[@]}"; do
    [[ ${#label} -le 63 && "${label}" =~ ^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?$ ]] || fail "invalid $2"
  done
}
public_ip() {
  local ip="$1" octet part left right group_count=0
  local -a octets=()
  if [[ "${ip}" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    IFS=. read -r -a octets <<<"${ip}"
    for octet in "${octets[@]}"; do
      [[ ${#octet} -eq 1 || "${octet}" != 0* ]] || fail "invalid $2"
      ((10#${octet} <= 255)) || fail "invalid $2"
    done
    return
  fi
  [[ "${ip}" == *:* && "${ip}" =~ ^[0-9A-Fa-f:]+$ && "${ip}" != :::* && "${ip}" != *:::* && "${ip}" != *::: ]] || fail "invalid $2"
  if [[ "${ip}" == *::* ]]; then
    [[ "${ip}" != :: && "${ip#*::}" != *::* ]] || fail "invalid $2"
    left="${ip%%::*}"; right="${ip#*::}"
    [[ "${left}" != :* && "${left}" != *: && "${right}" != :* && "${right}" != *: ]] || fail "invalid $2"
    for part in "${left}" "${right}"; do
      [[ -z "${part}" ]] && continue
      IFS=: read -r -a octets <<<"${part}"
      for octet in "${octets[@]}"; do [[ "${octet}" =~ ^[0-9A-Fa-f]{1,4}$ ]] || fail "invalid $2"; done
      group_count=$((group_count + ${#octets[@]}))
    done
    ((group_count < 8)) || fail "invalid $2"
  else
    [[ "${ip}" != :* && "${ip}" != *: ]] || fail "invalid $2"
    IFS=: read -r -a octets <<<"${ip}"
    [[ ${#octets[@]} -eq 8 ]] || fail "invalid $2"
    for octet in "${octets[@]}"; do [[ "${octet}" =~ ^[0-9A-Fa-f]{1,4}$ ]] || fail "invalid $2"; done
  fi
}
port() { [[ "$1" =~ ^[0-9]+$ ]] && ((10#$1 >= 1 && 10#$1 <= 65535)) || fail "invalid $2"; }
seen_contains() { [[ "|$1|" == *"|$2|"* ]]; }

valid 'tag == "!!map" and has("schema_version") and has("release") and has("hosts") and has("web") and has("nodes") and (keys | length == 5)' 'invalid top-level schema'
valid '.schema_version == 1 and (.release | tag == "!!map") and (.release | has("tag")) and (.release | has("sha256")) and (.release | keys | length == 2)' 'invalid release schema'
valid '(.hosts | tag == "!!map") and (.hosts | length > 0) and (.nodes | tag == "!!seq")' 'invalid hosts or nodes schema'
valid '(.web | tag == "!!map") and (.web | has("host")) and (.web | has("domain")) and (.web | has("public_ip")) and (.web | has("email")) and (.web | has("settings")) and (.web | has("passwords")) and (.web | keys | length == 6)' 'invalid Web schema'
valid '(.web.settings | tag == "!!map") and (.web.settings | has("mariadb_port")) and (.web.settings | has("redis_port")) and (.web.settings | has("panel_port")) and (.web.settings | has("ui_port")) and (.web.settings | keys | length == 4)' 'invalid Web settings'
valid '(.web.passwords | tag == "!!map") and (.web.passwords | has("mariadb")) and (.web.passwords | has("redis")) and (.web.passwords | has("sysadmin")) and (.web.passwords | keys | length == 3)' 'invalid Web password schema'
valid '(.web.passwords | .mariadb | tag == "!!str") and (.web.passwords | .redis | tag == "!!str") and (.web.passwords | .sysadmin | tag == "!!str")' 'invalid Web password type'

release_tag="$(scalar '.release.tag')"
release_sha="$(scalar '.release.sha256')"
[[ "${release_tag}" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$ && "${release_tag}" != *latest* ]] || fail 'release tag must be fixed and explicit'
[[ "${release_sha}" =~ ^[0-9a-f]{64}$ ]] || fail 'release SHA-256 must be 64 lowercase hex characters'
web_host="$(scalar '.web.host')"
identifier "${web_host}" 'Web host id'
web_domain="$(scalar '.web.domain')"
domain "${web_domain}" 'Web domain'
public_ip "$(scalar '.web.public_ip')" 'Web public IP'
[[ "$(scalar '.web.email')" =~ ^[^[:space:]@]+@[^[:space:]@]+$ ]] || fail 'invalid Web email'
web_ports='|'
for key in mariadb_port redis_port panel_port ui_port; do
  value="$(scalar ".web.settings.${key}")"
  port "${value}" "Web ${key}"
  ! seen_contains "${web_ports}" "${value}" || fail 'Web has a port conflict'
  web_ports+="${value}|"
done

host_ids="|"
local_host=""
host_count=0
host_aliases="|"
while IFS= read -r host_id; do
  identifier "${host_id}" 'host id'
  ! seen_contains "${host_ids}" "${host_id}" || fail "duplicate host id: ${host_id}"
  host_ids+="${host_id}|"
  host_count=$((host_count + 1))
  transport="$(scalar ".hosts.\"${host_id}\".transport")"
  case "${transport}" in
  local)
    valid "(.hosts.\"${host_id}\" | tag == \"!!map\") and (.hosts.\"${host_id}\" | has(\"transport\")) and (.hosts.\"${host_id}\" | keys | length == 1)" "invalid local host ${host_id}"
    [[ "${host_id}" == "${web_host}" && -z "${local_host}" ]] || fail 'only the Web host may use local transport'
    local_host="${host_id}"
    ;;
  ssh)
    valid "(.hosts.\"${host_id}\" | tag == \"!!map\") and (.hosts.\"${host_id}\" | has(\"transport\")) and (.hosts.\"${host_id}\" | has(\"ssh\")) and (.hosts.\"${host_id}\" | keys | length == 2)" "invalid SSH host ${host_id}"
    valid "(.hosts.\"${host_id}\".ssh | tag == \"!!map\") and (.hosts.\"${host_id}\".ssh | has(\"user\")) and (.hosts.\"${host_id}\".ssh | has(\"hostname\")) and (.hosts.\"${host_id}\".ssh | has(\"port\")) and (.hosts.\"${host_id}\".ssh | has(\"identity_file\")) and (.hosts.\"${host_id}\".ssh | has(\"config_file\")) and (.hosts.\"${host_id}\".ssh | keys | length == 5)" "invalid SSH target ${host_id}"
    user="$(scalar ".hosts.\"${host_id}\".ssh.user")"
    hostname="$(scalar ".hosts.\"${host_id}\".ssh.hostname")"
    [[ "${user}" =~ ^[A-Za-z_][A-Za-z0-9_-]*$ ]] || fail "invalid SSH user for ${host_id}"
    ssh_hostname "${hostname}" "SSH hostname for ${host_id}"
    port "$(scalar ".hosts.\"${host_id}\".ssh.port")" "SSH port for ${host_id}"
    alias="${hostname}:$(scalar ".hosts.\"${host_id}\".ssh.port")"
    ! seen_contains "${host_aliases}" "${alias}" || fail 'two host ids declare the same SSH endpoint'
    host_aliases+="${alias}|"
    valid "(.hosts.\"${host_id}\".ssh.identity_file | tag == \"!!str\") and (.hosts.\"${host_id}\".ssh.config_file | tag == \"!!str\")" "invalid SSH file path for ${host_id}"
    ;;
  *) fail "invalid transport for ${host_id}" ;;
  esac
done < <(raw '.hosts | keys | sort | .[]')
seen_contains "${host_ids}" "${web_host}" || fail 'Web references an undeclared host'

node_keys="|"
node_names="|"
node_domains="|${web_domain}|"
node_hosts="|"
combined_key=""
node_count="$(raw '.nodes | length')"
[[ "${node_count}" =~ ^[0-9]+$ ]] || fail 'invalid nodes array'
for ((i = 0; i < node_count; i++)); do
  base=".nodes[${i}]"
  valid "(${base} | tag == \"!!map\") and (${base} | has(\"node_key\")) and (${base} | has(\"host\")) and (${base} | has(\"name\")) and (${base} | has(\"domain\")) and (${base} | has(\"public_ip\")) and (${base} | has(\"settings\")) and (${base} | has(\"passwords\")) and (${base} | keys | length == 7)" 'invalid Node schema'
  valid "(${base}.settings | tag == \"!!map\") and (${base}.settings | has(\"grpc_port\")) and (${base}.settings | has(\"core_port\")) and (${base}.settings | has(\"node_caddy_http_port\")) and (${base}.settings | has(\"node_caddy_https_port\")) and (${base}.settings | keys | length == 4)" 'invalid Node settings'
  valid "(${base}.passwords | tag == \"!!map\") and (${base}.passwords | has(\"bundle\")) and (${base}.passwords | keys | length == 1) and (${base}.passwords.bundle | tag == \"!!str\")" 'invalid Node password schema'
  node_key="$(scalar "${base}.node_key")"
  node_host="$(scalar "${base}.host")"
  node_name="$(scalar "${base}.name")"
  node_domain="$(scalar "${base}.domain")"
  identifier "${node_key}" 'node_key'
  identifier "${node_host}" 'Node host id'
  identifier "${node_name}" 'Node name'
  domain "${node_domain}" 'Node domain'
  public_ip "$(scalar "${base}.public_ip")" 'Node public IP'
  seen_contains "${host_ids}" "${node_host}" || fail "Node ${node_key} references an undeclared host"
  ! seen_contains "${node_keys}" "${node_key}" || fail "duplicate node_key: ${node_key}"
  ! seen_contains "${node_names}" "${node_name}" || fail "duplicate Node name: ${node_name}"
  ! seen_contains "${node_domains}" "${node_domain}" || fail "duplicate Web/Node domain: ${node_domain}"
  ! seen_contains "${node_hosts}" "${node_host}" || fail "multiple Nodes reference host ${node_host}"
  node_keys+="${node_key}|"; node_names+="${node_name}|"; node_domains+="${node_domain}|"; node_hosts+="${node_host}|"
  ports="|"
  for key in grpc_port core_port node_caddy_http_port node_caddy_https_port; do
    value="$(scalar "${base}.settings.${key}")"
    port "${value}" "Node ${node_key} ${key}"
    ! seen_contains "${ports}" "${value}" || fail "Node ${node_key} has a port conflict"
    ports+="${value}|"
    if [[ "${node_host}" == "${web_host}" ]]; then
      for web_key in mariadb_port redis_port panel_port ui_port; do
        [[ "${value}" != "$(scalar ".web.settings.${web_key}")" ]] || fail "combined host has a port conflict: ${value}"
      done
    fi
  done
  if [[ "${node_host}" == "${web_host}" ]]; then
    [[ -z "${combined_key}" ]] || fail 'only one Web-local Node is supported'
    [[ "$(scalar "${base}.settings.node_caddy_http_port")" == 80 && "$(scalar "${base}.settings.node_caddy_https_port")" == 443 ]] || fail 'combined Entry requires shared ports 80/443'
    combined_key="${node_key}"
  fi
done

expected_host_count=$((node_count + 1))
[[ -z "${combined_key}" ]] || expected_host_count=$((expected_host_count - 1))
[[ "${host_count}" -eq "${expected_host_count}" ]] || fail 'unused or multiply assigned host id'
# A stable host-keyed plan. Array position is deliberately absent from state paths.
while IFS= read -r host_id; do
  transport="$(scalar ".hosts.\"${host_id}\".transport")"
  mode=node
  key=""
  [[ "${host_id}" != "${web_host}" ]] || { mode=web; [[ -z "${combined_key}" ]] || { mode=combined; key="${combined_key}"; }; }
  if [[ "${mode}" == node ]]; then
    for ((i = 0; i < node_count; i++)); do
      if [[ "$(scalar ".nodes[${i}].host")" == "${host_id}" ]]; then key="$(scalar ".nodes[${i}].node_key")"; break; fi
    done
  fi
  state=""
  [[ -z "${key}" ]] || state="nodes/${key}.json"
  printf '%s\t%s\t%s\t%s\t%s\n' "${host_id}" "${transport}" "${mode}" "${key}" "${state}"
done < <(raw '.hosts | keys | sort | .[]')
