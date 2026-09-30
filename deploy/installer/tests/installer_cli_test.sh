#!/usr/bin/env bash
set -Eeuo pipefail

INSTALLER="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/install.sh"
EXAMPLES="$(dirname "${INSTALLER}")/examples"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf -- "${TEST_DIR}"' EXIT

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
assert_fails() {
  if "$@" >"${TEST_DIR}/stdout" 2>"${TEST_DIR}/stderr"; then
    fail "command unexpectedly succeeded: $*"
  fi
}
assert_error() {
  local expected="$1"
  shift
  assert_fails "$@"
  cat "${TEST_DIR}/stdout" "${TEST_DIR}/stderr" | grep -Fq -- "${expected}" || fail "missing error: ${expected}"
}
assert_clean() {
  [[ ! -e "$1" ]] || fail "unexpected destination: $1"
  [[ -z "$(find "${TEST_DIR}" -name '*.tmp.*' -print -quit)" ]] || fail 'temporary configuration was not removed'
}

command -v yq >/dev/null || fail 'Install mikefarah/yq v4 before running these tests'
bash -n "${INSTALLER}"
"${INSTALLER}" --help | grep -q 'config web|node'
if "${INSTALLER}" --help | grep -q -- '--mode'; then fail 'old option appears in help'; fi
VERSION="$("${INSTALLER}" --version)"
[[ "${VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || fail 'invalid installer version'
"${INSTALLER}" config --help >/dev/null
"${INSTALLER}" config web --help >/dev/null
assert_error 'Unknown command' "${INSTALLER}" deploy
assert_error 'Unknown option: --mode' "${INSTALLER}" validate --mode web --config "${EXAMPLES}/web.yaml"
assert_error 'Unknown option: --mode' "${INSTALLER}" install --mode web --config "${EXAMPLES}/web.yaml"
assert_error 'Unknown option: --mode' "${INSTALLER}" remove --mode node --config "${EXAMPLES}/node-agent.yaml"
assert_error '--config is required' "${INSTALLER}" validate
assert_error '--config requires a value' "${INSTALLER}" validate --config --help
assert_error '--force is only valid with install' "${INSTALLER}" validate --config "${EXAMPLES}/web.yaml" --force
assert_error '--purge-data is only valid with remove' "${INSTALLER}" install --config "${EXAMPLES}/web.yaml" --purge-data
assert_error 'Unknown config option: --force' "${INSTALLER}" config web --force
assert_error '--output requires a value' "${INSTALLER}" config web --output
assert_error '--output requires a value' "${INSTALLER}" config web --output --help
assert_fails "${INSTALLER}" config invalid

"${INSTALLER}" validate --config "${EXAMPLES}/web.yaml" | grep -q 'valid for web purpose'
"${INSTALLER}" validate --config "${EXAMPLES}/node-agent.yaml" | grep -q 'valid for node purpose'
sed '/purpose: web/d' "${EXAMPLES}/web.yaml" >"${TEST_DIR}/missing.yaml"
assert_error 'trojanpanelnext.purpose is required' env TP_PURPOSE=web "${INSTALLER}" validate --config "${TEST_DIR}/missing.yaml"
sed 's/purpose: web/purpose: abc/' "${EXAMPLES}/web.yaml" >"${TEST_DIR}/invalid.yaml"
assert_error 'Unsupported trojanpanelnext.purpose: abc' "${INSTALLER}" validate --config "${TEST_DIR}/invalid.yaml"
sed 's/schema_version: 1/schema_version: 2/' "${EXAMPLES}/web.yaml" >"${TEST_DIR}/schema.yaml"
assert_error 'Unsupported schema version: 2' "${INSTALLER}" validate --config "${TEST_DIR}/schema.yaml"
printf 'trojanpanelnext: [\n' >"${TEST_DIR}/malformed.yaml"
assert_error "Configuration must contain a 'trojanpanelnext' root object" "${INSTALLER}" validate --config "${TEST_DIR}/malformed.yaml"
sed 's/grpc_tls_mode: mtls/grpc_tls_mode: legacy/' "${EXAMPLES}/node-agent.yaml" >"${TEST_DIR}/legacy.yaml"
assert_fails "${INSTALLER}" validate --config "${TEST_DIR}/legacy.yaml"

# Dependency failures must not download tools or require root for validate.
# Exported to the installer subprocess.
# shellcheck disable=SC2317,SC2329
command() {
  if [[ "${1:-}" == -v && "${2:-}" == "${MOCK_MISSING_COMMAND:-}" ]]; then return 1; fi
  builtin command "$@"
}
export -f command
assert_error 'Missing dependencies: yq' env MOCK_MISSING_COMMAND=yq "${INSTALLER}" validate --config "${EXAMPLES}/web.yaml"
assert_error 'Missing dependencies: curl' env MOCK_MISSING_COMMAND=curl "${INSTALLER}" config web --output "${TEST_DIR}/missing-curl.yaml"
bash -c 'source "$1"; yq() { printf "yq 3.0.0\n"; }; require_yq' test "${INSTALLER}" >"${TEST_DIR}/stdout" 2>"${TEST_DIR}/stderr" && fail 'incompatible yq accepted'
grep -q 'Unsupported yq' "${TEST_DIR}/stderr"

# Only network transport is mocked. Purpose and schema use real mikefarah/yq.
# Exported to the installer subprocess.
# shellcheck disable=SC2317,SC2329
curl() {
  local destination="" url=""
  while (($#)); do
    case "$1" in
    -o) destination="$2"; shift 2 ;;
    https://*) url="$1"; shift ;;
    *) shift ;;
    esac
  done
  [[ -n "${destination}" ]] || return 2
  printf '%s\n' "${url}" >>"${MOCK_REQUESTS}"
  case "${MOCK_DOWNLOAD:-success}" in
  fail) printf 'partial' >"${destination}"; return 22 ;;
  empty) : >"${destination}" ;;
  html) printf '<!doctype html><html>error</html>\n' >"${destination}" ;;
  race) cp "${MOCK_TEMPLATE}" "${destination}"; printf 'existing-secrets\n' >"${MOCK_OUTPUT}" ;;
  term) printf 'partial' >"${destination}"; kill -TERM "${BASHPID}" ;;
  *) cp "${MOCK_TEMPLATE}" "${destination}" ;;
  esac
}
export -f curl
export MOCK_REQUESTS="${TEST_DIR}/requests"
export MOCK_TEMPLATE="${EXAMPLES}/web.yaml"
"${INSTALLER}" config web --output "${TEST_DIR}/web.yaml"
test "$(stat -c '%a' "${TEST_DIR}/web.yaml")" = 600
cmp "${MOCK_TEMPLATE}" "${TEST_DIR}/web.yaml"
grep -Fq "/v${VERSION}/deploy/installer/examples/web.yaml" "${MOCK_REQUESTS}"
"${INSTALLER}" validate --config "${TEST_DIR}/web.yaml" >/dev/null
requests_before="$(wc -l <"${MOCK_REQUESTS}")"
assert_error 'Refusing to overwrite existing config' "${INSTALLER}" config web --output "${TEST_DIR}/web.yaml"
test "$(wc -l <"${MOCK_REQUESTS}")" = "${requests_before}"
cmp "${MOCK_TEMPLATE}" "${TEST_DIR}/web.yaml"
ln -s "${TEST_DIR}/missing" "${TEST_DIR}/link.yaml"
assert_error 'Refusing to overwrite existing config' "${INSTALLER}" config web --output "${TEST_DIR}/link.yaml"
test -L "${TEST_DIR}/link.yaml"
assert_error 'Output directory does not exist' "${INSTALLER}" config web --output "${TEST_DIR}/missing/file.yaml"

