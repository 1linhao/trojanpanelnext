#!/usr/bin/env bash
set -Eeuo pipefail

[[ "${1:-}" == provider ]]
action="${2:-}"
[[ "${3:-}" == --spec && -r "${4:-}" ]]
case "${action}" in
probe | prepare | activate | verify | rollback | remove) ;;
*) exit 2 ;;
esac
if [[ -n "${EXTERNAL_DRIVER_TRACE:-}" ]]; then
  printf '%s\n' "${action}" >>"${EXTERNAL_DRIVER_TRACE}"
fi
if [[ "${EXTERNAL_DRIVER_FAIL_AT:-}" == "${action}" ]]; then
  exit "${EXTERNAL_DRIVER_EXIT_CODE:-42}"
fi
deployment_id="$(jq -r '.deployment_id' "$4")"
case "${action}" in
probe) status=ok ;;
prepare) status=prepared ;;
activate) status=active ;;
verify) status=healthy ;;
rollback) status=rolled_back ;;
remove) status=removed ;;
esac
output_action="${EXTERNAL_DRIVER_ACTION_OVERRIDE:-${action}}"
output_deployment_id="${EXTERNAL_DRIVER_DEPLOYMENT_OVERRIDE:-${deployment_id}}"
output_status="${EXTERNAL_DRIVER_STATUS_OVERRIDE:-${status}}"
output_resources="${EXTERNAL_DRIVER_RESOURCES_JSON:-[]}"
jq -cn \
  --arg action "${output_action}" \
  --arg deployment_id "${output_deployment_id}" \
  --arg status "${output_status}" \
  --argjson resources "${output_resources}" '
  {
    schema_version: 1,
    driver_protocol_version: 1,
    provider: "external",
    action: $action,
    deployment_id: $deployment_id,
    status: $status,
    capabilities: [
      "certificate.material",
      "certificate.renew",
      "certificate.notify",
      "lifecycle.prepare",
      "lifecycle.rollback"
    ],
    resources: $resources,
    listeners: []
  }'
