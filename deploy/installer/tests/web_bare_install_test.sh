#!/usr/bin/env bash
set -Eeuo pipefail

INSTALLER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALLER="${INSTALLER_DIR}/install.sh"
GENERATOR="${INSTALLER_DIR}/release/generate-assets.sh"
FAKE_YQ_READER="$(dirname "${BASH_SOURCE[0]}")/fixtures/fake_yq_reader.sh"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

work="$(mktemp -d)"
trap 'rm -rf -- "${work}"' EXIT
config="${work}/web.yaml"
data="${work}/data"
export TP_TEST_DATA_ROOT=1
container_state="${work}/containers"
trace="${work}/host.trace"
curl_trace="${work}/curl.trace"
random_state="${work}/random-state"
os_release="${work}/debian-12"
mkdir -p "${container_state}"
mkdir -p "${work}/trojan-panel/config" "${work}/mariadb/data"
printf 'legacy-web-secret\n' >"${work}/trojan-panel/config/legacy-secret"
printf 'legacy-database\n' >"${work}/mariadb/data/legacy-db"
legacy_before="$(sha256sum "${work}/trojan-panel/config/legacy-secret" "${work}/mariadb/data/legacy-db")"
printf 'ID=debian\nVERSION_ID="12"\n' >"${os_release}"
cp "$(dirname "${INSTALLER}")/examples/web.yaml" "${config}"
sed -i \
  -e "s#/tpdata/trojanpanelnext/trojan-panel/pki/client.crt#${data}/trojan-panel/pki/client.crt#" \
  -e "s#/tpdata/trojanpanelnext/trojan-panel/pki/client.key#${data}/trojan-panel/pki/client.key#" \
  -e "s#/tpdata/trojanpanelnext/trojanpanelnext-pki#${data}/trojanpanelnext-pki#" \
  "${config}"

# Exercise the same immutable bundle and released installer entrypoint used by
# production, while retaining the fake Docker seam for the host-side web
# health contract. The product images are intentionally synthetic: this test
# validates asset binding and installer orchestration, not image contents.
release_bundle="${work}/release-assets"
digest() {
  printf 'sha256:%064d' "$1"
}
"${GENERATOR}" \
  --version 1.2.3 \
  --source-commit 0123456789abcdef0123456789abcdef01234567 \
  --output "${release_bundle}" \
  --api-image "example.invalid/tpn-api@$(digest 1)" \
  --web-image "example.invalid/tpn-web@$(digest 2)" \
  --node-agent-image "example.invalid/tpn-node@$(digest 3)" \
  --caddy-image "caddy@$(digest 4)" \
  --mariadb-image "mariadb@$(digest 5)" \
  --redis-image "redis@$(digest 6)" >/dev/null
INSTALLER="${release_bundle}/install.sh"
cp "${release_bundle}/config-web.yaml" "${config}"
sed -i \
  -e "s#/tpdata/trojanpanelnext/trojan-panel/pki/client.crt#${data}/trojan-panel/pki/client.crt#" \
  -e "s#/tpdata/trojanpanelnext/trojan-panel/pki/client.key#${data}/trojan-panel/pki/client.key#" \
  -e "s#/tpdata/trojanpanelnext/trojanpanelnext-pki#${data}/trojanpanelnext-pki#" \
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
  if [[ " $* " == *' -tu1 '* || " $* " == *' -t u1 '* ]]; then
    # The released verifier uses od -tu1 to enforce printable manifest bytes.
    # Keep this fake output inside the printable ASCII range while retaining
    # deterministic credential generation for the host-side test.
    printf ' 65 66 67 68 69 70 71 72 73 74 75 76 77 78 79 80 81 82 83 84\n'
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

