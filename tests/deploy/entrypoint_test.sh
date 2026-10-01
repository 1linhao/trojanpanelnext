#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../scripts/deploy" && pwd)"
ENTRYPOINT="${SCRIPT_DIR}/../tp.sh"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf -- "${TEST_DIR}"' EXIT
export SCRIPT_DIR
export MOCK_REQUESTS="${TEST_DIR}/requests"
export MOCK_DOWNLOADS="${TEST_DIR}/downloads"
mkdir "${TEST_DIR}/tmp"
export TMPDIR="${TEST_DIR}/tmp"
MOCK_VERSION="$("${ENTRYPOINT}" --entry-version)"
export MOCK_VERSION

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
assert_clean() { [[ -z "$(find "${TMPDIR}" -mindepth 1 -print -quit)" ]] || fail 'entrypoint left downloaded scripts'; }
assert_fails() {
  if "$@" >"${TEST_DIR}/out" 2>"${TEST_DIR}/err"; then fail 'entrypoint unexpectedly succeeded'; fi
  assert_clean
}

# Only HTTP is replaced. Each release fixture uses its own scripts and YAML.
# shellcheck disable=SC2317,SC2329
curl() {
  local destination="" url="" file ref version
  while (($#)); do
    case "$1" in
    -o) destination="$2"; shift 2 ;;
    https://*) url="$1"; shift ;;
    *) shift ;;
    esac
  done
  file="${url##*/}"
  ref="${url#https://raw.githubusercontent.com/1linhao/trojanpanelnext/}"
  ref="${ref%%/*}"
  version="${ref#v}"
  printf '%s\n' "${url}" >>"${MOCK_REQUESTS}"
  printf '%s\n' "${destination}" >>"${MOCK_DOWNLOADS}"
  if [[ "${file}" == "${MOCK_TARGET:-}" ]]; then
    case "${MOCK_FAILURE:-}" in
    fail) printf 'partial' >"${destination}"; return 22 ;;
    empty) : >"${destination}"; return ;;
    html) printf '<html>error</html>\n' >"${destination}"; return ;;
    syntax) printf 'SCRIPT_VERSION="%s"\nif\n' "${version}" >"${destination}"; return ;;
    version) printf 'SCRIPT_VERSION="9.9"\nexit 0\n' >"${destination}"; return ;;
    term) printf 'partial' >"${destination}"; kill -TERM "${BASHPID}"; return ;;
    esac
  fi
  case "${file}" in
  web.yaml | node.yaml) sed "s/${MOCK_VERSION}/${version}/g" "${SCRIPT_DIR}/templates/${file}" >"${destination}" ;;
  dependencies.sh | install.sh | update.sh | uninstall.sh | web.sh | node.sh)
    if [[ "${MOCK_DISPATCH:-0}" == 1 ]]; then
      printf '#!/usr/bin/env bash\nSCRIPT_VERSION="%s"\nprintf "arg:<%%s>\\n" "$@"\nexit %s\n' "${version}" "${MOCK_STATUS:-0}" >"${destination}"
    else sed "s/${MOCK_VERSION}/${version}/g" "${SCRIPT_DIR}/${file}" >"${destination}"; fi
    ;;
  *) sed "s/${MOCK_VERSION}/${version}/g" "${SCRIPT_DIR}/${file}" >"${destination}" ;;
  esac
}
export -f curl

"${ENTRYPOINT}" --help >/dev/null
test ! -e "${MOCK_REQUESTS}"
assert_fails "${ENTRYPOINT}" unknown
assert_fails "${ENTRYPOINT}" --version
assert_fails "${ENTRYPOINT}" --version --help
assert_fails "${ENTRYPOINT}" --version 1.0.2-rc.3 --version 1.0.2-rc.3 config web
assert_fails "${ENTRYPOINT}" --version '../main' config web
assert_fails "${ENTRYPOINT}" --version 1 config web
assert_fails "${ENTRYPOINT}" update --config ignored.yaml
grep -Fq 'update requires an explicit --version target release' "${TEST_DIR}/err"
test ! -e "${MOCK_REQUESTS}"

(cd "${TEST_DIR}"; "${ENTRYPOINT}" config web --output 'web config.yaml')
test "$(stat -c %a "${TEST_DIR}/web config.yaml")" = 600
grep -Fq "/v${MOCK_VERSION}/scripts/deploy/config.sh" "${MOCK_REQUESTS}"
"${ENTRYPOINT}" validate --config "${TEST_DIR}/web config.yaml" | grep -q 'valid for web'
"${ENTRYPOINT}" --version "v${MOCK_VERSION}" validate --config "${TEST_DIR}/web config.yaml" >/dev/null
assert_clean

for target in common.sh validate.sh; do
  for failure in fail empty html syntax version term; do
    assert_fails env MOCK_TARGET="${target}" MOCK_FAILURE="${failure}" "${ENTRYPOINT}" validate --config "${TEST_DIR}/web config.yaml"
  done
