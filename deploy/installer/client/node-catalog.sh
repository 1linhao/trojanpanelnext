#!/usr/bin/env bash
set -Eeuo pipefail

client_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fail() { printf 'node catalog: %s\n' "$1" >&2; exit 2; }
config=""; node_key=""
while (($#)); do
  case "$1" in
  --config) [[ $# -ge 2 && -z "${config}" ]] || fail 'one --config FILE is required'; config="$2"; shift 2 ;;
  --node-key) [[ $# -ge 2 && -z "${node_key}" ]] || fail 'one --node-key KEY is required'; node_key="$2"; shift 2 ;;
  *) fail 'usage: node-catalog.sh --config FILE --node-key KEY' ;;
  esac
done
[[ -n "${config}" && "${node_key}" =~ ^[a-z][a-z0-9-]{0,62}$ ]] || fail 'valid --config and --node-key are required'

# The plan validator owns YAML shape and topology rules. This Interface emits
# only public registration fields; the unified YAML and passwords stay local.
bash "${client_dir}/topology.sh" --config "${config}" >/dev/null
export TP_CATALOG_NODE_KEY="${node_key}"
count="$(yq -r '[.nodes[] | select(.node_key == strenv(TP_CATALOG_NODE_KEY))] | length' "${config}")"
[[ "${count}" == 1 ]] || fail "node_key ${node_key} is not declared"
yq -o=json -I=0 '.nodes[] | select(.node_key == strenv(TP_CATALOG_NODE_KEY)) |
  {"node_key": .node_key, "host_id": .host, "name": .name, "domain": .domain,
   "public_ip": .public_ip, "grpc_port": .settings.grpc_port}' "${config}"
