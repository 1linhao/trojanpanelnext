#!/usr/bin/env bash
set -Eeuo pipefail

INSTALLER="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/install.sh"
FAKE_YQ_READER="$(dirname "${BASH_SOURCE[0]}")/fixtures/fake_yq_reader.sh"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

work="$(mktemp -d)"
trap 'rm -rf -- "${work}"' EXIT
config="${work}/combined.yaml"
data="${work}/data"
containers="${work}/containers"
trace="${work}/docker.trace"
curl_trace="${work}/curl.trace"
os_release="${work}/debian-12"
mkdir -p "${containers}"
printf 'ID=debian\nVERSION_ID="12"\n' >"${os_release}"
cp "$(dirname "${INSTALLER}")/examples/combined.yaml" "${config}"
sed -i \
  -e "s#/tpdata/trojan-panel/config/node-identities/combined-node.json#${data}/trojan-panel/config/node-identities/combined-node.json#" \
  -e "s#/tpdata/trojanpanelnext-pki#${data}/trojanpanelnext-pki#" \
  "${config}"

id() {
  [[ "${1:-}" == -u ]] && { printf '0\n'; return; }
  /usr/bin/id "$@"
}

uname() {
  [[ "${1:-}" == -m ]] && { printf 'x86_64\n'; return; }
  /usr/bin/uname "$@"
}

sleep() { :; }
age() { :; }
getent() {
  [[ "${1:-}" == ahosts ]] || return 2
  printf '%s STREAM %s\n' "${TP_TEST_DNS_IP:-203.0.113.10}" "${2:-}"
}
ss() { printf '%s' "${TP_TEST_SS_OUTPUT:-}"; }

od() {
  if [[ " $* " == *' -tu1 '* ]]; then
    printf ' 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20\n'
  else
    printf ' 111111111111111111111111111111111111111111111111\n'
  fi
}

yq() {
  if [[ "${1:-}" == -i ]]; then
    local file="${3}"
    sed -i \
      -e "s/^  mariadb_password:.*/  mariadb_password: \"${MARIADB_PASSWORD}\"/" \
      -e "s/^  redis_password:.*/  redis_password: \"${REDIS_PASSWORD}\"/" \
      -e "s/^  sysadmin_password:.*/  sysadmin_password: \"${SYSADMIN_PASSWORD:-}\"/" \
      "${file}"
    return
  fi
  "${TP_TEST_FAKE_YQ_READER}" "$@"
}

openssl() {
  if [[ "${1:-}" == x509 && " $* " == *' -checkend '* ]]; then
    return
  fi
  if [[ "${1:-}" == x509 && " $* " == *' -checkhost '* ]]; then
    local domain="${*: -1}"
    printf 'Hostname %s does match certificate\n' "${domain}"
    return
  fi
  /usr/bin/openssl "$@"
}

