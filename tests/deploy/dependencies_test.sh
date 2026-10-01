#!/usr/bin/env bash
# Fixture globals and overrides are consumed by the sourced dependency manager.
# shellcheck disable=SC2034
set -euo pipefail

TEST_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/dependencies_test.sh"
DEPENDENCY_SCRIPT="$(dirname "${TEST_FILE}")/../../scripts/deploy/dependencies.sh"

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

# Every privileged command is replaced in the child process. Real file changes
# and locking are restricted to the fixture directory, with no network access.
run_fixture() {
  local fixture_root="$1"
  shift
  # shellcheck source=/dev/null
  source "${DEPENDENCY_SCRIPT}"
  TP_DEPENDENCY_STATE_DIR="${fixture_root}/state"
  TP_DEPENDENCY_YQ_PATH="${fixture_root}/bin/yq"
  TP_DEPENDENCY_HOST_UNIT_PATH="${fixture_root}/trojanpanelnext-host.service"
  TP_DEPENDENCY_CONTAINERD_SOCKET="${fixture_root}/containerd.sock"

  # Platform fixtures bypass host OS/systemd detection; package ownership,
  # download verification, installation/removal and argument checks stay real.
  # shellcheck disable=SC2317,SC2329
  detect_dependency_platform() {
    TP_DEPENDENCY_ARCH=amd64
    TP_DEPENDENCY_YQ_HASH="$(sha256sum "${fixture_root}/download-yq")"
    TP_DEPENDENCY_YQ_HASH="${TP_DEPENDENCY_YQ_HASH%% *}"
  }
  # shellcheck disable=SC2317,SC2329
  require_root() {
    printf 'require-root\n' >>"${fixture_root}/calls"
    [[ ! -f "${fixture_root}/deny-root" ]] || return 77
  }
  # shellcheck disable=SC2317,SC2329
  command() {
    if [[ "${1:-}" == -v ]]; then
      case "${2:-}" in
      docker)
        [[ -f "${fixture_root}/docker-present" ]] || return 1
        printf '%s\n' 'fixture-docker'; return ;;
      yq)
        [[ -f "${fixture_root}/existing-yq" || -f "${TP_DEPENDENCY_YQ_PATH}" ]] || return 1
        printf '%s\n' "${TP_DEPENDENCY_YQ_PATH}"; return ;;
      esac
    fi
    builtin command "$@"
  }
  # shellcheck disable=SC2317,SC2329
  yq() {
    if [[ -f "${fixture_root}/existing-yq" ]]; then
      cat "${fixture_root}/existing-yq"
    else
      "${TP_DEPENDENCY_YQ_PATH}" "$@"
    fi
  }
  # shellcheck disable=SC2317,SC2329
  dpkg() { [[ "$*" == --print-architecture ]] && printf 'amd64\n'; }
  # shellcheck disable=SC2317,SC2329
  dpkg-query() {
    [[ "$#" == 3 && "$1" == -W && "$2" == "-f="* ]] || return 99
    if grep -Fxq "$3" "${fixture_root}/installed-packages"; then printf 'installed'; else return 1; fi
  }
  # shellcheck disable=SC2317,SC2329
  apt-get() {
    printf 'apt-get %s\n' "$*" >>"${fixture_root}/calls"
    local package temp
    case "${1:-}" in
    update) return ;;
    install)
      [[ ! -f "${fixture_root}/apt-fail" ]] || return 100
      shift
      for package in "$@"; do
        [[ "${package}" != -* ]] || continue
        if ! grep -Fxq "${package}" "${fixture_root}/installed-packages"; then printf '%s\n' "${package}" >>"${fixture_root}/installed-packages"; fi
        [[ "${package}" != docker.io ]] || touch "${fixture_root}/docker-present"
        if [[ "${package}" == docker.io && -f "${fixture_root}/apt-partial-fail" ]]; then return 100; fi
      done
      ;;
    --simulate)
      [[ "${2:-}" == remove ]] || return 99
      shift 2
      for package in "$@"; do printf 'Remv %s [fixture]\n' "${package}"; done
      if [[ -f "${fixture_root}/extra-apt-removal" ]]; then printf 'Remv openssl [fixture]\n'; fi
      ;;
    remove)
      shift
      for package in "$@"; do
        [[ "${package}" != -* ]] || continue
        temp="${fixture_root}/installed-packages.next"
        awk -v package="${package}" '$0 != package' "${fixture_root}/installed-packages" >"${temp}"
        mv -- "${temp}" "${fixture_root}/installed-packages"
      done
      ;;
    *) printf 'Unexpected APT request: %s\n' "$*" >&2; return 99 ;;
    esac
  }
  # shellcheck disable=SC2317,SC2329
  docker() {
    printf 'docker %s\n' "$*" >>"${fixture_root}/calls"
    [[ "${1:-}" == --host && "${2:-}" == unix:///var/run/docker.sock ]] || return 99
    shift 2
    case "$*" in
    info) [[ ! -f "${fixture_root}/docker-offline" ]] ;;
    'ps -aq')
      [[ ! -f "${fixture_root}/docker-offline" ]] || return 1
      if [[ -f "${fixture_root}/containers" ]]; then cat "${fixture_root}/containers"; fi
      ;;
    *) printf 'Unexpected Docker request: %s\n' "$*" >&2; return 99 ;;
    esac
  }
  # shellcheck disable=SC2317,SC2329
  systemctl() {
    printf 'systemctl %s\n' "$*" >>"${fixture_root}/calls"
    case "$*" in
    'is-active --quiet trojanpanelnext-host.service') [[ -f "${fixture_root}/helper-active" ]] ;;
    'is-active --quiet containerd') [[ -f "${fixture_root}/containerd-active" ]] ;;
    'enable --now docker') rm -f -- "${fixture_root}/docker-offline" ;;
    *) printf 'Unexpected systemctl request: %s\n' "$*" >&2; return 99 ;;
    esac
  }
  # shellcheck disable=SC2317,SC2329
  ctr() {
    printf 'ctr %s\n' "$*" >>"${fixture_root}/calls"
    [[ "${1:-}" == --address && "${2:-}" == "${TP_DEPENDENCY_CONTAINERD_SOCKET}" ]] || return 99
    shift 2
    case "$*" in
    'namespaces list --quiet') printf 'moby\nk8s.io\n' ;;
    '--namespace moby containers list --quiet') return ;;
    '--namespace k8s.io containers list --quiet')
      if [[ -f "${fixture_root}/containerd-workload" ]]; then printf 'another-workload\n'; fi
      ;;
    *) printf 'Unexpected containerd request: %s\n' "$*" >&2; return 99 ;;
    esac
  }
  # shellcheck disable=SC2317,SC2329
  curl() {
    printf 'curl %s\n' "$*" >>"${fixture_root}/calls"
    local destination=""
    while (($#)); do
      case "$1" in -o) destination="$2"; shift 2 ;; *) shift ;; esac
    done
    [[ -n "${destination}" ]] || return 99
    if [[ -f "${fixture_root}/corrupt-download" ]]; then
      printf 'untrusted-download\n' >"${destination}"
    else
      cp -- "${fixture_root}/download-yq" "${destination}"
    fi
  }
  main "$@"
}

