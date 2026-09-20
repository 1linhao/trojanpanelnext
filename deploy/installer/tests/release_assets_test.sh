#!/usr/bin/env bash
set -Eeuo pipefail

INSTALLER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GENERATOR="${INSTALLER_DIR}/release/generate-assets.sh"
VERIFY="${INSTALLER_DIR}/release/verify-assets.sh"
PACKAGE="${INSTALLER_DIR}/release/package-assets.sh"
ATTESTATION_VERIFY="${INSTALLER_DIR}/release/verify-attestation-results.sh"
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

# Hermetic yq-compatible reader for direct release-installer validation. The
# release preflight must reject unsafe images before dependency installation.
yq() {
  local operation="$1"
  local expression="$2"
  local file="$3"
  if [[ "${operation}" == -e ]]; then
    grep -q '^trojanpanelnext:' "${file}"
    return
  fi
  [[ "${operation}" == -r ]] || return 2
  local key value
  key="${expression#.trojanpanelnext.}"
  key="${key%% *}"
  value="$(awk -F: -v key="${key}" '
    $1 ~ "^[[:space:]]+" key "$" {
      sub(/^[^:]*:[[:space:]]*/, "")
      sub(/[[:space:]]+#.*$/, "")
      gsub(/^"|"$/, "")
      print
      exit
    }
  ' "${file}")"
  printf '%s\n' "${value}"
}
export -f yq

digest() {
  printf 'sha256:%064d' "$1"
}

repeated_digest() {
  local digit="$1"
  local _
  printf 'sha256:'
  for _ in {1..64}; do
    printf '%s' "${digit}"
  done
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
release_validate_output="$("${bundle}/install.sh" validate --mode web --config "${bundle}/config-web.yaml")"
grep -q 'valid for web deployment mode' <<<"${release_validate_output}"

# A released installer must execute only the node-bundle binary covered by the
# verified Release asset set. NODE_BUNDLE_HELPER remains a development-only seam.
cat >"${work}/node-credential.json" <<'JSON'
{"schema_version":2,"node_identity_id":"11111111-2222-4333-8444-555555555555","node_server_id":42,"node_name":"node-sg","node_domain":"node.example.com","public_ip":"203.0.113.42","generation":1,"mariadb":{"database":"trojan_panel_db","username":"tpn_example","password":"db-secret"},"redis":{"username":"tpn-cache-example","password":"cache-secret","key_patterns":["trojan-panel-core:*"]},"redis_auth":{"username":"tpn-auth-example","password":"auth-secret","key_patterns":["trojan-panel:jwt-key","trojan-panel:token:*"]}}
JSON
chmod 0600 "${work}/node-credential.json"
cp "${bundle}/config-node.yaml" "${work}/node-config.yaml"
chmod 0600 "${work}/node-config.yaml"
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj /CN=control-plane-ca \
  -addext basicConstraints=critical,CA:TRUE \
  -keyout "${work}/discarded-ca.key" -out "${work}/client-ca.crt" >/dev/null 2>&1
release_bundle_password='release bundle helper trust password'
TP_NODE_BUNDLE_PASSWORD="${release_bundle_password}" "${bundle}/node-bundle" create \
  --credential-file "${work}/node-credential.json" --node-config "${work}/node-config.yaml" \
  --client-ca "${work}/client-ca.crt" --output "${work}/node.age" >/dev/null
malicious_helper="${work}/malicious-node-bundle"
helper_sentinel="${work}/malicious-helper-ran"
cat >"${malicious_helper}" <<'EOF'
#!/usr/bin/env bash
printf ran >"${TP_HELPER_SENTINEL}"
exit 91
EOF
chmod 0755 "${malicious_helper}"
assert_fails env TP_HELPER_SENTINEL="${helper_sentinel}" NODE_BUNDLE_HELPER="${malicious_helper}" \
  TP_NODE_BUNDLE_PASSWORD="${release_bundle_password}" \
  "${bundle}/install.sh" validate --mode node --bundle "${work}/node.age" >/dev/null
test ! -e "${helper_sentinel}" || fail 'release installer executed an environment-overridden node-bundle helper'

tag_only_config="${work}/tag-only-config.yaml"
cp "${bundle}/config-web.yaml" "${tag_only_config}"
sed -i 's#^  api_image:.*#  api_image: ghcr.io/1linhao/trojanpanelnext-api:latest#' \
  "${tag_only_config}"
assert_fails "${bundle}/install.sh" validate --mode web --config "${tag_only_config}"
for image_key in web_image node_agent_image caddy_image mariadb_image redis_image; do
  tag_config="${work}/tag-only-${image_key}.yaml"
  cp "${bundle}/config-web.yaml" "${tag_config}"
  sed -i "s#^  ${image_key}:.*#  ${image_key}: example.invalid/${image_key}:latest#" "${tag_config}"
  assert_fails "${bundle}/install.sh" validate --mode web --config "${tag_config}"
done

invalid_digest_config="${work}/invalid-digest-config.yaml"
cp "${bundle}/config-web.yaml" "${invalid_digest_config}"
sed -i 's#^  api_image:.*#  api_image: ghcr.io/1linhao/trojanpanelnext-api@sha256:abc#' \
  "${invalid_digest_config}"
assert_fails "${bundle}/install.sh" validate --mode web --config "${invalid_digest_config}"

other_digest_config="${work}/other-digest-config.yaml"
cp "${bundle}/config-web.yaml" "${other_digest_config}"
sed -i 's#^  api_image:.*#  api_image: ghcr.io/1linhao/trojanpanelnext-api@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff#' \
  "${other_digest_config}"
assert_fails "${bundle}/install.sh" validate --mode web --config "${other_digest_config}"

host_sentinel="${work}/host-side-effect"
for unsafe_config in "${tag_only_config}" "${invalid_digest_config}" "${other_digest_config}"; do
  assert_fails env TP_HOST_SENTINEL="${host_sentinel}" bash -c '
    set -Eeuo pipefail
    source "$1"
    host_side_effect() { printf called >"${TP_HOST_SENTINEL}"; }
    require_root() { host_side_effect; }
    load_config() { host_side_effect; }
    validate_config() { :; }
    validate_entry_spec_binding() { :; }
    deploy_web() { host_side_effect; }
    main install --mode web --config "$2"
  ' release-install-preflight "${bundle}/install.sh" "${unsafe_config}"
  test ! -e "${host_sentinel}" || fail 'release install crossed the host mutation boundary before image rejection'
done

"${VERIFY}" --assets-dir "${bundle}" --config "${bundle}/config-web.yaml"
test -x "${bundle}/bootstrap.sh"
test -f "${bundle}/release-contract.sh"
test -x "${bundle}/install.sh"
test -x "${bundle}/secure-file"
test -x "${bundle}/node-bundle"
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
test -x "${work}/extracted/secure-file"
test -x "${work}/extracted/node-bundle"
test -x "${work}/extracted/verify-assets.sh"
test -f "${work}/extracted/release-contract.sh"
test -x "${work}/extracted/entry/entryctl.sh"
"${work}/extracted/verify-assets.sh" --assets-dir "${work}/extracted" \
  --config "${work}/extracted/config-web.yaml" >/dev/null
test -f "${bundle}/config-web.yaml"
test -f "${bundle}/config-node.yaml"
test -f "${bundle}/config-combined.yaml"
test -f "${bundle}/release-manifest.json"
test -f "${bundle}/SHA256SUMS"
for config in "${bundle}"/config-*.yaml; do
  grep -q '^  deployment_mode:' "${config}"
  grep -q '^  api_image:' "${config}"
  grep -q '^  web_image:' "${config}"
  grep -q '^  node_agent_image:' "${config}"
  ! grep -Eq '^  (purpose|panel_image|ui_image|core_image):' "${config}"
done
jq -e '.release_version == "1.2.3" and (.assets | length == 13)' \
  "${bundle}/release-manifest.json" >/dev/null
EXPECTED_RELEASE_ASSET_PATHS=(
  bootstrap.sh
  release-contract.sh
  verify-assets.sh
  install.sh
  secure-file
  node-bundle
  config-web.yaml
  config-node.yaml
  config-combined.yaml
  entry/entryctl.sh
  entry/controller.sh
  entry/adapters/external.sh
  entry/adapters/nginx_certbot.sh
)
mapfile -t manifest_asset_paths < <(jq -r '.assets[].path' "${bundle}/release-manifest.json")
cmp -s \
  <(printf '%s\n' "${EXPECTED_RELEASE_ASSET_PATHS[@]}" | sort) \
  <(printf '%s\n' "${manifest_asset_paths[@]}" | sort) ||
  fail 'release manifest asset set differs from the independently expected contract'
example_bundle="${work}/example-bundle"
"${GENERATOR}" \
  --version 0.1.0 \
  --source-commit 671f5db70816572eec99f7eeae277f97bc1fec1b \
  --output "${example_bundle}" \
  --api-image "ghcr.io/1linhao/trojanpanelnext-api@$(repeated_digest 1)" \
  --web-image "ghcr.io/1linhao/trojanpanelnext-web@$(repeated_digest 2)" \
  --node-agent-image "ghcr.io/1linhao/trojanpanelnext-node-agent@$(repeated_digest 3)" \
  --caddy-image "caddy@$(repeated_digest 4)" \
  --mariadb-image "mariadb@$(repeated_digest 5)" \
  --redis-image "redis@$(repeated_digest 6)" >/dev/null
normalized_example_manifest="${work}/normalized-example-release-manifest.json"
awk '
  /"name": "(secure-file|node-bundle)"/ { generated_binary = 1 }
  generated_binary && /"sha256":/ {
    sub(/"sha256": "[^"]+"/, "\"sha256\": \"0000000000000000000000000000000000000000000000000000000000000000\"")
    generated_binary = 0
  }
  { print }
' "${example_bundle}/release-manifest.json" >"${normalized_example_manifest}"
cmp -s "${normalized_example_manifest}" \
  "${INSTALLER_DIR}/release/example-release-manifest.json" ||
  fail 'example release manifest is stale'
bash -c 'source "$1"; test "${INSTALLER_ASSET_VERSION}" = 1.2.3' \
  release-version-test "${bundle}/install.sh"
assert_fails env TP_INSTALLER_ASSET_VERSION=development bash -c '
  set -Eeuo pipefail
  source "$1"
  yaml_read_raw() { printf "1\n"; }
  TP_CONFIG_FILE=/no-read
  TP_DEPLOYMENT_MODE=web
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
unverified_config="${work}/unverified-config.yaml"
cp "${sentinel_bundle}/config-web.yaml" "${unverified_config}"
sed -Ei 's#^  (panel_image|api_image):.*#  api_image: ghcr.io/1linhao/trojanpanelnext-api:latest#' \
  "${unverified_config}"
assert_fails env TP_SENTINEL_TRACE="${sentinel_trace}" \
  "${sentinel_bundle}/bootstrap.sh" validate --mode web \
  --config "${sentinel_bundle}/config-web.yaml" --config "${unverified_config}"
test ! -e "${sentinel_trace}" || fail 'bootstrap invoked installer with a second unverified config'

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

attested_files="${work}/attested-files"
mkdir "${attested_files}"
cp "${archive}" "${bundle}/release-manifest.json" "${bundle}/SHA256SUMS" "${attested_files}/"
image_results="${work}/image-verification-results.json"
jq '[{
  verificationResult: {
    statement: {
      subject: [.attestations[] | {
        name: .subject,
        digest: {sha256: (.digest | sub("^sha256:"; ""))}
      }]
    }
  }
}]' "${bundle}/release-manifest.json" >"${image_results}"
file_subjects='[]'
for file in "${attested_files}"/*; do
  file_subjects="$(jq -c --arg name "${file##*/}" --arg digest "$(sha256sum "${file}" | awk '{print $1}')" \
    '. + [{name: $name, digest: {sha256: $digest}}]' <<<"${file_subjects}")"
