#!/usr/bin/env bash
set -Eeuo pipefail

INSTALLER="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../scripts/deploy" && pwd)/install.sh"
SCRIPT_DIR="$(dirname "${INSTALLER}")"
CONFIG="${SCRIPT_DIR}/config.sh"
VALIDATOR="${SCRIPT_DIR}/validate.sh"
UNINSTALLER="${SCRIPT_DIR}/uninstall.sh"
ENTRYPOINT="${SCRIPT_DIR}/../tp.sh"
TEMPLATES="${SCRIPT_DIR}/templates"
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
"${ENTRYPOINT}" --help | grep -q 'config web|node'
if "${INSTALLER}" --help | grep -q -- '--mode'; then fail 'old option appears in help'; fi
VERSION="$("${INSTALLER}" --version)"
[[ "${VERSION}" =~ ^[0-9]+\.[0-9]+(\.[0-9]+)?(-[0-9A-Za-z.-]+)?$ ]] || fail 'invalid installer version'
"${CONFIG}" --help >/dev/null
"${CONFIG}" web --help >/dev/null
assert_error 'Unknown command' "${ENTRYPOINT}" deploy
assert_error 'Unknown option: --mode' "${VALIDATOR}" --mode web --config "${TEMPLATES}/web.yaml"
assert_error 'Unknown option: --mode' "${INSTALLER}" --mode web --config "${TEMPLATES}/web.yaml"
assert_error 'Unknown option: --mode' "${UNINSTALLER}" --mode node --config "${TEMPLATES}/node.yaml"
assert_error '--config is required' "${VALIDATOR}"
assert_error '--config requires a value' "${VALIDATOR}" --config --help
assert_error '--force is only valid with install' "${VALIDATOR}" --config "${TEMPLATES}/web.yaml" --force
assert_error '--purge-data is only valid with remove' "${INSTALLER}" --config "${TEMPLATES}/web.yaml" --purge-data
assert_error '--client-ca requires a value' "${INSTALLER}" --config "${TEMPLATES}/node.yaml" --client-ca --help
assert_error '--client-ca is only valid with Node install' "${VALIDATOR}" --config "${TEMPLATES}/node.yaml" --client-ca ca.crt
assert_error '--client-ca is only valid with Node install' "${UNINSTALLER}" --config "${TEMPLATES}/node.yaml" --client-ca ca.crt
assert_error 'Unknown config option: --force' "${CONFIG}" web --force
assert_error '--output requires a value' "${CONFIG}" web --output
assert_error '--output requires a value' "${CONFIG}" web --output --help
assert_fails "${CONFIG}" invalid

