#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../scripts/deploy" && pwd)"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf -- "${TEST_DIR}"' EXIT
# shellcheck source=scripts/deploy/install.sh
source "${SCRIPT_DIR}/install.sh"
container_exists() { return 0; }
MOCK_SOURCE="${TEST_DIR}/bound data"
# Invoked by the sourced installation helper.
docker() {
  case "$1" in
  inspect) printf '%s\n' "${MOCK_SOURCE}" ;;
  *) printf 'Unexpected Docker request\n' >&2; return 1 ;;
  esac
}
mkdir -p "${MOCK_SOURCE}"
chmod 750 "${MOCK_SOURCE}"
before="$(stat -c '%u:%g:%a' "${MOCK_SOURCE}")"
persist_container_path redis /data "${MOCK_SOURCE}"
test "$(stat -c '%u:%g:%a' "${MOCK_SOURCE}")" = "${before}" || { printf 'FAIL upgrade changed active bind directory permissions\n' >&2; exit 1; }
printf 'current-data\n' >"${MOCK_SOURCE}/dump.rdb"
persist_container_path redis /data "${MOCK_SOURCE}"
grep -Fxq current-data "${MOCK_SOURCE}/dump.rdb"

unsupported="${TEST_DIR}/unsupported data"
mkdir -p "${unsupported}"
printf 'retain-data\n' >"${unsupported}/dump.rdb"
chmod 751 "${unsupported}"
before="$(stat -c '%u:%g:%a' "${unsupported}")"
if (persist_container_path redis /data "${unsupported}") >"${TEST_DIR}/out" 2>"${TEST_DIR}/err"; then
  printf 'FAIL accepted unsupported data mount\n' >&2
  exit 1
fi
grep -Fq 'unsupported data mount' "${TEST_DIR}/err"
test "$(stat -c '%u:%g:%a' "${unsupported}")" = "${before}"
grep -Fxq retain-data "${unsupported}/dump.rdb"
printf 'PASS persistent bind updates preserve data and reject unsupported mount layouts\n'
