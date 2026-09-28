#!/usr/bin/env bash
# Resource Ownership ledger and read-only host preflight. The installer must
# verify the release manifest and parse the configuration before calling this.
set -euo pipefail

usage() {
  printf 'Usage: ledger.sh check|begin|checkpoint|commit|prepare-replace --plan <verified-plan.json|-> [--replace <container>]\n' >&2
}

finding() {
  FINDINGS+=("$(jq -cn --arg code "$1" --arg resource "$2" '{code:$code,resource:$resource}')")
}

safe_path() {
  local path="$1" expected_type="$2" ancestor
  [[ "$path" == /* && "$path" != / && "$(realpath -m -- "$path")" == "$path" ]] || return 1
  [[ ! -L "$path" ]] || return 1
  ancestor="$(dirname -- "$path")"
  while [[ "$ancestor" != / ]]; do
    [[ ! -L "$ancestor" ]] || return 1
    if [[ -e "$ancestor" ]]; then
      [[ -d "$ancestor" ]] || return 1
      if [[ "${TP_TEST_DATA_ROOT:-0}" != 1 || "$ancestor" != /tmp ]]; then
        [[ "$(stat -c %u -- "$ancestor")" == "$EUID" || "$(stat -c %u -- "$ancestor")" == 0 ]] || return 1
        (( (8#$(stat -c %a -- "$ancestor") & 0022) == 0 )) || return 1
      fi
    fi
    ancestor="$(dirname -- "$ancestor")"
  done
  if [[ -e "$path" ]]; then
    case "$expected_type" in
    file) [[ -f "$path" ]] ;;
    directory) [[ -d "$path" ]] ;;
    esac
  fi
}

secure_file() {
  [[ -f "$1" && ! -L "$1" && "$(stat -c %u -- "$1")" == "$EUID" &&
    "$(stat -c %a -- "$1")" == 600 ]]
}

valid_plan() {
  jq -e '
    (keys == ["containers","deployment_id","images","mode","roles","root","schema_version"]) and
    .schema_version == 1 and
    (.deployment_id | type == "string" and test("^[a-z][a-z0-9-]{0,62}$")) and
    (.root | type == "string" and startswith("/")) and
    (.mode == "web" or .mode == "node" or .mode == "combined") and
    (.roles | type == "array" and length > 0 and (unique | length) == length and all(.[]; . == "web" or . == "node")) and
    (if .mode == "combined" then true else .roles == [.mode] end) and
    (.images | type == "object" and length > 0 and all(.[];
      (keys == ["reference"]) and
      (.reference | type == "string" and test("^[^@[:space:]]+@sha256:[a-f0-9]{64}$")))) and
    (.containers | type == "array" and length > 0 and
      ([.[].name] | unique | length) == length and all(.[];
        (keys == ["image","mounts","name","ports","role"]) and
        (.name | type == "string" and test("^[a-zA-Z0-9][a-zA-Z0-9_.-]*$")) and
        (.role == "web" or .role == "node" or .role == "shared") and
        (.image | type == "string") and
        (.mounts | type == "array" and all(.[];
          (keys == ["read_only","source","target"]) and
          (.source | type == "string" and startswith("/")) and
          (.target | type == "string" and startswith("/")) and
          (.read_only | type == "boolean"))) and
        all(.mounts[]; .source != $root.root) and
        ([.mounts[].target] | unique | length) == (.mounts | length) and
        (.ports | type == "array" and all(.[];
          (keys == ["port","protocol"]) and
          (.protocol == "tcp" or .protocol == "udp") and
          (.port | type == "number" and . >= 1 and . <= 65535))))) and
    all(.containers[]; .image as $key | . as $container |
      ($key | in($root.images)) and
      ($container.role == "shared" or ($root.roles | index($container.role) != null)))
  ' --argjson root "$PLAN_JSON" <<<"$PLAN_JSON" >/dev/null 2>&1
}

valid_ledger() {
  jq -e --arg id "$DEPLOYMENT_ID" --arg root "$ROOT" --arg mode "$MODE" \
    --argjson plan "$PLAN_JSON" '
    .schema_version == 1 and .deployment_id == $id and .root == $root and
    .mode == $mode and
    (.phase == "preparing" or .phase == "committed" or .phase == "replacing") and
    (.owner_sha256 | type == "string" and test("^[a-f0-9]{64}$")) and
    (.roles | type == "array") and (.containers | type == "array") and
    (.images | type == "array") and
    ([.containers[].name] | unique | length) == (.containers | length) and
    all(.containers[];
      (.name as $name | $plan.containers | any(.[]; .name == $name)) and
      (.id | type == "string" and test("^[a-f0-9]{64}$")) and
      (.image_id | type == "string" and test("^sha256:[a-f0-9]{64}$"))) and
    ([.images[].reference] | unique | length) == (.images | length) and
    all(.images[];
      (.reference as $ref | $plan.images | any(.[]; .reference == $ref)) and
      (.id | type == "string" and test("^sha256:[a-f0-9]{64}$"))) and
    (.plan == $plan or (.phase == "replacing" and .plan == $plan)) and
    (if .phase == "replacing" then
      (.target_plan == $plan) and
      (.replace_names | type == "array" and length > 0 and
        (unique | length) == length and all(.[]; . as $name | $plan.containers | any(.[]; .name == $name)))
     else true end) and
    (if .phase == "committed" then
      ([.containers[].name] | sort) == ([$plan.containers[].name] | sort) and
      ([.images[].reference] | sort) == ([$plan.images[].reference] | sort) and
      all(.containers[]; (.id | type == "string" and test("^[a-f0-9]{64}$")) and
        (.image_id | type == "string" and test("^sha256:[a-f0-9]{64}$"))) and
      all(.images[]; .id | type == "string" and test("^sha256:[a-f0-9]{64}$"))
     else true end)
  ' "$LEDGER" >/dev/null 2>&1
}

inspect_containers() {
  local ids id data name expected owner role digest recorded_id image_id image_ref mounts expected_mounts resolved_image
  if ! ids="$(docker ps -a --no-trunc --format '{{.ID}}' 2>/dev/null)"; then
    finding docker_enumeration_unavailable containers
    return
  fi
  while IFS= read -r id; do
    [[ -n "$id" ]] || continue
    if ! data="$(docker inspect --type container "$id" 2>/dev/null)" ||
      ! jq -e 'type == "array" and length == 1 and .[0].Id != null' <<<"$data" >/dev/null; then
      finding container_unobservable "$id"
      continue
    fi
    name="$(jq -r '.[0].Name | ltrimstr("/")' <<<"$data")"
    expected="$(jq -c --arg name "$name" '.containers[] | select(.name == $name)' <<<"$PLAN_JSON")"
    if jq -e --argjson images "$(jq -c '.images | [.[] .reference]' <<<"$PLAN_JSON")" \
      '.[0].Config.Image as $ref | $images | index($ref) != null' <<<"$data" >/dev/null && [[ -z "$expected" ]]; then
      finding foreign_image_user "$name"
    fi
    # Every container can reference the protected root even without a planned name.
    if jq -e --arg root "$ROOT" '.[0].Mounts // [] | any(.[]; .Source == $root or (.Source | startswith($root + "/")))' <<<"$data" >/dev/null; then
      [[ -n "$expected" ]] || finding foreign_root_mount "$name"
    fi
    [[ -n "$expected" ]] || continue
    if [[ "$LEDGER_PRESENT" != 1 ]]; then
      finding existing_container_without_ledger "$name"
      continue
    fi
    recorded_id="$(jq -r --arg name "$name" '.containers[] | select(.name == $name) | .id' "$LEDGER")"
    if [[ -n "$recorded_id" && "$recorded_id" != "$id" ]]; then
      if [[ "$LEDGER_PHASE" != replacing ]] ||
        ! jq -e --arg name "$name" '.replace_names | index($name) != null' "$LEDGER" >/dev/null; then
        finding container_id_mismatch "$name"
      fi
    fi
    owner="$(jq -r '.[0].Config.Labels["io.trojanpanelnext.deployment"] // ""' <<<"$data")"
    digest="$(jq -r '.[0].Config.Labels["io.trojanpanelnext.owner-sha256"] // ""' <<<"$data")"
    role="$(jq -r '.[0].Config.Labels["io.trojanpanelnext.role"] // ""' <<<"$data")"
    [[ "$owner" == "$DEPLOYMENT_ID" && "$digest" == "$OWNER_SHA256" &&
      "$role" == "$(jq -r '.role' <<<"$expected")" ]] || finding container_label_mismatch "$name"
    image_id="$(jq -r '.[0].Image // ""' <<<"$data")"
    image_ref="$(jq -r '.[0].Config.Image // ""' <<<"$data")"
    [[ "$image_ref" == "$(jq -r --arg key "$(jq -r '.image' <<<"$expected")" '.images[$key].reference' <<<"$PLAN_JSON")" ]] ||
      finding container_image_mismatch "$name"
    if ! resolved_image="$(docker image inspect "$image_ref" 2>/dev/null)" ||
      ! jq -e 'type == "array" and length == 1 and .[0].Id != null' <<<"$resolved_image" >/dev/null ||
      [[ "$image_id" != "$(jq -r '.[0].Id' <<<"$resolved_image")" ]]; then
      finding container_image_id_unobservable "$name"
    fi
    if [[ -n "$recorded_id" && "$recorded_id" == "$id" ]]; then
      [[ "$image_id" == "$(jq -r --arg name "$name" '.containers[] | select(.name == $name) | .image_id' "$LEDGER")" ]] ||
        finding container_image_mismatch "$name"
    fi
    local recorded_image_id
    recorded_image_id="$(jq -r --arg ref "$image_ref" '.images[] | select(.reference == $ref) | .id' "$LEDGER")"
    [[ -z "$recorded_image_id" || "$image_id" == "$recorded_image_id" ]] || finding container_image_id_unlinked "$name"
    mounts="$(jq -c '.[0].Mounts | map({source:.Source,target:.Destination,read_only:(.RW | not)}) | sort_by(.source,.target)' <<<"$data")"
    expected_mounts="$(jq -c '.mounts | sort_by(.source,.target)' <<<"$expected")"
    [[ "$mounts" == "$expected_mounts" ]] || finding container_mount_mismatch "$name"
  done <<<"$ids"
  # A ledger ID that vanished is a separate interrupted-state observation.
  if [[ "$LEDGER_PRESENT" == 1 ]]; then
    while IFS= read -r name; do
      [[ -n "$name" ]] || continue
      if [[ "$LEDGER_PHASE" == replacing ]] &&
        jq -e --arg name "$name" '.replace_names | index($name) != null' "$LEDGER" >/dev/null; then
        continue
      fi
      if ! jq -e --arg name "$name" --argjson ids "$(printf '%s\n' "$ids" | jq -Rsc 'split("\n")')" \
        '.containers[] | select(.name == $name and (.id | IN($ids[])))' "$LEDGER" >/dev/null; then
        finding recorded_container_missing "$name"
      fi
    done < <(jq -r '.containers[].name' "$LEDGER")
  fi
}

inspect_images() {
  local key ref repository data id refs tags recorded listings listed_repository listed_tag listed_digest listed_id found listed_ids
  if ! listings="$(docker image ls -a --no-trunc --digests --format '{{.Repository}}|{{.Tag}}|{{.Digest}}|{{.ID}}' 2>/dev/null)"; then
    finding image_enumeration_unavailable images
    return
  fi
  while IFS= read -r key; do
    ref="$(jq -r --arg key "$key" '.images[$key].reference' <<<"$PLAN_JSON")"
    repository="${ref%@*}"
    found=0
    listed_ids=""
    while IFS='|' read -r listed_repository listed_tag listed_digest listed_id; do
      [[ "$listed_repository" == "$repository" ]] || continue
      if [[ "$listed_repository@$listed_digest" == "$ref" ]]; then
        found=1
        listed_ids+=" $listed_id"
      else
        finding image_reference_drift "$listed_repository"
      fi
      [[ "$listed_tag" == '<none>' ]] || finding image_reference_drift "$listed_repository:$listed_tag"
    done <<<"$listings"
    if [[ "$found" != 1 ]]; then
      if [[ "$LEDGER_PRESENT" == 1 ]] && jq -e --arg ref "$ref" '.images[] | select(.reference == $ref)' "$LEDGER" >/dev/null; then
        finding recorded_image_unobservable "$ref"
      fi
      continue
    fi
    [[ "$LEDGER_PRESENT" == 1 ]] || finding existing_image_without_ledger "$ref"
    if ! data="$(docker image inspect "$ref" 2>/dev/null)"; then
      finding image_unobservable "$ref"
      continue
    fi
    if ! jq -e 'type == "array" and length == 1 and .[0].Id != null and (.[0].RepoDigests | type == "array")' <<<"$data" >/dev/null; then
      finding image_unobservable "$ref"
      continue
    fi
    id="$(jq -r '.[0].Id' <<<"$data")"
    [[ " $listed_ids " == *" $id "* ]] || finding image_enumeration_mismatch "$ref"
    refs="$(jq -r '.[0].RepoDigests[]' <<<"$data")"
    tags="$(jq -r '.[0].RepoTags[]?' <<<"$data")"
    [[ "$refs" == "$ref" && -z "$tags" ]] || finding image_reference_drift "$ref"
  if [[ "$LEDGER_PRESENT" == 1 ]]; then
      recorded="$(jq -r --arg ref "$ref" '.images[] | select(.reference == $ref) | .id' "$LEDGER")"
      [[ -z "$recorded" || "$recorded" == "$id" ]] || finding image_id_mismatch "$ref"
    fi
  done < <(jq -r '.images | keys[]' <<<"$PLAN_JSON")
}

inspect_ports() {
  local sockets row protocol port name id pids socket_pid owned
  if ! sockets="$(ss -H -lntup 2>/dev/null)"; then
    finding listener_observation_unavailable host
    return
  fi
  while IFS=$'\t' read -r name protocol port; do
    [[ -n "$name" ]] || continue
    while IFS= read -r row; do
      [[ -n "$row" ]] || continue
      [[ "$(awk '{print $1}' <<<"$row")" == "$protocol" ]] || continue
      [[ "$(awk '{print $5}' <<<"$row")" == *":$port" ]] || continue
      if [[ "$LEDGER_PRESENT" != 1 ]]; then
        finding occupied_listener "$protocol/$port"
        continue
      fi
      id="$(jq -r --arg name "$name" '.containers[] | select(.name == $name) | .id' "$LEDGER")"
      if [[ -z "$id" ]] || ! pids="$(docker top "$id" -eo pid 2>/dev/null)"; then
        finding listener_owner_unobservable "$protocol/$port"
        continue
      fi
      owned=0
      while IFS= read -r socket_pid; do
        [[ "$socket_pid" =~ ^[0-9]+$ ]] || continue
        if awk -v pid="$socket_pid" 'NR > 1 && $1 == pid { found=1 } END { exit !found }' <<<"$pids"; then
          owned=1
        fi
      done < <(grep -oE 'pid=[0-9]+' <<<"$row" | cut -d= -f2)
      [[ "$owned" == 1 ]] || finding listener_owner_mismatch "$protocol/$port"
    done <<<"$sockets"
  done < <(jq -r '.containers[] | .name as $name | .ports[] | [$name,.protocol,(.port|tostring)] | @tsv' <<<"$PLAN_JSON")
}

inspect_legacy_layout() {
  local parent path
  parent="$(dirname -- "$ROOT")"
  for path in trojan-panel trojan-panel-ui trojan-panel-core mariadb redis \
    custom/web-caddy custom/node-caddy trojanpanelnext-entry trojanpanelnext-installer; do
    if [[ -e "$parent/$path" || -L "$parent/$path" ]]; then
      finding legacy_layout "$parent/$path"
    fi
  done
}

check() {
  FINDINGS=()
  ROOT="$(jq -r '.root' <<<"$PLAN_JSON")"
  DEPLOYMENT_ID="$(jq -r '.deployment_id' <<<"$PLAN_JSON")"
  MODE="$(jq -r '.mode' <<<"$PLAN_JSON")"
  [[ "$ROOT" == /tpdata/trojanpanelnext || "${TP_TEST_DATA_ROOT:-0}" == 1 ]] || finding unsupported_root "$ROOT"
  safe_path "$ROOT" directory || finding unsafe_root "$ROOT"
  if [[ -d "$ROOT" ]] && { [[ "$(stat -c %u -- "$ROOT")" != "$EUID" ]] ||
    (( (8#$(stat -c %a -- "$ROOT") & 0022) != 0 )); }; then
    finding unsafe_root_permissions "$ROOT"
  fi
  MARKER="$ROOT/.trojanpanelnext-owner-token"
  LEDGER="${LEDGER_OVERRIDE:-$ROOT/trojanpanelnext-installer/ownership.json}"
  safe_path "$MARKER" file || finding unsafe_marker "$MARKER"
  safe_path "$LEDGER" file || finding unsafe_ledger "$LEDGER"
  LEDGER_PRESENT=0
  LEDGER_PHASE=none
  if [[ -e "$LEDGER" || -L "$LEDGER" || -e "$MARKER" || -L "$MARKER" ]]; then
    LEDGER_PRESENT=1
    if ! secure_file "$LEDGER" || ! secure_file "$MARKER" || ! valid_ledger; then
      finding invalid_ownership_record "$LEDGER"
      OWNER_SHA256=""
      LEDGER_PRESENT=0
    else
      LEDGER_PHASE="$(jq -r '.phase' "$LEDGER")"
      OWNER_SHA256="$(sha256sum "$MARKER" | awk '{print $1}')"
      [[ "$OWNER_SHA256" == "$(jq -r '.owner_sha256' "$LEDGER")" ]] || finding owner_marker_mismatch "$MARKER"
      [[ "$(jq -c '.roles | sort' "$LEDGER")" == "$(jq -c '.roles | sort' <<<"$PLAN_JSON")" ]] || finding role_mismatch "$LEDGER"
    fi
  else
    OWNER_SHA256=""
    if [[ -e "$ROOT" ]]; then
      finding existing_root_without_ledger "$ROOT"
    fi
  fi
  inspect_containers
  inspect_images
  inspect_ports
  inspect_legacy_layout
  local status findings_json
  case "$LEDGER_PHASE" in
  none) status=ready_new ;;
  committed) status=ready ;;
  preparing | replacing) status=ready_resume ;;
  esac
  ((${#FINDINGS[@]} == 0)) || status=blocked
  findings_json="$(printf '%s\n' "${FINDINGS[@]}" | jq -sc 'map(select(type == "object"))')"
  jq -cn --arg status "$status" --arg root "$ROOT" --argjson findings "$findings_json" \
    '{schema_version:1,status:$status,root:$root,findings:$findings}'
  case "$status" in ready | ready_new) return 0 ;; ready_resume) return 3 ;; *) return 1 ;; esac
}

begin() {
  local report token temporary status=0 parent staging ledger_parent
  report="$(check)" || status=$?
  [[ "$status" == 0 || "$status" == 3 ]] || { printf '%s\n' "$report"; return 1; }
  ROOT="$(jq -r '.root' <<<"$PLAN_JSON")"
  DEPLOYMENT_ID="$(jq -r '.deployment_id' <<<"$PLAN_JSON")"
  MODE="$(jq -r '.mode' <<<"$PLAN_JSON")"
  MARKER="$ROOT/.trojanpanelnext-owner-token"
  LEDGER="$ROOT/trojanpanelnext-installer/ownership.json"
  LEDGER_PRESENT=0
  [[ -e "$LEDGER" ]] && LEDGER_PRESENT=1
  if [[ "$(jq -r '.status' <<<"$report")" != ready_new ]]; then
    printf '%s\n' "$report"
    return 0
  fi
  # Stage the marker and ledger before exposing a new root. A crash before the
  # atomic rename leaves no ambiguous partially owned root to adopt on replay.
  parent="$(dirname -- "$ROOT")"
  if [[ ! -d "$parent" ]]; then
    install -d -m 0700 -- "$parent"
  fi
  safe_path "$ROOT" directory || return 1
  [[ ! -e "$ROOT" && ! -L "$ROOT" ]] || return 1
  staging="$(mktemp -d "$parent/.trojanpanelnext-staging.XXXXXX")"
  chmod 0700 "$staging"
  ledger_parent="$staging/trojanpanelnext-installer"
  install -d -m 0700 -- "$ledger_parent"
  token="$(openssl rand -hex 32)"
  temporary="$staging/.trojanpanelnext-owner-token"
  : >"$temporary"
  chmod 0600 "$temporary"
  printf '%s' "$token" >"$temporary"
  temporary="$ledger_parent/ownership.json"
  : >"$temporary"
  chmod 0600 "$temporary"
  jq -cn --arg id "$DEPLOYMENT_ID" --arg root "$ROOT" --arg mode "$MODE" \
    --arg sha "$(sha256sum "$staging/.trojanpanelnext-owner-token" | awk '{print $1}')" \
    --argjson roles "$(jq -c '.roles' <<<"$PLAN_JSON")" --argjson plan "$PLAN_JSON" \
    '{schema_version:1,deployment_id:$id,root:$root,mode:$mode,roles:$roles,
      owner_sha256:$sha,phase:"preparing",plan:$plan,containers:[],images:[]}' >"$temporary"
  [[ ! -e "$ROOT" && ! -L "$ROOT" ]] || return 1
  mv -T -- "$staging" "$ROOT"
  jq -cn --arg status preparing --arg root "$ROOT" '{schema_version:1,status:$status,root:$root,findings:[]}'
}

commit() {
  local marker ledger candidate original_sha name ref data id image_id container_records image_records report check_status=0
  ROOT="$(jq -r '.root' <<<"$PLAN_JSON")"
  DEPLOYMENT_ID="$(jq -r '.deployment_id' <<<"$PLAN_JSON")"
  MODE="$(jq -r '.mode' <<<"$PLAN_JSON")"
  marker="$ROOT/.trojanpanelnext-owner-token"
  ledger="$ROOT/trojanpanelnext-installer/ownership.json"
  if ! safe_path "$ROOT" directory || ! secure_file "$marker" ||
    ! secure_file "$ledger"; then
    jq -cn --arg resource "$ledger" '{schema_version:1,status:"blocked",findings:[{code:"invalid_ownership_record",resource:$resource}]}'
    return 1
  fi
  LEDGER="$ledger"
  valid_ledger || return 1
  [[ "$(jq -r '.phase' "$ledger")" == preparing || "$(jq -r '.phase' "$ledger")" == replacing ]] || return 1
  report="$(check)" || check_status=$?
  [[ "$check_status" == 3 ]] || { printf '%s\n' "$report"; return 1; }
  original_sha="$(sha256sum "$ledger" | awk '{print $1}')"
  [[ "$(sha256sum "$marker" | awk '{print $1}')" == "$(jq -r '.owner_sha256' "$ledger")" ]] || return 1
  container_records=""
  while IFS= read -r name; do
    data="$(docker inspect --type container "$name" 2>/dev/null)" || return 1
    jq -e 'type == "array" and length == 1' <<<"$data" >/dev/null || return 1
    id="$(jq -r '.[0].Id' <<<"$data")"
    image_id="$(jq -r '.[0].Image' <<<"$data")"
    container_records+="$(jq -cn --arg name "$name" --arg id "$id" --arg image_id "$image_id" \
      '{name:$name,id:$id,image_id:$image_id}')"$'\n'
  done < <(jq -r '.containers[].name' <<<"$PLAN_JSON")
  image_records=""
  while IFS= read -r ref; do
    data="$(docker image inspect "$ref" 2>/dev/null)" || return 1
    jq -e 'type == "array" and length == 1' <<<"$data" >/dev/null || return 1
    id="$(jq -r '.[0].Id' <<<"$data")"
    image_records+="$(jq -cn --arg reference "$ref" --arg id "$id" \
      '{reference:$reference,id:$id}')"$'\n'
  done < <(jq -r '.images[].reference' <<<"$PLAN_JSON")
  candidate="$(mktemp "$ROOT/trojanpanelnext-installer/.ownership.XXXXXX")" || return 1
  chmod 0600 "$candidate"
  jq --argjson containers "$(jq -sc . <<<"$container_records")" \
    --argjson images "$(jq -sc . <<<"$image_records")" \
    '.phase="committed" | .containers=$containers | .images=$images |
     del(.target_plan,.replace_names)' "$ledger" >"$candidate"
  LEDGER_OVERRIDE="$candidate"
  if ! report="$(check)"; then
    unset LEDGER_OVERRIDE
    rm -f -- "$candidate"
    printf '%s\n' "$report"
    return 1
  fi
  unset LEDGER_OVERRIDE
  if [[ "$(sha256sum "$ledger" | awk '{print $1}')" != "$original_sha" ]]; then
    rm -f -- "$candidate"
    return 1
  fi
  mv -T -- "$candidate" "$ledger"
  jq -cn --arg root "$ROOT" '{schema_version:1,status:"committed",root:$root,findings:[]}'
}

prepare_replace() {
  local report status=0 ledger candidate original_sha name
  report="$(check)" || status=$?
  [[ "$status" == 0 && "$(jq -r '.status' <<<"$report")" == ready ]] || {
    printf '%s\n' "$report"
    return 1
  }
  [[ "${#REPLACE_NAMES[@]}" -gt 0 ]] || return 2
  for name in "${REPLACE_NAMES[@]}"; do
    jq -e --arg name "$name" '.containers | any(.[]; .name == $name)' <<<"$PLAN_JSON" >/dev/null || return 2
  done
  ledger="$(jq -r '.root' <<<"$PLAN_JSON")/trojanpanelnext-installer/ownership.json"
  original_sha="$(sha256sum "$ledger" | awk '{print $1}')"
  candidate="$(mktemp "${ledger%/*}/.ownership.XXXXXX")" || return 1
  chmod 0600 "$candidate"
  jq --argjson plan "$PLAN_JSON" --argjson names "$(printf '%s\n' "${REPLACE_NAMES[@]}" | jq -Rsc 'split("\n") | map(select(length > 0)) | unique')" \
    '.phase="replacing" | .target_plan=$plan | .replace_names=$names' "$ledger" >"$candidate"
  [[ "$(sha256sum "$ledger" | awk '{print $1}')" == "$original_sha" ]] || {
    rm -f -- "$candidate"; return 1;
  }
  mv -T -- "$candidate" "$ledger"
  jq -cn '{schema_version:1,status:"ready_resume",findings:[]}'
}

checkpoint() {
  local report status=0 ledger candidate original_sha phase name data id image_id records ids
  report="$(check)" || status=$?
  [[ "$status" == 3 ]] || { printf '%s\n' "$report"; return 1; }
  ledger="$(jq -r '.root' <<<"$PLAN_JSON")/trojanpanelnext-installer/ownership.json"
  phase="$(jq -r '.phase' "$ledger")"
  [[ "$phase" == preparing ]] || {
    # Replacement keeps the old exact IDs until the final verified commit.
    printf '%s\n' "$report"; return 0;
  }
  original_sha="$(sha256sum "$ledger" | awk '{print $1}')"
  ids="$(docker ps -a --no-trunc --format '{{.ID}}' 2>/dev/null)" || return 1
  records=""
  while IFS= read -r id; do
    [[ -n "$id" ]] || continue
    data="$(docker inspect --type container "$id" 2>/dev/null)" || return 1
    jq -e 'type == "array" and length == 1' <<<"$data" >/dev/null || return 1
    name="$(jq -r '.[0].Name | ltrimstr("/")' <<<"$data")"
    jq -e --arg name "$name" '.containers | any(.[]; .name == $name)' <<<"$PLAN_JSON" >/dev/null || continue
    image_id="$(jq -r '.[0].Image' <<<"$data")"
    records+="$(jq -cn --arg name "$name" --arg id "$id" --arg image_id "$image_id" \
      '{name:$name,id:$id,image_id:$image_id}')"$'\n'
  done <<<"$ids"
  candidate="$(mktemp "${ledger%/*}/.ownership.XXXXXX")" || return 1
  chmod 0600 "$candidate"
  jq --argjson containers "$(jq -sc 'map(select(type == "object"))' <<<"$records")" \
    '.containers=$containers' "$ledger" >"$candidate"
  LEDGER_OVERRIDE="$candidate"
  status=0
  report="$(check)" || status=$?
  [[ "$status" == 3 ]] || { unset LEDGER_OVERRIDE; rm -f -- "$candidate"; printf '%s\n' "$report"; return 1; }
  unset LEDGER_OVERRIDE
  [[ "$(sha256sum "$ledger" | awk '{print $1}')" == "$original_sha" ]] || { rm -f -- "$candidate"; return 1; }
  mv -T -- "$candidate" "$ledger"
  printf '%s\n' "$report"
}

main() {
  local command="${1:-}"; shift || true
  local lock_fd root parent
  PLAN=""
  REPLACE_NAMES=()
  case "$command" in check | begin | checkpoint | commit | prepare-replace) ;; *) usage; return 2 ;; esac
  unset LEDGER_OVERRIDE || true
  while (($#)); do
    case "$1" in
    --plan) [[ $# -ge 2 ]] || { usage; return 2; }; PLAN="$2"; shift 2 ;;
    --replace) [[ "$command" == prepare-replace && $# -ge 2 ]] || { usage; return 2; }; REPLACE_NAMES+=("$2"); shift 2 ;;
    *) usage; return 2 ;;
    esac
  done
  [[ -n "$PLAN" ]] || { usage; return 2; }
  if [[ "$PLAN" == - ]]; then
    PLAN_JSON="$(cat)"
  else
    [[ -f "$PLAN" && ! -L "$PLAN" ]] || { usage; return 2; }
    PLAN_JSON="$(cat -- "$PLAN")"
  fi
  PLAN_JSON="$(jq -c . <<<"$PLAN_JSON" 2>/dev/null)" || {
    printf '{"schema_version":1,"status":"blocked","findings":[{"code":"invalid_plan","resource":"plan"}]}\n'; return 2;
  }
  valid_plan || { printf '{"schema_version":1,"status":"blocked","findings":[{"code":"invalid_plan","resource":"plan"}]}\n'; return 2; }
  if [[ "$command" != check ]]; then
    # The fixed existing inode serializes even when the data root does not yet
    # exist. No lock file or directory is created by read-only check.
    exec {lock_fd}</
    flock -x "$lock_fd"
  fi
  case "$command" in
  prepare-replace) prepare_replace ;;
  *) "$command" ;;
  esac
}

main "$@"
