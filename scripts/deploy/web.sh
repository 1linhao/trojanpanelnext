#!/usr/bin/env bash
set -euo pipefail
SCRIPT_VERSION="1.0.2-rc.13"
TP_SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/deploy/common.sh
source "${TP_SCRIPT_DIR}/common.sh"
require_matching_version "${SCRIPT_VERSION}"
# shellcheck source=scripts/deploy/quick.sh
source "${TP_SCRIPT_DIR}/quick.sh"
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then quick_deploy web "$@"; fi
