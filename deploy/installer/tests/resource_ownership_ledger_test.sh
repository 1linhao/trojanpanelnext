#!/usr/bin/env bash
set -euo pipefail

installer_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf -- "$fixture"' EXIT
export TP_TEST_DATA_ROOT=1
export TP_TEST_TRACE="$fixture/trace"
export TP_TEST_CONTAINERS="$fixture/containers"
export TP_TEST_CONTAINER_JSON="$fixture/container.json"
export TP_TEST_IMAGE_JSON="$fixture/image.json"
export TP_TEST_SOCKETS="$fixture/sockets"
: >"$TP_TEST_CONTAINERS"
: >"$TP_TEST_SOCKETS"
root="$fixture/root"
plan="$fixture/plan.json"
reference="example.test/tp-api@sha256:$(printf 'a%.0s' {1..64})"
container_id="$(printf 'b%.0s' {1..64})"
image_id="sha256:$(printf 'c%.0s' {1..64})"

jq -cn --arg root "$root" --arg ref "$reference" '
  {schema_version:1,deployment_id:"tpn-test-1",root:$root,mode:"web",roles:["web"],
   images:{api:{reference:$ref}},
   containers:[{name:"tp-api",role:"web",image:"api",
     mounts:[{source:($root + "/data"),target:"/tpdata/data",read_only:false}],
     ports:[{protocol:"tcp",port:18081}]}]}' >"$plan"

docker() {
  printf 'docker %s\n' "$*" >>"$TP_TEST_TRACE"
  case "$1 $2" in
  'ps -a') [[ "${TP_TEST_PS_FAIL:-0}" != 1 ]] && cat "$TP_TEST_CONTAINERS" ;;
  'inspect --type') cat "$TP_TEST_CONTAINER_JSON" ;;
  'image ls')
    if [[ -s "$TP_TEST_IMAGE_JSON" ]]; then
      printf '%s|<none>|%s|%s\n' "${TP_TEST_REFERENCE%@*}" "${TP_TEST_REFERENCE#*@}" "$TP_TEST_IMAGE_ID"
    fi
    ;;
  'image inspect') [[ "${TP_TEST_IMAGE_FAIL:-0}" != 1 && -s "$TP_TEST_IMAGE_JSON" ]] && cat "$TP_TEST_IMAGE_JSON" ;;
  *) return 99 ;;
  esac
}
ss() {
  printf 'ss\n' >>"$TP_TEST_TRACE"
  [[ "${TP_TEST_SS_FAIL:-0}" != 1 ]] && cat "$TP_TEST_SOCKETS"
}
for command in mkdir mktemp chmod install; do
  eval "$command() { printf 'forbidden $command\\n' >>\"\$TP_TEST_TRACE\"; return 98; }"
  export -f "$command"
done
export -f docker ss
export TP_TEST_REFERENCE="$reference" TP_TEST_IMAGE_ID="$image_id"

check() {
  local expected="$1" output status=0
  output="$(bash "$installer_dir/ownership/ledger.sh" check --plan - <"$plan")" || status=$?
  [[ "$(jq -r '.status' <<<"$output")" == "$expected" ]] || {
    printf 'Unexpected status: %s\n' "$output" >&2; exit 1;
  }
  if [[ "$expected" == ready ]]; then [[ "$status" == 0 ]]; else [[ "$status" != 0 ]]; fi
  printf '%s\n' "$output"
}

check ready >/dev/null
! grep -q forbidden "$TP_TEST_TRACE"
! grep -Eq 'docker (pull|run|start|rm)|entryctl|node-identity' "$TP_TEST_TRACE"
[[ ! -e "$root" ]]

touch "$fixture/trojan-panel"
check blocked | jq -e '.findings | any(.[]; .code == "legacy_layout")' >/dev/null
rm "$fixture/trojan-panel"

# Combined deployments may retain a single role after a later role removal.
jq --arg root "$fixture/combined-root" \
  '.root=$root | .mode="combined" | .roles=["node"] |
   .containers=[{name:"tp-core",role:"node",image:"api",mounts:[],ports:[]},
                {name:"tp-db",role:"shared",image:"api",mounts:[],ports:[]}]' \
  "$plan" >"$fixture/combined-plan.json"
output="$(bash "$installer_dir/ownership/ledger.sh" check --plan - <"$fixture/combined-plan.json")"
[[ "$(jq -r .status <<<"$output")" == ready ]]

TP_TEST_PS_FAIL=1
export TP_TEST_PS_FAIL
check blocked | jq -e '.findings | any(.[]; .code == "docker_enumeration_unavailable")' >/dev/null
unset TP_TEST_PS_FAIL
TP_TEST_SS_FAIL=1
export TP_TEST_SS_FAIL
check blocked | jq -e '.findings | any(.[]; .code == "listener_observation_unavailable")' >/dev/null
unset TP_TEST_SS_FAIL

jq -cn --arg id "$image_id" --arg ref "$reference" \
  '[{Id:$id,RepoDigests:[$ref],RepoTags:[]}]' >"$TP_TEST_IMAGE_JSON"
check blocked | jq -e '.findings | any(.[]; .code == "existing_image_without_ledger")' >/dev/null
: >"$TP_TEST_IMAGE_JSON"