if [[ "${1:-}" == --fixture ]]; then
  shift
  run_fixture "$@"
  exit
fi

TEST_DIR="$(mktemp -d)"
trap 'status=$?; if ((status != 0)) && [[ -f "${FIXTURE:-}/err" ]]; then cat "${FIXTURE}/err" >&2; fi; rm -rf -- "${TEST_DIR}"' EXIT
mkdir "${TEST_DIR}/tmp"
export TMPDIR="${TEST_DIR}/tmp"

new_fixture() {
  local name="$1"
  FIXTURE="${TEST_DIR}/${name}"
  mkdir -p "${FIXTURE}/bin" "${FIXTURE}/state" "${FIXTURE}/docker-data"
  : >"${FIXTURE}/calls"
  printf '%s\n' bash curl ca-certificates grep coreutils openssl tar findutils >"${FIXTURE}/installed-packages"
  printf '#!/usr/bin/env bash\nprintf "yq (https://github.com/mikefarah/yq/) version v4.53.6\\n"\n' >"${FIXTURE}/download-yq"
  printf 'retained-data\n' >"${FIXTURE}/docker-data/database"
}

own_docker() {
  printf '%s\n' docker.io containerd runc >"${FIXTURE}/state/docker-packages"
  cat "${FIXTURE}/state/docker-packages" >>"${FIXTURE}/installed-packages"
  touch "${FIXTURE}/docker-present"
}

own_yq() {
  cp -- "${FIXTURE}/download-yq" "${FIXTURE}/bin/yq"
  chmod 755 -- "${FIXTURE}/bin/yq"
  local hash
  hash="$(sha256sum "${FIXTURE}/bin/yq")"
  printf '%s\n' "${hash%% *}" >"${FIXTURE}/state/yq.sha256"
}

run_command() {
  bash "${TEST_FILE}" --fixture "${FIXTURE}" "$@" >"${FIXTURE}/out" 2>"${FIXTURE}/err"
}

expect_failure() {
  local message="$1"
  shift
  if run_command "$@"; then fail "${message}"; fi
}

