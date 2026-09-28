#!/usr/bin/env bash
set -Eeuo pipefail

installer_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf -- "${work}"' EXIT
fail() { printf 'FAIL Node revocation remove: %s\n' "$1" >&2; exit 1; }

(cd "${installer_dir}/nodebundle" && go build -trimpath -o "${work}/node-bundle" .)
(cd "${installer_dir}/nodebundle" && go build -trimpath -o "${work}/receipt-fixture" ./testing/receipt_fixture.go)

data="${work}/data"
mkdir -m 0700 -p "${data}/trojanpanelnext-installer" "${data}/trojan-panel-core/pki"
printf 'trojanpanelnext-data-root-v1\n' >"${data}/.trojanpanelnext-data-root"
chmod 0600 "${data}/.trojanpanelnext-data-root"
cat >"${data}/trojanpanelnext-installer/node.state" <<EOF
schema_version=1
mode=node
asset_version=development
tp_data=${data}
domain=node.example.com
node_identity_id=11111111-2222-4333-8444-555555555555
node_server_id=42
node_identity_generation=7
EOF
chmod 0600 "${data}/trojanpanelnext-installer/node.state"
"${work}/receipt-fixture" --public "${data}/trojan-panel-core/pki/revocation-public-key.txt" \
  --receipt "${work}/valid.receipt"
"${work}/receipt-fixture" --receipt "${work}/other-identity.receipt" \
  --identity 22222222-2222-4333-8444-555555555555
"${work}/receipt-fixture" --receipt "${work}/other-server.receipt" --server 43
"${work}/receipt-fixture" --receipt "${work}/old-generation.receipt" --through 6
"${work}/receipt-fixture" --receipt "${work}/other-issuer.receipt" --seed 8
cp "${work}/valid.receipt" "${work}/tampered.receipt"
printf 'x' >>"${work}/tampered.receipt"
chmod 0600 "${work}/tampered.receipt"
printf 'trojanpanelnext:\n  deployment_mode: node\n' >"${work}/node.yaml"
printf 'trojanpanelnext:\n  deployment_mode: combined\n  web_hostname: forged.example.com\n' >"${work}/forged-combined.yaml"
printf 'entry must remain intact\n' >"${work}/entry.json"

run_main() {
  local config="$1" receipt="$2"
  shift 2
  TP_DATA="${data}" TP_TEST_DATA_ROOT=1 NODE_BUNDLE_HELPER="${work}/node-bundle" \
    TRACE_FILE="${work}/trace" bash -c '
      set -Eeuo pipefail
      source "$1"
      verify_release_assets_before_host_change() { :; }
      require_supported_asset_architecture() { :; }
      require_root() { :; }
      preflight_install_dependencies() { :; }
      yq() {
        if grep -q "deployment_mode: combined" "$3"; then printf "combined\n"; else printf "node\n"; fi
      }
      prepare_secure_config() { printf "snapshot\n" >>"${TRACE_FILE}"; TP_CONFIG_READ_FILE="$1"; }
      load_config() {
        if grep -q "deployment_mode: combined" "$2"; then TP_DEPLOYMENT_MODE=combined; else TP_DEPLOYMENT_MODE=node; fi
        TLS_MODE=external
        NODE_IDENTITY_ID=11111111-2222-4333-8444-555555555555
        NODE_SERVER_ID=42
        NODE_IDENTITY_GENERATION=7
        TP_NODE_DOMAIN=node.example.com
      }
      translate_test_data_paths() { :; }
      validate_config() { :; }
      validate_entry_spec_binding() { :; }
      retain_verified_release_assets() { printf "retain\n" >>"${TRACE_FILE}"; }
      entry_controller() { printf "entry:%s\n" "$1" >>"${TRACE_FILE}"; }
      remove_node() { printf "remove-node\n" >>"${TRACE_FILE}"; }
      main remove --mode node --config "$2" --entry-spec "$3" "${@:4}"
    ' fixture "${installer_dir}/install.sh" "${config}" "${work}/entry.json" "$@" \
    >"${work}/run.out" 2>"${work}/run.err"
}

reject_before_effects() {
  local label="$1" config="$2" receipt="$3"
  : >"${work}/trace"
  if [[ "${receipt}" == NONE ]]; then
    if run_main "${config}" "${receipt}"; then fail "${label} unexpectedly succeeded"; fi
  else
    if run_main "${config}" "${receipt}" --receipt-file "${receipt}"; then fail "${label} unexpectedly succeeded"; fi
  fi
  [[ ! -s "${work}/trace" ]] || fail "${label} changed local state before proof validation"
  grep -Fxq 'entry must remain intact' "${work}/entry.json" || fail "${label} changed Entry input"
}

reject_before_effects missing "${work}/node.yaml" NONE
cp "${data}/trojanpanelnext-installer/node.state" "${data}/trojanpanelnext-installer/combined.state"
reject_before_effects conflicting-roles "${work}/node.yaml" "${work}/valid.receipt"
rm -- "${data}/trojanpanelnext-installer/combined.state"
reject_before_effects forged-combined "${work}/forged-combined.yaml" NONE
reject_before_effects forged-combined-valid-proof "${work}/forged-combined.yaml" "${work}/valid.receipt"
reject_before_effects missing-file "${work}/node.yaml" "${work}/absent.receipt"
if [[ -n "${TP_NODE_REVOCATION_TIMEOUT_RECEIPT:-}" ]]; then
  reject_before_effects uncertain-web-result "${work}/node.yaml" "${TP_NODE_REVOCATION_TIMEOUT_RECEIPT}"
fi
for label in other-identity other-server old-generation other-issuer tampered; do
  reject_before_effects "${label}" "${work}/node.yaml" "${work}/${label}.receipt"
done

mv "${data}/trojan-panel-core/pki/revocation-public-key.txt" "${work}/held-public-key"
reject_before_effects missing-pin "${work}/node.yaml" "${work}/valid.receipt"
mv "${work}/held-public-key" "${data}/trojan-panel-core/pki/revocation-public-key.txt"

: >"${work}/trace"
run_main "${work}/node.yaml" "${work}/valid.receipt" --receipt-file "${work}/valid.receipt" ||
  fail 'valid revoked receipt did not allow local removal'
test "$(cat "${work}/trace")" = $'snapshot\nretain\nentry:remove\nremove-node' ||
  fail "valid receipt did not preserve removal order: $(cat "${work}/trace")"

: >"${work}/trace"
run_main "${work}/node.yaml" "${work}/valid.receipt" --receipt-file "${work}/valid.receipt" ||
  fail 'replayed terminal receipt did not allow safe retry'
grep -Fxq remove-node "${work}/trace" || fail 'replay did not reach local removal'

printf 'PASS independent Node receipt gate precedes every main-path side effect\n'
