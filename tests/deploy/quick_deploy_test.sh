#!/usr/bin/env bash
# Literal shell metacharacters verify that YAML credentials are not expanded.
# shellcheck disable=SC2016
set -euo pipefail

REPOSITORY="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf -- "${TEST_DIR}"' EXIT
export MOCK_LIBRARY="${TEST_DIR}/library"
export MOCK_INSTALL_LOG="${TEST_DIR}/install.log"
export TP_DATA="${TEST_DIR}/data"
export TP_PKI_BUNDLE_DIR="${TP_DATA}/pki"
mkdir -p "${MOCK_LIBRARY}"
cp "${REPOSITORY}/scripts/deploy/"*.sh "${MOCK_LIBRARY}/"
cp -r "${REPOSITORY}/scripts/deploy/templates" "${MOCK_LIBRARY}/"
cat >"${MOCK_LIBRARY}/install.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '<%s>\n' "$@" >"${MOCK_INSTALL_LOG}"
exit "${MOCK_INSTALL_STATUS:-0}"
EOF

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
assert_fails() {
  if ("$@") >"${TEST_DIR}/out" 2>"${TEST_DIR}/err"; then fail 'unsafe deployment unexpectedly succeeded'; fi
}
# Only HTTP, root identity and interactive input are replaced. Real yq and PEM
# parsing exercise the YAML and certificate hand-off; deployment is stubbed.
# shellcheck disable=SC2317,SC2329
id() { if [[ "${1:-}" == -u ]]; then printf '0\n'; else command id "$@"; fi; }
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
  cp "${MOCK_LIBRARY}/templates/${url##*/}" "${destination}"
}
export -f id curl

TP_SCRIPT_DIR="${MOCK_LIBRARY}"
# shellcheck source=scripts/deploy/common.sh
source "${MOCK_LIBRARY}/common.sh"
# shellcheck source=scripts/deploy/quick.sh
source "${MOCK_LIBRARY}/quick.sh"
quick_prompt() {
  local variable="$1" value=""
  [[ -z "${!variable:-}" ]] || return 0
  case "${variable}" in
  hostname) value=node.example.com ;;
  email) value=admin@example.com ;;
  web_host) value=panel.example.com ;;
  node_id) value=7 ;;
  client_ca) value="${TEST_DIR}/client-ca.crt" ;;
  certificate) value=/etc/letsencrypt/live/node.example.com/fullchain.pem ;;
  private_key) value=/etc/letsencrypt/live/node.example.com/privkey.pem ;;
  database_password) value='db secret $dollar "quote"' ;;
  redis_password) value='redis secret with spaces' ;;
  *) fail 'unexpected prompt' ;;
  esac
  printf -v "${variable}" '%s' "${value}"
}

(quick_deploy web --hostname panel.example.com --email admin@example.com --output "${TEST_DIR}/web config.yaml") >"${TEST_DIR}/out"
test "$(stat -c %a "${TEST_DIR}/web config.yaml")" = 600
test "$(yq -r '.trojanpanelnext.hostname' "${TEST_DIR}/web config.yaml")" = panel.example.com
test "$(yq -r '.trojanpanelnext.release' "${TEST_DIR}/web config.yaml")" = 1.0.2-rc.10
grep -Fxq "<${TEST_DIR}/web config.yaml>" "${MOCK_INSTALL_LOG}"
assert_fails quick_deploy web --hostname panel.example.com --email admin@example.com --output "${TEST_DIR}/web config.yaml"
assert_fails quick_deploy web --config "${TEST_DIR}/web config.yaml" --hostname ignored.example.com
assert_fails quick_deploy web --node-id 7

openssl req -x509 -newkey rsa:2048 -sha256 -days 1 -nodes -subj /CN=Example-Web-CA \
  -addext basicConstraints=critical,CA:TRUE \
  -keyout "${TEST_DIR}/client-ca.key" -out "${TEST_DIR}/client-ca.crt" >/dev/null 2>&1
openssl req -x509 -newkey rsa:2048 -sha256 -days 1 -nodes -subj /CN=Next-Web-CA \
  -addext basicConstraints=critical,CA:TRUE \
  -keyout "${TEST_DIR}/next-ca.key" -out "${TEST_DIR}/next-ca.crt" >/dev/null 2>&1
