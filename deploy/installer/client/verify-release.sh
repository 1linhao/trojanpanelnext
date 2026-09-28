#!/usr/bin/env bash
set -Eeuo pipefail

# This trusted client-side verifier never executes a file from the archive.
fail() { printf 'release verification: %s\n' "$1" >&2; exit 1; }
archive=""; tag=""; expected_sha=""; assets=""
while (($#)); do
  case "$1" in
  --archive) archive="${2:-}"; shift 2 ;;
  --tag) tag="${2:-}"; shift 2 ;;
  --sha256) expected_sha="${2:-}"; shift 2 ;;
  --assets-dir) assets="${2:-}"; shift 2 ;;
  *) fail 'unknown argument' ;;
  esac
done
[[ -f "${archive}" && ! -L "${archive}" && -n "${assets}" && ! -e "${assets}" ]] || fail 'archive or output path is unsafe'
[[ "${tag}" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$ && "${tag}" != *latest* ]] || fail 'tag must be fixed'
[[ "${expected_sha}" =~ ^[0-9a-f]{64}$ ]] || fail 'SHA-256 is required'
if command -v sha256sum >/dev/null 2>&1; then
  actual_sha="$(sha256sum "${archive}" | awk '{print $1}')"
  checksum() { sha256sum "$1" | awk '{print $1}'; }
elif command -v shasum >/dev/null 2>&1; then
  actual_sha="$(shasum -a 256 "${archive}" | awk '{print $1}')"
  checksum() { shasum -a 256 "$1" | awk '{print $1}'; }
else
  fail 'install sha256sum or shasum for SHA-256 verification'
fi
[[ "${actual_sha}" == "${expected_sha}" ]] || fail 'archive SHA-256 mismatch'