done
file_results="${work}/file-verification-results.json"
jq -n --argjson subjects "${file_subjects}" \
  '[{verificationResult: {statement: {subject: $subjects}}}]' >"${file_results}"
"${ATTESTATION_VERIFY}" \
  --manifest "${bundle}/release-manifest.json" \
  --image-results "${image_results}" \
  --files-dir "${attested_files}" \
  --file-results "${file_results}" >/dev/null
bad_image_results="${work}/bad-image-verification-results.json"
jq '.[0].verificationResult.statement.subject[0].digest.sha256 =
  "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"' \
  "${image_results}" >"${bad_image_results}"
assert_fails "${ATTESTATION_VERIFY}" \
  --manifest "${bundle}/release-manifest.json" \
  --image-results "${bad_image_results}" \
  --files-dir "${attested_files}" \
  --file-results "${file_results}"
missing_file_results="${work}/missing-file-verification-results.json"
jq '.[0].verificationResult.statement.subject |= .[1:]' \
  "${file_results}" >"${missing_file_results}"
assert_fails "${ATTESTATION_VERIFY}" \
  --manifest "${bundle}/release-manifest.json" \
  --image-results "${image_results}" \
  --files-dir "${attested_files}" \
  --file-results "${missing_file_results}"

assert_fails generate "${work}/tag-only" \
  --api-image ghcr.io/1linhao/trojanpanelnext-api:1.2.3
