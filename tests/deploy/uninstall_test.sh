#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../scripts/deploy" && pwd)"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf -- "${TEST_DIR}"' EXIT
export TEST_DIR
export MOCK_DOCKER_LOG="${TEST_DIR}/docker.log"
mkdir -p "${TEST_DIR}/library"
cp "${SCRIPT_DIR}/common.sh" "${TEST_DIR}/library/common.sh"
sed "s|/run/trojanpanelnext-update.lock|${TEST_DIR}/update.lock|g" "${SCRIPT_DIR}/uninstall.sh" >"${TEST_DIR}/library/uninstall.sh"
export MOCK_ROOT="${TEST_DIR}/library"
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
# Exported to the subprocess; Docker receives realistic container/image replies.
# shellcheck disable=SC2317,SC2329
mock_docker() {
  printf '%s\n' "$*" >>"${MOCK_DOCKER_LOG}"
  case "$*" in
  'info') return 0 ;;
  'inspect --format {{.Config.Image}} '*node-caddy*) printf 'caddy:2.8.4\n' ;;
  'inspect --format {{.Config.Image}} '*core*) printf 'ghcr.io/1linhao/trojanpanelnext-node-agent:older\n' ;;
  'container inspect '*core* | 'container inspect '*node-caddy*) return 0 ;;
  'container inspect '*) return 1 ;;
  'image ls ghcr.io/1linhao/trojanpanelnext-node-agent '* ) printf 'ghcr.io/1linhao/trojanpanelnext-node-agent:older\n' ;;
  'image ls '*) return 0 ;;
  'image inspect --format {{.Id}} caddy:'*) printf 'sha256:shared\n' ;;
  'image inspect --format {{.Id}} ghcr.io/'*) printf 'sha256:project\n' ;;
  'ps -aq') printf 'another-project\n' ;;
  'inspect --format {{.Image}} another-project') printf 'sha256:shared\n' ;;
  'rm -fv '* | 'image rm '*) return 0 ;;
  *) printf 'Unexpected mock Docker request: %s\n' "$*" >&2; return 99 ;;
  esac
}
export -f mock_docker

for mode in keep purge; do
  root="${TEST_DIR}/${mode}"
  mkdir -p "${root}/data/trojan-panel-core/runtime" "${root}/data/custom/node-caddy" "${root}/pki" "${root}/site" "${root}/runtime"
  printf 'sensitive-fixture\n' >"${root}/pki/key"
  printf 'site-fixture\n' >"${root}/site/index.html"
  cp "${SCRIPT_DIR}/templates/node.yaml" "${root}/node.yaml"
  TP_TEST_ROOT="${root}" yq -i '.trojanpanelnext.pki_bundle_dir = strenv(TP_TEST_ROOT) + "/pki" | .trojanpanelnext.grpc_client_ca_path = strenv(TP_TEST_ROOT) + "/data/trojan-panel-core/pki/client-ca.crt" | .trojanpanelnext.kernel_runtime_path = strenv(TP_TEST_ROOT) + "/runtime"' "${root}/node.yaml"
  TP_DATA="${root}/data" WEB_PATH="${root}/site" TP_PKI_BUNDLE_DIR="${root}/pki" KERNEL_RUNTIME_PATH="${root}/runtime" \
    bash -c '
      source "$MOCK_ROOT/uninstall.sh"
      require_root() { :; }; docker() { mock_docker "$@"; }; cleanup_host_maintenance() { :; }
      flag=--keep-data; [[ "$1" == purge ]] && flag=--purge-data
      main --config "$2" "$flag"
    ' test "${mode}" "${root}/node.yaml" >"${root}/output"

  if [[ "${mode}" == keep ]]; then
    test -f "${root}/node.yaml"
    test -f "${root}/pki/key"
    test -f "${root}/site/index.html"
  else
    test ! -f "${root}/node.yaml"
    test ! -e "${root}/runtime"
    test ! -e "${root}/pki"
    test ! -e "${root}/site"
  fi
done
grep -q '^rm -fv trojan-panel-core$' "${MOCK_DOCKER_LOG}"
grep -q '^image rm ghcr.io/1linhao/trojanpanelnext-node-agent:older$' "${MOCK_DOCKER_LOG}"
if grep -q '^image rm caddy:' "${MOCK_DOCKER_LOG}"; then fail 'shared image removed'; fi

# Existing deployment data remains untouched when another update/removal or
# an asynchronous host update owns admission.
root="${TEST_DIR}/keep"
run_refused_removal() {
  TP_DATA="${root}/data" WEB_PATH="${root}/site" bash -c '
    source "$MOCK_ROOT/uninstall.sh"
    require_root() { :; }; docker() { mock_docker "$@"; }; cleanup_host_maintenance() { :; }
    main --config "$1" --keep-data
  ' test "${root}/node.yaml" >"${TEST_DIR}/out" 2>"${TEST_DIR}/err"
}
mkdir "${TEST_DIR}/update.lock"
if run_refused_removal; then fail 'uninstall raced a global update lock'; fi
rmdir "${TEST_DIR}/update.lock"
for state in queued running; do
  printf '{"status":"%s","jobId":"fixture"}\n' "${state}" >"${root}/runtime/container-update.json"
  if run_refused_removal; then fail 'uninstall admitted an active container update'; fi
  grep -q 'container update is active' "${TEST_DIR}/err"
  test -f "${root}/node.yaml"
  test -f "${root}/pki/key"
  test ! -d "${TEST_DIR}/update.lock"
done
rm -f "${root}/runtime/container-update.json"
(
  exec {fixture_lock_fd}>"${root}/runtime/maintenance.lock"
  flock -n "${fixture_lock_fd}"
  if run_refused_removal; then fail 'uninstall ignored a held runtime maintenance lock'; fi
  grep -q 'Node maintenance is active' "${TEST_DIR}/err"
)
test ! -d "${TEST_DIR}/update.lock"

for protected in / /var/lib/docker/containers /etc/ssh /root/.ssh/id_ed25519 /usr/local/bin; do
  bash -c 'source "$1/uninstall.sh"; validate_purge_path "$2"' test "${SCRIPT_DIR}" "${protected}" >"${TEST_DIR}/out" 2>"${TEST_DIR}/err" && fail 'protected path deletion accepted'
  grep -q 'Refusing to delete protected path' "${TEST_DIR}/err"
done
for flags in '--keep-data --purge-data' '--purge --keep-data'; do
  # Deliberately split this fixed list of literal options.
  # shellcheck disable=SC2086
  if "${SCRIPT_DIR}/uninstall.sh" --config "${SCRIPT_DIR}/templates/node.yaml" ${flags} >"${TEST_DIR}/out" 2>"${TEST_DIR}/err"; then fail 'conflicting removal flags accepted'; fi
  grep -q 'cannot be combined' "${TEST_DIR}/err"
done
printf 'PASS removal keep/purge modes, image ownership and protected paths\n'
