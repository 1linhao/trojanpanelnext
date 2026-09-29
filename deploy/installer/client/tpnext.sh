#!/usr/bin/env bash
set -Eeuo pipefail

CLIENT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
usage() {
  printf 'Usage:\n  %s init --config FILE --tag RELEASE_TAG --sha256 ARCHIVE_SHA256 [--archive FILE] [--web-transport ssh|local] [--work-dir DIR]\n  %s plan --config FILE\n' "$0" "$0"
}
fail() { printf 'tpnext: %s\n' "$1" >&2; exit "${2:-1}"; }
[[ $# -ge 1 ]] || { usage >&2; exit 2; }
command="$1"; shift
if [[ "${command}" == plan ]]; then
  exec bash "${CLIENT_DIR}/topology.sh" "$@"
fi
[[ "${command}" == init ]] || { usage >&2; exit 2; }

config=""; tag=""; sha=""; work_dir=""; source_archive=""; web_transport=ssh
while (($#)); do
  case "$1" in
  --config) [[ $# -ge 2 && -z "${config}" ]] || fail 'one --config FILE is required' 2; config="$2"; shift 2 ;;
  --tag) [[ $# -ge 2 && -z "${tag}" ]] || fail 'one --tag is required' 2; tag="$2"; shift 2 ;;
  --sha256) [[ $# -ge 2 && -z "${sha}" ]] || fail 'one --sha256 is required' 2; sha="$2"; shift 2 ;;
  --archive) [[ $# -ge 2 && -z "${source_archive}" ]] || fail 'one --archive FILE is required' 2; source_archive="$2"; shift 2 ;;
  --web-transport) [[ $# -ge 2 ]] || fail '--web-transport requires a value' 2; web_transport="$2"; shift 2 ;;
  --work-dir) [[ $# -ge 2 && -z "${work_dir}" ]] || fail 'one --work-dir DIR is required' 2; work_dir="$2"; shift 2 ;;
  -h | --help) usage; exit 0 ;;
  *) usage >&2; exit 2 ;;
  esac
done
[[ -n "${config}" && "${config}" != */ && "${config}" != -?* ]] || fail '--config FILE is required' 2
[[ "${tag}" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$ && "${tag}" != *latest* ]] || fail 'use an explicit fixed Release tag, never latest' 2
[[ "${sha}" =~ ^[0-9a-f]{64}$ ]] || fail 'a 64-character lowercase archive SHA-256 is required' 2
case "${web_transport}" in ssh | local) ;; *) fail '--web-transport must be ssh or local' 2 ;; esac
missing=()
for dependency in curl tar mktemp mkdir chmod mv ln sed find awk ssh scp cp; do
  command -v "${dependency}" >/dev/null 2>&1 || missing+=("${dependency}")
done
if ((${#missing[@]})); then fail "missing local commands: ${missing[*]}"; fi
command -v yq >/dev/null 2>&1 || fail 'install mikefarah yq v4: https://github.com/mikefarah/yq#install'
[[ "$(yq --version 2>/dev/null)" =~ version[[:space:]]+v?4\. ]] || fail 'mikefarah yq v4 is required; see https://github.com/mikefarah/yq#install'
if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
  fail 'install sha256sum or shasum for SHA-256 verification'
fi

path_has_symlink_component() {
  local path="$1" current=/ part
  local -a parts=()
  [[ "${path}" == /* ]] || path="${PWD}/${path}"
  IFS=/ read -r -a parts <<<"${path}"
  for part in "${parts[@]}"; do
    case "${part}" in '' | .) continue ;; ..) current="${current%/*}"; [[ -n "${current}" ]] || current=/ ;; *) current="${current%/}/${part}"; [[ ! -L "${current}" ]] || return 0 ;; esac
  done
  return 1
}
umask 077
config_parent="$(dirname "${config}")"
[[ ! -L "${config_parent}" ]] && ! path_has_symlink_component "${config_parent}" || fail 'configuration parent contains a symlink'
if [[ ! -d "${config_parent}" ]]; then mkdir -p -m 700 "${config_parent}"; fi
config_parent="$(cd "${config_parent}" && pwd -P)"
config="${config_parent}/$(basename "${config}")"
[[ ! -e "${config}" && ! -L "${config}" ]] || fail 'configuration already exists; init will not overwrite it'

# Inside a Git tree, the requested secret-bearing path must already be ignored.
if command -v git >/dev/null 2>&1 && git -C "${config_parent}" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  git -C "${config_parent}" check-ignore -q -- "${config}" || fail 'configuration path is not Git-ignored; choose a *.local.yaml path'
  ! git -C "${config_parent}" ls-files --error-unmatch -- "${config}" >/dev/null 2>&1 || fail 'configuration path is already tracked by Git'
fi

work_dir="${work_dir:-${config_parent}/trojanpanelnext-${tag#v}.local}"
work_parent="$(dirname "${work_dir}")"
[[ ! -L "${work_parent}" ]] && ! path_has_symlink_component "${work_parent}" || fail 'work directory parent contains a symlink'
if [[ ! -d "${work_parent}" ]]; then mkdir -p -m 700 "${work_parent}"; fi
work_parent="$(cd "${work_parent}" && pwd -P)"
work_dir="${work_parent}/$(basename "${work_dir}")"
[[ "${work_dir}" != "${config_parent}" && "${work_dir}" != "${config}" && ! -e "${work_dir}" && ! -L "${work_dir}" ]] || fail 'work directory already exists or is unsafe'
if command -v git >/dev/null 2>&1 && git -C "${work_parent}" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  git -C "${work_parent}" check-ignore -q -- "${work_dir}" || fail 'work directory must be Git-ignored; choose a *.local path'
fi

lock="${work_dir}.init-lock"
mkdir -m 700 "${lock}" 2>/dev/null || fail 'another init may be active for this work directory'
stage=""; temp_config=""; published_config=""
cleanup() {
  [[ -z "${stage}" || ! -d "${stage}" ]] || rm -r "${stage}"
  # The config is a hard link to our private temp file until publication ends.
  # Never recurse into a published path: another process may have added files.
  if [[ -n "${published_config}" && -f "${config}" && ! -L "${config}" &&
        -f "${temp_config}" && "${config}" -ef "${temp_config}" ]]; then
    rm "${config}"
  fi
  [[ -z "${temp_config}" || ! -f "${temp_config}" ]] || rm "${temp_config}"
  rmdir "${lock}" 2>/dev/null || true
}
trap cleanup EXIT
[[ ! -e "${work_dir}" && ! -e "${config}" ]] || fail 'configuration or work directory appeared during init'
stage="$(mktemp -d "${work_dir}.tmp.XXXXXXXX")"
chmod 700 "${stage}"
archive_name="trojanpanelnext-installer-${tag#v}.tar.gz"
archive="${stage}/${archive_name}"
if [[ -n "${source_archive}" ]]; then
  [[ -f "${source_archive}" && ! -L "${source_archive}" ]] || fail 'local Release archive is missing or unsafe'
  cp -- "${source_archive}" "${archive}" || fail 'local Release archive could not be copied'
else
  curl -fLsS --retry 3 "https://github.com/1linhao/trojanpanelnext/releases/download/${tag}/${archive_name}" -o "${archive}" || fail 'Release download failed; retry init'
fi
bash "${CLIENT_DIR}/verify-release.sh" --archive "${archive}" --tag "${tag}" --sha256 "${sha}" --assets-dir "${stage}/assets"
template="${CLIENT_DIR}/templates/unified-${web_transport}.yaml"
temp_config="$(mktemp "${config}.tmp.XXXXXXXX")"
sed -e "s/__RELEASE_TAG__/${tag}/g" -e "s/__ARCHIVE_SHA256__/${sha}/g" "${template}" >"${temp_config}"
chmod 600 "${temp_config}"
bash "${CLIENT_DIR}/topology.sh" --config "${temp_config}" >/dev/null
[[ ! -e "${work_dir}" && ! -e "${config}" ]] || fail 'configuration or work directory appeared during init'
stage_name="$(basename "${stage}")"
owner_marker=".init-owner-${stage_name}"
: >"${stage}/${owner_marker}"
ln "${temp_config}" "${config}" || fail 'configuration could not be published; retry init'
published_config=1
# A move can report failure after the rename. The private marker lets us
# recognize that completed state without deleting anything at the target.
mv -n "${stage}" "${work_dir}" || {
  [[ ! -e "${stage}" && -f "${work_dir}/${owner_marker}" ]] || fail 'cannot publish verified work directory; retry init'
}
[[ ! -e "${stage}" && -f "${work_dir}/${owner_marker}" && ! -L "${work_dir}/${owner_marker}" && -d "${work_dir}/assets" ]] || fail 'work directory appeared during publish'
stage=""
published_config=""
rm "${temp_config}"
temp_config=""
printf 'Initialized %s with verified assets in %s\n' "${config}" "${work_dir}"