"${VALIDATOR}" --config "${TEMPLATES}/web.yaml" | grep -q 'valid for web purpose'
"${VALIDATOR}" --config "${TEMPLATES}/node.yaml" | grep -q 'valid for node purpose'
sed '/purpose: web/d' "${TEMPLATES}/web.yaml" >"${TEST_DIR}/missing.yaml"
assert_error 'trojanpanelnext.purpose is required' env TP_PURPOSE=web "${VALIDATOR}" --config "${TEST_DIR}/missing.yaml"
sed 's/purpose: web/purpose: abc/' "${TEMPLATES}/web.yaml" >"${TEST_DIR}/invalid.yaml"
assert_error 'Unsupported trojanpanelnext.purpose: abc' "${VALIDATOR}" --config "${TEST_DIR}/invalid.yaml"
sed 's/schema_version: 1/schema_version: 2/' "${TEMPLATES}/web.yaml" >"${TEST_DIR}/schema.yaml"
assert_error 'Unsupported schema version: 2' "${VALIDATOR}" --config "${TEST_DIR}/schema.yaml"
sed '/release:/d' "${TEMPLATES}/web.yaml" >"${TEST_DIR}/missing-release.yaml"
assert_error 'Configuration release <missing> does not match script release' "${VALIDATOR}" --config "${TEST_DIR}/missing-release.yaml"
sed 's/release: "1.0.2-rc.9"/release: "2.0"/' "${TEMPLATES}/web.yaml" >"${TEST_DIR}/wrong-release.yaml"
assert_error 'Configuration release 2.0 does not match script release' "${VALIDATOR}" --config "${TEST_DIR}/wrong-release.yaml"
sed 's/trojanpanelnext-api:1.0.2-rc.9/trojanpanelnext-api:0.9/' "${TEMPLATES}/web.yaml" >"${TEST_DIR}/wrong-api.yaml"
assert_error 'PANEL_IMAGE must be ghcr.io/1linhao/trojanpanelnext-api:1.0.2-rc.9' "${VALIDATOR}" --config "${TEST_DIR}/wrong-api.yaml"
sed 's/trojanpanelnext-web:1.0.2-rc.9/trojanpanelnext-web:latest/' "${TEMPLATES}/web.yaml" >"${TEST_DIR}/wrong-ui.yaml"
assert_error 'UI_IMAGE must be ghcr.io/1linhao/trojanpanelnext-web:1.0.2-rc.9' "${VALIDATOR}" --config "${TEST_DIR}/wrong-ui.yaml"
sed 's/trojanpanelnext-node-agent:1.0.2-rc.9/trojanpanelnext-node-agent:2.0/' "${TEMPLATES}/node.yaml" >"${TEST_DIR}/wrong-agent.yaml"
assert_error 'CORE_IMAGE must be ghcr.io/1linhao/trojanpanelnext-node-agent:1.0.2-rc.9' "${VALIDATOR}" --config "${TEST_DIR}/wrong-agent.yaml"
assert_error 'Release ref must be v1.0.2-rc.9' env TP_RELEASE_REF=main "${VALIDATOR}" --config "${TEMPLATES}/web.yaml"
printf 'trojanpanelnext: [\n' >"${TEST_DIR}/malformed.yaml"
assert_error "Configuration must contain a 'trojanpanelnext' root object" "${VALIDATOR}" --config "${TEST_DIR}/malformed.yaml"
sed 's/grpc_tls_mode: mtls/grpc_tls_mode: legacy/' "${TEMPLATES}/node.yaml" >"${TEST_DIR}/legacy.yaml"
assert_fails "${VALIDATOR}" --config "${TEST_DIR}/legacy.yaml"

# Dependency failures must not download tools or require root for validate.
# Exported to the installer subprocess.
# shellcheck disable=SC2317,SC2329
command() {
  if [[ "${1:-}" == -v && "${2:-}" == "${MOCK_MISSING_COMMAND:-}" ]]; then return 1; fi
  builtin command "$@"
}
export -f command
assert_error 'Missing dependencies: yq' env MOCK_MISSING_COMMAND=yq "${VALIDATOR}" --config "${TEMPLATES}/web.yaml"
assert_error 'Missing dependencies: curl' env MOCK_MISSING_COMMAND=curl "${CONFIG}" web --output "${TEST_DIR}/missing-curl.yaml"
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
  version) sed 's/release: "1.0.2-rc.9"/release: "9.9"/' "${MOCK_TEMPLATE}" >"${destination}" ;;
  purpose) sed 's/purpose: web/purpose: node/' "${MOCK_TEMPLATE}" >"${destination}" ;;
  race) cp "${MOCK_TEMPLATE}" "${destination}"; printf 'existing-secrets\n' >"${MOCK_OUTPUT}" ;;
  term) printf 'partial' >"${destination}"; kill -TERM "${BASHPID}" ;;
  *) cp "${MOCK_TEMPLATE}" "${destination}" ;;
  esac
}
export -f curl
export MOCK_REQUESTS="${TEST_DIR}/requests"
export MOCK_TEMPLATE="${TEMPLATES}/web.yaml"
"${CONFIG}" web --output "${TEST_DIR}/web.yaml"
test "$(stat -c '%a' "${TEST_DIR}/web.yaml")" = 600
cmp "${MOCK_TEMPLATE}" "${TEST_DIR}/web.yaml"
grep -Fq "/v${VERSION}/scripts/deploy/templates/web.yaml" "${MOCK_REQUESTS}"
"${VALIDATOR}" --config "${TEST_DIR}/web.yaml" >/dev/null
requests_before="$(wc -l <"${MOCK_REQUESTS}")"
assert_error 'Refusing to overwrite existing config' "${CONFIG}" web --output "${TEST_DIR}/web.yaml"
test "$(wc -l <"${MOCK_REQUESTS}")" = "${requests_before}"
cmp "${MOCK_TEMPLATE}" "${TEST_DIR}/web.yaml"
ln -s "${TEST_DIR}/missing" "${TEST_DIR}/link.yaml"
assert_error 'Refusing to overwrite existing config' "${CONFIG}" web --output "${TEST_DIR}/link.yaml"
test -L "${TEST_DIR}/link.yaml"
assert_error 'Output directory does not exist' "${CONFIG}" web --output "${TEST_DIR}/missing/file.yaml"

