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
config="${work}/web.yaml"
data="${work}/data"
container_state="${work}/containers"
trace="${work}/host.trace"
curl_trace="${work}/curl.trace"
random_state="${work}/random-state"
os_release="${work}/debian-12"
mkdir -p "${container_state}"
printf 'ID=debian\nVERSION_ID="12"\n' >"${os_release}"
cp "$(dirname "${INSTALLER}")/examples/web.yaml" "${config}"
sed -i \
  -e "s#/tpdata/trojan-panel/pki/client.crt#${data}/trojan-panel/pki/client.crt#" \
  -e "s#/tpdata/trojan-panel/pki/client.key#${data}/trojan-panel/pki/client.key#" \
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

od() {
  local count=0
  [[ -f "${TP_TEST_RANDOM_STATE}" ]] && count="$(cat "${TP_TEST_RANDOM_STATE}")"
  count=$((count + 1))
  printf '%s\n' "${count}" >"${TP_TEST_RANDOM_STATE}"
  if [[ " $* " == *' -tu1 '* ]]; then
    printf ' 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20\n'
    return
  fi
  case "${count}" in
  1) printf ' 111111111111111111111111111111111111111111111111\n' ;;
  2) printf ' 222222222222222222222222222222222222222222222222\n' ;;
  *) printf ' 33333333333333333333\n' ;;
  esac
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
  /usr/bin/openssl "$@"
}

docker() {
  printf 'docker' >>"${TP_TEST_TRACE}"
  local argument
  for argument in "$@"; do
    if [[ "${argument}" == 111111* || "${argument}" == 222222* || "${argument}" == 333333* ]]; then
      printf ' <redacted>' >>"${TP_TEST_TRACE}"
    else
      printf ' %s' "${argument}" >>"${TP_TEST_TRACE}"
    fi
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
    [[ -n "${name}" && -f "${TP_TEST_CONTAINER_STATE}/${name}" ]] && printf '%s\n' "${name}"
    ;;
  image)
    [[ "${2:-}" == inspect ]] && return 1
    ;;
  pull | load) ;;
  inspect)
    return 0
    ;;
  run)
    local name=""
    shift
    while (($#)); do
      if [[ "$1" == --name ]]; then
        name="$2"
        shift 2
        continue
      fi
      shift
    done
    [[ -n "${name}" ]] || return 2
    : >"${TP_TEST_CONTAINER_STATE}/${name}"
    if [[ "${name}" == trojan-panel-web-caddy ]]; then
      local cert_dir="${TP_TEST_DATA}/custom/web-caddy/data/caddy/certificates/acme.test/${TP_TEST_DOMAIN}"
      mkdir -p "${cert_dir}"
      printf 'fake certificate\n' >"${cert_dir}/${TP_TEST_DOMAIN}.crt"
      printf 'fake private key\n' >"${cert_dir}/${TP_TEST_DOMAIN}.key"
    fi
    printf 'fake-container-id\n'
    ;;
  exec)
    local stdin_payload
    stdin_payload="$(cat || true)"
    if [[ "$*" == *TP_VERIFY_SYSADMIN_CREDENTIAL=1* ]]; then
      [[ "${TP_TEST_FAIL_PROBE:-}" == sysadmin ]] && return 2
      return 0
    elif [[ " $* " == *' redis-cli '* ]]; then
      [[ "${TP_TEST_FAIL_PROBE:-}" == Redis ]] && return 1
      printf 'OK\nPONG\n'
    else
      [[ "${TP_TEST_FAIL_PROBE:-}" == MariaDB ]] && return 1
      if [[ -n "${TP_TEST_MARIADB_ACCEPTED_PASSWORD:-}" && -n "${stdin_payload}" &&
        "${stdin_payload%%$'\n'*}" != "${TP_TEST_MARIADB_ACCEPTED_PASSWORD}" ]]; then
        return 1
      fi
      printf '1\n'
    fi
    ;;
  start | restart | rm | cp) ;;
  logs) printf 'sanitized fake log\n' ;;
  *) return 2 ;;
  esac
}

