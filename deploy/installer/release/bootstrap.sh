#!/usr/bin/env bash
set -Eeuo pipefail

ASSETS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
config=""
args=("$@")
for ((i = 0; i < ${#args[@]}; i++)); do
  if [[ "${args[$i]}" == --config && $((i + 1)) -lt ${#args[@]} ]]; then
    config="${args[$((i + 1))]}"
    break
  fi
done

if [[ -z "${config}" ]]; then
  printf 'bootstrap: --config is required\n' >&2
  exit 2
fi
if [[ "${config}" != /* ]]; then
  config="${PWD}/${config}"
fi

"${ASSETS_DIR}/verify-assets.sh" --assets-dir "${ASSETS_DIR}" --config "${config}"
exec "${ASSETS_DIR}/install.sh" "$@"