set_test_owner() {
  if [[ "${EUID}" == 0 ]]; then chown "$1" "$2"; else sudo -n chown "$1" "$2"; fi
}
assert_unsafe_root_rejected() {
  local root="$1" expected="$2" output_file="$3"
  if TP_DATA="${root}" TP_OS_RELEASE_FILE="${os_release}" \
    "${INSTALLER_DIR}/install.sh" install --mode web --config "${INSTALLER_DIR}/examples/web.yaml" >"${output_file}" 2>&1; then
    fail "unsafe host data root was accepted: ${root}"
  fi
  grep -Fq "${expected}" "${output_file}" || {
    sed -n '1,50p' "${output_file}" >&2
    fail "unsafe host data root omitted diagnostic: ${root}"
  }
  test ! -e "${root}/.trojanpanelnext-data-root" || fail 'unsafe root gained an ownership marker'
}
unsafe_root="${work}/foreign-empty-root"
mkdir -m 0700 "${unsafe_root}"
set_test_owner 65534 "${unsafe_root}"
assert_unsafe_root_rejected "${unsafe_root}" 'unsafe ownership or permissions' "${work}/foreign-root.out"
set_test_owner "${EUID}" "${unsafe_root}"
marked_root="${work}/foreign-marked-root"
mkdir -m 0700 "${marked_root}"
printf 'trojanpanelnext-data-root-v1\n' >"${marked_root}/.trojanpanelnext-data-root"
chmod 0600 "${marked_root}/.trojanpanelnext-data-root"
set_test_owner 65534 "${marked_root}"
if TP_DATA="${marked_root}" TP_OS_RELEASE_FILE="${os_release}" \
  "${INSTALLER_DIR}/install.sh" install --mode web --config "${INSTALLER_DIR}/examples/web.yaml" >"${work}/foreign-marked.out" 2>&1; then
  fail 'foreign-owned marked root was accepted'
fi
grep -Fq 'unsafe ownership or permissions' "${work}/foreign-marked.out" ||
  fail 'foreign-owned marked root omitted diagnostic'
set_test_owner "${EUID}" "${marked_root}"
test "$(cat "${marked_root}/.trojanpanelnext-data-root")" = trojanpanelnext-data-root-v1 ||
  fail 'foreign-owned marked root marker changed'
chmod 0770 "${unsafe_root}"
assert_unsafe_root_rejected "${unsafe_root}" 'unsafe ownership or permissions' "${work}/writable-root.out"
chmod 0700 "${unsafe_root}"
mkdir -m 0777 "${work}/writable-parent"
chmod 0777 "${work}/writable-parent"
mkdir -m 0700 "${work}/writable-parent/data"
assert_unsafe_root_rejected "${work}/writable-parent/data" 'parent has unsafe ownership or permissions' "${work}/writable-parent.out"

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
input_sha256="$(sha256sum "${config}" | awk '{print $1}')"
input_mode="$(stat -c '%a' "${config}")"
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
secret_config="${data}/effective-web.yaml"
test "$(sha256sum "${config}" | awk '{print $1}')" = "${input_sha256}" ||
  fail 'installer modified the caller-owned deployment configuration'
grep -Eq '^  sysadmin_password: "[[:alnum:]]{16,20}"$' "${secret_config}" ||
  fail 'installer did not persist a strong sysadmin password under the managed root'
test "$(stat -c '%a' "${secret_config}")" = 600 ||
  fail 'managed effective Web configuration is not mode 0600'
test "$(stat -c '%a' "${config}")" = "${input_mode}" ||
  fail 'caller-owned deployment configuration changed permissions'
test "$(stat -c '%a' "${data}/trojan-panel/config/initial-admin-password")" = 600 ||
  fail 'API initial sysadmin password file is not mode 0600'
if grep -Fq '/api/auth/installer-health' "${data}/trojan-panel-ui/nginx/default.conf"; then
  fail 'public UI configuration still knows about an administrator credential oracle'
fi

for secret_key in mariadb_password redis_password sysadmin_password; do
  secret="$(awk -F'"' -v key="${secret_key}" '$1 == "  " key ": " {print $2}' "${secret_config}")"
  [[ -n "${secret}" ]] || fail "${secret_key} was not generated"
  if grep -Fq "${secret}" "${output}"; then
    fail "${secret_key} leaked to installer output"
  fi
done
sysadmin_secret="$(awk -F'"' '$1 == "  sysadmin_password: " {print $2}' "${secret_config}")"
test "$(tr -d '\n' <"${data}/trojan-panel/config/initial-admin-password")" = "${sysadmin_secret}" ||
  fail 'API initial password file does not match the persisted sysadmin credential'

secret_trace_before="$(sha256sum "${trace}" | awk '{print $1}')"
chmod 0644 "${secret_config}"
if TP_DATA="${data}" "${INSTALLER}" install --mode web --config "${config}" >"${work}/effective-mode.out" 2>&1; then
  fail 'installer accepted an effective configuration readable by others'
fi
grep -Fq 'not a safe owned 0600 file' "${work}/effective-mode.out" || fail 'effective mode rejection omitted diagnostic'
chmod 0600 "${secret_config}"
set_test_owner 65534 "${secret_config}"
if TP_DATA="${data}" "${INSTALLER}" install --mode web --config "${config}" >"${work}/effective-owner.out" 2>&1; then
  fail 'installer adopted a foreign-owned effective configuration'
