#!/usr/bin/env bash
set -Eeuo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
client="${root}/deploy/installer/client/tpnext.sh"
command -v yq >/dev/null 2>&1 || { printf 'SKIP: install mikefarah yq v4 to run local_init_test.sh\n' >&2; exit 1; }
[[ "$(yq --version)" == *'version v4.'* ]] || { printf 'FAIL: mikefarah yq v4 required\n' >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { printf 'FAIL: jq is required for fixture generation\n' >&2; exit 1; }
if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
  printf 'FAIL: sha256sum or shasum is required\n' >&2; exit 1
fi
work="$(mktemp -d)"
work="$(cd "${work}" && pwd -P)"
trap 'rm -r -- "${work}"' EXIT
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
reject() {
  local label="$1"; shift
  if "$@" >"${work}/reject.out" 2>&1; then fail "accepted ${label}"; fi
  printf 'TRACE rejected=%s\n' "${label}"
}
digest() { printf 'sha256:%064d' "$1"; }
sha_files() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$@"
  else shasum -a 256 "$@"; fi
}
sha_file() { sha_files "$1" | awk '{print $1}'; }
file_mode() {
  stat -c %a "$1" 2>/dev/null || stat -f %Lp "$1"
}

source_dir="${work}/source"
mkdir -p "${source_dir}/entry/adapters" "${source_dir}/client/templates"
paths=(
  bootstrap.sh release-contract.sh verify-assets.sh install.sh secure-file node-bundle
  config-web.yaml config-node.yaml config-combined.yaml
  entry/entryctl.sh entry/controller.sh entry/v2.sh
  entry/adapters/external.sh entry/adapters/nginx_certbot.sh entry/adapters/caddy.sh
  client/tpnext.sh client/topology.sh client/verify-release.sh
  client/download-assets.sh client/upload-assets.sh
  client/templates/unified-ssh.yaml client/templates/unified-local.yaml
)
for path in "${paths[@]}"; do printf 'untrusted fixture asset: %s\n' "${path}" >"${source_dir}/${path}"; done
asset_json='[]'
for path in "${paths[@]}"; do
  asset_json="$(jq -cn --argjson old "${asset_json}" --arg name "${path%.*}" --arg path "${path}" --arg sha "$(sha_file "${source_dir}/${path}")" '$old + [{name:$name,path:$path,sha256:$sha}]')"
done
jq -n --argjson assets "${asset_json}" \
  --arg api "$(digest 1)" --arg web "$(digest 2)" --arg node "$(digest 3)" \
  --arg caddy "$(digest 4)" --arg mariadb "$(digest 5)" --arg redis "$(digest 6)" '
  def image($name;$digest;$kind): {kind:$kind,name:$name,digest:$digest,reference:($name+"@"+$digest)};
  {schema_version:1,release_version:"1.2.3",source_commit:"0123456789abcdef0123456789abcdef01234567",assets:$assets,
   images:{api:image("example/api";$api;"product"),web:image("example/web";$web;"product"),node_agent:image("example/node";$node;"product"),
     caddy:image("caddy";$caddy;"runtime"),mariadb:image("mariadb";$mariadb;"runtime"),redis:image("redis";$redis;"runtime")},
   attestations:[{subject:"example/api",digest:$api},{subject:"example/web",digest:$web},{subject:"example/node",digest:$node}]}' >"${source_dir}/release-manifest.json"
