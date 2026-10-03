package service

const nodeDeploymentDependencyScript = `#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

fail() { printf '%s\n' "$1" >&2; exit 1; }
(($# == 0)) || fail 'Usage: bash ./tpnext/install-dependencies.sh (the release version is fixed by this bundle).'
for dependency in bash curl grep id mktemp chmod rm stat install sha256sum mkdir cp ln readlink dirname cat flock apt-get dpkg dpkg-query systemctl; do
  command -v "$dependency" >/dev/null 2>&1 || fail "Missing bootstrap dependency: $dependency. Install the documented minimum packages first."
done
[[ "$(id -u)" == 0 ]] || fail 'Run this script as root on the Node host.'
[[ -f /etc/os-release ]] || fail 'Cannot identify the Node host operating system.'
ID=''
VERSION_ID=''
source /etc/os-release
case "${ID}:${VERSION_ID}" in
  debian:12 | debian:13 | ubuntu:22.04 | ubuntu:24.04) ;;
  *) fail 'Automatic dependencies support Debian 12/13 and Ubuntu 22.04/24.04.' ;;
esac
case "$(dpkg --print-architecture)" in
  amd64 | arm64) ;;
  *) fail 'Automatic dependencies require amd64 or arm64.' ;;
esac
SYSTEMD_STATE="$(systemctl is-system-running 2>/dev/null || true)"
case "$SYSTEMD_STATE" in
  running | degraded) ;;
  *) fail 'A running systemd host is required.' ;;
esac

TEMP_DIR="$(mktemp -d)"
cleanup() { rm -rf -- "$TEMP_DIR"; }
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
ENTRY="${TEMP_DIR}/tp.sh"
curl --fail --location --silent --show-error --proto '=https' --proto-redir '=https' \
  --retry 2 --retry-max-time 180 --connect-timeout 10 --max-time 60 --max-filesize 5242880 \
  'https://raw.githubusercontent.com/1linhao/trojanpanelnext/v@@VERSION@@/scripts/tp.sh' -o "$ENTRY" || fail 'Cannot download the release script entrypoint.'
chmod 600 -- "$ENTRY"
bash -n "$ENTRY" || fail 'Invalid release script entrypoint.'
ENTRY_VERSION="$(bash "$ENTRY" --entry-version)" || fail 'Cannot verify the release script entrypoint version.'
[[ "$ENTRY_VERSION" == '@@VERSION@@' ]] || fail 'Release script entrypoint version mismatch.'
bash "$ENTRY" --version 'v@@VERSION@@' deps install
`

const nodeDeploymentInstallScript = `#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

fail() { printf '%s\n' "$1" >&2; exit 1; }
[[ "$(id -u)" == 0 ]] || fail 'Run this script as root on the Node host.'
for dependency in bash curl yq docker openssl tar od sha256sum find seq awk realpath systemctl readlink dirname mktemp chmod rm mkdir cp cmp ln grep; do
  command -v "$dependency" >/dev/null 2>&1 || fail "Missing dependency: $dependency. Run the documented deps install command first."
done
yq_version="$(yq --version 2>/dev/null)" || fail 'Cannot run yq.'
[[ "$yq_version" == *mikefarah/yq* && "$yq_version" == *'version v4.'* ]] || fail 'Install mikefarah/yq v4 before continuing.'
docker info >/dev/null 2>&1 || fail 'Docker must be running before installation.'

PACKAGE_DIR="$(dirname -- "$(readlink -f -- "${BASH_SOURCE[0]}")")"
CONFIG="${PACKAGE_DIR}/node.yaml"
CA_SOURCE="${PACKAGE_DIR}/client-ca.crt"
[[ -f "$CONFIG" && ! -L "$CONFIG" && -f "$CA_SOURCE" && ! -L "$CA_SOURCE" ]] || fail 'The bundle must contain regular node.yaml and client-ca.crt files.'
chmod 600 -- "$CONFIG"
TEMP_DIR="$(mktemp -d)"
cleanup() { rm -rf -- "$TEMP_DIR"; }
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
ENTRY="${TEMP_DIR}/tp.sh"
curl --fail --location --silent --show-error --proto '=https' --proto-redir '=https' \
  --retry 2 --retry-max-time 180 --connect-timeout 10 --max-time 60 --max-filesize 5242880 \
  'https://raw.githubusercontent.com/1linhao/trojanpanelnext/v@@VERSION@@/scripts/tp.sh' -o "$ENTRY" || fail 'Cannot download the release script entrypoint.'
chmod 600 -- "$ENTRY"
bash -n "$ENTRY" || fail 'Invalid release script entrypoint.'
[[ "$(bash "$ENTRY" --entry-version)" == '@@VERSION@@' ]] || fail 'Release script entrypoint version mismatch.'
# The current release's validator checks purpose, version, images, paths and schema.
bash "$ENTRY" --version 'v@@VERSION@@' validate --config "$CONFIG"
[[ "$(yq -r '.trojanpanelnext.purpose' "$CONFIG")" == node && "$(yq -r '.trojanpanelnext.release' "$CONFIG")" == '@@VERSION@@' ]] || fail 'The configuration must match this Node deployment bundle.'
# The installer validates and backs up retained trust before importing this Web CA.
bash "$ENTRY" --version 'v@@VERSION@@' install --config "$CONFIG" --client-ca "$CA_SOURCE"
`