printf '%s\n' "$container_id" >"$TP_TEST_CONTAINERS"
jq -cn --arg id "$container_id" --arg image "$image_id" --arg ref "$reference" --arg root "$root" '
  [{Id:$id,Name:"/tp-api",Image:$image,Config:{Image:$ref,Labels:{}},
    Mounts:[{Source:($root + "/data"),Destination:"/tpdata/data",RW:true}]}]' >"$TP_TEST_CONTAINER_JSON"
check blocked | jq -e '.findings | any(.[]; .code == "existing_container_without_ledger")' >/dev/null
: >"$TP_TEST_CONTAINERS"

printf 'tcp LISTEN 0 128 0.0.0.0:18081 0.0.0.0:* users:(("foreign",pid=321,fd=3))\n' >"$TP_TEST_SOCKETS"
check blocked | jq -e '.findings | any(.[]; .code == "occupied_listener")' >/dev/null
status=0
output="$(bash "$installer_dir/ownership/ledger.sh" begin --plan - <"$plan")" || status=$?
[[ "$status" != 0 && "$(jq -r '.status' <<<"$output")" == blocked && ! -e "$root" ]]
! grep -q forbidden "$TP_TEST_TRACE"
: >"$TP_TEST_SOCKETS"

# The mutating command starts only after the identical read-only preflight.
unset -f mkdir mktemp chmod install
output="$(bash "$installer_dir/ownership/ledger.sh" begin --plan - <"$plan")"
[[ "$(jq -r .status <<<"$output")" == preparing ]]
marker="$root/.trojanpanelnext-owner-token"
ledger="$root/trojanpanelnext-installer/ownership.json"
[[ "$(stat -c %a "$marker")" == 600 && "$(stat -c %a "$ledger")" == 600 ]]
[[ "$(wc -c <"$marker")" == 64 ]]
! grep -q "$(cat "$marker")" "$ledger"
check blocked | jq -e '.findings | any(.[]; .code == "interrupted_phase")' >/dev/null

# Model completed deployment resources, then commit only after exact inspection.
printf '%s\n' "$container_id" >"$TP_TEST_CONTAINERS"
owner_sha="$(sha256sum "$marker" | awk '{print $1}')"
jq -cn --arg id "$container_id" --arg image "$image_id" --arg ref "$reference" \
  --arg root "$root" --arg sha "$owner_sha" '
  [{Id:$id,Name:"/tp-api",Image:$image,
    Config:{Image:$ref,Labels:{"io.trojanpanelnext.deployment":"tpn-test-1",
      "io.trojanpanelnext.owner-sha256":$sha,"io.trojanpanelnext.role":"web"}},
    Mounts:[{Source:($root + "/data"),Destination:"/tpdata/data",RW:true}]}]' >"$TP_TEST_CONTAINER_JSON"
jq -cn --arg id "$image_id" --arg ref "$reference" \
  '[{Id:$id,RepoDigests:[$ref],RepoTags:[]}]' >"$TP_TEST_IMAGE_JSON"
cp "$TP_TEST_CONTAINER_JSON" "$fixture/good-container.json"
jq '.[0].Mounts[0].Source="/foreign/data"' "$TP_TEST_CONTAINER_JSON" >"$fixture/mismatch.json"
cp "$fixture/mismatch.json" "$TP_TEST_CONTAINER_JSON"
status=0
output="$(bash "$installer_dir/ownership/ledger.sh" commit --plan - <"$plan")" || status=$?
[[ "$status" != 0 && "$(jq -r '.status' <<<"$output")" == blocked ]]
[[ "$(jq -r .phase "$ledger")" == preparing ]]
cp "$fixture/good-container.json" "$TP_TEST_CONTAINER_JSON"
output="$(bash "$installer_dir/ownership/ledger.sh" commit --plan - <"$plan")"
[[ "$(jq -r .status <<<"$output")" == committed ]]
check ready >/dev/null

: >"$TP_TEST_CONTAINERS"
check blocked | jq -e '.findings | any(.[]; .code == "recorded_container_missing")' >/dev/null
printf '%s\n' "$container_id" >"$TP_TEST_CONTAINERS"

TP_TEST_IMAGE_FAIL=1
export TP_TEST_IMAGE_FAIL
check blocked | jq -e '.findings | any(.[]; .code == "image_unobservable")' >/dev/null
unset TP_TEST_IMAGE_FAIL

jq '.[0].Mounts[0].Source="/foreign/data"' "$TP_TEST_CONTAINER_JSON" >"$fixture/mismatch.json"
cp "$fixture/mismatch.json" "$TP_TEST_CONTAINER_JSON"
check blocked | jq -e '.findings | any(.[]; .code == "container_mount_mismatch")' >/dev/null

jq '.[0].RepoTags=["example.test/tp-api:unexpected"]' "$TP_TEST_IMAGE_JSON" >"$fixture/mismatch.json"
cp "$fixture/mismatch.json" "$TP_TEST_IMAGE_JSON"
check blocked | jq -e '.findings | any(.[]; .code == "image_reference_drift")' >/dev/null

printf 'tampered' >"$marker"
check blocked | jq -e '.findings | any(.[]; .code == "owner_marker_mismatch")' >/dev/null
for command in mkdir mktemp chmod install; do
  eval "$command() { printf 'forbidden $command\\n' >>\"\$TP_TEST_TRACE\"; return 98; }"
  export -f "$command"
done
check blocked >/dev/null
! grep -q forbidden "$TP_TEST_TRACE"
printf 'resource ownership ledger tests passed\n'