docker() {
  printf 'docker' >>"${TP_TEST_TRACE}"
  local argument
  for argument in "$@"; do
    case "${argument}" in
    *root-secret* | *cache-secret* | *node-db-secret* | *node-cache-secret* | *node-auth-secret*)
      printf ' <redacted>' >>"${TP_TEST_TRACE}"
      ;;
    *) printf ' %s' "${argument}" >>"${TP_TEST_TRACE}" ;;
    esac
  done
  printf '\n' >>"${TP_TEST_TRACE}"

  case "${1:-}" in
  ps)
    local name="" running=0
    shift
    while (($#)); do
      [[ "$1" == *status=running* ]] && running=1
      if [[ "$1" == name=^* ]]; then
        name="${1#name=^}"
        name="${name%$}"
      fi
      shift
    done
    [[ -n "${name}" && -f "${TP_TEST_CONTAINERS}/${name}" ]] && printf '%s\n' "${name}"
    ;;
  image)
    [[ "${2:-}" == inspect ]] && return 1
    ;;
  pull | load) ;;
  inspect)
    local name="${2:-}"
    [[ -f "${TP_TEST_CONTAINERS}/${name}.env" ]] && cat "${TP_TEST_CONTAINERS}/${name}.env"
    ;;
  run)
    if [[ " $* " == *' node-identity revoke '* ]]; then
      printf 'revoke\n' >>"${TP_TEST_IDENTITY_TRACE}"
      return
    fi
    local name=""
    local -a environment=()
    shift
    while (($#)); do
      if [[ "$1" == --name ]]; then
        name="$2"
        shift 2
        continue
      fi
      if [[ "$1" == -e ]]; then
        environment+=("$2")
        shift 2
        continue
      fi
      shift
    done
    [[ -n "${name}" ]] || return 2
    : >"${TP_TEST_CONTAINERS}/${name}"
    printf '%s\n' "${environment[@]}" >"${TP_TEST_CONTAINERS}/${name}.env"
    if [[ "${name}" == trojan-panel-web-caddy ]]; then
      local domain cert_dir
      for domain in panel.example.com node.example.com; do
        cert_dir="${TP_TEST_DATA}/custom/web-caddy/data/caddy/certificates/acme.test/${domain}"
        mkdir -p "${cert_dir}"
        printf 'fake certificate for %s\n' "${domain}" >"${cert_dir}/${domain}.crt"
        printf 'fake key for %s\n' "${domain}" >"${cert_dir}/${domain}.key"
      done
    fi
    printf 'fake-container-id\n'
    ;;
  exec)
    if [[ " $* " == *' node-identity verify '* && "${TP_TEST_FAIL_MTLS:-0}" == 1 ]]; then
      return 1
    fi
    if [[ " $* " == *' node-identity register '* ]]; then
      local credential="" previous=""
      for argument in "$@"; do
        if [[ "${previous}" == --credential-file ]]; then credential="${argument}"; fi
        previous="${argument}"
      done
      [[ -n "${credential}" ]] || return 2
      if [[ ! -e "${credential}" ]]; then
        mkdir -p "$(dirname "${credential}")"
        cat >"${credential}" <<'EOF'
{"schema_version":2,"node_identity_id":"11111111-2222-4333-8444-555555555555","node_server_id":42,"node_name":"combined-node","node_domain":"node.example.com","public_ip":"203.0.113.10","generation":1,"mariadb":{"database":"trojan_panel_db","username":"tpn_combined","password":"node-db-secret"},"redis":{"username":"tpn-cache-combined","password":"node-cache-secret","key_patterns":["trojan-panel-core:*"]},"redis_auth":{"username":"tpn-auth-combined","password":"node-auth-secret","key_patterns":["trojan-panel:jwt-key","trojan-panel:token:*"]}}
EOF
        chmod 0600 "${credential}"
      fi
      printf 'Node identity registered\n'
      return
    fi
    if [[ " $* " == *' node-identity revoke '* ]]; then
      printf 'revoke\n' >>"${TP_TEST_IDENTITY_TRACE}"
      return
    fi
    if [[ "$*" == *TP_VERIFY_SYSADMIN_CREDENTIAL=1* || "$*" == *TP_VERIFY_NODE_DATA_SERVICES=* ]]; then
      return
    fi
    if [[ " $* " == *' redis-cli '* ]]; then
      cat >/dev/null || true
      printf 'OK\nPONG\n'
      return
    fi
    cat >/dev/null || true
    printf '1\n'
    ;;
  rm)
    shift
    while (($#)); do
      case "$1" in -f | -v | -fv | -vf) shift; continue ;; esac
      rm -f "${TP_TEST_CONTAINERS}/$1" "${TP_TEST_CONTAINERS}/$1.env"
      shift
    done
    ;;
  start | restart) ;;
  cp) ;;
  logs) printf 'sanitized fake log\n' ;;
  *) return 2 ;;
  esac
}

curl() {
  local url="${*: -1}"
  printf '%s\n' "${url}" >>"${TP_TEST_CURL_TRACE}"
  printf '<!doctype html><title>TrojanPanel Next</title>\n'
}

export -f id uname sleep age getent ss od yq openssl docker curl
export TP_TEST_FAKE_YQ_READER="${FAKE_YQ_READER}"
export TP_TEST_CONTAINERS="${containers}"
export TP_TEST_TRACE="${trace}"
export TP_TEST_CURL_TRACE="${curl_trace}"
export TP_TEST_IDENTITY_TRACE="${work}/identity.trace"
export TP_TEST_DATA="${data}"
export TP_TEST_DNS_IP=203.0.113.10
export TP_TEST_SS_OUTPUT=''

run_installer() {
  TP_DATA="${data}" \
    TP_OS_RELEASE_FILE="${os_release}" \
    TP_HEALTH_ATTEMPTS=2 \
    TP_HEALTH_DELAY_SECONDS=0 \
    "${INSTALLER}" "$@" --config "${config}"
}

sed -i 's/node_hostname: node.example.com/node_hostname: panel.example.com/' "${config}"
if run_installer install --mode combined >"${work}/same-domain.out" 2>&1; then
  fail 'combined installation accepted the same Web and Node domain'
fi
grep -Fq 'requires two different domains' "${work}/same-domain.out" || fail 'same-domain rejection omitted its diagnostic'
[[ ! -e "${trace}" ]] || ! grep -q '^docker run ' "${trace}" || fail 'same-domain rejection happened after container mutation'
sed -i 's/node_hostname: panel.example.com/node_hostname: node.example.com/' "${config}"
: >"${trace}"

