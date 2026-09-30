#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf -- "${TEST_DIR}"' EXIT
export TEST_DIR
# shellcheck source=deploy/installer/install.sh
source "${SCRIPT_DIR}/install.sh"
container_exists() { return 0; }
MOCK_SOURCE="${TEST_DIR}/bound data"
MOCK_COPIES=0
# shellcheck disable=SC2317
# Invoked by the sourced installation helper.
docker() {
  case "$1" in
  inspect) printf '%s\n' "${MOCK_SOURCE}" ;;
  cp)
    MOCK_COPIES=$((MOCK_COPIES + 1))
    printf 'fixture-data\n' >"${3}/.hidden-rdb"
    mkdir -p "${3}/nested"
    printf 'nested-fixture\n' >"${3}/nested/data"
    ;;
  *) printf 'Unexpected Docker request\n' >&2; return 1 ;;
  esac
}
mkdir -p "${MOCK_SOURCE}"
chmod 750 "${MOCK_SOURCE}"
before="$(stat -c '%u:%g:%a' "${MOCK_SOURCE}")"
persist_container_path redis /data "${MOCK_SOURCE}"
test "$(stat -c '%u:%g:%a' "${MOCK_SOURCE}")" = "${before}" || { printf 'FAIL upgrade changed active bind directory permissions\n' >&2; exit 1; }
test "${MOCK_COPIES}" = 0 || { printf 'FAIL exported already-persistent bind mount\n' >&2; exit 1; }

legacy="${TEST_DIR}/legacy data"
mkdir -p "${legacy}"
chmod 751 "${legacy}"
before="$(stat -c '%u:%g:%a' "${legacy}")"
MOCK_SOURCE="${TEST_DIR}/different volume"
persist_container_path redis /data "${legacy}"
test "$(stat -c '%u:%g:%a' "${legacy}")" = "${before}" || { printf 'FAIL migration changed destination directory permissions\n' >&2; exit 1; }
test -s "${legacy}/.hidden-rdb"
test -s "${legacy}/nested/data"
test "${MOCK_COPIES}" = 1
persist_container_path redis /data "${legacy}"
test "${MOCK_COPIES}" = 1 || { printf 'FAIL existing migration data overwritten\n' >&2; exit 1; }
printf 'PASS persistent bind upgrades and legacy migration preserve directory ownership/mode and content\n'
