#!/usr/bin/env bash
set -Eeuo pipefail

# Sent over stdin to one local or SSH host. Every operation below is a read;
# output is a small, secret-free fact protocol for check.sh.
mode="${1:?mode}"; ports="${2:?ports}"; web_domain="${3:?web domain}"
node_domain="${4:-}"; release="${5:?release}"
emit() { printf '%s\t%s\n' "$1" "$2"; }
have() { if command -v "$1" >/dev/null 2>&1; then emit "dependency.$1" ok; else emit "dependency.$1" missing; fi; }

emit privilege "$(id -u)"
for tool in bash curl tar docker jq ss df awk sha256sum; do have "$tool"; done
if [[ -r /etc/machine-id ]] && command -v sha256sum >/dev/null 2>&1; then
  emit machine "$(sha256sum /etc/machine-id | awk '{print $1}')"
else emit machine unverified; fi
if command -v nproc >/dev/null 2>&1; then emit cpu "$(nproc)"; else emit cpu unverified; fi
if [[ -r /proc/meminfo ]]; then emit memory_kib "$(awk '/^MemTotal:/ {print $2; exit}' /proc/meminfo)"; else emit memory_kib unverified; fi
if command -v df >/dev/null 2>&1; then
  emit disk_available_kib "$(df -Pk /tpdata 2>/dev/null | awk 'NR == 2 {print $4}' || df -Pk / | awk 'NR == 2 {print $4}')"
else emit disk_available_kib unverified; fi

if command -v ss >/dev/null 2>&1; then
  listeners="$(ss -H -ltn 2>/dev/null | awk '{print $4}' || true)"
  IFS=, read -r -a wanted <<<"${ports}"
  for port in "${wanted[@]}"; do
    if printf '%s\n' "${listeners}" | awk -v port="${port}" '$0 ~ (":" port "$") {found=1} END {exit !found}'; then
      emit "listener.${port}" occupied
    else emit "listener.${port}" free; fi
  done
else
  IFS=, read -r -a wanted <<<"${ports}"
  for port in "${wanted[@]}"; do emit "listener.${port}" unverified; done
fi

data=/tpdata/trojanpanelnext
if [[ -L "${data}" ]]; then emit data_root conflict-symlink
elif [[ ! -e "${data}" ]]; then emit data_root absent
elif [[ ! -d "${data}" ]]; then emit data_root conflict-type
elif [[ ! -f "${data}/.trojanpanelnext-data-root" || -L "${data}/.trojanpanelnext-data-root" ]]; then emit data_root conflict-unowned
elif [[ "$(<"${data}/.trojanpanelnext-data-root")" == trojanpanelnext-data-root-v1 ]]; then emit data_root owned
else emit data_root conflict-marker; fi

state_dir="${data}/trojanpanelnext-installer"
for kind in web node combined; do
  file="${state_dir}/${kind}.state"
  if [[ -L "${file}" ]]; then emit "installer.${kind}" conflict-symlink
  elif [[ ! -e "${file}" ]]; then emit "installer.${kind}" absent
  elif [[ ! -f "${file}" ]]; then emit "installer.${kind}" conflict-type
  elif [[ "$(sed -n 's/^schema_version=//p' "${file}" | head -1)" != 1 || "$(sed -n 's/^mode=//p' "${file}" | head -1)" != "${kind}" ]]; then
    emit "installer.${kind}" conflict-state
  else
    emit "installer.${kind}" present
    for key in domain web_domain node_domain node_name node_public_ip asset_version; do
      value="$(sed -n "s/^${key}=//p" "${file}" | head -1)"
      [[ -z "${value}" ]] || emit "installer.${kind}.${key}" "${value}"
    done
  fi
done

if command -v docker >/dev/null 2>&1 && docker info --format '{{.ServerVersion}}' >/dev/null 2>&1; then
  emit docker available
  for name in trojan-panel-mariadb trojan-panel-redis trojan-panel trojan-panel-ui trojan-panel-core trojan-panel-web-caddy trojan-panel-node-caddy; do
    if docker container inspect "${name}" >/dev/null 2>&1; then
      owner="$(docker inspect -f '{{ index .Config.Labels "io.trojanpanelnext.deployment" }}' "${name}" 2>/dev/null || true)"
      if [[ "${owner}" == trojanpanelnext-combined-entry ]]; then emit "container.${name}" combined-owned
      else emit "container.${name}" present-unverified-owner; fi
    else emit "container.${name}" absent; fi
  done
  emit images "$(docker images -q 2>/dev/null | awk 'END {print NR}')"
else emit docker unverified; fi

if command -v getent >/dev/null 2>&1; then
  for domain in "${web_domain}" "${node_domain}"; do
    [[ -n "${domain}" ]] || continue
    if getent ahosts "${domain}" >/dev/null 2>&1; then emit "dns.${domain}" resolved
    else emit "dns.${domain}" unresolved; fi
  done
else
  for domain in "${web_domain}" "${node_domain}"; do [[ -z "${domain}" ]] || emit "dns.${domain}" unverified; done
fi
if command -v curl >/dev/null 2>&1; then
  if curl -fIsS --connect-timeout 3 --max-time 8 "https://github.com/1linhao/trojanpanelnext/releases/tag/${release}" >/dev/null 2>&1; then emit release reachable
  else emit release unverified; fi
  code="$(curl -sS -o /dev/null -w '%{http_code}' --connect-timeout 3 --max-time 8 https://ghcr.io/v2/ 2>/dev/null || true)"
  case "${code}" in 200|401) emit registry reachable ;; *) emit registry unverified ;; esac
else emit release unverified; emit registry unverified; fi

# The retained release contains EntryController's read-only status Interface.
asset="$(sed -n 's/^asset_version=//p' "${state_dir}/${mode}.state" 2>/dev/null | head -1 || true)"
entryctl="${data}/releases/${asset}/entry/entryctl.sh"
if [[ -n "${asset}" && -f "${entryctl}" && ! -L "${entryctl}" && -f "${data}/releases/${asset}/.trojanpanelnext-release" ]]; then
  if bash "${entryctl}" status --deployment trojanpanelnext-combined-entry >/dev/null 2>&1; then emit entry combined-observed
  else emit entry unverified; fi
else emit entry unverified; fi
