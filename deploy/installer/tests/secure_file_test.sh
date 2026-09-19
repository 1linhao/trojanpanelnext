#!/usr/bin/env bash
set -Eeuo pipefail

INSTALLER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

work="$(mktemp -d)"
trap 'rm -rf -- "${work}"' EXIT
HELPER="${work}/secure-file"
(cd "${INSTALLER_DIR}/securefile" && CGO_ENABLED=0 go build -trimpath -tags securefiletest -o "${HELPER}" .)

wait_for_race_hook() {
  local ready="$1"
  local attempt
  for attempt in $(seq 1 200); do
    [[ -e "${ready}" ]] && return 0
    sleep 0.01
  done
  return 1
}
mkdir -p "${work}/safe/parent"
printf 'original\n' >"${work}/safe/parent/config.yaml"
chmod 0644 "${work}/safe/parent/config.yaml"

snapshot="${work}/snapshot"
token="$("${HELPER}" snapshot --path "${work}/safe/parent/config.yaml" --output "${snapshot}")"
test "$(cat "${snapshot}")" = original || fail 'snapshot content differs from the opened file'
test "$(stat -c %a "${snapshot}")" = 600 || fail 'snapshot is not mode 0600'

printf 'updated\n' >"${snapshot}"
"${HELPER}" atomic-write --path "${work}/safe/parent/config.yaml" \
  --input "${snapshot}" --expected "${token}" --mode 0600
test "$(cat "${work}/safe/parent/config.yaml")" = updated || fail 'atomic write did not replace the intended file'
test "$(stat -c %a "${work}/safe/parent/config.yaml")" = 600 || fail 'atomic write did not enforce mode 0600'

ln -s "${work}/safe/parent/config.yaml" "${work}/final-link"
if "${HELPER}" snapshot --path "${work}/final-link" --output "${work}/link-snapshot" >/dev/null 2>&1; then
  fail 'snapshot followed a final symlink'
fi

mkdir -p "${work}/real-parent"
printf 'parent target\n' >"${work}/real-parent/config.yaml"
ln -s "${work}/real-parent" "${work}/parent-link"
if "${HELPER}" snapshot --path "${work}/parent-link/config.yaml" --output "${work}/parent-snapshot" >/dev/null 2>&1; then
  fail 'snapshot followed a parent symlink'
fi

mkdir -p "${work}/ambiguous/target/child"
printf 'ambiguous\n' >"${work}/ambiguous/target/config.yaml"
ln -s "${work}/ambiguous/target/child" "${work}/ambiguous/linked"
if "${HELPER}" snapshot --path "${work}/ambiguous/linked/../config.yaml" \
  --output "${work}/ambiguous-snapshot" >/dev/null 2>&1; then
  fail 'snapshot accepted a symlink/.. path'
fi

printf 'race source\n' >"${work}/safe/parent/race.yaml"
race_snapshot="${work}/race-snapshot"
race_token="$("${HELPER}" snapshot --path "${work}/safe/parent/race.yaml" --output "${race_snapshot}")"
mv "${work}/safe/parent/race.yaml" "${work}/safe/parent/race-opened.yaml"
printf 'attacker target\n' >"${work}/safe/parent/race.yaml"
printf 'new secret\n' >"${race_snapshot}"
if "${HELPER}" atomic-write --path "${work}/safe/parent/race.yaml" \
  --input "${race_snapshot}" --expected "${race_token}" --mode 0600 >/dev/null 2>&1; then
  fail 'atomic write accepted a swapped final file'
fi
test "$(cat "${work}/safe/parent/race.yaml")" = 'attacker target' || fail 'swapped final file was overwritten'

mkdir -p "${work}/swap-parent"
printf 'parent race\n' >"${work}/swap-parent/config.yaml"
parent_snapshot="${work}/parent-race-snapshot"
parent_token="$("${HELPER}" snapshot --path "${work}/swap-parent/config.yaml" --output "${parent_snapshot}")"
mv "${work}/swap-parent" "${work}/swap-parent-opened"
mkdir "${work}/attacker-parent"
printf 'attacker parent\n' >"${work}/attacker-parent/config.yaml"
ln -s "${work}/attacker-parent" "${work}/swap-parent"
if "${HELPER}" atomic-write --path "${work}/swap-parent/config.yaml" \
  --input "${parent_snapshot}" --expected "${parent_token}" --mode 0600 >/dev/null 2>&1; then
  fail 'atomic write accepted a swapped parent path'
