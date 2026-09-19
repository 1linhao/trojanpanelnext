#!/usr/bin/env bash
set -Eeuo pipefail

fail() {
  printf 'package release assets: %s\n' "$1" >&2
  exit 1
}

assets_dir=""
output=""
while (($#)); do
  case "$1" in
  --assets-dir) assets_dir="${2:-}"; shift 2 ;;
  --output) output="${2:-}"; shift 2 ;;
  *) fail "unknown argument: $1" ;;
  esac
done
[[ -n "${assets_dir}" && -d "${assets_dir}" ]] || fail 'asset directory not found'
[[ -n "${output}" ]] || fail 'output archive is required'
[[ "${output}" == *.tar.gz ]] || fail 'output archive must end in .tar.gz'
[[ "$(realpath -m "${output}")" != "$(realpath -m "${assets_dir}")"/* ]] || fail 'output archive must be outside the asset directory'

"${assets_dir}/verify-assets.sh" --assets-dir "${assets_dir}" --config "${assets_dir}/config-web.yaml" >/dev/null
mkdir -p "$(dirname "${output}")"
tar -C "${assets_dir}" --sort=name --owner=0 --group=0 --numeric-owner -czf "${output}" .
printf 'Packaged release assets: %s\n' "${output}"