fi
grep -Fq 'not a safe owned 0600 file' "${work}/effective-owner.out" || fail 'effective owner rejection omitted diagnostic'
set_test_owner "${EUID}" "${secret_config}"
mv "${secret_config}" "${work}/effective-safe-backup.yaml"
printf 'outside-sentinel\n' >"${work}/outside-effective.yaml"
ln -s "${work}/outside-effective.yaml" "${secret_config}"
if TP_DATA="${data}" "${INSTALLER}" install --mode web --config "${config}" >"${work}/effective-symlink.out" 2>&1; then
  fail 'installer followed a symlink effective configuration'
fi
test "$(cat "${work}/outside-effective.yaml")" = outside-sentinel || fail 'effective symlink target was changed'
rm "${secret_config}"
mv "${work}/effective-safe-backup.yaml" "${secret_config}"
test "$(sha256sum "${trace}" | awk '{print $1}')" = "${secret_trace_before}" ||
  fail 'unsafe effective configuration caused Docker activity'

secret_link_dir="${data}/secret-link-dir"
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
rm "${secret_link_dir}/initial-admin-password"

secret_real_parent="${data}/secret-real-parent"
secret_link_parent="${data}/secret-link-parent"
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
rm "${secret_link_parent}"

saved_secrets="$(grep -E '^  (mariadb|redis|sysadmin)_password:' "${secret_config}")"
mariadb_secret="$(awk -F'"' '$1 == "  mariadb_password: " {print $2}' "${secret_config}")"
rerun_output="${work}/rerun.out"
if ! TP_DATA="${data}" \
  TP_OS_RELEASE_FILE="${os_release}" \
  TP_HEALTH_ATTEMPTS=2 \
  TP_HEALTH_DELAY_SECONDS=0 \
  "${INSTALLER}" install --mode web --config "${config}" >"${rerun_output}" 2>&1; then
  sed -n '1,240p' "${rerun_output}" >&2
  fail 'same-version Web installation replay failed'
fi
test "${saved_secrets}" = "$(grep -E '^  (mariadb|redis|sysadmin)_password:' "${secret_config}")" ||
  fail 'same-version replay changed an existing identity credential'
test "$(sha256sum "${config}" | awk '{print $1}')" = "${input_sha256}" ||
  fail 'same-version replay modified the caller-owned configuration'

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
grep -Fq 'Explicit MARIADB_PASSWORD differs from the committed Web credential' "${work}/mismatched-mariadb.out" ||
  fail 'MariaDB credential mismatch omitted its explicit migration diagnostic'
test "${saved_secrets}" = "$(grep -E '^  (mariadb|redis|sysadmin)_password:' "${secret_config}")" ||
  fail 'explicit credential mismatch changed the committed effective configuration'

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
    secret="$(awk -F'"' -v key="${secret_key}" '$1 == "  " key ": " {print $2}' "${secret_config}")"
    if grep -Fq "${secret}" "${failure_output}"; then
      fail "${expected_label} diagnostic leaked ${secret_key}"
    fi
  done
}

assert_health_failure MariaDB MariaDB
assert_health_failure Redis Redis
assert_health_failure HTTPS 'Web HTTPS'
for _ in 1 2 3 4; do
  assert_health_failure sysadmin 'sysadmin container credential'
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
grep -Fq "${data}/mariadb/data:/var/lib/mysql" "${trace}" || fail 'MariaDB host data mapping is wrong'
grep -Fq "${data}/redis/data:/data" "${trace}" || fail 'Redis host data mapping is wrong'
grep -Fq "${data}/trojan-panel/config/:/tpdata/trojan-panel/config/" "${trace}" ||
  fail 'API configuration host-to-container mapping is wrong'
grep -Fq "${data}/trojan-panel/pki/:/tpdata/trojan-panel/pki/:ro" "${trace}" ||
  fail 'API client certificate mount is not read-only'
if grep -Fq "${data}:/tpdata" "${trace}" || grep -Fq ' -v /tpdata:/tpdata' "${trace}"; then
  fail 'installer mounted the complete data root'
fi

retained="${data}/releases/1.2.3"
test -s "${retained}/release-manifest.json" && test -x "${retained}/entry/entryctl.sh" ||
  fail 'verified Release assets were not retained in the host data root'