curl() {
  local url="${*: -1}"
  printf '%s\n' "${url}" >>"${TP_TEST_CURL_TRACE}"
  if [[ "${url}" == */api/auth/installer-health || "${url}" == */api/auth/login ]]; then
    cat >/dev/null
    [[ "${TP_TEST_FAIL_PROBE:-}" == sysadmin ]] && {
      printf '{"code":50000,"type":"error","message":"authentication failed"}\n'
      return
    }
    printf '{"code":20000,"type":"success","data":{"token":"fake"}}\n'
    return
  fi
  [[ "${TP_TEST_FAIL_PROBE:-}" == HTTPS ]] && return 22
  printf '<!doctype html><title>TrojanPanel Next</title>\n'
}

export -f id uname sleep age od yq openssl docker curl
export TP_TEST_FAKE_YQ_READER="${FAKE_YQ_READER}"
export TP_TEST_CONTAINER_STATE="${container_state}"
export TP_TEST_TRACE="${trace}"
export TP_TEST_CURL_TRACE="${curl_trace}"
export TP_TEST_RANDOM_STATE="${random_state}"
export TP_TEST_DATA="${data}"
export TP_TEST_DOMAIN=panel.example.com

invalid_config="${work}/invalid-web.yaml"
sed 's/^  sysadmin_password:.*/  sysadmin_password: "weakpass"/' "${config}" >"${invalid_config}"
if "${INSTALLER}" validate --mode web --config "${invalid_config}" >"${work}/invalid.out" 2>&1; then
  fail 'validation accepted a weak sysadmin password'
fi
grep -Fq 'sysadmin_password must contain 16 to 20 ASCII letters or digits' "${work}/invalid.out" ||
  fail 'weak sysadmin password validation omitted its diagnostic'

symlink_target="${work}/symlink-target.yaml"
symlink_config="${work}/symlink-web.yaml"
cp "${config}" "${symlink_target}"
ln -s "${symlink_target}" "${symlink_config}"
if "${INSTALLER}" validate --mode web --config "${symlink_config}" >"${work}/symlink.out" 2>&1; then
  fail 'validation accepted a symlink deployment configuration'
fi
grep -Fq 'must not contain symbolic links' "${work}/symlink.out" ||
  fail 'symlink deployment configuration rejection omitted its diagnostic'

real_parent="${work}/real-parent"
linked_parent="${work}/linked-parent"
mkdir -p "${real_parent}"
cp "${config}" "${real_parent}/web.yaml"
ln -s "${real_parent}" "${linked_parent}"
if "${INSTALLER}" validate --mode web --config "${linked_parent}/web.yaml" >"${work}/parent-symlink.out" 2>&1; then
  fail 'validation accepted a deployment configuration below a symlink parent'
fi
grep -Fq 'must not contain symbolic links' "${work}/parent-symlink.out" ||
  fail 'symlink parent rejection omitted its diagnostic'

ambiguous_root="${work}/ambiguous"
mkdir -p "${ambiguous_root}/target/child"
cp "${config}" "${ambiguous_root}/target/web.yaml"
ln -s "${ambiguous_root}/target/child" "${ambiguous_root}/linked"
if "${INSTALLER}" validate --mode web \
  --config "${ambiguous_root}/linked/../web.yaml" >"${work}/ambiguous.out" 2>&1; then
  fail 'validation accepted a configuration path containing symlink/.. ambiguity'
fi
grep -Fq 'must not contain symbolic links or .. components' "${work}/ambiguous.out" ||
  fail 'symlink/.. configuration rejection omitted its diagnostic'

output="${work}/install.out"
if ! TP_DATA="${data}" \
  TP_OS_RELEASE_FILE="${os_release}" \
  TP_HEALTH_ATTEMPTS=2 \
  TP_HEALTH_DELAY_SECONDS=0 \
  "${INSTALLER}" install --mode web --config "${config}" >"${output}" 2>&1; then
  sed -n '1,240p' "${output}" >&2
  fail 'Web installation unexpectedly failed in the success scenario'
fi

grep -Fq 'Web control plane is healthy' "${output}" ||
  fail 'install returned without the strong Web health success marker'
