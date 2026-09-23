#!/usr/bin/env bash
set -Eeuo pipefail

SMOKE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/release_web_docker_smoke_test.sh"
HELPERS="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/fixtures/release_web_smoke_helpers.sh"
source "${HELPERS}"

fail() {
  printf 'FAIL release Web smoke cleanup: %s\n' "$1" >&2
  exit 1
}

work="$(mktemp -d)"
sentinel="$(realpath -m -- "${work}/../release-web-smoke-preexisting-$$")"
trap 'rm -rf -- "${work}" "${sentinel}"' EXIT
trace="${work}/docker.trace"
mkdir -p "${sentinel}"
printf 'operator-data\n' >"${sentinel}/keep"
mkdir -p "${work}/bin"
cat >"${work}/bin/docker" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${TP_SMOKE_DOCKER_TRACE}"
case "${1:-}" in
info) exit 0 ;;
inspect)
  [[ "${2:-}" == "${TP_SMOKE_FAKE_CONTAINER_EXISTS:-}" ]] && exit 0
  exit 1
  ;;
*) exit 97 ;;
esac
EOF
chmod 0755 "${work}/bin/docker"

if PATH="${work}/bin:${PATH}" TP_SMOKE_DOCKER_TRACE="${trace}" \
  TP_RELEASE_WEB_SMOKE_DATA_DIR="${sentinel}" bash "${SMOKE}" >"${work}/output" 2>&1; then
  fail 'pre-existing data directory was accepted'
fi
grep -Fq "smoke data directory must be inside the per-run temporary workspace: ${sentinel}" "${work}/output" ||
  fail 'preflight did not reject an unowned data directory'
test "$(cat "${sentinel}/keep")" = operator-data ||
  fail 'preflight failure deleted or changed operator data'
if grep -Eq '^(rm|run) ' "${trace}"; then
  fail 'preflight failure invoked Docker resource mutation'
fi

container_trace="${work}/container.trace"
if PATH="${work}/bin:${PATH}" TP_SMOKE_DOCKER_TRACE="${container_trace}" \
  TP_SMOKE_FAKE_CONTAINER_EXISTS=tp-web-smoke-api-regression \
  TP_RELEASE_WEB_SMOKE_SUFFIX=regression bash "${SMOKE}" >"${work}/container-output" 2>&1; then
  fail 'pre-existing installer container was accepted'
fi
grep -Fq 'smoke container name is unexpectedly occupied: tp-web-smoke-api-regression' "${work}/container-output" ||
  fail 'preflight did not identify the conflicting container'
if grep -Eq '^(rm|run) ' "${container_trace}"; then
  fail 'container conflict invoked Docker resource mutation'
fi

diagnostic_config="${work}/diagnostic-config.yaml"
diagnostic_stdout="${work}/diagnostic-stdout"
diagnostic_stderr="${work}/diagnostic-stderr"
cat >"${diagnostic_config}" <<'EOF'
trojanpanelnext:
  sysadmin_password: "ADMIN-SECRET-12345"
  mariadb_password: "DB-SECRET-abcdef"
  redis_password: "REDIS-SECRET-abcdef"
EOF
printf 'admin=%s db=%s redis=%s\n' \
  ADMIN-SECRET-12345 DB-SECRET-abcdef REDIS-SECRET-abcdef >"${diagnostic_stdout}"
printf 'failure: ADMIN-SECRET-12345 / DB-SECRET-abcdef / REDIS-SECRET-abcdef\n' >"${diagnostic_stderr}"
smoke_print_install_failure_diagnostics \
  "${diagnostic_config}" "${diagnostic_stdout}" "${diagnostic_stderr}" \
  >"${work}/sanitized-output" 2>&1
smoke_assert_diagnostics_sanitized "${diagnostic_config}" "${work}/sanitized-output" ||
  fail 'sanitized failure diagnostics still contain a configured credential'
for secret in ADMIN-SECRET-12345 DB-SECRET-abcdef REDIS-SECRET-abcdef; do
  if grep -Fq -- "${secret}" "${work}/sanitized-output"; then
    fail 'failure diagnostics leaked a configured credential'
  fi
done
grep -Fq '[REDACTED]' "${work}/sanitized-output" ||
  fail 'failure diagnostics did not visibly redact credentials'

printf '%s\n' 'PASS release Web smoke preflight preserves pre-existing host resources'