(cd "${source_dir}" && sha_files "${paths[@]}" release-manifest.json >SHA256SUMS)
tar -C "${source_dir}" -czf "${work}/fixture.tar.gz" .
TP_TEST_ARCHIVE="${work}/fixture.tar.gz"; export TP_TEST_ARCHIVE
curl() {
  local previous="" arg destination=""
  for arg in "$@"; do [[ "${previous}" != -o ]] || destination="${arg}"; previous="${arg}"; done
  [[ -n "${destination}" ]] || return 2
  if [[ "${TP_TEST_CURL_FAIL:-0}" == 1 ]]; then printf partial >"${destination}"; return 22; fi
  cp "${TP_TEST_ARCHIVE}" "${destination}"
}
export -f curl
ln() {
  if [[ "${TP_TEST_LINK_MODE:-}" != "" && "${2:-}" == "${TP_TEST_LINK_CONFIG:-}" ]]; then
    return 1
  fi
  command ln "$@"
}
export -f ln
mv() {
  if [[ -n "${TP_TEST_MOVE_MODE:-}" && "${3:-}" == "${TP_TEST_MOVE_WORK_DIR:-}" ]]; then
    case "${TP_TEST_MOVE_MODE}" in
    fail) return 1 ;;
    foreign)
      mkdir "${TP_TEST_MOVE_WORK_DIR}"
      printf 'foreign directory\n' >"${TP_TEST_MOVE_WORK_DIR}/foreign-sentinel"
      return 1
      ;;
    after)
      command mv "$@" || return
      printf 'foreign file\n' >"${TP_TEST_MOVE_WORK_DIR}/foreign-sentinel"
      return 1
      ;;
    esac
  fi
  command mv "$@"
}
export -f mv
sha="$(sha_file "${TP_TEST_ARCHIVE}")"

config="${work}/deployment.local.yaml"
PATH="${PATH}" bash "${client}" init --config "${config}" --tag v1.2.3 --sha256 "${sha}" --work-dir "${work}/assets.local" >"${work}/init.out"
[[ "$(file_mode "${config}")" == 600 && "$(file_mode "${work}/assets.local")" == 700 ]] || fail 'config/work directory permissions'
[[ -f "${work}/assets.local/assets/release-manifest.json" ]] || fail 'verified assets missing'
plan="$(bash "${client}" plan --config "${config}")"
[[ "${plan}" == *$'web-host\tssh\tweb\t'* && "${plan}" == *$'node-host\tssh\tnode\tnode-one\tnodes/node-one.json'* ]] || fail 'separate SSH topology plan'
printf 'TRACE plan=%s\n' "${plan//$'\n'/; }"
before="$(sha_file "${config}")"
reject 'repeat init' bash "${client}" init --config "${config}" --tag v1.2.3 --sha256 "${sha}" --work-dir "${work}/assets.local"
[[ "$(sha_file "${config}")" == "${before}" ]] || fail 'repeat init modified configuration'
reject 'wrong archive SHA' bash "${client}" init --config "${work}/bad.local.yaml" --tag v1.2.3 --sha256 "$(printf '%064d' 0)" --work-dir "${work}/bad-assets.local"
[[ ! -e "${work}/bad.local.yaml" && ! -e "${work}/bad-assets.local" ]] || fail 'wrong SHA published files'
reject 'latest tag' bash "${client}" init --config "${work}/latest.local.yaml" --tag latest --sha256 "${sha}"
TP_TEST_CURL_FAIL=1; export TP_TEST_CURL_FAIL
reject 'partial download' bash "${client}" init --config "${work}/retry.local.yaml" --tag v1.2.3 --sha256 "${sha}" --work-dir "${work}/retry-assets.local"
[[ ! -e "${work}/retry.local.yaml" && ! -e "${work}/retry-assets.local" ]] || fail 'partial download left published files'
unset TP_TEST_CURL_FAIL
bash "${client}" init --config "${work}/retry.local.yaml" --tag v1.2.3 --sha256 "${sha}" --work-dir "${work}/retry-assets.local" >/dev/null
TP_TEST_LINK_MODE=fail TP_TEST_LINK_CONFIG="${work}/link-fail.local.yaml" \
  reject 'final config link failure' bash "${client}" init --config "${work}/link-fail.local.yaml" --tag v1.2.3 --sha256 "${sha}" --work-dir "${work}/link-fail-assets.local"
[[ ! -e "${work}/link-fail.local.yaml" && ! -e "${work}/link-fail-assets.local" ]] || fail 'failed final config link left a published path'
bash "${client}" init --config "${work}/link-fail.local.yaml" --tag v1.2.3 --sha256 "${sha}" --work-dir "${work}/link-fail-assets.local" >/dev/null
TP_TEST_MOVE_MODE=fail TP_TEST_MOVE_WORK_DIR="${work}/move-fail-assets.local" \
  reject 'final asset publish failure' bash "${client}" init --config "${work}/move-fail.local.yaml" --tag v1.2.3 --sha256 "${sha}" --work-dir "${work}/move-fail-assets.local"
