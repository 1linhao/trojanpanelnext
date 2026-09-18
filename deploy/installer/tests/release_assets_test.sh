#!/usr/bin/env bash
set -Eeuo pipefail

INSTALLER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GENERATOR="${INSTALLER_DIR}/release/generate-assets.sh"
VERIFY="${INSTALLER_DIR}/release/verify-assets.sh"
PACKAGE="${INSTALLER_DIR}/release/package-assets.sh"
REPO_ROOT="$(cd "${INSTALLER_DIR}/../.." && pwd)"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

assert_fails() {
  if "$@" >/dev/null 2>&1; then
    fail "command unexpectedly succeeded: $*"
  fi
}

digest() {
  printf 'sha256:%064d' "$1"
}

generate() {
  local output="$1"
  shift
  "${GENERATOR}" \
    --version 1.2.3 \
    --source-commit 0123456789abcdef0123456789abcdef01234567 \
    --output "${output}" \
    --api-image "ghcr.io/1linhao/trojanpanelnext-api@$(digest 1)" \
    --web-image "ghcr.io/1linhao/trojanpanelnext-web@$(digest 2)" \
    --node-agent-image "ghcr.io/1linhao/trojanpanelnext-node-agent@$(digest 3)" \
    --caddy-image "caddy@$(digest 4)" \
    --mariadb-image "mariadb@$(digest 5)" \
    --redis-image "redis@$(digest 6)" \
    "$@"
}

work="$(mktemp -d)"
trap 'rm -rf -- "${work}"' EXIT
bundle="${work}/bundle"
generate "${bundle}"

"${VERIFY}" --assets-dir "${bundle}" --config "${bundle}/config-web.yaml"
test -x "${bundle}/bootstrap.sh"
test -x "${bundle}/install.sh"
test -x "${bundle}/entry/entryctl.sh"
test -f "${bundle}/entry/controller.sh"
test -f "${bundle}/entry/adapters/external.sh"
test -x "${bundle}/entry/adapters/nginx_certbot.sh"
"${bundle}/entry/entryctl.sh" --help | grep -q 'Usage:'
archive="${work}/trojanpanelnext-installer-1.2.3.tar.gz"
"${PACKAGE}" --assets-dir "${bundle}" --output "${archive}" >/dev/null
mkdir "${work}/extracted"
tar -C "${work}/extracted" -xzf "${archive}"
test -x "${work}/extracted/bootstrap.sh"
test -x "${work}/extracted/install.sh"
test -x "${work}/extracted/verify-assets.sh"
test -x "${work}/extracted/entry/entryctl.sh"
"${work}/extracted/verify-assets.sh" --assets-dir "${work}/extracted" \
  --config "${work}/extracted/config-web.yaml" >/dev/null
test -f "${bundle}/config-web.yaml"
test -f "${bundle}/config-node.yaml"
test -f "${bundle}/config-combined.yaml"
test -f "${bundle}/release-manifest.json"
test -f "${bundle}/SHA256SUMS"
jq -e '.release_version == "1.2.3" and (.assets | length == 10)' \
  "${bundle}/release-manifest.json" >/dev/null
bash -c 'source "$1"; test "${INSTALLER_ASSET_VERSION}" = 1.2.3' \
  release-version-test "${bundle}/install.sh"
assert_fails bash -c '
  set -Eeuo pipefail
  source "$1"
  yaml_read_raw() { printf "1\n"; }
  TP_CONFIG_FILE=/no-read
  TP_PURPOSE=web
  TP_ASSET_VERSION=1.2.4
  TP_WEB_DOMAIN=panel.example.com
  validate_config web
' release-version-test "${bundle}/install.sh"

sentinel_installer="${work}/sentinel-installer.sh"
sentinel_trace="${work}/installer-ran"
printf '#!/usr/bin/env bash\nprintf ran >"${TP_SENTINEL_TRACE}"\n' >"${sentinel_installer}"
chmod 0755 "${sentinel_installer}"
sentinel_bundle="${work}/sentinel-bundle"
generate "${sentinel_bundle}" --installer-source "${sentinel_installer}"
printf '\n# tampered\n' >>"${sentinel_bundle}/config-web.yaml"
assert_fails env TP_SENTINEL_TRACE="${sentinel_trace}" \
  "${sentinel_bundle}/bootstrap.sh" validate --mode web --config "${sentinel_bundle}/config-web.yaml"
test ! -e "${sentinel_trace}" || fail 'bootstrap invoked installer after failed preflight'


"${VERIFY}" --assets-dir "${bundle}" --config "${bundle}/config-node.yaml" >/dev/null
"${VERIFY}" --assets-dir "${bundle}" --config "${bundle}/config-combined.yaml" >/dev/null
deployment_config="${work}/deployment.yaml"
cp "${bundle}/config-web.yaml" "${deployment_config}"
sed -i 's/panel.example.com/control.example.net/' "${deployment_config}"
"${VERIFY}" --assets-dir "${bundle}" --config "${deployment_config}" >/dev/null

