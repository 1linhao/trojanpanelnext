#!/usr/bin/env bash
set -Eeuo pipefail

ASSETS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
config=""
config_count=0
bundle=""
bundle_count=0
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
  elif [[ "${args[$i]}" == --bundle ]]; then
    if ((i + 1 >= ${#args[@]})); then
      printf 'bootstrap: --bundle requires a value\n' >&2
      exit 2
    fi
    bundle_count=$((bundle_count + 1))
    if ((bundle_count > 1)); then
      printf 'bootstrap: --bundle may be specified only once\n' >&2
      exit 2
    fi
    bundle="${args[$((i + 1))]}"
    i=$((i + 1))
  fi
done

if [[ -n "${config}" && -n "${bundle}" ]] || [[ -z "${config}" && -z "${bundle}" ]]; then
  printf 'bootstrap: exactly one of --config or --bundle is required\n' >&2
  exit 2
fi
if [[ -n "${config}" && "${config}" != /* ]]; then
  config="${PWD}/${config}"
fi

if [[ -n "${config}" ]]; then
  "${ASSETS_DIR}/verify-assets.sh" --assets-dir "${ASSETS_DIR}" --config "${config}"
else
  "${ASSETS_DIR}/verify-assets.sh" --assets-dir "${ASSETS_DIR}" --assets-only
fi
exec "${ASSETS_DIR}/install.sh" "$@"