MOCK_TEMPLATE="${TEMPLATES}/node.yaml" \
  "${CONFIG}" node --output "${TEST_DIR}/node.yaml"
grep -q "/v${VERSION}/scripts/deploy/templates/node.yaml$" "${MOCK_REQUESTS}"
"${VALIDATOR}" --config "${TEST_DIR}/node.yaml" >/dev/null
test "$(stat -c '%a' "${TEST_DIR}/node.yaml")" = 600
for outcome in fail empty html version purpose term; do
  assert_fails env MOCK_DOWNLOAD="${outcome}" "${CONFIG}" web --output "${TEST_DIR}/${outcome}.yaml"
  assert_clean "${TEST_DIR}/${outcome}.yaml"
done
assert_fails env MOCK_DOWNLOAD=race MOCK_OUTPUT="${TEST_DIR}/race.yaml" "${CONFIG}" web --output "${TEST_DIR}/race.yaml"
grep -qx 'existing-secrets' "${TEST_DIR}/race.yaml"
[[ -z "$(find "${TEST_DIR}" -name '*.tmp.*' -print -quit)" ]] || fail 'temporary file remains after collision'
# Exported to the installer subprocess.
# shellcheck disable=SC2317,SC2329
chmod() { if [[ "${MOCK_CHMOD_FAIL:-0}" == 1 ]]; then return 1; fi; builtin command chmod "$@"; }
export -f chmod
assert_fails env MOCK_CHMOD_FAIL=1 "${CONFIG}" web --output "${TEST_DIR}/chmod.yaml"
assert_clean "${TEST_DIR}/chmod.yaml"
unset -f chmod
(cd "${TEST_DIR}"; "${CONFIG}" web >/dev/null) && fail 'default web path overwritten'
(cd "${TEST_DIR}"; "${CONFIG}" node >/dev/null) && fail 'default node path overwritten'
mkdir "${TEST_DIR}/default-output"
(cd "${TEST_DIR}/default-output"; MOCK_MISSING_COMMAND=yq "${CONFIG}" web >/dev/null)
unset -f command
(cd "${TEST_DIR}/default-output"; MOCK_TEMPLATE="${TEMPLATES}/node.yaml" "${CONFIG}" node >/dev/null)
cmp "${TEMPLATES}/web.yaml" "${TEST_DIR}/default-output/web.yaml"
cmp "${TEMPLATES}/node.yaml" "${TEST_DIR}/default-output/node.yaml"
unset -f curl