assert_fails generate "${work}/invalid-version" --version latest
assert_fails generate "${work}/invalid-build-version" --version 1.2.3+a+b
assert_fails generate "${work}/invalid-prerelease-version" --version 1.2.3-rc+meta+extra
assert_fails generate "${work}/leading-zero-version" --version 01.2.3
assert_fails generate "${work}/numeric-prerelease-leading-zero" --version 1.2.3-01
assert_fails generate "${work}/numeric-prerelease-component-leading-zero" --version 1.2.3-rc.01
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

resign_manifest() {
  local target="$1"
  (
    cd "${target}"
    sha256sum "${EXPECTED_RELEASE_ASSET_PATHS[@]}" release-manifest.json >SHA256SUMS
  )
}

inject_asset_name_bytes() {
  local target="$1"
  local hex_bytes="$2"
  python3 - "${target}/release-manifest.json" "${hex_bytes}" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
payload = bytes.fromhex(sys.argv[2])
content = path.read_bytes()
needle = b'"name": "bootstrap"'
replacement = b'"name": "bootstrap' + payload + b'"'
if content.count(needle) != 1:
    raise SystemExit("expected exactly one bootstrap asset name")
path.write_bytes(content.replace(needle, replacement, 1))
PY
}

case_dir="$(copy_case missing-asset)"
rm "${case_dir}/install.sh"
assert_fails "${VERIFY}" --assets-dir "${case_dir}" --config "${case_dir}/config-web.yaml"