[[ ! -e "${work}/move-fail.local.yaml" && ! -e "${work}/move-fail-assets.local" ]] || fail 'failed final asset publish left a published path'
bash "${client}" init --config "${work}/move-fail.local.yaml" --tag v1.2.3 --sha256 "${sha}" --work-dir "${work}/move-fail-assets.local" >/dev/null
TP_TEST_MOVE_MODE=foreign TP_TEST_MOVE_WORK_DIR="${work}/foreign-assets.local" \
  reject 'foreign directory replacement' bash "${client}" init --config "${work}/foreign.local.yaml" --tag v1.2.3 --sha256 "${sha}" --work-dir "${work}/foreign-assets.local"
[[ ! -e "${work}/foreign.local.yaml" && -f "${work}/foreign-assets.local/foreign-sentinel" ]] || fail 'rollback deleted or changed a foreign work directory'
TP_TEST_MOVE_MODE=after TP_TEST_MOVE_WORK_DIR="${work}/foreign-file-assets.local" \
  bash "${client}" init --config "${work}/foreign-file.local.yaml" --tag v1.2.3 --sha256 "${sha}" --work-dir "${work}/foreign-file-assets.local" >/dev/null
[[ -f "${work}/foreign-file.local.yaml" && -f "${work}/foreign-file-assets.local/foreign-sentinel" ]] || fail 'publish deleted a foreign file'

# A digest-correct archive with an internally mismatched version still fails.
cp -R "${source_dir}" "${work}/wrong-version"
jq '.release_version = "1.2.4"' "${source_dir}/release-manifest.json" >"${work}/wrong-version/release-manifest.json"
(cd "${work}/wrong-version" && sha_files "${paths[@]}" release-manifest.json >SHA256SUMS)
tar -C "${work}/wrong-version" -czf "${work}/wrong-version.tar.gz" .
TP_TEST_ARCHIVE="${work}/wrong-version.tar.gz"; export TP_TEST_ARCHIVE
reject 'manifest version mismatch' bash "${client}" init --config "${work}/version.local.yaml" --tag v1.2.3 --sha256 "$(sha_file "${TP_TEST_ARCHIVE}")" --work-dir "${work}/version-assets.local"
[[ ! -e "${work}/version.local.yaml" ]] || fail 'version mismatch created config'
TP_TEST_ARCHIVE="${work}/fixture.tar.gz"; export TP_TEST_ARCHIVE

cp -R "${source_dir}" "${work}/wrong-digest"
printf 'tampered\n' >>"${work}/wrong-digest/config-web.yaml"
tar -C "${work}/wrong-digest" -czf "${work}/wrong-digest.tar.gz" .
TP_TEST_ARCHIVE="${work}/wrong-digest.tar.gz"; export TP_TEST_ARCHIVE
reject 'internal asset SHA' bash "${client}" init --config "${work}/digest.local.yaml" --tag v1.2.3 --sha256 "$(sha_file "${TP_TEST_ARCHIVE}")" --work-dir "${work}/digest-assets.local"
[[ ! -e "${work}/digest.local.yaml" ]] || fail 'internal SHA mismatch created config'

cp -R "${source_dir}" "${work}/bad-image"
jq '.images.api.reference = "example/api:latest"' "${source_dir}/release-manifest.json" >"${work}/bad-image/release-manifest.json"
(cd "${work}/bad-image" && sha_files "${paths[@]}" release-manifest.json >SHA256SUMS)
tar -C "${work}/bad-image" -czf "${work}/bad-image.tar.gz" .
TP_TEST_ARCHIVE="${work}/bad-image.tar.gz"; export TP_TEST_ARCHIVE
reject 'tag-only manifest image' bash "${client}" init --config "${work}/image.local.yaml" --tag v1.2.3 --sha256 "$(sha_file "${TP_TEST_ARCHIVE}")" --work-dir "${work}/image-assets.local"

cp -R "${source_dir}" "${work}/link-source"
ln -s "${work}/outside" "${work}/link-source/entry/redirect"
tar -C "${work}/link-source" -czf "${work}/link.tar.gz" .
reject 'archive symlink' bash "${root}/deploy/installer/client/verify-release.sh" \
  --archive "${work}/link.tar.gz" --tag v1.2.3 --sha256 "$(sha_file "${work}/link.tar.gz")" --assets-dir "${work}/link-assets"
