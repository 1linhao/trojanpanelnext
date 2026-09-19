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
probe_trace="${work}/probe.trace"
command_state="${work}/command-state"
mkdir -p "${command_state}"

debian_release="${work}/debian-12"
ubuntu_release="${work}/ubuntu-24.04"
printf 'ID=debian\nVERSION_ID="12"\n' >"${debian_release}"
printf 'ID=ubuntu\nVERSION_ID="24.04"\n' >"${ubuntu_release}"

development_config="${work}/development-invalid-after-preflight.yaml"
sed 's/^  hostname:.*/  hostname: ""/' \
  "${INSTALLER_DIR}/examples/web.yaml" >"${development_config}"

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
release_config="${work}/release-invalid-after-preflight.yaml"
sed 's/^  hostname:.*/  hostname: ""/' "${bundle}/config-web.yaml" >"${release_config}"

fake_command_is_missing() {
  if [[ -e "${TP_FAKE_COMMAND_STATE}/$1" ]]; then
    return 1
  fi
  case ",${TP_FAKE_MISSING_COMMANDS:-}," in
  *",$1,"*) return 0 ;;
  *) return 1 ;;
  esac
}

command() {
  if [[ "${1:-}" == -v && $# -ge 2 ]]; then
    printf 'probe %s\n' "$2" >>"${TP_DEP_PROBE_TRACE}"
    if [[ "$2" == "${TP_FAKE_DELAYED_MISSING_COMMAND:-}" ]]; then
      printf 'probe\n' >>"${TP_FAKE_COMMAND_STATE}/delayed-missing-probes"
      if [[ "$(wc -l <"${TP_FAKE_COMMAND_STATE}/delayed-missing-probes")" -le "${TP_FAKE_PRESENT_PROBES_BEFORE_MISSING:-0}" ]]; then
        builtin command "$@"
        return
      fi
    fi
    if [[ -e "${TP_FAKE_COMMAND_STATE}/$2" ]]; then
      printf '%s/%s\n' "${TP_FAKE_COMMAND_STATE}" "$2"
      return
    fi
    if fake_command_is_missing "$2"; then
      return 1
    fi
  fi
  builtin command "$@"
}

mark_fake_command_available() {
  local command_name="$1"
  case ",${TP_FAKE_INSTALLABLE_COMMANDS:-}," in
  *",${command_name},"*) : >"${TP_FAKE_COMMAND_STATE}/${command_name}" ;;
  esac
}

id() {
  if [[ "${1:-}" == -u ]]; then
    printf '0\n'
    return
  fi
  /usr/bin/id "$@"
}

uname() {
  if [[ "${1:-}" == -m ]]; then
    printf '%s\n' "${TP_FAKE_ARCH}"
    return
  fi
  /usr/bin/uname "$@"
}

apt-get() {
  printf 'apt-get %s\n' "$*" >>"${TP_DEP_TRACE}"
  if [[ "${1:-}" == install ]]; then
    local argument
    for argument in "$@"; do
      case "${argument}" in
      age | curl | tar | openssl | jq) mark_fake_command_available "${argument}" ;;
      coreutils)
        mark_fake_command_available od
        mark_fake_command_available sha256sum
        mark_fake_command_available install
        ;;
      esac
    done
  fi
}