fi
test "$(cat "${work}/attacker-parent/config.yaml")" = 'attacker parent' || fail 'swapped parent target was overwritten'

mkdir -p "${work}/commit-race"
printf 'commit original\n' >"${work}/commit-race/config.yaml"
commit_snapshot="${work}/commit-race-snapshot"
commit_token="$("${HELPER}" snapshot --path "${work}/commit-race/config.yaml" --output "${commit_snapshot}")"
printf 'commit secret\n' >"${commit_snapshot}"
commit_ready="${work}/commit-target.ready"
commit_continue="${work}/commit-target.continue"
TP_SECURE_FILE_TEST_READY="${commit_ready}" TP_SECURE_FILE_TEST_CONTINUE="${commit_continue}" \
  "${HELPER}" atomic-write --path "${work}/commit-race/config.yaml" \
  --input "${commit_snapshot}" --expected "${commit_token}" --mode 0600 \
  >"${work}/commit-target.out" 2>&1 &
commit_pid=$!
wait_for_race_hook "${commit_ready}" || fail 'atomic write did not expose the deterministic pre-commit race hook'
mv "${work}/commit-race/config.yaml" "${work}/commit-race/opened.yaml"
printf 'commit attacker\n' >"${work}/commit-race/config.yaml"
: >"${commit_continue}"
if wait "${commit_pid}"; then
  fail 'atomic write accepted a final target swap after its last check'
fi
test "$(cat "${work}/commit-race/config.yaml")" = 'commit attacker' ||
  fail 'final target swapped after the last check was overwritten'
test "$(cat "${work}/commit-race/opened.yaml")" = 'commit original' ||
  fail 'original final target changed during rejected commit'
if find "${work}/commit-race" -maxdepth 1 -name '.config.yaml.tmp.*' -print -quit | grep -q .; then
  fail 'rejected final target race left a temporary sensitive file'
fi

mkdir -p "${work}/commit-parent"
printf 'parent original\n' >"${work}/commit-parent/config.yaml"
commit_parent_snapshot="${work}/commit-parent-snapshot"
commit_parent_token="$("${HELPER}" snapshot --path "${work}/commit-parent/config.yaml" --output "${commit_parent_snapshot}")"
printf 'parent secret\n' >"${commit_parent_snapshot}"
parent_ready="${work}/commit-parent.ready"
parent_continue="${work}/commit-parent.continue"
TP_SECURE_FILE_TEST_READY="${parent_ready}" TP_SECURE_FILE_TEST_CONTINUE="${parent_continue}" \
  "${HELPER}" atomic-write --path "${work}/commit-parent/config.yaml" \
  --input "${commit_parent_snapshot}" --expected "${commit_parent_token}" --mode 0600 \
  >"${work}/commit-parent.out" 2>&1 &
parent_pid=$!
wait_for_race_hook "${parent_ready}" || fail 'atomic write did not expose the deterministic parent pre-commit race hook'
mv "${work}/commit-parent" "${work}/commit-parent-opened"
mkdir "${work}/commit-parent-attacker"
printf 'parent attacker\n' >"${work}/commit-parent-attacker/config.yaml"
ln -s "${work}/commit-parent-attacker" "${work}/commit-parent"
: >"${parent_continue}"
if wait "${parent_pid}"; then
  fail 'atomic write accepted a parent swap after its last check'
fi
test "$(cat "${work}/commit-parent-attacker/config.yaml")" = 'parent attacker' ||
  fail 'parent target swapped after the last check was overwritten'
test "$(cat "${work}/commit-parent-opened/config.yaml")" = 'parent original' ||
  fail 'original parent target changed during rejected commit'
if find "${work}/commit-parent-opened" -maxdepth 1 -name '.config.yaml.tmp.*' -print -quit | grep -q .; then
  fail 'rejected parent race left a temporary sensitive file'
fi

printf 'fresh secret\n' >"${work}/fresh-input"
"${HELPER}" atomic-write --path "${work}/created/tree/secret" \
  --input "${work}/fresh-input" --mode 0600 --create-parents
test "$(cat "${work}/created/tree/secret")" = 'fresh secret' || fail 'fresh atomic write content differs'
test "$(stat -c %a "${work}/created/tree/secret")" = 600 || fail 'fresh atomic write is not mode 0600'

printf 'PASS secure file descriptor and atomic replacement contract\n'