TP_TEST_DNS_IP=198.51.100.20
if run_installer install --mode combined >"${work}/bad-dns.out" 2>&1; then
  fail 'combined installation accepted DNS that points elsewhere'
fi
grep -Fq 'combined DNS prerequisite failed' "${work}/bad-dns.out" || fail 'DNS rejection omitted its diagnostic'
[[ ! -e "${trace}" ]] || ! grep -q '^docker run ' "${trace}" || fail 'DNS rejection happened after container mutation'
: >"${trace}"
TP_TEST_DNS_IP=203.0.113.10
TP_TEST_SS_OUTPUT='LISTEN 0 4096 0.0.0.0:443'
if run_installer install --mode combined >"${work}/busy-port.out" 2>&1; then
  fail 'combined installation accepted an existing 443 listener'
fi
grep -Fq 'another listener is active' "${work}/busy-port.out" || fail 'port conflict rejection omitted its diagnostic'
! grep -q '^docker run ' "${trace}" || fail 'port conflict rejection happened after container mutation'
: >"${trace}"
TP_TEST_SS_OUTPUT=''

: >"${containers}/trojan-panel-node-caddy"
if run_installer install --mode combined >"${work}/standalone-node-entry.out" 2>&1; then
  fail 'combined installation removed an existing standalone Node Entry'
fi
grep -Fq 'standalone Node Entry already exists' "${work}/standalone-node-entry.out" || fail 'standalone Node Entry rejection omitted its diagnostic'
test -f "${containers}/trojan-panel-node-caddy" || fail 'standalone Node Entry was deleted'
! grep -q '^docker run ' "${trace}" || fail 'standalone Node Entry rejection happened after container mutation'
rm -f "${containers}/trojan-panel-node-caddy"
: >"${trace}"

: >"${containers}/trojan-panel-web-caddy"
printf 'TP_ENTRY_DEPLOYMENT_ID=another-deployment\n' >"${containers}/trojan-panel-web-caddy.env"
if run_installer install --mode combined >"${work}/foreign-entry.out" 2>&1; then
  fail 'combined installation adopted an Entry owned by another deployment'
fi
grep -Fq 'Shared Entry ownership conflict' "${work}/foreign-entry.out" || fail 'foreign Entry rejection omitted its diagnostic'
! grep -q '^docker run ' "${trace}" || fail 'foreign Entry rejection happened after container mutation'
rm -f "${containers}/trojan-panel-web-caddy" "${containers}/trojan-panel-web-caddy.env"
: >"${trace}"

run_installer install --mode combined >"${work}/install.out" 2>&1 || {
  sed -n '1,260p' "${work}/install.out" >&2
  fail 'combined installation failed'
}
grep -Fq 'Combined deployment is healthy' "${work}/install.out" || fail 'combined health marker is missing'
grep -Fq 'panel.example.com' "${data}/custom/web-caddy/Caddyfile" || fail 'shared Entry omitted the Web domain'
grep -Fq 'node.example.com' "${data}/custom/web-caddy/Caddyfile" || fail 'shared Entry omitted the Node domain'
test -s "${data}/custom/web-caddy/data/caddy/certificates/acme.test/panel.example.com/panel.example.com.crt" || fail 'Web certificate is missing'
test -s "${data}/custom/web-caddy/data/caddy/certificates/acme.test/node.example.com/node.example.com.crt" || fail 'Node certificate is missing'
test -f "${containers}/trojan-panel-web-caddy" || fail 'shared Entry container was not created'
test ! -e "${containers}/trojan-panel-node-caddy" || fail 'a second Entry container competes for 80/443'
grep -Fxq 'TP_ENTRY_DEPLOYMENT_ID=trojanpanelnext-combined-entry' "${containers}/trojan-panel-web-caddy.env" || fail 'shared Entry ownership identity is missing'
test "$(grep -c '^docker run .*--name trojan-panel-web-caddy ' "${trace}")" = 1 || fail 'more than one shared Entry was started'
grep -Fq "${data}/custom/web-caddy/data:${data}/custom/web-caddy/data:ro" "${trace}" || fail 'Core does not consume shared certificates read-only'
grep -Fxq 'mariadb_user=tpn_combined' "${containers}/trojan-panel-core.env" || fail 'Core omitted its dedicated MariaDB identity'
grep -Fxq 'REDIS_USERNAME=tpn-cache-combined' "${containers}/trojan-panel-core.env" || fail 'Core omitted its dedicated Redis identity'
! grep -Fq 'root-secret' "${containers}/trojan-panel-core.env" || fail 'Core received the Web MariaDB root credential'
test "$(stat -c '%a' "${data}/trojan-panel/config/node-identities/combined-node.json")" = 600 || fail 'Node identity credential file is not mode 0600'
grep -Fq 'https://panel.example.com/' "${curl_trace}" || fail 'Web HTTPS was not probed'
grep -Fq 'https://node.example.com/' "${curl_trace}" || fail 'Node HTTPS was not probed'
grep -q '^docker exec trojan-panel /tpdata/trojan-panel/trojan-panel node-identity verify --id ' "${trace}" || fail 'combined install did not verify Web-to-Node mTLS/gRPC'

