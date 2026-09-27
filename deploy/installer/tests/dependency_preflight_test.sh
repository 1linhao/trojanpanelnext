#!/usr/bin/env bash
set -Eeuo pipefail

INSTALLER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALLER="${INSTALLER_DIR}/install.sh"
GENERATOR="${INSTALLER_DIR}/release/generate-assets.sh"
FAKE_YQ_READER="${INSTALLER_DIR}/tests/fixtures/fake_yq_reader.sh"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

digest() {
  printf 'sha256:%064d' "$1"
}

work="$(mktemp -d)"
trap 'rm -rf -- "${work}"' EXIT
trace="${work}/host.trace"
development_config="${work}/invalid-web.yaml"
sed 's/^  hostname:.*/  hostname: ""/' "${INSTALLER_DIR}/examples/web.yaml" >"${development_config}"
bundle="${work}/bundle"
"${GENERATOR}" \
  --version 1.2.3 \
  --source-commit 0123456789abcdef0123456789abcdef01234567 \
  --output "${bundle}" \
  --api-image "ghcr.io/1linhao/trojanpanelnext-api@$(digest 1)" \
  --web-image "ghcr.io/1linhao/trojanpanelnext-web@$(digest 2)" \
  --node-agent-image "ghcr.io/1linhao/trojanpanelnext-node-agent@$(digest 3)" \
  --caddy-image "caddy@$(digest 4)" \
  --mariadb-image "mariadb@$(digest 5)" \
  --redis-image "redis@$(digest 6)" >/dev/null
release_config="${work}/invalid-release-web.yaml"
sed 's/^  hostname:.*/  hostname: ""/' "${bundle}/config-web.yaml" >"${release_config}"

command() {
  if [[ "${1:-}" == -v && $# -ge 2 ]]; then
    case ",${TP_TEST_MISSING:-}," in
    *",$2,"*) return 1 ;;
    esac
  fi
  builtin command "$@"
}

uname() {
  if [[ "${1:-}" == -m ]]; then
    printf '%s\n' "${TP_TEST_ARCH:-x86_64}"
    return
  fi
  /usr/bin/uname "$@"
}

id() {
  if [[ "${1:-}" == -u ]]; then
    printf '0\n'
    return
  fi
  /usr/bin/id "$@"
}

yq() {
  "${FAKE_YQ_READER}" "$@"
}

curl() { printf 'curl\n' >>"${TP_TEST_TRACE}"; return 97; }
apt-get() { printf 'apt-get\n' >>"${TP_TEST_TRACE}"; return 97; }
dnf() { printf 'dnf\n' >>"${TP_TEST_TRACE}"; return 97; }
docker() { printf 'docker\n' >>"${TP_TEST_TRACE}"; return 97; }
export -f command uname id yq curl apt-get dnf docker

run_install() {
  local entrypoint="$1" config="$2" arch="$3" missing="$4"
  shift 4
  TP_TEST_ARCH="${arch}" TP_TEST_MISSING="${missing}" TP_TEST_TRACE="${trace}" \
    TP_INSTALL_DEPS=1 \
    TP_OS_RELEASE_FILE="${work}/nonexistent-os-release" \
    FAKE_YQ_READER="${FAKE_YQ_READER}" \
    "${entrypoint}" install --mode web --config "${config}" "$@"
}

assert_missing_is_reported_without_installation() {
  local entrypoint="$1" config="$2" label="$3"
  local output="${work}/${label}.out"
  : >"${trace}"
  if run_install "${entrypoint}" "${config}" x86_64 docker,curl,yq,jq \
    --entry-spec /synthetic/entry-spec.json >"${output}" 2>&1; then
    fail "${label} accepted missing dependencies"
  fi
  grep -Fq 'Missing required software dependencies:' "${output}" || {
    sed -n '1,80p' "${output}" >&2
    fail "${label} omitted the aggregated dependency heading"
  }
  for dependency in docker curl yq jq; do
    grep -Fq -- "- ${dependency}:" "${output}" ||
      fail "${label} omitted ${dependency}"
  done
  grep -Fq 'Install the missing software using your system package manager' "${output}" ||
    fail "${label} omitted recovery guidance"
  test ! -s "${trace}" || fail "${label} attempted host installation"
  printf 'TRACE entrypoint=%s missing=docker,curl,yq,jq host-installs=0\n' "${label}"
}

assert_missing_is_reported_without_installation "${INSTALLER}" "${development_config}" development
assert_missing_is_reported_without_installation "${bundle}/install.sh" "${release_config}" release-direct
assert_missing_is_reported_without_installation "${bundle}/bootstrap.sh" "${release_config}" release-bootstrap

: >"${trace}"
if run_install "${INSTALLER}" "${development_config}" x86_64 od,sha256sum,install >"${work}/coreutils.out" 2>&1; then
  fail 'installer accepted missing coreutils commands'
fi
test "$(grep -Fc -- '- coreutils:' "${work}/coreutils.out")" = 1 ||
  fail 'missing coreutils were not reported as one dependency'
test ! -s "${trace}" || fail 'coreutils preflight attempted host installation'

: >"${trace}"
if TP_TEST_ARCH=x86_64 TP_TEST_MISSING=jq TP_TEST_TRACE="${trace}" \
  FAKE_YQ_READER="${FAKE_YQ_READER}" \
  "${bundle}/install.sh" install --mode combined --config "${bundle}/config-combined.yaml" \
  >"${work}/combined-jq.out" 2>&1; then
  fail 'combined installer accepted missing jq'
fi
grep -Fq -- '- jq:' "${work}/combined-jq.out" || fail 'combined installer omitted jq'
test ! -s "${trace}" || fail 'combined preflight attempted host installation'

: >"${trace}"
if run_install "${bundle}/install.sh" "${release_config}" x86_64 '' >"${work}/other-linux.out" 2>&1; then
  fail 'invalid config unexpectedly installed'
fi
grep -Fq 'TP_WEB_DOMAIN is required' "${work}/other-linux.out" ||
  fail 'installer stopped on missing /etc/os-release instead of validating configuration'
test ! -s "${trace}" || fail 'invalid config mutated the host'

: >"${trace}"
if run_install "${bundle}/install.sh" "${release_config}" aarch64 '' >"${work}/architecture.out" 2>&1; then
  fail 'incompatible binary architecture was accepted'
fi
grep -Fq 'Bundled secure-file helper supports Linux x86_64 only' "${work}/architecture.out" ||
  fail 'incompatible binary architecture was not explained'
test ! -s "${trace}" || fail 'incompatible architecture mutated the host'

printf 'PASS distribution-agnostic, report-only dependency preflight\n'
