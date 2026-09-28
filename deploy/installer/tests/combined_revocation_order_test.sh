#!/usr/bin/env bash
set -Eeuo pipefail

installer="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/install.sh"
work="$(mktemp -d)"
trap 'rm -rf -- "${work}"' EXIT
data="${work}/data"
mkdir -m 0700 -p "${data}/trojanpanelnext-installer" "${data}/trojanpanelnext-entry/state"
printf 'schema_version=1\nmode=combined\ntp_data=%s\n' "${data}" >"${data}/trojanpanelnext-installer/combined.state"
chmod 0600 "${data}/trojanpanelnext-installer/combined.state"
printf '{"active_roles":["web","node"],"desired_revision":1}\n' >"${data}/trojanpanelnext-entry/state/trojanpanelnext-combined-entry.json"
printf 'Entry unchanged\n' >"${work}/entry"

run_case() {
  TP_DATA="${data}" TP_TEST_TRACE="${work}/trace" TP_TEST_ENTRY="${work}/entry" \
    TP_TEST_REVOKE_FAIL="$1" bash -c '
      set -Eeuo pipefail
      source "$1"
      verify_release_assets_before_host_change() { :; }
      require_supported_asset_architecture() { :; }
      require_root() { :; }
      preflight_install_dependencies() { :; }
      validate_host_data_root() { :; }
      preflight_node_removal_config_role() { :; }
      prepare_secure_config() { TP_CONFIG_READ_FILE="$1"; }
      load_config() { TP_DEPLOYMENT_MODE=combined; TLS_MODE=external; }
      translate_test_data_paths() { :; }
      validate_config() { :; }
      validate_entry_spec_binding() { :; }
      retain_verified_release_assets() { :; }
      combined_owner_prepare() { :; }
      combined_resource_check() { :; }
      require_combined_entry_ownership() { :; }
      revoke_combined_node_identity() {
        printf "revoke\n" >>"${TP_TEST_TRACE}"
        [[ "${TP_TEST_REVOKE_FAIL}" != 1 ]]
      }
      entry_controller() {
        printf "entry:%s\n" "$1" >>"${TP_TEST_TRACE}"
        printf "Entry changed\n" >"${TP_TEST_ENTRY}"
      }
      combined_entry_write_spec() { printf "spec\n" >>"${TP_TEST_TRACE}"; }
      combined_entry_reconcile() { printf "reconcile\n" >>"${TP_TEST_TRACE}"; }
      remove_combined_container_if_exists() { printf "container:%s\n" "$1" >>"${TP_TEST_TRACE}"; }
      main remove --mode node --config "$2" --entry-spec "$3"
    ' combined-order "${installer}" "${work}/combined.yaml" "${work}/entry-spec.json" \
    >"${work}/out" 2>"${work}/err"
}

: >"${work}/trace"
if run_case 1; then
  printf 'FAIL combined revocation order: failed revoke allowed removal\n' >&2
  exit 1
fi
test "$(cat "${work}/trace")" = revoke || {
  printf 'FAIL combined revocation order: Entry or container changed after failed revoke\n' >&2
  exit 1
}
grep -Fxq 'Entry unchanged' "${work}/entry"

: >"${work}/trace"
run_case 0
test "$(sed -n '1,3p' "${work}/trace")" = $'revoke\nentry:remove\nspec' || {
  printf 'FAIL combined revocation order: successful revoke did not precede Entry\n' >&2
  exit 1
}
grep -Fxq 'container:trojan-panel-core' "${work}/trace"
printf 'PASS combined Node revoke precedes Entry and container changes\n'