# Reject traversal and link entries before extraction, including entries that
# could redirect a later member outside the private staging directory.
members="$(tar -tzf "${archive}" 2>/dev/null)" || fail 'archive listing failed'
while IFS= read -r member; do
  member="${member#./}"
  [[ -z "${member}" || "${member}" == . ]] && continue
  [[ "${member}" != /* && "${member}" != *'..'* && "${member}" != *'\\'* && "${member}" != *$'\r'* && "${member}" != *$'\t'* && "${member}" != *$'\n'* ]] || fail 'unsafe archive member path'
  [[ "${member}" =~ ^[A-Za-z0-9][A-Za-z0-9._/-]*$ ]] || fail 'unsafe archive member name'
done <<<"${members}"
listings="$(tar -tvzf "${archive}" 2>/dev/null)" || fail 'archive listing failed'
while IFS= read -r listing; do
  case "${listing:0:1}" in - | d) ;; *) fail 'archive contains a link or special file' ;; esac
done <<<"${listings}"
mkdir -m 700 "${assets}"
tar -xzf "${archive}" -C "${assets}" >/dev/null 2>&1 || fail 'archive extraction failed'

expected_paths=(
  bootstrap.sh release-contract.sh verify-assets.sh install.sh secure-file node-bundle
  config-web.yaml config-node.yaml config-combined.yaml
  entry/entryctl.sh entry/controller.sh entry/v2.sh
  entry/adapters/external.sh entry/adapters/nginx_certbot.sh entry/adapters/caddy.sh
  release-manifest.json
)
[[ -f "${assets}/SHA256SUMS" && ! -L "${assets}/SHA256SUMS" ]] || fail 'SHA256SUMS is missing'
seen='|'; count=0
while IFS= read -r line || [[ -n "${line}" ]]; do
  [[ "${line}" =~ ^([0-9a-f]{64})[[:space:]][\ \*]([A-Za-z0-9][A-Za-z0-9._/-]*)$ ]] || fail 'invalid SHA256SUMS syntax'
  sha="${BASH_REMATCH[1]}"; path="${BASH_REMATCH[2]}"
  allowed=0
  for expected in "${expected_paths[@]}"; do [[ "${path}" != "${expected}" ]] || allowed=1; done
  [[ "${allowed}" == 1 && "|${seen}|" != *"|${path}|"* ]] || fail 'unexpected or duplicate asset in SHA256SUMS'
  [[ -f "${assets}/${path}" && ! -L "${assets}/${path}" ]] || fail "missing asset: ${path}"
  [[ "$(checksum "${assets}/${path}")" == "${sha}" ]] || fail "asset SHA-256 mismatch: ${path}"
  seen+="${path}|"; count=$((count + 1))
done <"${assets}/SHA256SUMS"
[[ "${count}" -eq "${#expected_paths[@]}" ]] || fail 'incomplete SHA256SUMS asset set'
while IFS= read -r file; do
  path="${file#"${assets}/"}"
  [[ "${path}" == SHA256SUMS || "|${seen}|" == *"|${path}|"* ]] || fail 'archive contains an unverified file'
done < <(find "${assets}" -type f -print)
[[ -z "$(find "${assets}" -type l -print -quit)" ]] || fail 'archive contains a symlink'

manifest="${assets}/release-manifest.json"
yq_json() { yq -p=json -r "$1" "${manifest}" 2>/dev/null; }
[[ "$(yq_json 'keys | length')" == 6 && "$(yq_json '.images | keys | length')" == 6 ]] || fail 'manifest structure mismatch'
[[ "$(yq_json '.schema_version')" == 1 && "$(yq_json '.release_version')" == "${tag#v}" ]] || fail 'manifest version mismatch'
[[ "$(yq_json '.assets | length')" == 15 ]] || fail 'manifest asset set mismatch'
[[ "$(yq_json '.source_commit')" =~ ^[0-9a-f]{40}$ ]] || fail 'invalid manifest source commit'
manifest_paths='|'
for ((i = 0; i < 15; i++)); do
  path="$(yq_json ".assets[${i}].path")"
  sha="$(yq_json ".assets[${i}].sha256")"
  name="$(yq_json ".assets[${i}].name")"
  [[ "${path}" != release-manifest.json && "|${seen}|" == *"|${path}|"* && "${sha}" =~ ^[0-9a-f]{64}$ && "|${manifest_paths}|" != *"|${path}|"* ]] || fail 'manifest asset entry mismatch'
  [[ "${name}" == "${path%.*}" && "$(yq_json ".assets[${i}] | keys | length")" == 3 ]] || fail 'manifest asset entry schema mismatch'
  manifest_paths+="${path}|"
  [[ "$(checksum "${assets}/${path}")" == "${sha}" ]] || fail "manifest asset digest mismatch: ${path}"
done
for key in api web node_agent caddy mariadb redis; do
  name="$(yq_json ".images.${key}.name")"
  digest="$(yq_json ".images.${key}.digest")"
  ref="$(yq_json ".images.${key}.reference")"
  kind=runtime
  case "${key}" in api | web | node_agent) kind=product ;; esac
  [[ "${digest}" =~ ^sha256:[0-9a-f]{64}$ && "${name}" =~ ^[A-Za-z0-9._:/-]+$ && "${ref}" == "${name}@${digest}" && "$(yq_json ".images.${key}.kind")" == "${kind}" && "$(yq_json ".images.${key} | keys | length")" == 4 ]] || fail "manifest image ${key} is not pinned by digest"
done
[[ "$(yq_json '.attestations | length')" == 3 ]] || fail 'manifest attestations mismatch'
for ((i = 0; i < 3; i++)); do
  case "$i" in 0) key=api ;; 1) key=web ;; 2) key=node_agent ;; esac
  [[ "$(yq_json ".attestations[${i}].subject")" == "$(yq_json ".images.${key}.name")" && "$(yq_json ".attestations[${i}].digest")" == "$(yq_json ".images.${key}.digest")" && "$(yq_json ".attestations[${i}] | keys | length")" == 2 ]] || fail 'manifest attestation mismatch'
done
printf 'Verified release assets for %s\n' "${tag}"
