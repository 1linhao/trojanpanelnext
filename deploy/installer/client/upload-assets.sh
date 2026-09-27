#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  printf 'Usage: %s --mode web|node-config|node|combined --host SSH_TARGET --work-dir DIR [--bundle FILE]\n' "$0"
}

mode=""
host=""
work_dir=""
bundle=""
while (($#)); do
  case "$1" in
  --mode) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; mode="$2"; shift 2 ;;
  --host) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; host="$2"; shift 2 ;;
  --work-dir) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; work_dir="$2"; shift 2 ;;
  --bundle) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; bundle="$2"; shift 2 ;;
  -h | --help) usage; exit 0 ;;
  *) usage >&2; exit 2 ;;
  esac
done

case "${mode}" in web | node-config | node | combined) ;; *) usage >&2; exit 2 ;; esac
[[ -n "${host}" && "${host}" != -* && "${host}" != *[[:space:]]* && "${host}" != *:* ]] || {
  printf 'Invalid SSH target: %s\n' "${host}" >&2
  exit 2
}
[[ -d "${work_dir}" ]] || { printf 'Work directory not found: %s\n' "${work_dir}" >&2; exit 2; }
for dependency in ssh scp; do
  command -v "${dependency}" >/dev/null 2>&1 || {
    printf 'Missing local command: %s\n' "${dependency}" >&2
    exit 1
  }
done

files=()
case "${mode}" in
web | combined)
  files=("${work_dir}/config/${mode}.yaml")
  ;;
node-config)
  files=("${work_dir}/config/node.yaml")
  ;;
node)
  [[ -n "${bundle}" ]] || { printf 'Node upload requires --bundle FILE\n' >&2; exit 2; }
  files=("${bundle}")
  ;;
esac
if [[ "${mode}" != node-config ]]; then
  archives=("${work_dir}"/trojanpanelnext-installer-*.tar.gz)
  [[ ${#archives[@]} -eq 1 && -f "${archives[0]}" ]] || {
    printf 'Expected exactly one installer archive in %s\n' "${work_dir}" >&2
    exit 1
  }
  files=("${archives[0]}" "${files[@]}")
fi
for file in "${files[@]}"; do
  [[ -f "${file}" && ! -L "${file}" ]] || {
    printf 'Upload file missing or unsafe: %s\n' "${file}" >&2
    exit 1
  }
done

ssh "${host}" 'umask 077; mkdir -p ~/tpnext-upload && chmod 700 ~/tpnext-upload'
scp "${files[@]}" "${host}:tpnext-upload/"
names=()
for file in "${files[@]}"; do names+=("${file##*/}"); done
printf -v names_shell '%q ' "${names[@]}"
ssh "${host}" "cd ~/tpnext-upload && chmod 600 ${names_shell}"
printf 'Uploaded %s to %s:tpnext-upload/\n' "${mode}" "${host}"
