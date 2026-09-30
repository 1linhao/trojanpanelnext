#!/usr/bin/env bash
# Quoted programs are intentionally expanded by the child Bash process.
# shellcheck disable=SC2016
set -Eeuo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf -- "${TEST_DIR}"' EXIT
export TEST_DIR
# shellcheck source=deploy/installer/install.sh
source "${SCRIPT_DIR}/install.sh"
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
assert_error() {
  local expected="$1"; shift
  if ("$@") >"${TEST_DIR}/out" 2>"${TEST_DIR}/err"; then fail 'invalid certificate accepted'; fi
  cat "${TEST_DIR}/out" "${TEST_DIR}/err" | grep -Fq "${expected}" || fail "missing error: ${expected}"
}

TP_PURPOSE=node
TP_NODE_DOMAIN=node.example.com
NODE_CERTIFICATE_MODE=external
TP_DATA="${TEST_DIR}/project"
TP_PKI_BUNDLE_DIR="${TP_DATA}/pki"
WEB_PATH="${TP_DATA}/site"
KERNEL_RUNTIME_PATH="${TP_DATA}/runtime"
store="${TEST_DIR}/certificate store"
mkdir -p "${store}/live/node.example.com" "${store}/archive/node.example.com"
ln -s "${store}" "${TEST_DIR}/letsencrypt"
NODE_CERTIFICATE_PATH="${TEST_DIR}/letsencrypt/live/node.example.com/fullchain.pem"
NODE_PRIVATE_KEY_PATH="${TEST_DIR}/letsencrypt/live/node.example.com/privkey.pem"
openssl req -x509 -newkey rsa:2048 -nodes -days 2 -subj /CN=node.example.com \
  -addext subjectAltName=DNS:node.example.com \
  -keyout "${store}/archive/node.example.com/privkey1.pem" \
  -out "${store}/archive/node.example.com/fullchain1.pem" >/dev/null 2>&1
ln -s ../../archive/node.example.com/fullchain1.pem "${NODE_CERTIFICATE_PATH}"
ln -s ../../archive/node.example.com/privkey1.pem "${NODE_PRIVATE_KEY_PATH}"
prepare_node_certificate
printf '%s\n' "${NODE_CERTIFICATE_MOUNTS[@]}" >"${TEST_DIR}/mounts"
for directory in \
  "${TEST_DIR}/letsencrypt/live/node.example.com" \
  "${TEST_DIR}/letsencrypt/archive/node.example.com" \
  "${store}/archive/node.example.com"; do
  grep -Fxq "type=bind,src=${directory},dst=${directory},readonly" "${TEST_DIR}/mounts" || fail 'Certbot renewal directory is not mounted read-only'
done
if grep -q 'fullchain1.pem\|privkey1.pem' "${TEST_DIR}/mounts"; then fail 'certificate inode mounted instead of directory'; fi
protect_external_certificates "${TP_DATA}"
assert_error 'overlaps a project removal path' protect_external_certificates "${store}"
assert_error 'overlaps a project removal path' protect_external_certificates "${NODE_CERTIFICATE_PATH}"
assert_error 'missing, empty or unreadable' bash -c 'source "$1/install.sh"; TP_NODE_DOMAIN=node.example.com; NODE_CERTIFICATE_MODE=external; NODE_CERTIFICATE_PATH="$2/missing.pem"; NODE_PRIVATE_KEY_PATH="$3"; prepare_node_certificate' test "${SCRIPT_DIR}" "${store}" "${NODE_PRIVATE_KEY_PATH}"
assert_error 'cover hostname' bash -c 'source "$1/install.sh"; TP_NODE_DOMAIN=wrong.example.com; NODE_CERTIFICATE_MODE=external; NODE_CERTIFICATE_PATH="$2"; NODE_PRIVATE_KEY_PATH="$3"; prepare_node_certificate' test "${SCRIPT_DIR}" "${NODE_CERTIFICATE_PATH}" "${NODE_PRIVATE_KEY_PATH}"
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "${store}/wrong.key" >/dev/null 2>&1
assert_error 'matching PEM pair' bash -c 'source "$1/install.sh"; TP_NODE_DOMAIN=node.example.com; NODE_CERTIFICATE_MODE=external; NODE_CERTIFICATE_PATH="$2"; NODE_PRIVATE_KEY_PATH="$3"; prepare_node_certificate' test "${SCRIPT_DIR}" "${NODE_CERTIFICATE_PATH}" "${store}/wrong.key"
openssl req -new -key "${NODE_PRIVATE_KEY_PATH}" -subj /CN=node.example.com -out "${store}/request.csr"
printf 'subjectAltName=DNS:node.example.com\n' >"${store}/extensions"
openssl x509 -req -in "${store}/request.csr" -signkey "${NODE_PRIVATE_KEY_PATH}" -days 0 \
  -extfile "${store}/extensions" -out "${store}/expired.pem" >/dev/null 2>&1
assert_error 'must be valid' bash -c 'source "$1/install.sh"; TP_NODE_DOMAIN=node.example.com; NODE_CERTIFICATE_MODE=external; NODE_CERTIFICATE_PATH="$2"; NODE_PRIVATE_KEY_PATH="$3"; prepare_node_certificate' test "${SCRIPT_DIR}" "${store}/expired.pem" "${NODE_PRIVATE_KEY_PATH}"

# Both mode changes and changed file locations require new container mounts.
container_exists() { [[ "$1" == "${CORE_CONTAINER}" ]]; }
container_env_value() {
  case "$2" in
  TP_NODE_CERTIFICATE_MODE) printf '%s' "${MOCK_CERTIFICATE_MODE}" ;;
  crt_path) printf '%s' "${NODE_CERTIFICATE_PATH}" ;;
  key_path) printf '%s' "${NODE_PRIVATE_KEY_PATH}" ;;
  esac
}
MOCK_CERTIFICATE_MODE=caddy
TP_FORCE=0
assert_error 'requires install --force' check_node_certificate_migration
TP_FORCE=1
check_node_certificate_migration

