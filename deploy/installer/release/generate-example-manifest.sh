#!/usr/bin/env bash
set -Eeuo pipefail

fail() {
  printf 'generate example manifest: %s\n' "$1" >&2
  exit 1
}

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
output=""
while (($#)); do
  case "$1" in
  --output) [[ $# -ge 2 ]] || fail '--output requires a value'; output="$2"; shift 2 ;;
  *) fail "unknown argument: $1" ;;
  esac
done
[[ -n "${output}" ]] || fail 'output path is required'
[[ ! -L "${output}" ]] || fail 'output path must not be a symlink'

work="$(mktemp -d)"
trap 'rm -rf -- "${work}"' EXIT
digest() {
  local digit="$1"
  local index
  printf 'sha256:'
  for ((index = 0; index < 64; index++)); do
    printf '%s' "${digit}"
  done
}

"${script_dir}/generate-assets.sh" \
  --version 0.1.0 \
  --source-commit 671f5db70816572eec99f7eeae277f97bc1fec1b \
  --output "${work}/assets" \
  --api-image "ghcr.io/1linhao/trojanpanelnext-api@$(digest 1)" \
  --web-image "ghcr.io/1linhao/trojanpanelnext-web@$(digest 2)" \
  --node-agent-image "ghcr.io/1linhao/trojanpanelnext-node-agent@$(digest 3)" \
  --caddy-image "caddy@$(digest 4)" \
  --mariadb-image "mariadb@$(digest 5)" \
  --redis-image "redis@$(digest 6)" >/dev/null
"${work}/assets/verify-assets.sh" --assets-dir "${work}/assets" --assets-only >/dev/null

# Go binaries can differ across toolchains. Keep their paths in the example,
# while using zero placeholders for their machine-dependent SHA values.
jq '(.assets[] | select(.path == "secure-file" or .path == "node-bundle") | .sha256) =
  "0000000000000000000000000000000000000000000000000000000000000000"' \
  "${work}/assets/release-manifest.json" >"${work}/example-release-manifest.json"
mkdir -p "$(dirname "${output}")"
mv "${work}/example-release-manifest.json" "${output}"