grep -Eq '^  sysadmin_password: "[[:alnum:]]{16,20}"$' "${config}" ||
  fail 'installer did not persist a strong sysadmin password'
test "$(stat -c '%a' "${config}")" = 600 ||
  fail 'generated deployment configuration is not mode 0600'
test "$(stat -c '%a' "${data}/trojan-panel/config/initial-admin-password")" = 600 ||
  fail 'API initial sysadmin password file is not mode 0600'
if grep -Fq '/api/auth/installer-health' "${data}/trojan-panel-ui/nginx/default.conf"; then
  fail 'public UI configuration still knows about an administrator credential oracle'
fi

for secret_key in mariadb_password redis_password sysadmin_password; do
  secret="$(awk -F'"' -v key="${secret_key}" '$1 == "  " key ": " {print $2}' "${config}")"
  [[ -n "${secret}" ]] || fail "${secret_key} was not generated"
  if grep -Fq "${secret}" "${output}"; then
    fail "${secret_key} leaked to installer output"
  fi
done
sysadmin_secret="$(awk -F'"' '$1 == "  sysadmin_password: " {print $2}' "${config}")"
test "$(tr -d '\n' <"${data}/trojan-panel/config/initial-admin-password")" = "${sysadmin_secret}" ||
  fail 'API initial password file does not match the persisted sysadmin credential'

secret_link_dir="${work}/secret-link-dir"
mkdir -p "${secret_link_dir}"
printf 'do-not-overwrite\n' >"${secret_link_dir}/target"
ln -s "${secret_link_dir}/target" "${secret_link_dir}/initial-admin-password"
if INITIAL_SYSADMIN_PASSWORD_FILE="${secret_link_dir}/initial-admin-password" \
  TP_DATA="${data}" \
  TP_OS_RELEASE_FILE="${os_release}" \
  TP_HEALTH_ATTEMPTS=1 \
  TP_HEALTH_DELAY_SECONDS=0 \
  "${INSTALLER}" install --mode web --config "${config}" >"${work}/secret-link.out" 2>&1; then
  fail 'installation accepted a symlink initial administrator password file'
fi
grep -Fq 'must not contain symbolic links' "${work}/secret-link.out" ||
  fail 'symlink initial administrator password file rejection omitted its diagnostic'
test "$(cat "${secret_link_dir}/target")" = 'do-not-overwrite' ||
  fail 'symlink initial administrator password target was overwritten'

secret_real_parent="${work}/secret-real-parent"
secret_link_parent="${work}/secret-link-parent"
mkdir -p "${secret_real_parent}"
ln -s "${secret_real_parent}" "${secret_link_parent}"
if INITIAL_SYSADMIN_PASSWORD_FILE="${secret_link_parent}/initial-admin-password" \
  TP_DATA="${data}" \
  TP_OS_RELEASE_FILE="${os_release}" \
  TP_HEALTH_ATTEMPTS=1 \
  TP_HEALTH_DELAY_SECONDS=0 \
  "${INSTALLER}" install --mode web --config "${config}" >"${work}/secret-parent-link.out" 2>&1; then
  fail 'installation accepted an initial administrator password below a symlink parent'
fi
grep -Fq 'must not contain symbolic links' "${work}/secret-parent-link.out" ||
  fail 'symlink initial administrator password parent rejection omitted its diagnostic'

saved_secrets="$(grep -E '^  (mariadb|redis|sysadmin)_password:' "${config}")"
mariadb_secret="$(awk -F'"' '$1 == "  mariadb_password: " {print $2}' "${config}")"
rerun_output="${work}/rerun.out"
if ! TP_DATA="${data}" \
  TP_OS_RELEASE_FILE="${os_release}" \
  TP_HEALTH_ATTEMPTS=2 \
  TP_HEALTH_DELAY_SECONDS=0 \
  "${INSTALLER}" install --mode web --config "${config}" >"${rerun_output}" 2>&1; then
  sed -n '1,240p' "${rerun_output}" >&2
  fail 'same-version Web installation replay failed'
fi
test "${saved_secrets}" = "$(grep -E '^  (mariadb|redis|sysadmin)_password:' "${config}")" ||
  fail 'same-version replay changed an existing identity credential'
