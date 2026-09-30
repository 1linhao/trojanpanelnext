#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf -- "${TEST_DIR}"' EXIT
export SCRIPT_DIR
export MOCK_REQUESTS="${TEST_DIR}/requests"
export MOCK_DOWNLOADS="${TEST_DIR}/downloads"
mkdir "${TEST_DIR}/tmp"
export TMPDIR="${TEST_DIR}/tmp"
MOCK_VERSION="$("${SCRIPT_DIR}/tp.sh" --version)"
export MOCK_VERSION

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
assert_clean() { [[ -z "$(find "${TMPDIR}" -mindepth 1 -print -quit)" ]] || fail 'entrypoint left downloaded scripts'; }
assert_fails() {
  if "$@" >"${TEST_DIR}/out" 2>"${TEST_DIR}/err"; then fail 'entrypoint unexpectedly succeeded'; fi
  assert_clean
}

# Exported into the entrypoint and selected command; only HTTP is replaced.
# shellcheck disable=SC2317,SC2329
curl() {
  local destination="" url="" file
  while (($#)); do
    case "$1" in
    -o) destination="$2"; shift 2 ;;
    https://*) url="$1"; shift ;;
    *) shift ;;
    esac
  done
  file="${url##*/}"
  printf '%s\n' "${url}" >>"${MOCK_REQUESTS}"
  printf '%s\n' "${destination}" >>"${MOCK_DOWNLOADS}"
  if [[ "${file}" == "${MOCK_TARGET:-}" ]]; then
    case "${MOCK_FAILURE:-}" in
    fail) printf 'partial' >"${destination}"; return 22 ;;
    empty) : >"${destination}"; return ;;
    html) printf '<html>error</html>\n' >"${destination}"; return ;;
    syntax) printf 'SCRIPT_VERSION="%s"\nif\n' "${MOCK_VERSION}" >"${destination}"; return ;;
    version) printf 'SCRIPT_VERSION="0.0.0"\nexit 0\n' >"${destination}"; return ;;
    term) printf 'partial' >"${destination}"; kill -TERM "${BASHPID}"; return ;;
    esac
  fi
  case "${file}" in
  web.yaml | node-agent.yaml) cp "${SCRIPT_DIR}/examples/${file}" "${destination}" ;;
  install.sh | uninstall.sh)
    if [[ "${MOCK_DISPATCH:-0}" == 1 ]]; then
      printf '#!/usr/bin/env bash\nSCRIPT_VERSION="%s"\nprintf "arg:<%%s>\\n" "$@"\nexit %s\n' "${MOCK_VERSION}" "${MOCK_STATUS:-0}" >"${destination}"
    else cp "${SCRIPT_DIR}/${file}" "${destination}"; fi
    ;;
  *) cp "${SCRIPT_DIR}/${file}" "${destination}" ;;
  esac
}
export -f curl

"${SCRIPT_DIR}/tp.sh" --help >/dev/null
test ! -e "${MOCK_REQUESTS}"
assert_fails "${SCRIPT_DIR}/tp.sh" unknown
test ! -e "${MOCK_REQUESTS}"

(cd "${TEST_DIR}"; "${SCRIPT_DIR}/tp.sh" config web --output 'web config.yaml')
test "$(stat -c %a "${TEST_DIR}/web config.yaml")" = 600
grep -Fq "/v${MOCK_VERSION}/deploy/installer/config.sh" "${MOCK_REQUESTS}"
"${SCRIPT_DIR}/tp.sh" validate --config "${TEST_DIR}/web config.yaml" | grep -q 'valid for web'
assert_clean

for target in common.sh validate.sh; do
  for failure in fail empty html syntax version term; do
    assert_fails env MOCK_TARGET="${target}" MOCK_FAILURE="${failure}" "${SCRIPT_DIR}/tp.sh" validate --config "${TEST_DIR}/web config.yaml"
  done
done

env TP_SCRIPT_REF=feat/installer-entrypoint "${SCRIPT_DIR}/tp.sh" config node --output "${TEST_DIR}/node.yaml" >/dev/null
grep -Fq '/feat/installer-entrypoint/deploy/installer/common.sh' "${MOCK_REQUESTS}"
grep -Fq '/feat/installer-entrypoint/deploy/installer/config.sh' "${MOCK_REQUESTS}"
grep -Fq '/feat/installer-entrypoint/deploy/installer/examples/node-agent.yaml' "${MOCK_REQUESTS}"
assert_fails env TP_SCRIPT_REF='../main' "${SCRIPT_DIR}/tp.sh" validate --config "${TEST_DIR}/node.yaml"

for action in install remove; do
  env MOCK_DISPATCH=1 "${SCRIPT_DIR}/tp.sh" "${action}" --config "file with spaces.yaml" --help >"${TEST_DIR}/out"
  grep -Fxq 'arg:<file with spaces.yaml>' "${TEST_DIR}/out"
  assert_clean
  if env MOCK_DISPATCH=1 MOCK_STATUS=17 "${SCRIPT_DIR}/tp.sh" "${action}" --config missing.yaml >"${TEST_DIR}/out"; then fail 'lost child exit code'; else test "$?" = 17; fi
  assert_clean
done
printf 'PASS entrypoint remote dispatch, version binding, argument/status propagation and failure cleanup\n'
