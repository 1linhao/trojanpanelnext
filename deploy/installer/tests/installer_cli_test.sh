#!/usr/bin/env bash
set -Eeuo pipefail

INSTALLER="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/install.sh"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

assert_fails() {
  if "$@" >/dev/null 2>&1; then
    fail "command unexpectedly succeeded: $*"
  fi
}

# Minimal yq-compatible reader for the flat example schema. This keeps the CLI
# contract test hermetic; production installations use mikefarah/yq.
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

bash -n "${INSTALLER}"
"${INSTALLER}" --help | grep -q 'install.*--mode web|node'
assert_fails "${INSTALLER}" deploy --mode web --config /dev/null
assert_fails "${INSTALLER}" validate --mode invalid --config /dev/null
assert_fails "${INSTALLER}" validate --mode web
assert_fails "${INSTALLER}" validate --mode web --config /dev/null --force
assert_fails "${INSTALLER}" install --mode web --config /dev/null --purge-data

"${INSTALLER}" validate --mode web \
  --config "$(dirname "${INSTALLER}")/examples/web.yaml" | grep -q 'valid for web purpose'
"${INSTALLER}" validate --mode node \
  --config "$(dirname "${INSTALLER}")/examples/node-agent.yaml" | grep -q 'valid for node purpose'
assert_fails "${INSTALLER}" validate --mode node \
  --config "$(dirname "${INSTALLER}")/examples/web.yaml"

legacy_config="$(mktemp)"
missing_purpose_config="$(mktemp)"
pki_dir="$(mktemp -d)"
node_pki_dir="$(mktemp -d)"
node_runtime_dir="$(mktemp -d)"
trap 'rm -f "${legacy_config}" "${missing_purpose_config}"; rm -rf -- "${pki_dir}" "${node_pki_dir}" "${node_runtime_dir}"' EXIT
sed 's/grpc_tls_mode: mtls/grpc_tls_mode: legacy/' \
  "$(dirname "${INSTALLER}")/examples/node-agent.yaml" >"${legacy_config}"
sed '/purpose: web/d' "$(dirname "${INSTALLER}")/examples/web.yaml" >"${missing_purpose_config}"
assert_fails "${INSTALLER}" validate --mode node --config "${legacy_config}"
assert_fails env TP_PURPOSE=web "${INSTALLER}" validate --mode web --config "${missing_purpose_config}"

bash -c 'set -Eeuo pipefail; source "$1"; TP_PKI_BUNDLE_DIR="$2"; generate_web_client_pki' \
  installer-test "${INSTALLER}" "${pki_dir}"
openssl verify -CAfile "${pki_dir}/client-ca.crt" "${pki_dir}/client.crt" >/dev/null
test "$(stat -c '%a' "${pki_dir}/client-ca.key")" = 600
test "$(stat -c '%a' "${pki_dir}/client.key")" = 600
assert_fails bash -c 'set -Eeuo pipefail; source "$1"; TP_PKI_BUNDLE_DIR="$2"; GRPC_CLIENT_CA_PATH="$3/client-ca.crt"; install_pki_material node' \
  installer-test "${INSTALLER}" "${node_pki_dir}" "${node_runtime_dir}"
cp "${pki_dir}/client-ca.crt" "${node_pki_dir}/client-ca.crt"
bash -c 'set -Eeuo pipefail; source "$1"; TP_PKI_BUNDLE_DIR="$2"; GRPC_CLIENT_CA_PATH="$3/client-ca.crt"; install_pki_material node' \
  installer-test "${INSTALLER}" "${node_pki_dir}" "${node_runtime_dir}"
cmp "${pki_dir}/client-ca.crt" "${node_runtime_dir}/client-ca.crt"

printf 'PASS installer CLI and purpose/config contract\n'