[[ ! -e "${work}/link-assets" ]] || fail 'symlink archive was extracted'
if [[ "$(tar --version 2>/dev/null)" == *'GNU tar'* ]]; then
  tar -C "${source_dir}" --transform='s|^./bootstrap.sh$|../escape|' -czf "${work}/traversal.tar.gz" .
else
  tar -C "${source_dir}" -s ',^\./bootstrap\.sh$,../escape,' -czf "${work}/traversal.tar.gz" .
fi
reject 'archive traversal' bash "${root}/deploy/installer/client/verify-release.sh" \
  --archive "${work}/traversal.tar.gz" --tag v1.2.3 --sha256 "$(sha_file "${work}/traversal.tar.gz")" --assets-dir "${work}/traversal-assets"
[[ ! -e "${work}/traversal-assets" && ! -e "${work}/escape" ]] || fail 'traversal archive was extracted'
TP_TEST_ARCHIVE="${work}/fixture.tar.gz"; export TP_TEST_ARCHIVE

# Simulate macOS's SHA tool selection with an isolated PATH lacking sha256sum.
mkdir "${work}/macbin"
for utility in bash awk tar gzip mkdir find yq shasum; do
  ln -s "$(command -v "${utility}")" "${work}/macbin/${utility}"
done
PATH="${work}/macbin" bash "${root}/deploy/installer/client/verify-release.sh" \
  --archive "${TP_TEST_ARCHIVE}" --tag v1.2.3 --sha256 "${sha}" --assets-dir "${work}/mac-assets" >/dev/null
printf 'TRACE macOS-shasum=verified\n'

# Local combined, remote combined, and appended independent Node retain the
# same node-key-derived state path after array reordering.
local_config="${work}/combined.local.yaml"
bash "${client}" init --config "${local_config}" --tag v1.2.3 --sha256 "${sha}" --web-transport local --work-dir "${work}/combined-assets.local" >/dev/null
[[ "$(bash "${client}" plan --config "${local_config}")" == *$'web-host\tlocal\tcombined\tnode-one\tnodes/node-one.json'* ]] || fail 'local combined plan'
yq -i '.hosts.node-host = {"transport":"ssh","ssh":{"user":"root","hostname":"203.0.113.20","port":22,"identity_file":"","config_file":""}} | .nodes[0].host = "node-host" | .nodes[0].public_ip = "203.0.113.20"' "${local_config}"
[[ "$(bash "${client}" plan --config "${local_config}")" == *$'web-host\tlocal\tweb\t'* ]] || fail 'local Web plus remote Node plan'
yq -i '.nodes = [] | del(.hosts.node-host)' "${local_config}"
[[ "$(bash "${client}" plan --config "${local_config}")" == *$'web-host\tlocal\tweb\t'* ]] || fail 'zero-Node plan'
yq -i '.nodes[0].host = "web-host" | del(.hosts.node-host) | .nodes[0].public_ip = "203.0.113.10" | .nodes[0].settings.node_caddy_https_port = 443' "${config}"
[[ "$(bash "${client}" plan --config "${config}")" == *$'web-host\tssh\tcombined\tnode-one\tnodes/node-one.json'* ]] || fail 'remote combined plan'
yq -i '.hosts.other-host = {"transport":"ssh","ssh":{"user":"operator","hostname":"203.0.113.30","port":22,"identity_file":"","config_file":""}} | .nodes += [(.nodes[0] | .node_key = "node-two" | .host = "other-host" | .name = "node-two" | .domain = "node-two.example.com" | .public_ip = "203.0.113.30")]' "${config}"
first_plan="$(bash "${client}" plan --config "${config}")"
yq -i '.nodes |= reverse' "${config}"
[[ "$(bash "${client}" plan --config "${config}")" == "${first_plan}" ]] || fail 'reordering changed stable plan'
printf 'TRACE appended-plan=%s\n' "${first_plan//$'\n'/; }"