MOCK_TEMPLATE="${EXAMPLES}/node-agent.yaml" TP_CONFIG_REF=feature/test-installer \
  "${INSTALLER}" config node --output "${TEST_DIR}/node.yaml"
grep -q '/feature/test-installer/deploy/installer/examples/node-agent.yaml$' "${MOCK_REQUESTS}"
"${INSTALLER}" validate --config "${TEST_DIR}/node.yaml" >/dev/null
test "$(stat -c '%a' "${TEST_DIR}/node.yaml")" = 600
for outcome in fail empty html term; do
  assert_fails env MOCK_DOWNLOAD="${outcome}" "${INSTALLER}" config web --output "${TEST_DIR}/${outcome}.yaml"
  assert_clean "${TEST_DIR}/${outcome}.yaml"
done
assert_fails env MOCK_DOWNLOAD=race MOCK_OUTPUT="${TEST_DIR}/race.yaml" "${INSTALLER}" config web --output "${TEST_DIR}/race.yaml"
grep -qx 'existing-secrets' "${TEST_DIR}/race.yaml"
[[ -z "$(find "${TEST_DIR}" -name '*.tmp.*' -print -quit)" ]] || fail 'temporary file remains after collision'
# Exported to the installer subprocess.
# shellcheck disable=SC2317,SC2329
chmod() { if [[ "${MOCK_CHMOD_FAIL:-0}" == 1 ]]; then return 1; fi; builtin command chmod "$@"; }
export -f chmod
assert_fails env MOCK_CHMOD_FAIL=1 "${INSTALLER}" config web --output "${TEST_DIR}/chmod.yaml"
assert_clean "${TEST_DIR}/chmod.yaml"
unset -f chmod
(cd "${TEST_DIR}"; "${INSTALLER}" config web >/dev/null) && fail 'default web path overwritten'
(cd "${TEST_DIR}"; "${INSTALLER}" config node >/dev/null) && fail 'default node path overwritten'
mkdir "${TEST_DIR}/default-output"
(cd "${TEST_DIR}/default-output"; MOCK_MISSING_COMMAND=yq "${INSTALLER}" config web >/dev/null)
unset -f command
(cd "${TEST_DIR}/default-output"; MOCK_TEMPLATE="${EXAMPLES}/node-agent.yaml" "${INSTALLER}" config node >/dev/null)
cmp "${EXAMPLES}/web.yaml" "${TEST_DIR}/default-output/web.yaml"
cmp "${EXAMPLES}/node-agent.yaml" "${TEST_DIR}/default-output/node.yaml"
unset -f curl

