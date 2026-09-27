#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  printf 'Usage: %s [--tag v0.1.0-rc.3] [--sha256 HEX] [--work-dir DIR]\n' "$0"
}

tag=v0.1.0-rc.3
sha256=fe4e2b297756bf3a58db31f69636dd1ca8034196ef14e1184db14c4f8362d668
work_dir=""
sha_explicit=0
while (($#)); do
  case "$1" in
  --tag) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; tag="$2"; shift 2 ;;
  --sha256) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; sha256="$2"; sha_explicit=1; shift 2 ;;
  --work-dir) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; work_dir="$2"; shift 2 ;;
  -h | --help) usage; exit 0 ;;
  *) usage >&2; exit 2 ;;
  esac
done

[[ "${tag}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9.]+)?$ ]] || {
  printf 'Invalid release tag: %s\n' "${tag}" >&2
  exit 2
}
if [[ "${tag}" != v0.1.0-rc.3 && "${sha_explicit}" != 1 ]]; then
  printf 'A different release tag requires its published --sha256 value\n' >&2
  exit 2
fi
[[ "${sha256}" =~ ^[0-9a-f]{64}$ ]] || {
  printf 'SHA-256 must be 64 lowercase hexadecimal characters\n' >&2
  exit 2
}
for dependency in curl tar mkdir chmod cp; do
  command -v "${dependency}" >/dev/null 2>&1 || {
    printf 'Missing local command: %s\n' "${dependency}" >&2
    exit 1
  }
done
if command -v sha256sum >/dev/null 2>&1; then
  checksum_tool=sha256sum
elif command -v shasum >/dev/null 2>&1; then
  checksum_tool=shasum
else
  printf 'Missing local SHA-256 tool: install sha256sum or shasum\n' >&2
  exit 1
fi

work_dir="${work_dir:-${PWD}/trojanpanelnext-${tag#v}}"
archive="trojanpanelnext-installer-${tag#v}.tar.gz"
for directory in "${work_dir}" "${work_dir}/assets" "${work_dir}/config"; do
  if [[ -L "${directory}" ]]; then
    printf 'Work directory contains a symlink: %s\n' "${directory}" >&2
    exit 1
  fi
done
umask 077
mkdir -p "${work_dir}/assets" "${work_dir}/config"
chmod 700 "${work_dir}" "${work_dir}/config"
curl -fL --retry 3 "https://github.com/1linhao/trojanpanelnext/releases/download/${tag}/${archive}" -o "${work_dir}/${archive}"

check_sha256() {
  if [[ "${checksum_tool}" == sha256sum ]]; then
    sha256sum -c "${1}"
  else
    shasum -a 256 -c "${1}"
  fi
}
printf '%s  %s\n' "${sha256}" "${work_dir}/${archive}" | check_sha256 -
tar -xzf "${work_dir}/${archive}" -C "${work_dir}/assets"
(cd "${work_dir}/assets" && check_sha256 SHA256SUMS)

for mode in web node combined; do
  if [[ -L "${work_dir}/config/${mode}.yaml" ]]; then
    printf 'Configuration path is a symlink: %s\n' "${work_dir}/config/${mode}.yaml" >&2
    exit 1
  fi
  if [[ ! -e "${work_dir}/config/${mode}.yaml" ]]; then
    cp "${work_dir}/assets/config-${mode}.yaml" "${work_dir}/config/${mode}.yaml"
    chmod 600 "${work_dir}/config/${mode}.yaml"
  fi
done
printf 'Verified release: %s\nWork directory: %s\nEdit configuration copies in: %s/config\n' \
  "${tag}" "${work_dir}" "${work_dir}"