# Exercise each independent command without creating/removing real containers.
for purpose in web node; do
  if [[ "${purpose}" == web ]]; then file="${TEMPLATES}/web.yaml"; else file="${TEMPLATES}/node.yaml"; fi
  for action in install remove; do
    if [[ "${action}" == install ]]; then flag=--force; script="${INSTALLER}"; else flag=--purge-data; script="${UNINSTALLER}"; fi
    for override in 0 1; do
      options=()
      [[ "${override}" == 1 ]] && options+=("${flag}")
      bash -c '
        source "$1"; shift
        require_root() { :; }; require_commands() { :; }; docker() { return 0; }
        deploy_web() { printf "called:install:web:%s\n" "$TP_FORCE"; }
        deploy_node() { printf "called:install:node:%s\n" "$TP_FORCE"; }
        remove_project() { printf "called:remove:%s:%s\n" "$TP_PURPOSE" "$TP_PURGE_DATA"; }
        main "$@"
      ' test "${script}" --config "${file}" "${options[@]}" >"${TEST_DIR}/dispatch"
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

# Exercise the install CLI's trust hand-off with a real configuration and CA.
# The unrelated container deployment is replaced by its PKI preparation step.
invoke_node_install() (
  # shellcheck source=scripts/deploy/install.sh
  source "${INSTALLER}"
  require_root() { :; }
  require_commands() { :; }
  docker() { return 0; }
  deploy_node() { install_pki_material node; }
  main "$@"
)
assert_error '--client-ca is only valid with Node install' invoke_node_install --config "${TEMPLATES}/web.yaml" --client-ca ca.crt
ca_install_dir="${TEST_DIR}/ca-install"
mkdir -p "${ca_install_dir}/runtime" "${ca_install_dir}/bootstrap"
openssl req -x509 -newkey rsa:2048 -sha256 -days 2 -nodes \
  -subj /CN=Current-Web-CA -addext basicConstraints=critical,CA:TRUE \
  -keyout "${ca_install_dir}/current.key" -out "${ca_install_dir}/current.crt" >/dev/null 2>&1
cp "${TEMPLATES}/node.yaml" "${ca_install_dir}/node.yaml"
TP_TEST_BUNDLE="${ca_install_dir}/bootstrap" \
  TP_TEST_RUNTIME_CA="${ca_install_dir}/runtime/client-ca.crt" \
  yq -i '.trojanpanelnext.pki_bundle_dir = strenv(TP_TEST_BUNDLE) | .trojanpanelnext.grpc_client_ca_path = strenv(TP_TEST_RUNTIME_CA)' "${ca_install_dir}/node.yaml"
cp "${pki_dir}/client-ca.crt" "${ca_install_dir}/runtime/client-ca.crt"
cp "${ca_install_dir}/current.crt" "${ca_install_dir}/bootstrap/client-ca.crt"
invoke_node_install --config "${ca_install_dir}/node.yaml" --client-ca "${ca_install_dir}/current.crt" >"${TEST_DIR}/dispatch"
cmp "${ca_install_dir}/current.crt" "${ca_install_dir}/runtime/client-ca.crt"
cmp "${ca_install_dir}/current.crt" "${ca_install_dir}/bootstrap/client-ca.crt"
backup="$(find "${ca_install_dir}/bootstrap/client-ca-backups" -name runtime-client-ca.crt -print -quit)"
test -n "${backup}"
cmp "${pki_dir}/client-ca.crt" "${backup}"
test "$(stat -c %a "${backup}")" = 600
test "$(stat -c %a "$(dirname "${backup}")")" = 700

# Rebinding also archives a stale bootstrap copy, and never reads it back over
# the package CA. The package source may itself be the bootstrap path.
printf 'old-bootstrap-copy\n' >"${ca_install_dir}/bootstrap/client-ca.crt"
cp "${pki_dir}/client-ca.crt" "${ca_install_dir}/runtime/client-ca.crt"
invoke_node_install --config "${ca_install_dir}/node.yaml" --client-ca "${ca_install_dir}/current.crt" >"${TEST_DIR}/dispatch"
bootstrap_backup="$(find "${ca_install_dir}/bootstrap/client-ca-backups" -name bootstrap-client-ca.crt -print -quit)"
grep -qx 'old-bootstrap-copy' "${bootstrap_backup}"
cp "${pki_dir}/client-ca.crt" "${ca_install_dir}/runtime/client-ca.crt"
invoke_node_install --config "${ca_install_dir}/node.yaml" --client-ca "${ca_install_dir}/bootstrap/client-ca.crt" >"${TEST_DIR}/dispatch"
cmp "${ca_install_dir}/current.crt" "${ca_install_dir}/runtime/client-ca.crt"