assert_no_remove() {
  if grep -q '^apt-get remove ' "${FIXTURE}/calls"; then fail 'refused operation removed packages'; fi
  test -f "${FIXTURE}/state/docker-packages"
}

new_fixture install-new
touch "${FIXTURE}/docker-offline"
run_command install
diff -u <(printf '%s\n' docker.io containerd runc) "${FIXTURE}/state/docker-packages"
test -x "${FIXTURE}/bin/yq"
test "$(stat -c %a "${FIXTURE}/state")" = 700
test "$(stat -c %a "${FIXTURE}/state/docker-packages")" = 600
test "$(stat -c %a "${FIXTURE}/state/yq.sha256")" = 600
grep -Fxq 'apt-get install -y --no-install-recommends --no-remove docker.io containerd runc' "${FIXTURE}/calls"
grep -Fxq 'systemctl enable --now docker' "${FIXTURE}/calls"
cp -- "${FIXTURE}/state/docker-packages" "${FIXTURE}/before-packages"
cp -- "${FIXTURE}/state/yq.sha256" "${FIXTURE}/before-yq"
run_command install
cmp -- "${FIXTURE}/before-packages" "${FIXTURE}/state/docker-packages"
cmp -- "${FIXTURE}/before-yq" "${FIXTURE}/state/yq.sha256"
test "$(grep -c '^apt-get install ' "${FIXTURE}/calls")" = 1
test "$(grep -c '^curl ' "${FIXTURE}/calls")" = 1

new_fixture reuse-existing
touch "${FIXTURE}/docker-present"
printf 'yq (https://github.com/mikefarah/yq/) version v4.44.1\n' >"${FIXTURE}/existing-yq"
run_command install
test ! -e "${FIXTURE}/state/docker-packages"
test ! -e "${FIXTURE}/state/yq.sha256"
test ! -e "${FIXTURE}/bin/yq"
if grep -Eq '^apt-get |^curl ' "${FIXTURE}/calls"; then fail 'existing dependencies were installed again'; fi

new_fixture mixed-existing-packages
printf '%s\n' containerd runc >>"${FIXTURE}/installed-packages"
printf 'yq (https://github.com/mikefarah/yq/) version v4.44.1\n' >"${FIXTURE}/existing-yq"
run_command install
diff -u <(printf 'docker.io\n') "${FIXTURE}/state/docker-packages"
run_command remove
grep -Fxq 'apt-get remove -y --no-auto-remove docker.io' "${FIXTURE}/calls"
grep -Fxq containerd "${FIXTURE}/installed-packages"
grep -Fxq runc "${FIXTURE}/installed-packages"
test -f "${FIXTURE}/existing-yq"

new_fixture incompatible-yq
printf 'yq 3.4.3\n' >"${FIXTURE}/existing-yq"
expect_failure 'incompatible yq accepted' install
grep -Fq 'Unsupported yq' "${FIXTURE}/err"
test ! -e "${FIXTURE}/state/docker-packages"
test ! -e "${FIXTURE}/bin/yq"
if grep -Eq '^apt-get |^curl ' "${FIXTURE}/calls"; then fail 'incompatible yq changed dependencies'; fi

new_fixture apt-install-failed
touch "${FIXTURE}/apt-fail"
if run_command install; then fail 'failed APT install reported success'; else test "$?" = 100; fi
test ! -e "${FIXTURE}/state/docker-packages"
test ! -e "${FIXTURE}/state/yq.sha256"
# A later manual installation must not be claimed by the unsuccessful command.
printf '%s\n' docker.io containerd runc >>"${FIXTURE}/installed-packages"
touch "${FIXTURE}/docker-present"
printf 'yq (https://github.com/mikefarah/yq/) version v4.44.1\n' >"${FIXTURE}/existing-yq"
rm -f -- "${FIXTURE}/apt-fail"
run_command install
test ! -e "${FIXTURE}/state/docker-packages"
run_command remove
grep -Fxq docker.io "${FIXTURE}/installed-packages"
if grep -q '^apt-get remove ' "${FIXTURE}/calls"; then fail 'failed install later removed manual Docker'; fi

new_fixture apt-install-partial
touch "${FIXTURE}/apt-partial-fail"
if run_command install; then fail 'partial APT install reported success'; else test "$?" = 100; fi
diff -u <(printf 'docker.io\n') "${FIXTURE}/state/docker-packages"
test ! -e "${FIXTURE}/state/yq.sha256"
printf '%s\n' containerd runc >>"${FIXTURE}/installed-packages"
run_command remove
grep -Fxq 'apt-get remove -y --no-auto-remove docker.io' "${FIXTURE}/calls"
grep -Fxq containerd "${FIXTURE}/installed-packages"
grep -Fxq runc "${FIXTURE}/installed-packages"