done

# A single dispatcher selects two- and three-component releases and keeps every
# dependency, template and image tag within that release. No branch fallback.
for version in 2.5 3.4.2; do
  : >"${MOCK_REQUESTS}"
  "${ENTRYPOINT}" --version "${version}" config web --output "${TEST_DIR}/web-${version}.yaml" >/dev/null
  "${ENTRYPOINT}" validate --version "${version}" --config "${TEST_DIR}/web-${version}.yaml" >/dev/null
  grep -Fxq "  release: \"${version}\"" "${TEST_DIR}/web-${version}.yaml"
  grep -Fq "trojanpanelnext-api:${version}" "${TEST_DIR}/web-${version}.yaml"
  if grep -vFq "/v${version}/scripts/deploy/" "${MOCK_REQUESTS}"; then fail 'mixed release download'; fi
  assert_clean
done
assert_fails "${ENTRYPOINT}" --version 2.5 validate --config "${TEST_DIR}/web config.yaml"
grep -Fq 'does not match script release' "${TEST_DIR}/err"

: >"${MOCK_REQUESTS}"
env TP_SCRIPT_REF=main TP_CONFIG_REF=main "${ENTRYPOINT}" --version "${MOCK_VERSION}" config node --output "${TEST_DIR}/node.yaml" >/dev/null
if grep -Fq '/main/' "${MOCK_REQUESTS}"; then fail 'legacy ref override accepted'; fi
grep -Fq "/v${MOCK_VERSION}/scripts/deploy/templates/node.yaml" "${MOCK_REQUESTS}"

for action in install update remove web node; do
  env MOCK_DISPATCH=1 "${ENTRYPOINT}" "${action}" --version "${MOCK_VERSION}" --config "file with spaces.yaml" --help >"${TEST_DIR}/out"
  grep -Fxq 'arg:<file with spaces.yaml>' "${TEST_DIR}/out"
  if grep -Fq 'arg:<--version>' "${TEST_DIR}/out"; then fail 'global release option passed to child'; fi
  assert_clean
  if env MOCK_DISPATCH=1 MOCK_STATUS=17 "${ENTRYPOINT}" --version "${MOCK_VERSION}" "${action}" --config missing.yaml >"${TEST_DIR}/out"; then fail 'lost child exit code'; else test "$?" = 17; fi
  assert_clean
done
: >"${MOCK_REQUESTS}"
for dependency_action in install remove; do
  env MOCK_DISPATCH=1 "${ENTRYPOINT}" deps "${dependency_action}" --version 2.5 >"${TEST_DIR}/out"
  grep -Fxq "arg:<${dependency_action}>" "${TEST_DIR}/out"
  grep -Fq '/v2.5/scripts/deploy/dependencies.sh' "${MOCK_REQUESTS}"
  if grep -Fq '/scripts/deploy/install.sh' "${MOCK_REQUESTS}"; then fail 'dependency command fetched installer'; fi
  assert_clean
  if env MOCK_DISPATCH=1 MOCK_STATUS=17 "${ENTRYPOINT}" deps "${dependency_action}" >"${TEST_DIR}/out"; then fail 'lost dependency child exit code'; else test "$?" = 17; fi
  assert_clean
done
for failure in fail empty html syntax version term; do
  assert_fails env MOCK_TARGET=dependencies.sh MOCK_FAILURE="${failure}" "${ENTRYPOINT}" deps install
done
"${ENTRYPOINT}" deps --help >"${TEST_DIR}/out"
grep -Fq 'Existing Docker and compatible yq are reused' "${TEST_DIR}/out"
assert_clean
: >"${MOCK_REQUESTS}"
env MOCK_DISPATCH=1 "${ENTRYPOINT}" --version 2.5 install --config ignored.yaml >/dev/null
grep -Fq '/v2.5/scripts/deploy/uninstall.sh' "${MOCK_REQUESTS}"
assert_clean
: >"${MOCK_REQUESTS}"
env MOCK_DISPATCH=1 "${ENTRYPOINT}" --version 2.5 update --config 'original web.yaml' >/dev/null
for dependency in common.sh update.sh install.sh uninstall.sh; do
  grep -Fq "/v2.5/scripts/deploy/${dependency}" "${MOCK_REQUESTS}"
done
for failure in fail empty html syntax version term; do
  assert_fails env MOCK_TARGET=update.sh MOCK_FAILURE="${failure}" "${ENTRYPOINT}" --version 2.5 update --config ignored.yaml
done
assert_clean
: >"${MOCK_REQUESTS}"
"${ENTRYPOINT}" update --help >"${TEST_DIR}/out"
grep -Fq 'TrojanPanel Next image update' "${TEST_DIR}/out"
assert_clean
printf 'PASS release-selected remote dispatch, argument/status propagation and failure cleanup\n'