assets_dir_link="${work}/assets-dir-link"
ln -s "${bundle}" "${assets_dir_link}"
assert_fails "${VERIFY}" --assets-dir "${assets_dir_link}" --config "${bundle}/config-web.yaml"

case_dir="$(copy_case symlinked-entry-directory)"
external_entry="${work}/external-entry"
mv "${case_dir}/entry" "${external_entry}"
ln -s "${external_entry}" "${case_dir}/entry"
assert_fails "${VERIFY}" --assets-dir "${case_dir}" --config "${case_dir}/config-web.yaml"

case_dir="$(copy_case symlinked-adapters-directory)"
external_adapters="${work}/external-adapters"
mv "${case_dir}/entry/adapters" "${external_adapters}"
ln -s "${external_adapters}" "${case_dir}/entry/adapters"
assert_fails "${VERIFY}" --assets-dir "${case_dir}" --config "${case_dir}/config-web.yaml"

case_dir="$(copy_case bad-sums)"
printf '%064d  config-web.yaml\n' 0 >"${case_dir}/SHA256SUMS"
assert_fails "${VERIFY}" --assets-dir "${case_dir}" --config "${case_dir}/config-web.yaml"

case_dir="$(copy_case wrong-version)"
sed -i 's/asset_version: 1.2.3/asset_version: 1.2.4/' "${case_dir}/config-web.yaml"
resign_asset "${case_dir}" config-web.yaml
assert_fails "${VERIFY}" --assets-dir "${case_dir}" --config "${case_dir}/config-web.yaml"

case_dir="$(copy_case invalid-manifest-semver)"
jq '.release_version = "1.2.3-01"' "${case_dir}/release-manifest.json" >"${case_dir}/manifest.tmp"
mv "${case_dir}/manifest.tmp" "${case_dir}/release-manifest.json"
resign_manifest "${case_dir}"
assert_fails "${VERIFY}" --assets-dir "${case_dir}" --config "${case_dir}/config-web.yaml"