new_fixture bad-sha
touch "${FIXTURE}/docker-present" "${FIXTURE}/corrupt-download"
expect_failure 'unverified yq installed' install
test ! -e "${FIXTURE}/bin/yq"
test ! -e "${FIXTURE}/state/yq.sha256"

new_fixture remove-owned
own_docker
own_yq
printf 'nginx\n' >>"${FIXTURE}/installed-packages"
touch "${FIXTURE}/containerd-active"
run_command remove
grep -Fxq 'apt-get remove -y --no-auto-remove docker.io containerd runc' "${FIXTURE}/calls"
grep -Fxq nginx "${FIXTURE}/installed-packages"
grep -Fxq openssl "${FIXTURE}/installed-packages"
test ! -e "${FIXTURE}/bin/yq"
test ! -e "${FIXTURE}/state/docker-packages"
test ! -e "${FIXTURE}/state/yq.sha256"
test -f "${FIXTURE}/docker-data/database"

new_fixture invalid-owner
own_docker
printf 'nginx\n' >>"${FIXTURE}/state/docker-packages"
expect_failure 'unowned package accepted from record' remove
grep -Fq 'Invalid Docker package ownership record' "${FIXTURE}/err"
assert_no_remove

for scenario in apt-extra stopped-containers docker-offline helper-active helper-unit containerd-workload changed-yq symlink-yq invalid-yq-record; do
  new_fixture "${scenario}"
  own_docker
  own_yq
  case "${scenario}" in
  apt-extra) touch "${FIXTURE}/extra-apt-removal"; expected='APT would also remove unowned package openssl' ;;
  stopped-containers) printf 'stopped-container-id\n' >"${FIXTURE}/containers"; expected='including stopped containers' ;;
  docker-offline) touch "${FIXTURE}/docker-offline"; expected='Cannot verify Docker containers' ;;
  helper-active) touch "${FIXTURE}/helper-active"; expected='Remove the Node host maintenance service' ;;
  helper-unit) touch "${FIXTURE}/trojanpanelnext-host.service"; expected='Remove the Node host maintenance service' ;;
  containerd-workload) touch "${FIXTURE}/containerd-active" "${FIXTURE}/containerd-workload"; expected='containerd namespace k8s.io still has containers' ;;
  changed-yq) printf 'updated-by-user\n' >>"${FIXTURE}/bin/yq"; expected='Managed yq has changed' ;;
  symlink-yq) mv -- "${FIXTURE}/bin/yq" "${FIXTURE}/user-yq"; ln -s "${FIXTURE}/user-yq" "${FIXTURE}/bin/yq"; expected='Managed yq has been replaced' ;;
  invalid-yq-record) printf 'invalid-checksum\n' >"${FIXTURE}/state/yq.sha256"; expected='Invalid yq ownership record' ;;
  esac
  expect_failure "unsafe dependency removal accepted: ${scenario}" remove
  grep -Fq "${expected}" "${FIXTURE}/err"
  assert_no_remove
  test -f "${FIXTURE}/bin/yq"
  test -f "${FIXTURE}/state/yq.sha256"
done

new_fixture remove-only-yq
own_yq
touch "${FIXTURE}/docker-present" "${FIXTURE}/docker-offline" "${FIXTURE}/containerd-active"
printf 'unowned-container\n' >"${FIXTURE}/containers"
run_command remove
test ! -e "${FIXTURE}/bin/yq"
if grep -Eq '^apt-get |^docker |^ctr ' "${FIXTURE}/calls"; then fail 'yq-only ownership touched existing container software'; fi

new_fixture no-owner
touch "${FIXTURE}/docker-present" "${FIXTURE}/helper-active"
printf 'existing-container\n' >"${FIXTURE}/containers"
run_command remove
grep -Fq 'No dependencies owned by this command' "${FIXTURE}/out"
if grep -Eq '^apt-get |^docker |^systemctl |^ctr ' "${FIXTURE}/calls"; then fail 'unowned dependencies caused host operations'; fi

new_fixture metadata
touch "${FIXTURE}/deny-root"
for flag in --help -h help --version -V version; do
  run_command "${flag}"
done
expect_failure 'empty dependency action accepted'
for action in unknown --purge install-extra; do
  expect_failure 'invalid dependency action accepted' "${action}"
done
expect_failure 'extra dependency arguments accepted' install --force
expect_failure 'extra removal arguments accepted' remove extra
test ! -s "${FIXTURE}/calls"

test -z "$(find "${TMPDIR}" -mindepth 1 -print -quit)"
printf 'PASS dependency ownership, verified downloads, repeat installation, safe removal and root-free metadata\n'