assert_fails generate "${work}/tag-only" \
  --api-image ghcr.io/1linhao/trojanpanelnext-api:1.2.3
assert_fails generate "${work}/invalid-version" --version latest
assert_fails generate "${work}/invalid-build-version" --version 1.2.3+a+b
assert_fails generate "${work}/invalid-prerelease-version" --version 1.2.3-rc+meta+extra
assert_fails generate "${work}/leading-zero-version" --version 01.2.3
generate "${work}/prerelease" --version 1.2.3-rc.1+build.2 >/dev/null
"${VERIFY}" --assets-dir "${work}/prerelease" --config "${work}/prerelease/config-web.yaml" >/dev/null
mkdir "${work}/nonempty-output"
printf stale >"${work}/nonempty-output/stale"
assert_fails generate "${work}/nonempty-output"

copy_case() {
  local name="$1"
  local target="${work}/${name}"
  cp -a "${bundle}" "${target}"
  printf '%s\n' "${target}"
}

resign_asset() {
  local target="$1"
  local path="$2"
  local sha temporary
  sha="$(sha256sum "${target}/${path}" | awk '{print $1}')"
  temporary="${target}/manifest.tmp"
  jq --arg path "${path}" --arg sha "${sha}" \
    '(.assets[] | select(.path == $path) | .sha256) = $sha' \
    "${target}/release-manifest.json" >"${temporary}"
  mv "${temporary}" "${target}/release-manifest.json"
  (
    cd "${target}"
    mapfile -t paths < <(jq -r '.assets[].path' release-manifest.json)
    sha256sum "${paths[@]}" release-manifest.json >SHA256SUMS
  )
}

case_dir="$(copy_case missing-asset)"
rm "${case_dir}/install.sh"
assert_fails "${VERIFY}" --assets-dir "${case_dir}" --config "${case_dir}/config-web.yaml"

case_dir="$(copy_case bad-sums)"
printf '%064d  config-web.yaml\n' 0 >"${case_dir}/SHA256SUMS"
assert_fails "${VERIFY}" --assets-dir "${case_dir}" --config "${case_dir}/config-web.yaml"

case_dir="$(copy_case wrong-version)"
sed -i 's/asset_version: 1.2.3/asset_version: 1.2.4/' "${case_dir}/config-web.yaml"
resign_asset "${case_dir}" config-web.yaml
assert_fails "${VERIFY}" --assets-dir "${case_dir}" --config "${case_dir}/config-web.yaml"

case_dir="$(copy_case wrong-purpose)"
sed -i 's/purpose: web/purpose: worker/' "${case_dir}/config-web.yaml"
resign_asset "${case_dir}" config-web.yaml
assert_fails "${VERIFY}" --assets-dir "${case_dir}" --config "${case_dir}/config-web.yaml"

case_dir="$(copy_case traversal)"
jq '(.assets[0].path) = "../bootstrap.sh"' "${case_dir}/release-manifest.json" >"${case_dir}/manifest.tmp"
mv "${case_dir}/manifest.tmp" "${case_dir}/release-manifest.json"
assert_fails "${VERIFY}" --assets-dir "${case_dir}" --config "${case_dir}/config-web.yaml"

case_dir="$(copy_case duplicate-path)"
jq '(.assets[1].path) = .assets[0].path' "${case_dir}/release-manifest.json" >"${case_dir}/manifest.tmp"
mv "${case_dir}/manifest.tmp" "${case_dir}/release-manifest.json"
assert_fails "${VERIFY}" --assets-dir "${case_dir}" --config "${case_dir}/config-web.yaml"

case_dir="$(copy_case bad-attestation)"
jq '(.attestations[0].digest) = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"' \
  "${case_dir}/release-manifest.json" >"${case_dir}/manifest.tmp"
mv "${case_dir}/manifest.tmp" "${case_dir}/release-manifest.json"
assert_fails "${VERIFY}" --assets-dir "${case_dir}" --config "${case_dir}/config-web.yaml"

workflow="${REPO_ROOT}/.github/workflows/publish-images.yml"
grep -Fq 'uses: actions/attest@' "${workflow}"
for image_env in API_IMAGE WEB_IMAGE NODE_AGENT_IMAGE; do
  grep -Fq 'subject-name: ${{ env.'"${image_env}"' }}' "${workflow}"
done
test "$(grep -Fxc '          subject-digest: ${{ steps.build.outputs.digest }}' "${workflow}")" = 3
grep -Fq 'subject-path: release-upload/*' "${workflow}"
test "$(grep -Fxc '          path: release-upload/*' "${workflow}")" = 1

printf 'PASS release asset generation and verification contract\n'