cp "${retained}/verify-assets.sh" "${work}/trusted-verifier"
cat >"${retained}/verify-assets.sh" <<EOF
#!/usr/bin/env bash
touch "${work}/untrusted-verifier-executed"
exit 0
EOF
chmod 0755 "${retained}/verify-assets.sh"
if TP_DATA="${data}" TP_OS_RELEASE_FILE="${os_release}" \
  "${INSTALLER}" install --mode web --config "${config}" >"${work}/tampered-release.out" 2>&1; then
  fail 'replay accepted a modified retained Release verifier'
fi
test ! -e "${work}/untrusted-verifier-executed" ||
  fail 'installer executed unverified retained code'
cp "${work}/trusted-verifier" "${retained}/verify-assets.sh"
mv "${release_bundle}" "${work}/upload-removed"
if ! TP_DATA="${data}" TP_OS_RELEASE_FILE="${os_release}" \
  TP_HEALTH_ATTEMPTS=2 TP_HEALTH_DELAY_SECONDS=0 \
  "${retained}/install.sh" install --mode web --config "${config}" >"${work}/retained-rerun.out" 2>&1; then
  fail 'retained Release cannot rerun after the upload directory disappears'
fi
grep -Fq 'Web control plane is healthy' "${work}/retained-rerun.out" ||
  fail 'retained Release rerun omitted the health gate'
test "${legacy_before}" = "$(sha256sum "${work}/trojan-panel/config/legacy-secret" "${work}/mariadb/data/legacy-db")" ||
  fail 'installation or replay changed the legacy sibling layout'

# A failed first health gate still commits generated secrets inside the owned
# root, so an unchanged caller input can safely retry without rotation.
fresh_data="${work}/failed-first-data"
fresh_containers="${work}/failed-first-containers"
fresh_config="${work}/failed-first-web.yaml"
sed "s#${data}/#${fresh_data}/#g" "${config}" >"${fresh_config}"
fresh_input_sha256="$(sha256sum "${fresh_config}" | awk '{print $1}')"
mkdir -p "${fresh_containers}"
if TP_DATA="${fresh_data}" TP_TEST_DATA="${fresh_data}" \
  TP_TEST_CONTAINER_STATE="${fresh_containers}" TP_TEST_TRACE="${work}/failed-first.trace" \
  TP_TEST_CURL_TRACE="${work}/failed-first-curl.trace" TP_TEST_FAIL_PROBE=HTTPS \
  TP_OS_RELEASE_FILE="${os_release}" TP_HEALTH_ATTEMPTS=1 TP_HEALTH_DELAY_SECONDS=0 \
  "${retained}/install.sh" install --mode web --config "${fresh_config}" >"${work}/failed-first.out" 2>&1; then
  fail 'fresh Web installation accepted unhealthy HTTPS'
fi
grep -Fq 'Health check failed: Web HTTPS' "${work}/failed-first.out" || {
  sed -n '1,100p' "${work}/failed-first.out" >&2
  fail 'fresh health failure omitted HTTPS diagnostic'
}
fresh_secret_config="${fresh_data}/effective-web.yaml"
test -s "${fresh_secret_config}" || fail 'failed first install did not retain generated credentials'
fresh_secrets="$(grep -E '^  (mariadb|redis|sysadmin)_password:' "${fresh_secret_config}")"
if ! TP_DATA="${fresh_data}" TP_TEST_DATA="${fresh_data}" \
  TP_TEST_CONTAINER_STATE="${fresh_containers}" TP_TEST_TRACE="${work}/failed-first.trace" \
  TP_TEST_CURL_TRACE="${work}/failed-first-curl.trace" \
  TP_OS_RELEASE_FILE="${os_release}" TP_HEALTH_ATTEMPTS=2 TP_HEALTH_DELAY_SECONDS=0 \
  "${retained}/install.sh" install --mode web --config "${fresh_config}" >"${work}/failed-first-retry.out" 2>&1; then
  sed -n '1,100p' "${work}/failed-first-retry.out" >&2
  fail 'fresh Web installation could not retry with retained credentials'
fi
test "${fresh_secrets}" = "$(grep -E '^  (mariadb|redis|sysadmin)_password:' "${fresh_secret_config}")" ||
  fail 'retry after failed health gate rotated generated credentials'
test "$(sha256sum "${config}" | awk '{print $1}')" = "${input_sha256}" ||
  fail 'failed-first retry modified the caller-owned configuration'
test "$(sha256sum "${fresh_config}" | awk '{print $1}')" = "${fresh_input_sha256}" ||
  fail 'failed-first retry modified the fresh caller-owned configuration'

printf 'PASS Web bare installation and strong health contract\n'