case_dir="$(copy_case wrong-deployment-mode)"
sed -i 's/deployment_mode: web/deployment_mode: worker/' "${case_dir}/config-web.yaml"
resign_asset "${case_dir}" config-web.yaml
assert_fails "${VERIFY}" --assets-dir "${case_dir}" --config "${case_dir}/config-web.yaml"

case_dir="$(copy_case traversal)"
jq '(.assets[0].path) = "../bootstrap.sh"' "${case_dir}/release-manifest.json" >"${case_dir}/manifest.tmp"
mv "${case_dir}/manifest.tmp" "${case_dir}/release-manifest.json"
resign_manifest "${case_dir}"
assert_fails "${VERIFY}" --assets-dir "${case_dir}" --config "${case_dir}/config-web.yaml"

case_dir="$(copy_case duplicate-path)"
jq '(.assets[1].path) = .assets[0].path' "${case_dir}/release-manifest.json" >"${case_dir}/manifest.tmp"
mv "${case_dir}/manifest.tmp" "${case_dir}/release-manifest.json"
resign_manifest "${case_dir}"
assert_fails "${VERIFY}" --assets-dir "${case_dir}" --config "${case_dir}/config-web.yaml"

case_dir="$(copy_case bad-attestation)"
jq '(.attestations[0].digest) = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"' \
  "${case_dir}/release-manifest.json" >"${case_dir}/manifest.tmp"
mv "${case_dir}/manifest.tmp" "${case_dir}/release-manifest.json"
resign_manifest "${case_dir}"
assert_fails "${VERIFY}" --assets-dir "${case_dir}" --config "${case_dir}/config-web.yaml"

case_dir="$(copy_case duplicate-json-key)"
sed -i '/"release_version":/a\  "release_version": "1.2.3",' "${case_dir}/release-manifest.json"
resign_manifest "${case_dir}"
assert_fails "${VERIFY}" --assets-dir "${case_dir}" --config "${case_dir}/config-web.yaml"

case_dir="$(copy_case unknown-manifest-field)"
sed -i '/"schema_version":/a\  "unexpected": "field",' "${case_dir}/release-manifest.json"
resign_manifest "${case_dir}"
assert_fails "${VERIFY}" --assets-dir "${case_dir}" --config "${case_dir}/config-web.yaml"

case_dir="$(copy_case raw-utf8-manifest)"
inject_asset_name_bytes "${case_dir}" c3a9
resign_manifest "${case_dir}"
assert_fails "${VERIFY}" --assets-dir "${case_dir}" --config "${case_dir}/config-web.yaml"

for byte_case in form-feed:0c nul:00 control-01:01; do
  case_name="${byte_case%%:*}"
  hex_bytes="${byte_case##*:}"
  case_dir="$(copy_case "${case_name}-manifest")"
  inject_asset_name_bytes "${case_dir}" "${hex_bytes}"
  resign_manifest "${case_dir}"
  assert_fails "${VERIFY}" --assets-dir "${case_dir}" --config "${case_dir}/config-web.yaml"
done

workflow="${REPO_ROOT}/.github/workflows/publish-images.yml"
grep -Fq 'uses: actions/attest@' "${workflow}"
grep -Fq 'source deploy/installer/release/release-contract.sh' "${workflow}"
grep -Fq 'release_semver_is_valid "$version"' "${workflow}"
for image_env in API_IMAGE WEB_IMAGE NODE_AGENT_IMAGE; do
  grep -Fq 'subject-name: ${{ env.'"${image_env}"' }}' "${workflow}"
done
test "$(grep -Fxc '          subject-digest: ${{ steps.build.outputs.digest }}' "${workflow}")" = 3
grep -Fq 'subject-path: release-upload/*' "${workflow}"
test "$(grep -Fc 'gh attestation verify' "${workflow}")" = 2
grep -Fq 'deploy/installer/release/verify-attestation-results.sh "${verify_args[@]}"' "${workflow}"
for bundle in api web node-agent; do
  grep -Fq "name: image-attestation-${bundle}" "${workflow}"
done
test "$(grep -Fxc '          path: release-upload/*' "${workflow}")" = 1

printf 'PASS release asset generation and verification contract\n'
