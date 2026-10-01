#!/usr/bin/env bash
set -euo pipefail

SCRIPT_VERSION="1.0.2-rc.2"
TP_SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if [[ ! -f "${TP_SCRIPT_DIR}/common.sh" ]]; then
  printf 'Missing common.sh. Use tp.sh to download the command and its dependencies.\n' >&2
  exit 1
fi
# shellcheck source=scripts/deploy/common.sh
source "${TP_SCRIPT_DIR}/common.sh"
require_matching_version "${SCRIPT_VERSION}"

usage() {
  cat <<EOF
TrojanPanel Next configuration validation ${INSTALLER_VERSION}
Usage: $0 --config <file>
  --config <file>  YAML deployment configuration; purpose selects web or node
  -V, --version    Show version
  -h, --help       Show help
Requires mikefarah/yq v4. Does not check DNS, connectivity or service health.
EOF
}

main() {
  if handle_metadata "$@"; then return; fi
  parse_config_options validate "$@"
  validate_config
  echo_content green "Configuration is valid for ${TP_PURPOSE} purpose: ${TP_CONFIG_FILE}"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
