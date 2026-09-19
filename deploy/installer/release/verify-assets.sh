#!/usr/bin/env bash
set -Eeuo pipefail

fail() {
  printf 'release assets: %s\n' "$1" >&2
  exit 1
}

assets_dir=""
config=""
while (($#)); do
  case "$1" in
  --assets-dir) [[ $# -ge 2 ]] || fail '--assets-dir requires a value'; assets_dir="$2"; shift 2 ;;
  --config) [[ $# -ge 2 ]] || fail '--config requires a value'; config="$2"; shift 2 ;;
  *) fail "unknown argument: $1" ;;
  esac
done
[[ -n "${assets_dir}" && -d "${assets_dir}" ]] || fail 'asset directory not found'
[[ -n "${config}" && -f "${config}" ]] || fail 'configuration file not found'
command -v awk >/dev/null 2>&1 || fail 'awk is required'
command -v sha256sum >/dev/null 2>&1 || fail 'sha256sum is required'

# This list is intentionally local to the verifier. It is the dependency-free
# trust root for checking SHA256SUMS before any other bundled program is read or
# executed. The release contract test independently checks that generation and
# verification agree on the complete asset set.
EXPECTED_RELEASE_ASSET_PATHS=(
  bootstrap.sh
  release-contract.sh
  verify-assets.sh
  install.sh
  config-web.yaml
  config-node.yaml
  config-combined.yaml
  entry/entryctl.sh
  entry/controller.sh
  entry/adapters/external.sh
  entry/adapters/nginx_certbot.sh
)

manifest="${assets_dir}/release-manifest.json"
sums="${assets_dir}/SHA256SUMS"
[[ -f "${manifest}" && ! -L "${manifest}" ]] || fail 'release-manifest.json is missing or unsafe'
[[ -f "${sums}" && ! -L "${sums}" ]] || fail 'SHA256SUMS is missing or unsafe'

sum_paths=()
while IFS= read -r sum_line || [[ -n "${sum_line}" ]]; do
  if [[ ! "${sum_line}" =~ ^[0-9a-f]{64}[[:space:]][\ \*]([A-Za-z0-9][A-Za-z0-9._/-]*)$ ]]; then
    fail 'SHA256SUMS has invalid syntax'
  fi
  sum_path="${BASH_REMATCH[1]}"
  [[ "${sum_path}" != *..* ]] || fail 'SHA256SUMS has an unsafe asset path'
  sum_paths+=("${sum_path}")
done <"${sums}"
expected_sum_paths=("${EXPECTED_RELEASE_ASSET_PATHS[@]}" release-manifest.json)
[[ "${#sum_paths[@]}" -eq "${#expected_sum_paths[@]}" ]] || fail 'SHA256SUMS has an unexpected asset set'
printf '%s\n' "${sum_paths[@]}" | sort -u | cmp -s - <(printf '%s\n' "${expected_sum_paths[@]}" | sort) ||
  fail 'SHA256SUMS has an unexpected asset set'
for path in "${expected_sum_paths[@]}"; do
  [[ -f "${assets_dir}/${path}" && ! -L "${assets_dir}/${path}" ]] || fail "asset is missing or unsafe: ${path}"
done
(
  cd "${assets_dir}"
  sha256sum -c SHA256SUMS >/dev/null
) || fail 'SHA256SUMS verification failed'

# Parse the generated JSON without jq or another downloadable runtime. The
# release manifest intentionally uses a strict ASCII subset of JSON: escapes,
# control characters, duplicate object keys, trailing data, and unknown schema
# fields are rejected. This parser is part of the already attested verifier,
# rather than another bundled executable that would need bootstrapping trust.
manifest_records="$(awk '
  function invalid() { exit 2 }
  function skip_space(    c) {
    while (pos <= length(document)) {
      c = substr(document, pos, 1)
      if (c != " " && c != "\t" && c != "\r" && c != "\n") break
      pos++
    }
  }
  function parse_string(    value,c) {
    if (substr(document, pos, 1) != "\"") invalid()
    pos++
    value = ""
    while (pos <= length(document)) {
      c = substr(document, pos, 1)
      if (c == "\"") { pos++; return value }
      if (c == "\\" || c == "\t" || c == "\r" || c == "\n") invalid()
      value = value c
      pos++
    }
    invalid()
  }
  function child_path(parent, child) {
    return parent == "/" ? "/" child : parent "/" child
  }
  function emit(kind, path, value) { print kind "\t" path "\t" value }
  function parse_number(path,    rest,number) {
    rest = substr(document, pos)
    if (!match(rest, /^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?/)) invalid()
    number = substr(rest, RSTART, RLENGTH)
    pos += RLENGTH
    emit("N", path, number)
  }
  function parse_array(path,    item_index,c) {
    emit("A", path, "")
    pos++
    skip_space()
    if (substr(document, pos, 1) == "]") { pos++; return }
    item_index = 0
    while (1) {
      parse_value(child_path(path, item_index))
      item_index++
      skip_space()
      c = substr(document, pos, 1)
      if (c == "]") { pos++; return }
      if (c != ",") invalid()
      pos++
      skip_space()
    }
  }
  function parse_object(path,    key,c,seen_key) {
    emit("O", path, "")
    pos++
    skip_space()
    if (substr(document, pos, 1) == "}") { pos++; return }
    while (1) {
      key = parse_string()
      if (key !~ /^[A-Za-z_][A-Za-z0-9_]*$/) invalid()
      seen_key = path SUBSEP key
      if (object_key_seen[seen_key]++) invalid()
      skip_space()
      if (substr(document, pos, 1) != ":") invalid()
      pos++
      skip_space()
      parse_value(child_path(path, key))
      skip_space()
      c = substr(document, pos, 1)
      if (c == "}") { pos++; return }
      if (c != ",") invalid()
      pos++
      skip_space()
    }
  }
  function parse_value(path,    c,value) {
    skip_space()
    c = substr(document, pos, 1)
    if (c == "{") { parse_object(path); return }
    if (c == "[") { parse_array(path); return }
    if (c == "\"") { value = parse_string(); emit("S", path, value); return }
    if (c == "-" || c ~ /^[0-9]$/) { parse_number(path); return }
    if (substr(document, pos, 4) == "true") { pos += 4; emit("L", path, "true"); return }
    if (substr(document, pos, 5) == "false") { pos += 5; emit("L", path, "false"); return }
    if (substr(document, pos, 4) == "null") { pos += 4; emit("L", path, "null"); return }
    invalid()
  }
  { document = document $0 "\n" }
  END {
    pos = 1
    skip_space()
    parse_value("/")
    skip_space()
    if (pos <= length(document)) invalid()
  }
' "${manifest}")" || fail 'manifest JSON is invalid'

normalized_manifest="$(awk -F '\t' '
  function reject() { invalid = 1 }
  function expect(path, expected_type) {
    expected[path] = 1
    if (!(path in node_type) || node_type[path] != expected_type) reject()
  }
  {
    if ($2 in node_type) reject()
    node_type[$2] = $1
    node_value[$2] = $3
  }
  END {
    expect("/", "O")
    expect("/schema_version", "N")
    expect("/release_version", "S")
    expect("/source_commit", "S")
    expect("/assets", "A")
    expect("/images", "O")
    expect("/attestations", "A")
    if (node_value["/schema_version"] != "1") reject()
    if (node_value["/release_version"] == "") reject()
    if (length(node_value["/source_commit"]) != 40 || node_value["/source_commit"] !~ /^[0-9a-f]+$/) reject()

    for (i = 0; i < 11; i++) {
      base = "/assets/" i
      expect(base, "O")
      expect(base "/name", "S")
      expect(base "/path", "S")
      expect(base "/sha256", "S")
      name = node_value[base "/name"]
      path = node_value[base "/path"]
      digest = node_value[base "/sha256"]
      if (name == "" || name_seen[name]++) reject()
      if (path !~ /^[A-Za-z0-9][A-Za-z0-9._\/-]*$/ || path ~ /\.\./ || path_seen[path]++) reject()
      if (length(digest) != 64 || digest !~ /^[0-9a-f]+$/) reject()
      asset_path[i] = path
      asset_digest[i] = digest
    }

    image_key[0] = "api"; image_kind[0] = "product"
    image_key[1] = "web"; image_kind[1] = "product"
    image_key[2] = "node_agent"; image_kind[2] = "product"
    image_key[3] = "caddy"; image_kind[3] = "runtime"
    image_key[4] = "mariadb"; image_kind[4] = "runtime"
    image_key[5] = "redis"; image_kind[5] = "runtime"
    for (i = 0; i < 6; i++) {
      base = "/images/" image_key[i]
      expect(base, "O")
      expect(base "/kind", "S")
      expect(base "/name", "S")
      expect(base "/digest", "S")
      expect(base "/reference", "S")
      kind = node_value[base "/kind"]
      name = node_value[base "/name"]
      digest = node_value[base "/digest"]
      reference = node_value[base "/reference"]
      if (kind != image_kind[i]) reject()
      if (name == "" || name !~ /^[A-Za-z0-9._\/-]+$/) reject()
      if (digest !~ /^sha256:[0-9a-f]+$/ || length(digest) != 71) reject()
      if (reference != name "@" digest) reject()
      image_name[image_key[i]] = name
      image_digest[image_key[i]] = digest
      image_reference[image_key[i]] = reference
    }

    for (i = 0; i < 3; i++) {
      base = "/attestations/" i
      expect(base, "O")
      expect(base "/subject", "S")
      expect(base "/digest", "S")
      subject = node_value[base "/subject"]
      digest = node_value[base "/digest"]
      if (subject_seen[subject]++) reject()
      matched = ""
      for (j = 0; j < 3; j++) {
        key = image_key[j]
        if (subject == image_name[key] && digest == image_digest[key]) matched = key
      }
      if (matched == "") reject()
      attested[matched] = 1
    }
    for (i = 0; i < 3; i++) if (!attested[image_key[i]]) reject()
    for (path in node_type) if (!(path in expected)) reject()
    if (invalid) exit 2

    print "VERSION\t" node_value["/release_version"]
    for (i = 0; i < 11; i++) print "ASSET\t" asset_path[i] "\t" asset_digest[i]
    for (i = 0; i < 6; i++) print "IMAGE\t" image_key[i] "\t" image_reference[image_key[i]]
  }
' <<<"${manifest_records}")" || fail 'manifest structure is invalid'

manifest_version="$(awk -F '\t' '$1 == "VERSION" {print $2}' <<<"${normalized_manifest}")"
release_semver_is_valid() {
  local version="${1:-}"
  local without_build prerelease identifier
  local -a identifiers=()

  [[ "${version}" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$ ]] || return 1
  without_build="${version%%+*}"
  [[ "${without_build}" == *-* ]] || return 0
  prerelease="${without_build#*-}"
  IFS=. read -r -a identifiers <<<"${prerelease}"
  for identifier in "${identifiers[@]}"; do
    if [[ "${identifier}" =~ ^[0-9]+$ && "${identifier}" == 0[0-9]* ]]; then
      return 1
    fi
  done
}
release_semver_is_valid "${manifest_version}" || fail 'manifest release version is invalid'

mapfile -t manifest_asset_paths < <(awk -F '\t' '$1 == "ASSET" {print $2}' <<<"${normalized_manifest}")
cmp -s \
  <(printf '%s\n' "${manifest_asset_paths[@]}" | sort) \
  <(printf '%s\n' "${EXPECTED_RELEASE_ASSET_PATHS[@]}" | sort) ||
  fail 'manifest structure is invalid'
while IFS=$'\t' read -r record path expected; do
  [[ "${record}" == ASSET ]] || continue
  actual="$(sha256sum "${assets_dir}/${path}" | awk '{print $1}')"
  [[ "${actual}" == "${expected}" ]] || fail "asset digest mismatch: ${path}"
done <<<"${normalized_manifest}"

grep -qx 'trojanpanelnext:' "${config}" || fail 'configuration must contain one trojanpanelnext root'
config_count() {
  local key="$1"
  awk -F: -v key="${key}" '$1 == "  " key {count++} END {print count + 0}' "${config}"
}
config_value() {
  local key="$1"
  awk -F: -v key="${key}" '$1 == "  " key {sub(/^[^:]*:[[:space:]]*/, ""); gsub(/^"|"$/, ""); print; exit}' "${config}"
}
for key in schema_version asset_version deployment_mode api_image web_image node_agent_image caddy_image mariadb_image redis_image; do
  [[ "$(config_count "${key}")" == 1 ]] || fail "configuration key must appear exactly once: ${key}"
done
for legacy_key in purpose panel_image ui_image core_image; do
  [[ "$(config_count "${legacy_key}")" == 0 ]] || fail "legacy configuration key is not allowed in the release contract: ${legacy_key}"
done

[[ "$(config_value schema_version)" == 1 ]] || fail 'configuration schema_version must be 1'
version="$(config_value asset_version)"
[[ -n "${version}" && "${version}" == "${manifest_version}" ]] || fail 'configuration asset_version does not match release'
deployment_mode="$(config_value deployment_mode)"
case "${deployment_mode}" in web | node | combined) ;; *) fail "unsupported deployment mode: ${deployment_mode}" ;; esac

while IFS=$'\t' read -r config_key manifest_key; do
  configured="$(config_value "${config_key}")"
  expected="$(awk -F '\t' -v key="${manifest_key}" '$1 == "IMAGE" && $2 == key {print $3}' <<<"${normalized_manifest}")"
  [[ "${configured}" == "${expected}" ]] || fail "configuration image does not match manifest: ${config_key}"
done <<'IMAGE_KEYS'
api_image	api
web_image	web
node_agent_image	node_agent
caddy_image	caddy
mariadb_image	mariadb
redis_image	redis
IMAGE_KEYS

printf 'Release assets are valid for %s deployment mode (version %s)\n' "${deployment_mode}" "${version}"