# Same-version replay converges without a second Entry or identity.
run_installer install --mode combined >"${work}/replay.out" 2>&1 || {
  sed -n '1,260p' "${work}/replay.out" >&2
  fail 'combined replay failed'
}
test "$(grep -c '^docker run .*--name trojan-panel-web-caddy ' "${trace}")" = 1 || fail 'combined replay started another Entry'
test "$(grep -c '^docker restart trojan-panel-core$' "${trace}" || true)" = 0 || fail 'unchanged replay restarted the Node certificate consumer'

export TP_TEST_FAIL_MTLS=1
if run_installer install --mode combined >"${work}/mtls-failure.out" 2>&1; then
  fail 'combined install succeeded after Web-to-Node mTLS/gRPC verification failed'
fi
grep -Fq 'Health check failed: Web-to-Node mTLS/gRPC' "${work}/mtls-failure.out" || fail 'mTLS/gRPC failure omitted its health diagnostic'
TP_TEST_FAIL_MTLS=0

# Caddy owns renewal; refresh only notifies the Node certificate consumer when
# the shared certificate generation changes.
run_installer refresh-cert --mode combined | grep -Fxq unchanged || fail 'unchanged certificate refresh was not a no-op'
printf 'renewed certificate\n' >>"${data}/custom/web-caddy/data/caddy/certificates/acme.test/node.example.com/node.example.com.crt"
run_installer refresh-cert --mode combined >"${work}/refresh.out"
grep -Fxq changed "${work}/refresh.out" || fail 'changed certificate generation was not reported'
test "$(grep -c '^docker restart trojan-panel-core$' "${trace}")" = 1 || fail 'changed Node certificate did not notify its consumer exactly once'

# Web role removal preserves Node, shared data services, and the shared Entry.
mv "${containers}/trojan-panel-web-caddy" "${containers}/trojan-panel-web-caddy.paused"
if run_installer remove --mode web >"${work}/missing-entry-remove.out" 2>&1; then
  fail 'Web role removal succeeded without the shared Entry it must preserve'
fi
test -f "${containers}/trojan-panel" || fail 'missing shared Entry check happened after removing Web'
mv "${containers}/trojan-panel-web-caddy.paused" "${containers}/trojan-panel-web-caddy"
run_installer remove --mode web >"${work}/remove-web.out"
test -f "${containers}/trojan-panel-core" || fail 'Web removal deleted the Node role'
test -f "${containers}/trojan-panel-mariadb" || fail 'Web removal deleted shared MariaDB'
test -f "${containers}/trojan-panel-redis" || fail 'Web removal deleted shared Redis'
test -f "${containers}/trojan-panel-web-caddy" || fail 'Web removal deleted the shared Entry'
! grep -Fq 'panel.example.com' "${data}/custom/web-caddy/Caddyfile" || fail 'removed Web role remains in shared Entry config'
grep -Fq 'node.example.com' "${data}/custom/web-caddy/Caddyfile" || fail 'Web removal deleted Node Entry config'

# Node removal still revokes its identity after Web removal through the
# control-plane CLI in a one-shot container.
run_installer remove --mode node >"${work}/remove-node.out"
test ! -e "${containers}/trojan-panel-core" || fail 'Node removal retained the Core container'
test -f "${containers}/trojan-panel-mariadb" || fail 'Node removal deleted shared MariaDB'
test -f "${containers}/trojan-panel-redis" || fail 'Node removal deleted shared Redis'
grep -Fxq revoke "${work}/identity.trace" || fail 'Node removal did not revoke its control-plane identity first'
grep -q '^docker run --rm --network=host .* node-identity revoke --id ' "${trace}" || fail 'Node removal did not use the one-shot identity CLI after Web removal'
grep -Fq 'panel.example.com' "${data}/custom/web-caddy/Caddyfile" || fail 'Node removal deleted Web Entry config'
! grep -Fq 'node.example.com' "${data}/custom/web-caddy/Caddyfile" || fail 'removed Node role remains in shared Entry config'

printf 'PASS combined shared Entry, dual-certificate, identity, replay, renewal, and role ownership smoke\n'
