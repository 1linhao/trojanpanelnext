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
(cd "${INSTALLER_DIR}/securefile" && CGO_ENABLED=0 go build -trimpath -o "${HELPER}" .)
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

printf 'fresh secret\n' >"${work}/fresh-input"
"${HELPER}" atomic-write --path "${work}/created/tree/secret" \
  --input "${work}/fresh-input" --mode 0600 --create-parents
test "$(cat "${work}/created/tree/secret")" = 'fresh secret' || fail 'fresh atomic write content differs'
test "$(stat -c %a "${work}/created/tree/secret")" = 600 || fail 'fresh atomic write is not mode 0600'

printf 'PASS secure file descriptor and atomic replacement contract\n'