# Exercise CLI dispatch without creating/removing real containers.
for purpose in web node; do
  if [[ "${purpose}" == web ]]; then file="${EXAMPLES}/web.yaml"; else file="${EXAMPLES}/node-agent.yaml"; fi
  for action in install remove; do
    if [[ "${action}" == install ]]; then flag=--force; else flag=--purge-data; fi
    for override in 0 1; do
      options=()
      [[ "${override}" == 1 ]] && options+=("${flag}")
      bash -c '
        source "$1"; shift
        require_root() { :; }; require_commands() { :; }; docker() { return 0; }
        deploy_web() { printf "called:install:web:%s\n" "$TP_FORCE"; }
        deploy_node() { printf "called:install:node:%s\n" "$TP_FORCE"; }
        remove_web() { printf "called:remove:web:%s\n" "$TP_PURGE_DATA"; }
        remove_node() { printf "called:remove:node:%s\n" "$TP_PURGE_DATA"; }
        main "$@"
      ' test "${INSTALLER}" "${action}" --config "${file}" "${options[@]}" >"${TEST_DIR}/dispatch"
      grep -q "called:${action}:${purpose}:${override}" "${TEST_DIR}/dispatch"
    done
  done
done

pki_dir="${TEST_DIR}/pki"
node_pki_dir="${TEST_DIR}/node-pki"
mkdir -p "${pki_dir}" "${node_pki_dir}"
bash -c 'source "$1"; TP_PKI_BUNDLE_DIR="$2"; generate_web_client_pki' test "${INSTALLER}" "${pki_dir}"
openssl verify -CAfile "${pki_dir}/client-ca.crt" "${pki_dir}/client.crt" >/dev/null
test "$(stat -c '%a' "${pki_dir}/client-ca.key")" = 600
test "$(stat -c '%a' "${pki_dir}/client.key")" = 600
# shellcheck disable=SC2016
assert_fails bash -c 'source "$1"; TP_PKI_BUNDLE_DIR="$2"; GRPC_CLIENT_CA_PATH="$3/client-ca.crt"; install_pki_material node' \
  test "${INSTALLER}" "${node_pki_dir}" "${TEST_DIR}/node-runtime"
cp "${pki_dir}/client-ca.crt" "${node_pki_dir}/client-ca.crt"
bash -c 'source "$1"; TP_PKI_BUNDLE_DIR="$2"; GRPC_CLIENT_CA_PATH="$3/client-ca.crt"; install_pki_material node' \
  test "${INSTALLER}" "${node_pki_dir}" "${TEST_DIR}/node-runtime"
cmp "${pki_dir}/client-ca.crt" "${TEST_DIR}/node-runtime/client-ca.crt"
printf 'stale-bootstrap-copy\n' >"${node_pki_dir}/client-ca.crt"
bash -c 'source "$1"; TP_PKI_BUNDLE_DIR="$2"; GRPC_CLIENT_CA_PATH="$3/client-ca.crt"; install_pki_material node' \
  test "${INSTALLER}" "${node_pki_dir}" "${TEST_DIR}/node-runtime"
cmp "${pki_dir}/client-ca.crt" "${TEST_DIR}/node-runtime/client-ca.crt"

printf 'PASS installer CLI, remote configuration, dependency, dispatch and PKI contracts\n'