OLD_NODE_CERTIFICATE_PATH=/old/fullchain.pem
OLD_NODE_PRIVATE_KEY_PATH=/old/privkey.pem
for kernel in naiveproxy hysteria2 xray; do
  mkdir -p "${TP_DATA}/trojan-panel-core/bin/${kernel}/config"
done
printf '%s\n' '{"apps":{"tls":{"certificates":{"load_files":[{"certificate":"/old/fullchain.pem","key":"/old/privkey.pem","tags":["preserve"]}]}},"http":{"users":["runtime-user"]}}}' >"${TP_DATA}/trojan-panel-core/bin/naiveproxy/config/config-44444.json"
printf '%s\n' '{"tls":{"cert":"/old/fullchain.pem","key":"/old/privkey.pem"},"auth":{"type":"http","http":{"url":"http://127.0.0.1:44445"}}}' >"${TP_DATA}/trojan-panel-core/bin/hysteria2/config/config-44445.json"
printf '%s\n' '{"inbounds":[{"streamSettings":{"tlsSettings":{"certificates":[{"certificateFile":"/old/fullchain.pem","keyFile":"/old/privkey.pem"},{"certificateFile":"/custom/fullchain.pem","keyFile":"/custom/key.pem"}]}}}]}' >"${TP_DATA}/trojan-panel-core/bin/xray/config/config-44443-vless.json"
migrate_node_kernel_certificates
[[ "$(yq -r '.apps.tls.certificates.load_files[0].certificate' "${TP_DATA}/trojan-panel-core/bin/naiveproxy/config/config-44444.json")" == "${NODE_CERTIFICATE_PATH}" ]]
[[ "$(yq -r '.apps.http.users[0]' "${TP_DATA}/trojan-panel-core/bin/naiveproxy/config/config-44444.json")" == runtime-user ]]
[[ "$(yq -r '.tls.key' "${TP_DATA}/trojan-panel-core/bin/hysteria2/config/config-44445.json")" == "${NODE_PRIVATE_KEY_PATH}" ]]
[[ "$(yq -r '.inbounds[0].streamSettings.tlsSettings.certificates[0].keyFile' "${TP_DATA}/trojan-panel-core/bin/xray/config/config-44443-vless.json")" == "${NODE_PRIVATE_KEY_PATH}" ]]
[[ "$(yq -r '.inbounds[0].streamSettings.tlsSettings.certificates[1].certificateFile' "${TP_DATA}/trojan-panel-core/bin/xray/config/config-44443-vless.json")" == /custom/fullchain.pem ]]
MOCK_CERTIFICATE_MODE=external
TP_FORCE=0
check_node_certificate_migration

cp "${SCRIPT_DIR}/examples/node-agent.yaml" "${TEST_DIR}/node.yaml"
TP_TEST_CERT="${NODE_CERTIFICATE_PATH}" TP_TEST_KEY="${NODE_PRIVATE_KEY_PATH}" \
  yq -i '.trojanpanelnext.node_certificate_mode = "external" | .trojanpanelnext.node_certificate_path = strenv(TP_TEST_CERT) | .trojanpanelnext.node_private_key_path = strenv(TP_TEST_KEY)' "${TEST_DIR}/node.yaml"
"${SCRIPT_DIR}/validate.sh" --config "${TEST_DIR}/node.yaml" >/dev/null
yq -i '.trojanpanelnext.node_certificate_path = "relative/fullchain.pem"' "${TEST_DIR}/node.yaml"
assert_error 'must be absolute' "${SCRIPT_DIR}/validate.sh" --config "${TEST_DIR}/node.yaml"

# External certificate files and the independent signer survive full removal.
# shellcheck source=deploy/installer/uninstall.sh
source "${SCRIPT_DIR}/uninstall.sh"
TP_PURPOSE=node
NODE_CERTIFICATE_MODE=external
TP_PURGE_DATA=1
TP_CONFIG_FILE="${TEST_DIR}/node.yaml"
GRPC_CLIENT_CERT_PATH="${TP_DATA}/client.crt"
GRPC_CLIENT_KEY_PATH="${TP_DATA}/client.key"
GRPC_CLIENT_CA_PATH="${TP_DATA}/client-ca.crt"
mkdir -p "${TP_DATA}/trojan-panel-core" "${TP_PKI_BUNDLE_DIR}" "${WEB_PATH}" "${KERNEL_RUNTIME_PATH}"
printf 'business-data\n' >"${TP_DATA}/trojan-panel-core/data"
docker() {
  case "$1 $2" in
  'inspect --format' | 'container inspect' | 'image inspect') return 1 ;;
  'image ls') return 0 ;;
  *) fail "Unexpected Docker mutation: $*" ;;
  esac
}
cleanup_host_maintenance() { :; }
remove_project >"${TEST_DIR}/purge-output"
test -s "${NODE_CERTIFICATE_PATH}"
test -s "${NODE_PRIVATE_KEY_PATH}"
test ! -e "${TP_DATA}/trojan-panel-core"
test ! -e "${TP_CONFIG_FILE}"
[[ " ${TP_REMOVE_CONTAINERS[*]} " != *" ${NODE_CADDY_CONTAINER} "* ]] || fail 'external mode removes independent Caddy'
[[ " ${TP_REMOVE_IMAGES[*]} " != *" ${CADDY_IMAGE} "* ]] || fail 'external mode removes Caddy images'
printf 'PASS external PEM validation, Certbot symlink directory mounts, migration and purge protection\n'
