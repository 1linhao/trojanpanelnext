#!/usr/bin/env bash
set -Eeuo pipefail

operation="$1"
expression="$2"
file="$3"
if [[ "${operation}" == -e ]]; then
  grep -q '^trojanpanelnext:' "${file}"
  exit
fi
[[ "${operation}" == -r ]] || exit 2
key="${expression#.trojanpanelnext.}"
key="${key%% *}"
awk -F: -v key="${key}" '
  $1 == "  " key {
    sub(/^[^:]*:[[:space:]]*/, "")
    sub(/[[:space:]]+#.*$/, "")
    gsub(/^"|"$/, "")
    print
    exit
  }
' "${file}"
