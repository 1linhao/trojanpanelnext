#!/usr/bin/env bash
set -Eeuo pipefail

fail() {
  printf 'attestation results: %s\n' "$1" >&2
  exit 1
}

manifest=""
image_results=""
files_dir=""
file_results=""
while (($#)); do
  case "$1" in
  --manifest) [[ $# -ge 2 ]] || fail '--manifest requires a value'; manifest="$2"; shift 2 ;;
  --image-results) [[ $# -ge 2 ]] || fail '--image-results requires a value'; image_results="$2"; shift 2 ;;
  --files-dir) [[ $# -ge 2 ]] || fail '--files-dir requires a value'; files_dir="$2"; shift 2 ;;
  --file-results) [[ $# -ge 2 ]] || fail '--file-results requires a value'; file_results="$2"; shift 2 ;;
  *) fail "unknown argument: $1" ;;
  esac
done

[[ -f "${manifest}" && ! -L "${manifest}" ]] || fail 'manifest is missing or unsafe'
[[ -d "${files_dir}" && ! -L "${files_dir}" ]] || fail 'attested files directory is missing or unsafe'
[[ -f "${file_results}" && ! -L "${file_results}" ]] || fail 'file verification results are missing or unsafe'
if [[ -n "${image_results}" ]]; then
  [[ -f "${image_results}" && ! -L "${image_results}" ]] || fail 'image verification results are missing or unsafe'
fi
command -v jq >/dev/null 2>&1 || fail 'jq is required'
command -v sha256sum >/dev/null 2>&1 || fail 'sha256sum is required'

work="$(mktemp -d)"
trap 'rm -rf -- "${work}"' EXIT

verified_subjects() {
  local results="$1"
  jq -ce '
    if type != "array" then error("verification results must be an array") else . end |
    if any(.[]; (.verificationResult.statement.subject | type) != "array")
      then error("each verification result must contain a subject array")
      else .
    end |
    [.[].verificationResult.statement.subject[]] as $subjects |
    if any($subjects[];
      ((.name | type) != "string") or
      ((.digest.sha256 | type) != "string") or
      (.digest.sha256 | test("^[0-9a-f]{64}$") | not))
      then error("verification result contains an invalid subject")
      else $subjects
    end |
    [ .[] |
      {name: .name, digest: .digest.sha256}
    ] | unique_by([.name, .digest]) | sort_by(.name, .digest)
  ' "${results}"
}

compare_subjects() {
  local kind="$1"
  local expected="$2"
  local actual="$3"
  jq -S . <<<"${expected}" >"${work}/${kind}-expected.json"
  jq -S . <<<"${actual}" >"${work}/${kind}-actual.json"
  cmp -s "${work}/${kind}-expected.json" "${work}/${kind}-actual.json" ||
    fail "${kind} attestation subjects or digests do not match"
}

if [[ -n "${image_results}" ]]; then
  expected_images="$(jq -ce '
    [.attestations[] | {name: .subject, digest: (.digest | sub("^sha256:"; ""))}] |
    unique_by([.name, .digest]) | sort_by(.name, .digest)
  ' "${manifest}")" || fail 'manifest image attestation expectations are invalid'
  actual_images="$(verified_subjects "${image_results}")" || fail 'image verification results are invalid'
  compare_subjects images "${expected_images}" "${actual_images}"
fi

expected_files='[]'
file_count=0
for file in "${files_dir}"/*; do
  [[ -f "${file}" && ! -L "${file}" ]] || fail "attested file is missing or unsafe: ${file##*/}"
  name="${file##*/}"
  digest="$(sha256sum "${file}" | awk '{print $1}')"
  expected_files="$(jq -c --arg name "${name}" --arg digest "${digest}" \
    '. + [{name: $name, digest: $digest}]' <<<"${expected_files}")"
  file_count=$((file_count + 1))
done
((file_count > 0)) || fail 'no attested files found'
expected_files="$(jq -c 'unique_by([.name, .digest]) | sort_by(.name, .digest)' <<<"${expected_files}")"
actual_files="$(verified_subjects "${file_results}")" || fail 'file verification results are invalid'
compare_subjects files "${expected_files}" "${actual_files}"

scope=""
if [[ -n "${image_results}" ]]; then
  scope=' and release images'
fi
printf 'Verified attestation subjects and digests for %d files%s\n' "${file_count}" "${scope}"
