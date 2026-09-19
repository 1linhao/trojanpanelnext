#!/usr/bin/env bash
set -Eeuo pipefail

INSTALLER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GENERATOR="${INSTALLER_DIR}/release/generate-assets.sh"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

digest() {
  printf 'sha256:%064d' "$1"
}

work="$(mktemp -d)"
trap 'rm -rf -- "${work}"' EXIT
bundle="${work}/bundle"

"${GENERATOR}" \
  --version 1.2.3 \
  --source-commit 0123456789abcdef0123456789abcdef01234567 \
  --output "${bundle}" \
  --api-image "ghcr.io/1linhao/trojanpanelnext-api@$(digest 1)" \
  --web-image "ghcr.io/1linhao/trojanpanelnext-web@$(digest 2)" \
  --node-agent-image "ghcr.io/1linhao/trojanpanelnext-node-agent@$(digest 3)" \
  --caddy-image "caddy@$(digest 4)" \
  --mariadb-image "mariadb@$(digest 5)" \
  --redis-image "redis@$(digest 6)" >/dev/null

runtime_bin="${work}/runtime-bin"
mkdir "${runtime_bin}"
for command in awk bash cmp dirname grep od sed sha256sum sort; do
  command_path="$(command -v "${command}")"
  ln -s "${command_path}" "${runtime_bin}/${command}"
done

ln -s "${INSTALLER_DIR}/tests/fixtures/fake_yq_reader.sh" "${runtime_bin}/yq"

host_trace="${work}/host-side-effects"
cat >"${runtime_bin}/host-mutation-sentinel" <<'SENTINEL'
#!/usr/bin/env bash
printf '%s\n' "$0" >>"${TP_HOST_TRACE}"
exit 97
SENTINEL
chmod +x "${runtime_bin}/host-mutation-sentinel"
for command in apt apt-get chmod curl dnf docker install mkdir rm systemctl yum; do
  ln -s host-mutation-sentinel "${runtime_bin}/${command}"
done

if PATH="${runtime_bin}" command -v jq >/dev/null 2>&1; then
  fail 'restricted runtime PATH unexpectedly contains jq'
fi

output="$(PATH="${runtime_bin}" "${bundle}/verify-assets.sh" \
  --assets-dir "${bundle}" --config "${bundle}/config-web.yaml")"
grep -Fq 'Release assets are valid for web deployment mode (version 1.2.3)' <<<"${output}" ||
  fail 'jq-free verifier did not validate the generated release bundle'

for entrypoint in bootstrap.sh install.sh; do
  output="$(/usr/bin/env -i PATH="${runtime_bin}" TP_HOST_TRACE="${host_trace}" \
    "${bundle}/${entrypoint}" validate --mode web --config "${bundle}/config-web.yaml")"
  grep -Fq 'Release assets are valid for web deployment mode (version 1.2.3)' <<<"${output}" ||
    fail "${entrypoint} did not run jq-free release verification"
  grep -Fq 'Configuration is valid for web deployment mode' <<<"${output}" ||
    fail "${entrypoint} did not continue past release verification"
  test ! -e "${host_trace}" || fail "${entrypoint} changed the host during validation"
  printf 'TRACE jq-free entrypoint=%s release=verified installer=config-validated host-changes=0\n' \
    "${entrypoint}"
done

rejection_output=""
release_contract_rejection_observed() {
  local case_bundle="$1"
  local case_config="$2"
  local entrypoint="$3"
  local expected_error="$4"

  rejection_output=""
  rm -f "${host_trace}"
  if rejection_output="$(/usr/bin/env -i PATH="${runtime_bin}" TP_HOST_TRACE="${host_trace}" \
    "${case_bundle}/${entrypoint}" install --mode web --config "${case_config}" 2>&1)"; then
    return 1
  fi
  grep -Fq "${expected_error}" <<<"${rejection_output}" || return 1
  test ! -e "${host_trace}" || return 1
}

assert_rejected_before_host_change() {
  local case_name="$1"
  local case_bundle="$2"
  local case_config="$3"
  local expected_error="$4"
  local entrypoint
  for entrypoint in bootstrap.sh install.sh; do
    if ! release_contract_rejection_observed \
      "${case_bundle}" "${case_config}" "${entrypoint}" "${expected_error}"; then
      printf '%s\n' "${rejection_output}" >&2
      fail "${case_name} did not produce the expected release-contract rejection through ${entrypoint}: ${expected_error}"
    fi
    printf 'TRACE attack=%s entrypoint=%s rejected=preflight host-changes=0\n' \
      "${case_name}" "${entrypoint}"
  done
}

copy_case() {
  local name="$1"
  local target="${work}/${name}"
  cp -a "${bundle}" "${target}"
  printf '%s\n' "${target}"
}

case_bundle="$(copy_case tampered-asset)"
printf '\n# tampered\n' >>"${case_bundle}/config-web.yaml"
assert_rejected_before_host_change tampered-asset "${case_bundle}" \
  "${case_bundle}/config-web.yaml" 'SHA256SUMS verification failed'

case_bundle="$(copy_case tampered-manifest)"
sed -i 's/"release_version": "1.2.3"/"release_version": "9.9.9"/' \
  "${case_bundle}/release-manifest.json"
assert_rejected_before_host_change tampered-manifest "${case_bundle}" \
  "${case_bundle}/config-web.yaml" 'SHA256SUMS verification failed'

wrong_version_config="${work}/wrong-version.yaml"
cp "${bundle}/config-web.yaml" "${wrong_version_config}"
sed -i 's/asset_version: 1.2.3/asset_version: 1.2.4/' "${wrong_version_config}"
assert_rejected_before_host_change wrong-version "${bundle}" "${wrong_version_config}" \
  'configuration asset_version does not match release'

tag_only_config="${work}/tag-only.yaml"
cp "${bundle}/config-web.yaml" "${tag_only_config}"
sed -i 's#^  api_image:.*#  api_image: ghcr.io/1linhao/trojanpanelnext-api:latest#' \
  "${tag_only_config}"
assert_rejected_before_host_change tag-only-image "${bundle}" "${tag_only_config}" \
  'configuration image does not match manifest: api_image'

replacement_digest_config="${work}/replacement-digest.yaml"
cp "${bundle}/config-web.yaml" "${replacement_digest_config}"
sed -i 's#^  api_image:.*#  api_image: ghcr.io/1linhao/trojanpanelnext-api@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff#' \
  "${replacement_digest_config}"
assert_rejected_before_host_change replacement-digest "${bundle}" "${replacement_digest_config}" \
  'configuration image does not match manifest: api_image'

noop_bundle="$(copy_case noop-verifier-negative-control)"
printf '#!/usr/bin/env bash\nexit 0\n' >"${noop_bundle}/verify-assets.sh"
chmod +x "${noop_bundle}/verify-assets.sh"
for entrypoint in bootstrap.sh install.sh; do
  if release_contract_rejection_observed "${noop_bundle}" "${wrong_version_config}" \
    "${entrypoint}" 'configuration asset_version does not match release'; then
    fail "no-op verifier was mistaken for a release-contract rejection through ${entrypoint}"
  fi
  printf 'TRACE negative-control entrypoint=%s verifier=noop preflight-proof=rejected\n' "${entrypoint}"
done

printf 'PASS jq-free release verification contract\n'
