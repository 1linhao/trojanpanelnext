#!/usr/bin/env bash
set -Eeuo pipefail

fail() {
  printf 'generate release assets: %s\n' "$1" >&2
  exit 1
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALLER_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
version=""
source_commit=""
output=""
api_image=""
web_image=""
node_agent_image=""
caddy_image=""
mariadb_image=""
redis_image=""
installer_source="${INSTALLER_DIR}/install.sh"

while (($#)); do
  case "$1" in
  --version) version="${2:-}"; shift 2 ;;
  --source-commit) source_commit="${2:-}"; shift 2 ;;
  --output) output="${2:-}"; shift 2 ;;
  --api-image) api_image="${2:-}"; shift 2 ;;
  --web-image) web_image="${2:-}"; shift 2 ;;
  --node-agent-image) node_agent_image="${2:-}"; shift 2 ;;
  --caddy-image) caddy_image="${2:-}"; shift 2 ;;
  --mariadb-image) mariadb_image="${2:-}"; shift 2 ;;
  --redis-image) redis_image="${2:-}"; shift 2 ;;
  --installer-source) installer_source="${2:-}"; shift 2 ;;
  *) fail "unknown argument: $1" ;;
  esac
done

[[ "${version}" =~ ^[0-9]+\.[0-9]+\.[0-9]+([+-][0-9A-Za-z.-]+)?$ ]] || fail 'version must be an explicit semantic version'
[[ "${source_commit}" =~ ^[0-9a-f]{40}$ ]] || fail 'source commit must be a 40-character lowercase SHA'
[[ -n "${output}" ]] || fail 'output directory is required'
[[ ! -L "${output}" ]] || fail 'output directory must not be a symlink'
if [[ -d "${output}" ]] && find "${output}" -mindepth 1 -print -quit | grep -q .; then
  fail 'output directory must be empty'
fi
[[ -f "${installer_source}" && ! -L "${installer_source}" ]] || fail 'installer source must be a regular non-symlink file'
command -v jq >/dev/null 2>&1 || fail 'jq is required'
command -v sha256sum >/dev/null 2>&1 || fail 'sha256sum is required'

image_pattern='^[a-zA-Z0-9._/-]+@sha256:[0-9a-f]{64}$'
for image in "${api_image}" "${web_image}" "${node_agent_image}" "${caddy_image}" "${mariadb_image}" "${redis_image}"; do
  [[ "${image}" =~ ${image_pattern} ]] || fail "image must be pinned by digest: ${image:-<missing>}"
done

mkdir -p "${output}"
install -m 0755 "${SCRIPT_DIR}/bootstrap.sh" "${output}/bootstrap.sh"
install -m 0755 "${SCRIPT_DIR}/verify-assets.sh" "${output}/verify-assets.sh"
install -m 0755 "${installer_source}" "${output}/install.sh"

render_template() {
  local source="$1"
  local target="$2"
  sed \
    -e "s|__ASSET_VERSION__|${version}|g" \
    -e "s|__API_IMAGE__|${api_image}|g" \
    -e "s|__WEB_IMAGE__|${web_image}|g" \
    -e "s|__NODE_AGENT_IMAGE__|${node_agent_image}|g" \
    -e "s|__CADDY_IMAGE__|${caddy_image}|g" \
    -e "s|__MARIADB_IMAGE__|${mariadb_image}|g" \
    -e "s|__REDIS_IMAGE__|${redis_image}|g" \
    "${source}" >"${target}"
  chmod 0644 "${target}"
}
render_template "${SCRIPT_DIR}/templates/config-web.yaml" "${output}/config-web.yaml"
render_template "${SCRIPT_DIR}/templates/config-node.yaml" "${output}/config-node.yaml"
render_template "${SCRIPT_DIR}/templates/config-combined.yaml" "${output}/config-combined.yaml"

asset_json='[]'
for path in bootstrap.sh verify-assets.sh install.sh config-web.yaml config-node.yaml config-combined.yaml; do
  sha="$(sha256sum "${output}/${path}" | awk '{print $1}')"
  asset_json="$(jq -c --arg name "${path%.*}" --arg path "${path}" --arg sha "${sha}" '. + [{name:$name,path:$path,sha256:$sha}]' <<<"${asset_json}")"
done

image_json() {
  local kind="$1"
  local reference="$2"
  local name="${reference%@*}"
  local digest="${reference##*@}"
  jq -cn --arg kind "${kind}" --arg name "${name}" --arg digest "${digest}" --arg reference "${reference}" \
    '{kind:$kind,name:$name,digest:$digest,reference:$reference}'
}
api_json="$(image_json product "${api_image}")"
web_json="$(image_json product "${web_image}")"
node_json="$(image_json product "${node_agent_image}")"
caddy_json="$(image_json runtime "${caddy_image}")"
mariadb_json="$(image_json runtime "${mariadb_image}")"
redis_json="$(image_json runtime "${redis_image}")"

jq -n \
  --arg release_version "${version}" \
  --arg source_commit "${source_commit}" \
  --argjson assets "${asset_json}" \
  --argjson api "${api_json}" --argjson web "${web_json}" --argjson node_agent "${node_json}" \
  --argjson caddy "${caddy_json}" --argjson mariadb "${mariadb_json}" --argjson redis "${redis_json}" \
  '{
    schema_version: 1,
    release_version: $release_version,
    source_commit: $source_commit,
    assets: $assets,
    images: {api:$api,web:$web,node_agent:$node_agent,caddy:$caddy,mariadb:$mariadb,redis:$redis},
    attestations: [
      {subject:$api.name,digest:$api.digest},
      {subject:$web.name,digest:$web.digest},
      {subject:$node_agent.name,digest:$node_agent.digest}
    ]
  }' >"${output}/release-manifest.json"

(
  cd "${output}"
  sha256sum bootstrap.sh config-combined.yaml config-node.yaml config-web.yaml install.sh release-manifest.json verify-assets.sh >SHA256SUMS
)
printf 'Generated release assets for %s in %s\n' "${version}" "${output}"