# Ordinary installation retains a live rotation bundle, including multiple CAs.
cat "${pki_dir}/client-ca.crt" "${ca_install_dir}/current.crt" >"${ca_install_dir}/rotation.crt"
cp "${ca_install_dir}/rotation.crt" "${ca_install_dir}/runtime/client-ca.crt"
cp "${ca_install_dir}/current.crt" "${ca_install_dir}/bootstrap/client-ca.crt"
invoke_node_install --config "${ca_install_dir}/node.yaml" >"${TEST_DIR}/dispatch"
cmp "${ca_install_dir}/rotation.crt" "${ca_install_dir}/runtime/client-ca.crt"
cmp "${ca_install_dir}/rotation.crt" "${ca_install_dir}/bootstrap/client-ca.crt"

# Invalid explicit inputs must leave both active trust files and backups intact.
openssl req -x509 -newkey rsa:2048 -sha256 -days 2 -nodes \
  -subj /CN=Not-A-CA -addext basicConstraints=critical,CA:FALSE \
  -keyout "${ca_install_dir}/leaf.key" -out "${ca_install_dir}/leaf.crt" >/dev/null 2>&1
openssl req -x509 -newkey rsa:2048 -sha256 -days 2 -nodes \
  -subj /CN=Non-Signing-CA -addext basicConstraints=critical,CA:TRUE \
  -addext keyUsage=critical,digitalSignature \
  -keyout "${ca_install_dir}/no-sign.key" -out "${ca_install_dir}/no-sign.crt" >/dev/null 2>&1
openssl req -new -key "${ca_install_dir}/current.key" -subj /CN=Future-CA -out "${ca_install_dir}/future.csr" >/dev/null 2>&1
: >"${ca_install_dir}/ca-index"
printf '1000\n' >"${ca_install_dir}/ca-serial"
cat >"${ca_install_dir}/ca.conf" <<EOF
[ca]
default_ca = issuer
[issuer]
database = ${ca_install_dir}/ca-index
serial = ${ca_install_dir}/ca-serial
new_certs_dir = ${ca_install_dir}
certificate = ${ca_install_dir}/current.crt
private_key = ${ca_install_dir}/current.key
default_md = sha256
policy = any_name
x509_extensions = signing_ca
[any_name]
commonName = supplied
[signing_ca]
basicConstraints = critical,CA:TRUE
keyUsage = critical,keyCertSign,cRLSign
EOF
openssl ca -batch -notext -config "${ca_install_dir}/ca.conf" -in "${ca_install_dir}/future.csr" \
  -startdate "$(date -u -d '+1 day' +%Y%m%d%H%M%SZ)" -enddate "$(date -u -d '+2 days' +%Y%m%d%H%M%SZ)" \
  -out "${ca_install_dir}/future.crt" >/dev/null 2>&1
printf 'not a CA\n' >"${ca_install_dir}/invalid.crt"
cat "${ca_install_dir}/current.crt" "${ca_install_dir}/current.key" >"${ca_install_dir}/private.crt"
ln -s current.crt "${ca_install_dir}/link.crt"
: >"${ca_install_dir}/empty.crt"
backups_before="$(find "${ca_install_dir}/bootstrap/client-ca-backups" -type f | wc -l)"
for source in missing.crt leaf.crt invalid.crt private.crt link.crt empty.crt runtime no-sign.crt future.crt; do
  assert_error 'Invalid public client CA certificate' invoke_node_install --config "${ca_install_dir}/node.yaml" --client-ca "${ca_install_dir}/${source}"
  cmp "${ca_install_dir}/rotation.crt" "${ca_install_dir}/runtime/client-ca.crt"
  cmp "${ca_install_dir}/rotation.crt" "${ca_install_dir}/bootstrap/client-ca.crt"
  test "$(find "${ca_install_dir}/bootstrap/client-ca-backups" -type f | wc -l)" = "${backups_before}"