test "$(cat "${random_state}")" = 3 ||
  fail 'same-version replay generated replacement credentials'

mismatched_config="${work}/mismatched-mariadb.yaml"
cp "${config}" "${mismatched_config}"
sed -i 's/^  mariadb_password:.*/  mariadb_password: "WrongMariaDBCredential123"/' "${mismatched_config}"
if TP_TEST_MARIADB_ACCEPTED_PASSWORD="${mariadb_secret}" \
  TP_DATA="${data}" \
  TP_OS_RELEASE_FILE="${os_release}" \
  TP_HEALTH_ATTEMPTS=1 \
  TP_HEALTH_DELAY_SECONDS=0 \
  "${INSTALLER}" install --mode web --config "${mismatched_config}" >"${work}/mismatched-mariadb.out" 2>&1; then
  fail 'installation accepted a MariaDB credential that only matched the existing container environment'
fi
grep -Fq 'Health check failed: MariaDB' "${work}/mismatched-mariadb.out" ||
  fail 'MariaDB credential mismatch omitted its health diagnostic'

assert_health_failure() {
  local injected_failure="$1"
  local expected_label="$2"
  local failure_output="${work}/failure-${injected_failure}.out"
  local login_calls_before
  login_calls_before="$(grep -Fc 'TP_VERIFY_SYSADMIN_CREDENTIAL=1' "${trace}" 2>/dev/null || true)"
  if TP_TEST_FAIL_PROBE="${injected_failure}" \
    TP_DATA="${data}" \
    TP_OS_RELEASE_FILE="${os_release}" \
    TP_HEALTH_ATTEMPTS=2 \
    TP_HEALTH_DELAY_SECONDS=0 \
    "${INSTALLER}" install --mode web --config "${config}" >"${failure_output}" 2>&1; then
    fail "installation succeeded when ${expected_label} was unhealthy"
  fi
  grep -Fq "Health check failed: ${expected_label}" "${failure_output}" ||
    fail "${expected_label} failure omitted its actionable diagnostic"
  if grep -Fq 'Web control plane is healthy' "${failure_output}"; then
    fail "${expected_label} failure printed the success marker"
  fi
  if [[ "${injected_failure}" == sysadmin ]]; then
    test "$(grep -Fc 'TP_VERIFY_SYSADMIN_CREDENTIAL=1' "${trace}")" = "$((login_calls_before + 1))" ||
      fail 'sysadmin credential health did not use the container-internal verifier exactly once'
    test "$(grep -Fc '/api/auth/login' "${curl_trace}" 2>/dev/null || true)" = 0 ||
      fail 'installer used the stateful login endpoint and may lock the account'
    test "$(grep -Fc '/api/auth/installer-health' "${curl_trace}" 2>/dev/null || true)" = 0 ||
      fail 'installer used the loopback HTTP credential oracle'
  fi
  local secret_key secret
  for secret_key in mariadb_password redis_password sysadmin_password; do
    secret="$(awk -F'"' -v key="${secret_key}" '$1 == "  " key ": " {print $2}' "${config}")"
    if grep -Fq "${secret}" "${failure_output}"; then
      fail "${expected_label} diagnostic leaked ${secret_key}"
    fi
  done
}

assert_health_failure MariaDB MariaDB
assert_health_failure Redis Redis
assert_health_failure HTTPS 'Web HTTPS'
for _ in 1 2 3 4; do
  assert_health_failure sysadmin 'sysadmin API credential'
done

grep -Fq 'docker run -d --name trojan-panel-mariadb' "${trace}" ||
  fail 'MariaDB was not deployed'
grep -Fq 'docker run -d --name trojan-panel-redis' "${trace}" ||
  fail 'Redis was not deployed'
grep -Fq 'docker run -d --name trojan-panel ' "${trace}" ||
  fail 'control-plane API was not deployed'
grep -Fq 'docker run -d --name trojan-panel-ui' "${trace}" ||
  fail 'control-plane UI was not deployed'
grep -Fq 'docker run -d --name trojan-panel-web-caddy' "${trace}" ||
  fail 'ACME entry was not deployed'

printf 'PASS Web bare installation and strong health contract\n'
