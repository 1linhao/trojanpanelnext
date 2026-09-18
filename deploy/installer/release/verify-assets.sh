#!/usr/bin/env bash
set -Eeuo pipefail

fail() {
  printf 'release assets: %s\n' "$1" >&2
  exit 1
}

assets_dir=""
config=""
while (($#)); do
  case "$1" in
  --assets-dir) [[ $# -ge 2 ]] || fail '--assets-dir requires a value'; assets_dir="$2"; shift 2 ;;
  --config) [[ $# -ge 2 ]] || fail '--config requires a value'; config="$2"; shift 2 ;;
  *) fail "unknown argument: $1" ;;
  esac
done
[[ -n "${assets_dir}" && -d "${assets_dir}" ]] || fail 'asset directory not found'
[[ -n "${config}" && -f "${config}" ]] || fail 'configuration file not found'
command -v jq >/dev/null 2>&1 || fail 'jq is required'
command -v sha256sum >/dev/null 2>&1 || fail 'sha256sum is required'

manifest="${assets_dir}/release-manifest.json"
sums="${assets_dir}/SHA256SUMS"
[[ -f "${manifest}" && ! -L "${manifest}" ]] || fail 'release-manifest.json is missing or unsafe'
[[ -f "${sums}" && ! -L "${sums}" ]] || fail 'SHA256SUMS is missing or unsafe'

jq -e '
  .schema_version == 1 and
  (.release_version | type == "string" and test("^[0-9]+\\.[0-9]+\\.[0-9]+([+-][0-9A-Za-z.-]+)?$")) and
  (.source_commit | type == "string" and test("^[0-9a-f]{40}$")) and
  (.assets | type == "array" and length == 6) and
  ([.assets[].name] | unique | length) == 6 and
  ([.assets[].path] | unique | length) == 6 and
  (all(.assets[];
    (.name | type == "string" and length > 0) and
    (.path | type == "string" and test("^[A-Za-z0-9][A-Za-z0-9._-]*$")) and
    (.sha256 | type == "string" and test("^[0-9a-f]{64}$")))) and
  (.images | keys | sort) == ["api", "caddy", "mariadb", "node_agent", "redis", "web"] and
  (all(.images[];
    (.kind == "product" or .kind == "runtime") and
    (.name | type == "string" and length > 0) and
    (.digest | type == "string" and test("^sha256:[0-9a-f]{64}$")) and
    .reference == (.name + "@" + .digest))) and
  (.attestations | type == "array" and length == 3) and
  ([.attestations[].subject] | unique | length) == 3 and
  (all(.attestations[]; (.subject | type == "string") and (.digest | type == "string" and test("^sha256:[0-9a-f]{64}$")))) and
  (. as $root | all(.images[] | select(.kind == "product"); . as $image | any($root.attestations[]; .subject == $image.name and .digest == $image.digest)))
' "${manifest}" >/dev/null || fail 'manifest structure is invalid'

required='["bootstrap.sh","config-combined.yaml","config-node.yaml","config-web.yaml","install.sh","verify-assets.sh"]'
jq -e --argjson required "${required}" '([.assets[].path] | sort) == $required' "${manifest}" >/dev/null ||
  fail 'manifest asset set is incomplete'

while IFS=$'\t' read -r path expected; do
  file="${assets_dir}/${path}"
  [[ -f "${file}" && ! -L "${file}" ]] || fail "asset is missing or unsafe: ${path}"
  actual="$(sha256sum "${file}" | awk '{print $1}')"
  [[ "${actual}" == "${expected}" ]] || fail "asset digest mismatch: ${path}"
done < <(jq -r '.assets[] | [.path, .sha256] | @tsv' "${manifest}")

mapfile -t sum_paths < <(awk '{print $2}' "${sums}" | sed 's/^\*\?//')
[[ "${#sum_paths[@]}" -eq 7 ]] || fail 'SHA256SUMS has an unexpected asset set'
printf '%s\n' "${sum_paths[@]}" | sort -u | cmp -s - <(printf '%s\n' bootstrap.sh config-combined.yaml config-node.yaml config-web.yaml install.sh release-manifest.json verify-assets.sh | sort) ||
  fail 'SHA256SUMS has an unexpected asset set'
(
  cd "${assets_dir}"
  sha256sum -c SHA256SUMS >/dev/null
) || fail 'SHA256SUMS verification failed'

grep -qx 'trojanpanelnext:' "${config}" || fail 'configuration must contain one trojanpanelnext root'
config_count() {
  local key="$1"
  awk -F: -v key="${key}" '$1 == "  " key {count++} END {print count + 0}' "${config}"
}
config_value() {
  local key="$1"
  awk -F: -v key="${key}" '$1 == "  " key {sub(/^[^:]*:[[:space:]]*/, ""); gsub(/^"|"$/, ""); print; exit}' "${config}"
}
for key in schema_version asset_version purpose panel_image ui_image core_image caddy_image mariadb_image redis_image; do
  [[ "$(config_count "${key}")" == 1 ]] || fail "configuration key must appear exactly once: ${key}"
done

[[ "$(config_value schema_version)" == 1 ]] || fail 'configuration schema_version must be 1'
version="$(config_value asset_version)"
manifest_version="$(jq -r '.release_version' "${manifest}")"
[[ -n "${version}" && "${version}" == "${manifest_version}" ]] || fail 'configuration asset_version does not match release'
purpose="$(config_value purpose)"
case "${purpose}" in web | node | combined) ;; *) fail "unsupported configuration purpose: ${purpose}" ;; esac

while IFS=$'\t' read -r config_key manifest_key; do
  configured="$(config_value "${config_key}")"
  expected="$(jq -r --arg key "${manifest_key}" '.images[$key].reference' "${manifest}")"
  [[ "${configured}" == "${expected}" ]] || fail "configuration image does not match manifest: ${config_key}"
done <<'IMAGE_KEYS'
panel_image	api
ui_image	web
core_image	node_agent
caddy_image	caddy
mariadb_image	mariadb
redis_image	redis
IMAGE_KEYS

printf 'Release assets are valid for %s purpose (version %s)\n' "${purpose}" "${version}"