done
[[ -z "$(find "${ca_install_dir}" -name '*.tmp.*' -print -quit)" ]] || fail 'temporary CA replacement remained'

# A deployment cannot follow symlinked trust directories into another service's
# files or change that service's directory permissions.
mkdir -m 755 "${ca_install_dir}/external-pki"
cp "${pki_dir}/client-ca.crt" "${ca_install_dir}/external-pki/client-ca.crt"
ln -s external-pki "${ca_install_dir}/linked-pki"
cp "${ca_install_dir}/node.yaml" "${ca_install_dir}/linked-pki.yaml"
TP_TEST_BUNDLE="${ca_install_dir}/linked-pki" \
  yq -i '.trojanpanelnext.pki_bundle_dir = strenv(TP_TEST_BUNDLE)' "${ca_install_dir}/linked-pki.yaml"
assert_error 'Client CA directory must be a regular directory without symlinks' invoke_node_install --config "${ca_install_dir}/linked-pki.yaml" --client-ca "${ca_install_dir}/current.crt"
cmp "${pki_dir}/client-ca.crt" "${ca_install_dir}/external-pki/client-ca.crt"
cmp "${ca_install_dir}/rotation.crt" "${ca_install_dir}/runtime/client-ca.crt"
test "$(stat -c %a "${ca_install_dir}/external-pki")" = 755
test ! -e "${ca_install_dir}/external-pki/client-ca-backups"
TP_TEST_BUNDLE="${ca_install_dir}/linked-pki/new-pki" \
  yq -i '.trojanpanelnext.pki_bundle_dir = strenv(TP_TEST_BUNDLE)' "${ca_install_dir}/linked-pki.yaml"
assert_error 'Client CA directory must be a regular directory without symlinks' invoke_node_install --config "${ca_install_dir}/linked-pki.yaml" --client-ca "${ca_install_dir}/current.crt"
test ! -e "${ca_install_dir}/external-pki/new-pki"
test "$(stat -c %a "${ca_install_dir}/external-pki")" = 755

mkdir -m 755 "${ca_install_dir}/external-runtime"
cp "${pki_dir}/client-ca.crt" "${ca_install_dir}/external-runtime/client-ca.crt"
ln -s external-runtime "${ca_install_dir}/linked-runtime"
cp "${ca_install_dir}/node.yaml" "${ca_install_dir}/linked-runtime.yaml"
TP_TEST_RUNTIME_CA="${ca_install_dir}/linked-runtime/client-ca.crt" \
  yq -i '.trojanpanelnext.grpc_client_ca_path = strenv(TP_TEST_RUNTIME_CA)' "${ca_install_dir}/linked-runtime.yaml"
assert_error 'Client CA directory must be a regular directory without symlinks' invoke_node_install --config "${ca_install_dir}/linked-runtime.yaml" --client-ca "${ca_install_dir}/current.crt"
cmp "${pki_dir}/client-ca.crt" "${ca_install_dir}/external-runtime/client-ca.crt"
cmp "${ca_install_dir}/rotation.crt" "${ca_install_dir}/bootstrap/client-ca.crt"
test "$(stat -c %a "${ca_install_dir}/external-runtime")" = 755
test "$(find "${ca_install_dir}/bootstrap/client-ca-backups" -type f | wc -l)" = "${backups_before}"

printf 'PASS independent command CLI, remote configuration, dependency, dispatch and PKI contracts\n'