cp "${config}" "${work}/invalid.local.yaml"
yq -i '.nodes[0].node_key = "node-one"' "${work}/invalid.local.yaml"
reject 'duplicate node_key' bash "${client}" plan --config "${work}/invalid.local.yaml"
yq -i '.nodes[0].node_key = "node-two" | .nodes[0].domain = .web.domain' "${work}/invalid.local.yaml"
reject 'duplicate domain' bash "${client}" plan --config "${work}/invalid.local.yaml"
yq -i '.nodes[0].domain = "node-two.example.com" | .nodes[0].host = "web-host"' "${work}/invalid.local.yaml"
reject 'second combined Node' bash "${client}" plan --config "${work}/invalid.local.yaml"
yq -i '.nodes[0].host = "missing-host"' "${work}/invalid.local.yaml"
reject 'undeclared host' bash "${client}" plan --config "${work}/invalid.local.yaml"
yq -i '.nodes[0].host = "other-host" | .hosts."other-host".transport = "local" | del(.hosts."other-host".ssh)' "${work}/invalid.local.yaml"
reject 'Node-only local host' bash "${client}" plan --config "${work}/invalid.local.yaml"
yq -i '.hosts."other-host".transport = "ssh" | .hosts."other-host".ssh = .hosts."web-host".ssh' "${work}/invalid.local.yaml"
reject 'two host ids same SSH endpoint' bash "${client}" plan --config "${work}/invalid.local.yaml"
yq -i '.hosts."other-host".ssh.hostname = "203.0.113.30" | .nodes[0].host = "web-host" | .nodes[0].settings.grpc_port = .web.settings.panel_port' "${work}/invalid.local.yaml"
reject 'combined port conflict' bash "${client}" plan --config "${work}/invalid.local.yaml"

for bad_ip in ':' ':::' '2001::db8::1' '12345::1' '1:2:3:4:5:6:7:8:9' '999.1.1.1'; do
  cp "${config}" "${work}/invalid-address.local.yaml"
  yq -i ".web.public_ip = \"${bad_ip}\"" "${work}/invalid-address.local.yaml"
  reject "invalid public IP ${bad_ip}" bash "${client}" plan --config "${work}/invalid-address.local.yaml"
done
for bad_domain in 'panel..example.com' 'panel-.example.com' 'panel.example..com' 'panel.example.com.' 'panel.12'; do
  cp "${config}" "${work}/invalid-address.local.yaml"
  yq -i ".web.domain = \"${bad_domain}\"" "${work}/invalid-address.local.yaml"
  reject "invalid domain ${bad_domain}" bash "${client}" plan --config "${work}/invalid-address.local.yaml"
done
cp "${config}" "${work}/valid-ipv6.local.yaml"
yq -i '.web.public_ip = "2001:db8::10" | .nodes[0].public_ip = "2001:db8:0:0:0:0:0:20"' "${work}/valid-ipv6.local.yaml"
bash "${client}" plan --config "${work}/valid-ipv6.local.yaml" >/dev/null

yq -i '.web.passwords.sysadmin = "TEST_SECRET_MUST_NOT_APPEAR"' "${config}"
[[ "$(bash "${client}" plan --config "${config}")" != *TEST_SECRET_MUST_NOT_APPEAR* ]] || fail 'plan logged a secret'
bash -x "${client}" plan --config "${config}" >"${work}/trace.out" 2>&1
! grep -Fq TEST_SECRET_MUST_NOT_APPEAR "${work}/trace.out" || fail 'trace logged a secret'

# Git worktree paths must be ignored even when the caller chooses another name.
mkdir "${work}/repo"
git -C "${work}/repo" init -q
printf '*.local.yaml\n*.local\n' >"${work}/repo/.gitignore"
reject 'Git-trackable config' bash "${client}" init --config "${work}/repo/unsafe.yaml" --tag v1.2.3 --sha256 "${sha}" --work-dir "${work}/repo/assets.local"
[[ ! -e "${work}/repo/unsafe.yaml" ]] || fail 'Git-trackable config was created'
bash "${client}" init --config "${work}/repo/safe.local.yaml" --tag v1.2.3 --sha256 "${sha}" --work-dir "${work}/repo/assets.local" >/dev/null
git -C "${work}/repo" add -A
[[ "$(git -C "${work}/repo" ls-files)" == .gitignore ]] || fail 'Git add tracked a sensitive configuration or work directory'

printf 'PASS local unified init and topology contract (mikefarah yq v4)\n'
