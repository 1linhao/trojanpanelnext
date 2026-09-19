#!/usr/bin/env bash
set -Eeuo pipefail

ASSETS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
config=""
config_count=0
args=("$@")
for ((i = 0; i < ${#args[@]}; i++)); do
  if [[ "${args[$i]}" == --config ]]; then
    if ((i + 1 >= ${#args[@]})); then
      printf 'bootstrap: --config requires a value\n' >&2
      exit 2
    fi
    config_count=$((config_count + 1))
    if ((config_count > 1)); then
      printf 'bootstrap: --config may be specified only once\n' >&2
      exit 2
    fi
    config="${args[$((i + 1))]}"
    i=$((i + 1))
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