cat "${TEST_DIR}/client-ca.crt" "${TEST_DIR}/next-ca.crt" >"${TEST_DIR}/rotation-bundle.crt"
quick_validate_client_ca "${TEST_DIR}/rotation-bundle.crt" || fail 'public CA rotation bundle rejected'
(printf ' \t\n'; cat "${TEST_DIR}/rotation-bundle.crt"; printf '\n') >"${TEST_DIR}/whitespace-bundle.crt"
quick_validate_client_ca "${TEST_DIR}/whitespace-bundle.crt" || fail 'public CA whitespace rejected'
(quick_deploy node --certificate-mode external --output "${TEST_DIR}/node.yaml") >"${TEST_DIR}/out"
test "$(stat -c %a "${TEST_DIR}/node.yaml")" = 600
test "$(yq -r '.trojanpanelnext.node_server_id' "${TEST_DIR}/node.yaml")" = 7
test "$(yq -r '.trojanpanelnext.node_certificate_mode' "${TEST_DIR}/node.yaml")" = external
test "$(yq -r '.trojanpanelnext.mariadb_password' "${TEST_DIR}/node.yaml")" = 'db secret $dollar "quote"'
test "$(yq -r '.trojanpanelnext.redis_password' "${TEST_DIR}/node.yaml")" = 'redis secret with spaces'
cmp "${TEST_DIR}/client-ca.crt" "${TP_PKI_BUNDLE_DIR}/client-ca.crt"
if grep -Fq 'db secret' "${TEST_DIR}/out"; then fail 'credential appeared in command output'; fi

assert_fails quick_deploy web --config "${TEST_DIR}/node.yaml"
(quick_deploy node --config "${TEST_DIR}/node.yaml" --force) >"${TEST_DIR}/out"
grep -Fxq '<--force>' "${MOCK_INSTALL_LOG}"
if (MOCK_INSTALL_STATUS=17 quick_deploy web --config "${TEST_DIR}/web config.yaml") >"${TEST_DIR}/out"; then
  fail 'installation exit status lost'
else test "$?" = 17; fi
assert_fails quick_deploy node --certificate-mode invalid --output "${TEST_DIR}/invalid.yaml"
printf 'not a certificate\n' >"${TEST_DIR}/invalid-ca.crt"
assert_fails quick_deploy node --client-ca "${TEST_DIR}/invalid-ca.crt" --output "${TEST_DIR}/invalid-ca.yaml"
openssl req -x509 -newkey rsa:2048 -sha256 -days 1 -nodes -subj /CN=Example-Leaf \
  -addext basicConstraints=critical,CA:FALSE \
  -keyout "${TEST_DIR}/leaf.key" -out "${TEST_DIR}/leaf.crt" >/dev/null 2>&1
cat "${TEST_DIR}/client-ca.crt" "${TEST_DIR}/client-ca.key" >"${TEST_DIR}/private-bundle.crt"
cat "${TEST_DIR}/client-ca.crt" "${TEST_DIR}/leaf.crt" >"${TEST_DIR}/leaf-bundle.crt"
openssl req -new -key "${TEST_DIR}/client-ca.key" -subj /CN=Expired-Web-CA -out "${TEST_DIR}/expired-ca.csr"
printf 'basicConstraints=critical,CA:TRUE\n' >"${TEST_DIR}/ca-extensions"
openssl x509 -req -in "${TEST_DIR}/expired-ca.csr" -signkey "${TEST_DIR}/client-ca.key" -days 0 \
  -extfile "${TEST_DIR}/ca-extensions" -out "${TEST_DIR}/expired-ca.crt" >/dev/null 2>&1
(cat "${TEST_DIR}/client-ca.crt"; printf 'unexpected material\n') >"${TEST_DIR}/junk-bundle.crt"
sed '/-----END CERTIFICATE-----/d' "${TEST_DIR}/client-ca.crt" >"${TEST_DIR}/incomplete-ca.crt"
for material in leaf private-bundle leaf-bundle expired-ca junk-bundle incomplete-ca; do
  assert_fails quick_deploy node --client-ca "${TEST_DIR}/${material}.crt" --output "${TEST_DIR}/${material}.yaml"
  grep -Fq 'Invalid public client CA certificate' "${TEST_DIR}/err"
  test ! -e "${TEST_DIR}/${material}.yaml"
  cmp "${TEST_DIR}/client-ca.crt" "${TP_PKI_BUNDLE_DIR}/client-ca.crt"
done
[[ -z "$(find "${TEST_DIR}" -name '.tpnext.*' -print -quit)" ]] || fail 'temporary configuration remained'
printf 'PASS one-command deployment, protected YAML, literal credentials, public CA hand-off and existing config\n'