curl() {
  printf 'curl %s\n' "$*" >>"${TP_DEP_TRACE}"
  local argument output="" docker_installer=0
  for argument in "$@"; do
    [[ "${argument}" == https://get.docker.com ]] && docker_installer=1
    if [[ -n "${output}" ]]; then
      printf '#!/bin/sh\nexit 0\n' >"${argument}"
      return
    fi
    [[ "${argument}" == -o ]] && output=1
  done
  [[ "${docker_installer}" == 1 ]] && mark_fake_command_available docker
  printf 'exit 0\n'
}

systemctl() {
  printf 'systemctl %s\n' "$*" >>"${TP_DEP_TRACE}"
}

sha256sum() {
  if [[ "${1:-}" == -c && "${2:-}" == - ]]; then
    return
  fi
  /usr/bin/sha256sum "$@"
}

install() {
  printf 'install %s\n' "$*" >>"${TP_DEP_TRACE}"
  if [[ "${*: -1}" == /usr/local/bin/yq ]]; then
    mark_fake_command_available yq
  fi
}

yq() {
  "${TP_FAKE_YQ_READER}" "$@"
}

jq() {
  printf 'jq %s\n' "$*" >>"${TP_DEP_TRACE}"
  return 97
}

export -f fake_command_is_missing command mark_fake_command_available id uname apt-get curl systemctl sha256sum install yq jq

run_install_cli() {
  local entrypoint="$1"
  local config="$2"
  local os_release="$3"
  local architecture="$4"
  local install_deps="$5"
  local missing_commands="$6"
  local installable_commands="$7"
  shift 7
  find "${command_state}" -type f -delete
  : >"${probe_trace}"

  TP_DEP_TRACE="${trace}" \
    TP_DEP_PROBE_TRACE="${probe_trace}" \
    TP_FAKE_ARCH="${architecture}" \
    TP_FAKE_COMMAND_STATE="${command_state}" \
    TP_FAKE_DELAYED_MISSING_COMMAND="${TP_FAKE_DELAYED_MISSING_COMMAND:-}" \
    TP_FAKE_PRESENT_PROBES_BEFORE_MISSING="${TP_FAKE_PRESENT_PROBES_BEFORE_MISSING:-0}" \
    TP_FAKE_INSTALLABLE_COMMANDS="${installable_commands}" \
    TP_FAKE_MISSING_COMMANDS="${missing_commands}" \
    TP_FAKE_YQ_READER="${FAKE_YQ_READER}" \
    TP_INSTALL_DEPS="${install_deps}" \
    TP_OS_RELEASE_FILE="${os_release}" \
    TP_DATA="${work}/data" \
    "${entrypoint}" install --mode web --config "${config}" "$@"
}

assert_default_installs_declared_dependencies() {
  local entrypoint="$1"
  local config="$2"
  local label="$3"
  local output="${work}/${label}-default.out"

  : >"${trace}"
  if run_install_cli "${entrypoint}" "${config}" "${debian_release}" x86_64 1 \
    docker,age,yq,jq docker,age,yq >"${output}" 2>&1; then
    fail "${label} unexpectedly passed the intentionally invalid post-preflight config"
  fi
  if [[ "${label}" != development ]]; then
    grep -Fq 'Release assets are valid for web deployment mode' "${output}" ||
      fail "${label} did not verify its jq-free release bundle"
  fi
  grep -Fq 'TP_WEB_DOMAIN is required' "${output}" ||
    fail "${label} did not continue beyond dependency installation"
  grep -Fxq 'apt-get update' "${trace}" ||
    fail "${label} did not refresh Debian package metadata"
  grep -Fxq 'apt-get install -y age' "${trace}" ||
    fail "${label} did not install the missing age package"
  grep -Fq 'curl -fsSL https://get.docker.com' "${trace}" ||
    fail "${label} did not invoke the Docker installer"
  grep -Fq "curl -fsSL https://github.com/mikefarah/yq/releases/download/v4.53.6/yq_linux_amd64 -o" "${trace}" ||
    fail "${label} did not download the pinned yq binary"
  grep -Eq '^install -m 0755 .+ /usr/local/bin/yq$' "${trace}" ||
    fail "${label} did not install the pinned yq binary"
  if grep -Eq '^apt-get .* (curl|tar|coreutils|openssl|jq)( |$)' "${trace}"; then
    fail "${label} installed an undeclared or unnecessary package"
  fi
  test "$(grep -Fc 'probe age' "${probe_trace}")" -ge 2 ||
    fail "${label} did not probe age again after installation"
  printf 'TRACE entrypoint=%s mode=default packages=age yq=pinned docker-installer=1 recheck=passed jq-host=absent\n' "${label}"
}

assert_successful_installer_without_command_is_blocked() {
  local entrypoint="$1"
  local config="$2"
  local label="$3"
  local output="${work}/${label}-post-install-missing.out"

  : >"${trace}"
  if run_install_cli "${entrypoint}" "${config}" "${debian_release}" x86_64 1 \
    docker,age,yq,jq docker,yq >"${output}" 2>&1; then
    fail "${label} unexpectedly passed when apt-get returned success without providing age"
  fi
  grep -Fq 'Missing required Debian 12 dependencies:' "${output}" ||
    fail "${label} did not aggregate dependencies still missing after installation"
  grep -Fq -- '- age: apt-get install -y age' "${output}" ||
    fail "${label} did not report age after the successful installer left it unavailable"
  if grep -Fq -- '- docker:' "${output}" || grep -Fq -- '- yq:' "${output}"; then
    fail "${label} reported a dependency that became available after installation"
  fi
  if grep -Fq 'TP_WEB_DOMAIN is required' "${output}"; then
    fail "${label} entered config loading with age still unavailable"
  fi
  test "$(grep -Fc 'probe age' "${probe_trace}")" -ge 2 ||
    fail "${label} did not recheck age after apt-get returned success"
  printf 'TRACE entrypoint=%s mode=post-install-missing dependency=age installer-exit=0 recheck=blocked config-loaded=0\n' "${label}"
}

assert_package_prerequisites_are_rechecked_before_special_installers() {
  local entrypoint="$1"
  local config="$2"
  local label="$3"
  local coreutils_command="$4"
  local delayed_missing_command=""
  local present_probes_before_missing=0
  local output="${work}/${label}-${coreutils_command}-package-prerequisites-missing.out"

  if [[ "${label}" != development && "${coreutils_command}" == sha256sum ]]; then
    : >"${trace}"
    if run_install_cli "${entrypoint}" "${config}" "${debian_release}" x86_64 1 \
      "docker,curl,sha256sum,yq,jq" '' >"${output}" 2>&1; then
      fail "${label} unexpectedly passed without the trusted release checksum tool"
    fi
    grep -Fq 'sha256sum is required' "${output}" ||
      fail "${label} did not reject the release before executing unverified assets"
    test ! -s "${trace}" ||
      fail "${label} changed the host before its release checksum prerequisite passed"
    if grep -Fq 'TP_WEB_DOMAIN is required' "${output}"; then
      fail "${label} entered config loading without a trusted release checksum tool"
    fi
    printf 'TRACE entrypoint=%s mode=release-trust-prerequisite missing=sha256sum host-changes=0 config-loaded=0\n' "${label}"
    return
  fi
  : >"${trace}"
  if TP_FAKE_DELAYED_MISSING_COMMAND="${delayed_missing_command}" \
    TP_FAKE_PRESENT_PROBES_BEFORE_MISSING="${present_probes_before_missing}" \
    run_install_cli "${entrypoint}" "${config}" "${debian_release}" x86_64 1 \
    "docker,curl,${coreutils_command},yq,jq" '' >"${output}" 2>&1; then
    fail "${label} unexpectedly passed when apt-get left curl and ${coreutils_command} unavailable"
  fi
  grep -Fq 'Missing required Debian 12 dependencies:' "${output}" ||
    fail "${label} did not aggregate package prerequisites still missing after apt-get"
  grep -Fq -- '- curl: apt-get install -y curl' "${output}" ||
    fail "${label} omitted curl after apt-get returned success"
  test "$(grep -Fc -- '- coreutils: apt-get install -y coreutils' "${output}")" = 1 ||
    fail "${label} omitted or duplicated coreutils after apt-get returned success"
  grep -Eq '^apt-get install -y .*curl.*coreutils' "${trace}" ||
    fail "${label} did not ask apt-get for curl and coreutils"
  if grep -Fq 'curl -fsSL https://get.docker.com' "${trace}" ||
    grep -Fq 'github.com/mikefarah/yq' "${trace}" ||
    grep -Eq '^install -m 0755 .+ /usr/local/bin/yq$' "${trace}"; then
    fail "${label} entered a Docker/yq installer before package prerequisites passed recheck"
  fi
  if grep -Fq 'TP_WEB_DOMAIN is required' "${output}"; then
    fail "${label} entered config loading with package prerequisites unavailable"
  fi
  for dependency_command in curl "${coreutils_command}"; do
    test "$(grep -Fc "probe ${dependency_command}" "${probe_trace}")" -ge 2 ||
      fail "${label} did not recheck ${dependency_command} after apt-get returned success"
  done
  printf 'TRACE entrypoint=%s mode=package-prerequisites-missing dependencies=curl,%s apt-exit=0 aggregate=1 special-installers=0 config-loaded=0\n' "${label}" "${coreutils_command}"
}

assert_disabled_reports_all_without_installing() {
  local entrypoint="$1"
  local config="$2"
  local label="$3"
  local output="${work}/${label}-disabled.out"

  : >"${trace}"
  if run_install_cli "${entrypoint}" "${config}" "${debian_release}" x86_64 0 \
    docker,age,curl,tar,openssl,yq,jq '' --entry-spec /synthetic/entry-spec.json \
    >"${output}" 2>&1; then
    fail "${label} unexpectedly passed with dependency installation disabled"
  fi
  for dependency in docker age curl tar openssl yq jq; do
    grep -Fq -- "- ${dependency}:" "${output}" ||
      fail "${label} omitted ${dependency} from the aggregated report"
  done
  grep -Fq 'apt-get install -y age' "${output}" ||
    fail "${label} omitted Debian 12 package advice"
  test ! -s "${trace}" || fail "${label} invoked a host command with TP_INSTALL_DEPS=0"
  printf 'TRACE entrypoint=%s mode=disabled missing=7 installer-calls=0 jq-host=absent\n' "${label}"
}

assert_platform_rejected_before_host_change() {
  local os_release="$1"
  local architecture="$2"
  local label="$3"
  local output="${work}/${label}.out"

  : >"${trace}"
  if run_install_cli "${bundle}/install.sh" "${release_config}" "${os_release}" \
    "${architecture}" 1 docker,age,jq '' >"${output}" 2>&1; then
    fail "${label} unexpectedly accepted an unsupported platform"
  fi
  grep -Fq 'Debian 12 x86_64 only' "${output}" ||
    fail "${label} omitted the supported platform"
  test ! -s "${trace}" || fail "${label} crossed the host-change boundary"
  printf 'TRACE platform=%s rejected=1 host-changes=0\n' "${label}"
}

assert_coreutils_is_aggregated_once() {
  local output="${work}/coreutils-disabled.out"
  : >"${trace}"
  if run_install_cli "${INSTALLER}" "${development_config}" "${debian_release}" \
    x86_64 0 od,sha256sum,install '' >"${output}" 2>&1; then
    fail 'development installer unexpectedly passed without coreutils commands'
  fi
  test "$(grep -Fc -- '- coreutils: apt-get install -y coreutils' "${output}")" = 1 ||
    fail 'coreutils advice was missing or duplicated'
  test ! -s "${trace}" || fail 'coreutils aggregation invoked an installer'
  printf 'TRACE dependency=coreutils missing-commands=3 report-lines=1 installer-calls=0\n'
}

assert_default_installs_declared_dependencies "${INSTALLER}" "${development_config}" development
assert_default_installs_declared_dependencies "${bundle}/install.sh" "${release_config}" release-direct
assert_default_installs_declared_dependencies "${bundle}/bootstrap.sh" "${release_config}" release-bootstrap

assert_successful_installer_without_command_is_blocked "${INSTALLER}" "${development_config}" development
assert_successful_installer_without_command_is_blocked "${bundle}/install.sh" "${release_config}" release-direct
assert_successful_installer_without_command_is_blocked "${bundle}/bootstrap.sh" "${release_config}" release-bootstrap

for coreutils_command in sha256sum install; do
  assert_package_prerequisites_are_rechecked_before_special_installers "${INSTALLER}" "${development_config}" development "${coreutils_command}"
  assert_package_prerequisites_are_rechecked_before_special_installers "${bundle}/install.sh" "${release_config}" release-direct "${coreutils_command}"
  assert_package_prerequisites_are_rechecked_before_special_installers "${bundle}/bootstrap.sh" "${release_config}" release-bootstrap "${coreutils_command}"
done

assert_disabled_reports_all_without_installing "${INSTALLER}" "${development_config}" development
assert_disabled_reports_all_without_installing "${bundle}/install.sh" "${release_config}" release-direct
assert_disabled_reports_all_without_installing "${bundle}/bootstrap.sh" "${release_config}" release-bootstrap
assert_coreutils_is_aggregated_once

assert_platform_rejected_before_host_change "${ubuntu_release}" x86_64 wrong-os
assert_platform_rejected_before_host_change "${debian_release}" aarch64 wrong-architecture

printf 'PASS Debian 12 dependency preflight contract\n'
