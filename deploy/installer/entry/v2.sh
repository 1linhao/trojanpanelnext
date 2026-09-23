#!/usr/bin/env bash

# Combined contract and transaction engine. Adapters are injected by sourcing
# this file with entry_v2_adapter_* functions; production has no v2 mutator yet.

entry_v2_validate_spec() {
  jq -e '
    def fqdn: type == "string" and length <= 253 and
      test("^(?=.{1,253}$)[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)+$");
    def absolute: type == "string" and test("^/[^/]");
    def keys_only($allowed): (keys - $allowed | length) == 0;
    . as $spec | .schema_version == 2 and .topology == "combined" and
    keys_only(["schema_version","topology","revision","deployment_id","provider","domains","active_roles","roles","certificate_targets","restore_intent"]) and
    (.revision | type == "number" and floor == . and . >= 1) and
    (.deployment_id | type == "string" and test("^[a-z][a-z0-9-]{0,62}$")) and
    .provider == "caddy-legacy" and
    (.domains | type == "object" and keys == ["node","web"] and
      (.web | fqdn) and (.node | fqdn) and .web != .node) and
    (.active_roles | type == "array" and length >= 1 and length <= 2 and
      length == (unique | length) and all(.[]; . == "web" or . == "node")) and
    (.roles | type == "object" and keys == ($spec.active_roles | sort)) and
    (.certificate_targets | type == "object" and keys == ($spec.active_roles | sort)) and
    (if .active_roles | index("web") then
      (.roles.web | type == "object" and keys == ["web_upstream"] and
        (.web_upstream | type == "string" and test("^(127\\.0\\.0\\.1|\\[::1\\]):[0-9]{1,5}$")))
     else true end) and
    (if .active_roles | index("node") then
      (.roles.node | type == "object" and keys == ["certificate_consumer","node_exposure","route_manifest"] and
        .node_exposure == "direct" and (.route_manifest | absolute) and
        (.certificate_consumer | absolute))
     else true end) and
    (.certificate_targets | to_entries | all(.[];
      (.value | type == "object" and
        keys == ["cert_path","key_path","managed_dir","renewal_owner"] and
        (.managed_dir | absolute) and (.cert_path | absolute) and
        (.key_path | absolute) and .cert_path != .key_path and
        .renewal_owner == "caddy-legacy"))) and
    (if .active_roles | length == 2 then
      ([.certificate_targets.web.cert_path,.certificate_targets.web.key_path,
        .certificate_targets.node.cert_path,.certificate_targets.node.key_path] |
       length == (unique | length))
     else true end) and
    (if has("restore_intent") then
      (.restore_intent | type == "object" and
        keys == ["expected_committed_digest","roles"] and
        (.expected_committed_digest | type == "string" and test("^[a-f0-9]{64}$")) and
        (.roles | type == "array" and length > 0 and length == (unique | length) and
          all(.[]; . == "web" or . == "node")))
     else true end)
  ' "$1" >/dev/null 2>&1
}

entry_v2_validate_state() {
  jq -e '
    def hash: type == "string" and test("^[a-f0-9]{64}$");
    def fqdn: type == "string" and length <= 253 and
      test("^(?=.{1,253}$)[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)+$");
    def absolute: type == "string" and test("^/[^/]");
    def valid_spec($deployment;$domains;$provider):
      . as $spec |
      type == "object" and
      (keys - ["schema_version","topology","revision","deployment_id","provider","domains","active_roles","roles","certificate_targets","restore_intent"] | length) == 0 and
      .schema_version == 2 and .topology == "combined" and
      (.revision | type == "number" and floor == . and . >= 1) and
      .deployment_id == $deployment and .provider == $provider and
      (.domains | type == "object" and keys == ["node","web"] and
        (.web | fqdn) and (.node | fqdn) and .web != .node) and
      .domains == $domains and
      (.active_roles | type == "array" and length >= 1 and length <= 2 and
        length == (unique | length) and all(.[]; . == "web" or . == "node")) and
      (.roles | type == "object" and keys == ($spec.active_roles | sort)) and
      (.certificate_targets | type == "object" and keys == ($spec.active_roles | sort)) and
      (if .active_roles | index("web") then
        (.roles.web | type == "object" and keys == ["web_upstream"] and
          (.web_upstream | type == "string" and test("^(127\\.0\\.0\\.1|\\[::1\\]):[0-9]{1,5}$")))
       else true end) and
      (if .active_roles | index("node") then
        (.roles.node | type == "object" and keys == ["certificate_consumer","node_exposure","route_manifest"] and
          .node_exposure == "direct" and (.route_manifest | absolute) and (.certificate_consumer | absolute))
       else true end) and
      (.certificate_targets | to_entries | all(.[];
        (.value | type == "object" and
          keys == ["cert_path","key_path","managed_dir","renewal_owner"] and
          (.managed_dir | absolute) and (.cert_path | absolute) and (.key_path | absolute) and
          .cert_path != .key_path and .renewal_owner == "caddy-legacy"))) and
      (if .active_roles | length == 2 then
        ([.certificate_targets.web.cert_path,.certificate_targets.web.key_path,
          .certificate_targets.node.cert_path,.certificate_targets.node.key_path] |
         length == (unique | length))
       else true end) and
      (if has("restore_intent") then
        (.restore_intent | type == "object" and keys == ["expected_committed_digest","roles"] and
          (.expected_committed_digest | type == "string" and test("^[a-f0-9]{64}$")) and
          (.roles | type == "array" and length > 0 and length == (unique | length) and
            all(.[]; . == "web" or . == "node")))
       else true end);
    def resource($deployment):
      type == "object" and
      (keys - ["kind","id","owner","deployment_id","scope","role","retention","identity"] | length) == 0 and
      (.kind | type == "string" and length > 0) and
      (.id | type == "string" and length > 0) and
      (.owner == "controller" or .owner == "provider" or .owner == "external") and
      .deployment_id == $deployment and
      (.scope == "shared" or .scope == "role") and
      (if .scope == "role" then .role == "web" or .role == "node" else (has("role") | not) end) and
      (.retention == "managed" or .retention == "preserve") and
        (.identity | type == "object" and keys == ["digest","marker"] and
        (.digest | hash) and (.marker | type == "string" and length > 0));
    def certificate:
      type == "object" and
      keys == ["cert_path","domain","fingerprint","generation","key_path","last_hook_status","not_after","renewal_owner"] and
      (.domain | fqdn) and (.cert_path | absolute) and (.key_path | absolute) and
      (.fingerprint | type == "string" and length > 0) and
      (.generation | type == "number" and floor == . and . >= 1) and
      .renewal_owner == "caddy-legacy" and
      (.last_hook_status == "ok" or .last_hook_status == "unchanged" or
       .last_hook_status == "failed" or .last_hook_status == "unknown") and
      (.not_after | type == "string" and test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T"));
    . as $state | .schema_version == 2 and .topology == "combined" and
    (keys - ["schema_version","topology","deployment_id","desired_revision","observed_revision","desired_digest","generation","active_provider","phase","health","domains","active_roles","committed_target","candidate_target","resources","previous_resources","candidate_resources","certificates","listeners","capabilities","last_error"] | length) == 0 and
    (.deployment_id | type == "string" and test("^[a-z][a-z0-9-]{0,62}$")) and
    (.desired_revision | type == "number" and floor == . and . >= 1) and
    (.observed_revision | type == "number" and floor == . and . >= 0) and
    (.generation | type == "number" and floor == . and . >= 0) and
    .active_provider == "caddy-legacy" and
    (.phase as $p | ["stable","preparing","prepared","activating","verifying","rolling_back","failed"] | index($p) != null) and
    (.health as $h | ["healthy","degraded","unhealthy","unknown"] | index($h) != null) and
    (.domains | type == "object" and keys == ["node","web"] and .node != .web) and
    (.active_roles | type == "array" and length >= 1 and length == (unique | length)) and
    (.desired_digest | hash) and
    (.committed_target == null or
      (.committed_target | type == "object" and keys == ["digest","spec"] and (.digest | hash) and
        (.spec | valid_spec($state.deployment_id;$state.domains;$state.active_provider)))) and
    (.candidate_target == null or
      (.candidate_target | type == "object" and keys == ["digest","spec"] and (.digest | hash) and
        (.spec | valid_spec($state.deployment_id;$state.domains;$state.active_provider)))) and
    (.resources | type == "array" and all(.[]; resource($state.deployment_id) and (if .scope == "role" then (.role as $r | $state.active_roles | index($r) != null) else true end))) and
    (.previous_resources | type == "array" and all(.[]; resource($state.deployment_id))) and
    (.candidate_resources | type == "array" and all(.[]; resource($state.deployment_id) and (if .scope == "role" then (.role as $r | $state.active_roles | index($r) != null) else true end))) and
    (.certificates | type == "object" and
      all(to_entries[]; (.key == "web" or .key == "node") and (.value | certificate))) and
    (.listeners | type == "array" and all(.[];
      type == "object" and
      (keys - ["transport","address","port","purpose","owner","scope","role"] | length) == 0 and
      .transport == "tcp" and (.address | type == "string") and
      (.port | type == "number" and floor == . and . >= 1 and . <= 65535) and
      (.purpose | type == "string" and length > 0) and
      (.owner == "provider" or .owner == "kernel") and
      (.scope == "shared" or .scope == "role") and
      (if .scope == "role" then .role == "node" or .role == "web" else (has("role") | not) end))) and
    (.capabilities | type == "array" and all(.[]; type == "string")) and
    (if .phase == "stable" then .candidate_target == null else .candidate_target != null and .candidate_target.digest == .desired_digest end)
  ' "$1" >/dev/null 2>&1
}

entry_v2_verify_snapshot_file() {
  local state_file="$1" kind target_json
  for kind in committed candidate; do
    target_json="$(jq -c ".${kind}_target" "$state_file")" || return 1
    [[ "$target_json" == null ]] && continue
    [[ "$(jq -r '.digest' <<<"$target_json")" == "$(jq -S -c '.spec | del(.revision, .restore_intent) | .active_roles |= sort' <<<"$target_json" | sha256sum | awk '{print $1}')" ]] || return 1
  done
}

entry_v2_target_digest() {
  jq -S -c 'del(.revision, .restore_intent) | .active_roles |= sort' "$1" | sha256sum | awk '{print $1}'
}

entry_v2_error() {
  local code="$1" phase="$2" message="$3" resource="${4:-trojanpanelnext-combined}" \
    retryable="${5:-false}" rollback_status="${6:-not-needed}"
  jq -cn --arg code "$code" --arg phase "$phase" --arg message "$message" \
    --arg resource "$resource" --argjson retryable "$retryable" --arg rollback_status "$rollback_status" \
    '{schema_version:2,code:$code,phase:$phase,resource:$resource,retryable:$retryable,rollback_status:$rollback_status,message:$message}'
}

entry_v2_adapter_available() {
  local action
  for action in probe prepare activate verify rollback remove; do
    declare -F "entry_v2_adapter_${action}" >/dev/null || return 1
  done
}

entry_v2_plan() {
  local spec="$1" root="$2" existing="" digest phase=stable action=prepare_target
  digest="$(entry_v2_target_digest "$spec")"
  local path
  path="$(entry_state_path "$root" "$(jq -r '.deployment_id' "$spec")")" || return 2
  if [[ -e "$path" || -L "$path" ]]; then
    existing="$(entry_read_state "$root" "$(jq -r '.deployment_id' "$spec")")" || {
      entry_v2_error ownership_conflict stable 'Existing journal is invalid or belongs to v1'; return 4;
    }
    phase="$(jq -r '.phase' <<<"$existing")"
    entry_v2_check_target "$spec" "$existing" "$digest" || return $?
    if [[ "$phase" == stable && "$(jq -r '.health' <<<"$existing")" == healthy && "$digest" == "$(jq -r '.committed_target.digest // ""' <<<"$existing")" ]]; then
      action=unchanged
    elif [[ "$phase" != stable ]]; then
      action=recover_then_reconcile
    else
      action=reconcile
    fi
  fi
  if [[ "$existing" == "" && "$(jq -r '.active_roles | length' "$spec")" != 2 ]]; then
    entry_v2_error invalid_spec stable 'Initial combined deployment requires both roles'; return 2
  fi
  jq -cn --argjson spec "$(jq -c . "$spec")" --arg digest "$digest" \
    --arg action "$action" --arg phase "$phase" \
    '{schema_version:2,deployment_id:$spec.deployment_id,desired_revision:$spec.revision,target_digest:$digest,provider:$spec.provider,topology:"combined",active_roles:$spec.active_roles,actions:[$action],current_phase:$phase,mutation_enabled:false,executable:false}'
}

# A v1 journal never proves ownership of a combined deployment. The same
# revision is immutable; role expansion after removal needs an explicit intent.
entry_v2_check_target() {
  local spec="$1" state="$2" digest="$3" revision known old_digest added committed_roles
  [[ "$(jq -r '.schema_version' <<<"$state")" == 2 ]] || {
    entry_v2_error ownership_conflict stable 'v1 resources require an explicit migration'; return 4;
  }
  jq -e --argjson spec "$(jq -c . "$spec")" '.deployment_id == $spec.deployment_id and .domains == $spec.domains and .active_provider == $spec.provider' <<<"$state" >/dev/null || {
    entry_v2_error ownership_conflict stable 'Deployment owner, provider or domain anchor changed'; return 4;
  }
  revision="$(jq -r '.revision' "$spec")"
  known="$(jq -r '.desired_revision' <<<"$state")"
  old_digest="$(jq -r '.desired_digest' <<<"$state")"
  if (( revision < known )) || { (( revision == known )) && [[ "$digest" != "$old_digest" ]]; }; then
    entry_v2_error invalid_spec stable 'Revision is stale or reuses a revision for a different target'; return 2
  fi
  committed_roles="$(jq -c '.committed_target.spec.active_roles // .active_roles' <<<"$state")"
  added="$(jq -cn --argjson spec "$(jq -c . "$spec")" --argjson committed_roles "$committed_roles" '$spec.active_roles - $committed_roles')"
  if [[ "$added" != '[]' ]]; then
    jq -e --argjson added "$added" --argjson state "$state" \
      '.restore_intent.roles == $added and .restore_intent.expected_committed_digest == $state.committed_target.digest' "$spec" >/dev/null || {
        entry_v2_error invalid_spec stable 'Restoring a removed role requires matching restore_intent'; return 2;
      }
  fi
}

entry_v2_validate_observation() {
  local observation="$1" spec="$2" mode="$3" journal="${4:-}"
  jq -e --argjson spec "$(jq -c . "$spec")" --arg mode "$mode" '
    .schema_version == 2 and .deployment_id == $spec.deployment_id and
    .provider == $spec.provider and .ownership_verified == true and
    (.resources | type == "array") and (.candidate_resources | type == "array") and
    (.resources | all(.[]; .owner == "controller" or .owner == "provider")) and
    (.candidate_resources | all(.[]; .owner == "controller" or .owner == "provider")) and
    (.resources | all(.[]; if $mode != "verify" and $mode != "prepare" or .scope != "role" then true else (.role as $r | $spec.active_roles | index($r) != null) end)) and
    (.candidate_resources | all(.[]; if $mode != "verify" and $mode != "prepare" or .scope != "role" then true else (.role as $r | $spec.active_roles | index($r) != null) end)) and
    (.candidate_resources | map(.kind + ":" + .id) | length == (unique | length)) and
    (.listeners | type == "array") and (.capabilities | type == "array") and
    (.certificates | type == "object") and
    (if $mode == "verify" then
      (.certificates | keys == ($spec.active_roles | sort)) and
      (.certificates | to_entries | all(.[];
        .value.domain == $spec.domains[.key] and
        .value.cert_path == $spec.certificate_targets[.key].cert_path and
        .value.key_path == $spec.certificate_targets[.key].key_path and
        (.value.fingerprint | type == "string" and length > 0) and
        (.value.generation | type == "number" and . >= 1) and
        (.value.not_after | type == "string" and (try (fromdateiso8601 > now) catch false)) and
        (.value.renewal_owner == $spec.provider) and
        (.value.last_hook_status == "ok" or .value.last_hook_status == "unchanged"))) and
      ([.listeners[] | select(.owner == "provider" and .scope == "shared" and .port == 80)] | length == 1) and
      ([.listeners[] | select(.owner == "provider" and .scope == "shared" and .port == 443)] | length == 1) and
      (if $spec.active_roles | index("node") then
        ([.listeners[] | select(.owner == "kernel" and .scope == "role" and .role == "node" and .purpose == "node-direct")] | length >= 1)
       else true end)
     else true end)
  ' <<<"$observation" >/dev/null 2>&1 || return 1
  if [[ "${mode}" == recovery ]]; then
    [[ -n "${journal}" ]] || return 1
    jq -n -e --argjson observation "$observation" --argjson journal "$journal" '
      ($journal.resources + $journal.previous_resources + $journal.candidate_resources) as $known |
      def same_identity($a;$b):
        $a.kind == $b.kind and $a.id == $b.id and
        $a.owner == $b.owner and $a.deployment_id == $b.deployment_id and
        $a.scope == $b.scope and ($a.role // null) == ($b.role // null) and
        $a.identity.marker == $b.identity.marker and
        $a.identity.digest == $b.identity.digest;
      (all($observation.resources[]; . as $seen | any($known[]; same_identity($seen;.)))) and
      (all($observation.candidate_resources[]; . as $seen | any($known[]; same_identity($seen;.))))
    ' >/dev/null 2>&1 || return 1
    return 0
  fi
  # Reuse the state validator for all resource ownership and identity fields.
  jq -cn --argjson spec "$(jq -c . "$spec")" --argjson obs "$observation" \
    '{schema_version:2,topology:"combined",deployment_id:$spec.deployment_id,desired_revision:$spec.revision,observed_revision:0,generation:0,active_provider:$spec.provider,phase:"preparing",health:"unknown",domains:$spec.domains,active_roles:(([ $spec.active_roles[] ] + ([ $obs.resources[]?, $obs.candidate_resources[]? ] | map(select(type == "object" and .scope == "role") | .role)) | unique)),desired_digest:("0"*64),committed_target:null,candidate_target:{digest:("0"*64),spec:$spec},resources:$obs.resources,previous_resources:[],candidate_resources:$obs.candidate_resources,certificates:$obs.certificates,listeners:$obs.listeners,capabilities:$obs.capabilities}' | entry_v2_validate_state /dev/stdin
}

entry_v2_state() {
  local spec="$1" phase="$2" health="$3" digest="$4" existing="$5" observation="$6" error="${7:-null}"
  jq -cn --argjson spec "$(jq -c . "$spec")" --arg phase "$phase" --arg health "$health" \
    --arg digest "$digest" --argjson old "$existing" --argjson obs "$observation" --argjson error "$error" '
    {
      schema_version:2,topology:"combined",deployment_id:$spec.deployment_id,
      desired_revision:$spec.revision,observed_revision:($old.observed_revision // 0),
      generation:($old.generation // 0),active_provider:$spec.provider,
      phase:$phase,health:$health,domains:$spec.domains,
      # Keep both the committed and requested role sets while a transaction is
      # in flight. This lets a role removal retain its old resources for
      # rollback and lets an explicit role restoration stage new resources.
      active_roles:((($old.active_roles // []) + $spec.active_roles) | unique),desired_digest:$digest,
      committed_target:($old.committed_target // null),
      candidate_target:{digest:$digest,spec:$spec},
      resources:($old.resources // []),previous_resources:($old.resources // []),
      candidate_resources:($obs.candidate_resources // []),
      certificates:($old.certificates // {}),listeners:($old.listeners // []),
      capabilities:($old.capabilities // [])
    } + (if $error == null then {} else {last_error:$error} end)'
}

entry_v2_persist() {
  local root="$1" json="$2" temp
  temp="$(mktemp)" || return 1
  chmod 0600 "$temp"
  printf '%s\n' "$json" >"$temp"
  entry_write_state "$root" "$temp" >/dev/null
  local result=$?
  rm -f "$temp"
  return "$result"
}

entry_v2_rollback() {
  local requested_spec="$1" state="$2" rollback_spec temporary=""
  rollback_spec="${requested_spec}"
  if [[ "$(jq -r '.committed_target == null' <<<"${state}")" == true ]]; then
    ENTRY_V2_ROLLBACK_KIND=first-install
  else
    temporary="$(mktemp)" || return 1
    chmod 0600 "${temporary}"
    jq -c '.committed_target.spec' <<<"${state}" >"${temporary}" || { rm -f "${temporary}"; return 1; }
    rollback_spec="${temporary}"
    ENTRY_V2_ROLLBACK_KIND=committed-target
  fi
  entry_v2_adapter_rollback "${rollback_spec}" "${state}"
  local status=$?
  [[ -z "${temporary}" ]] || rm -f "${temporary}"
  return "${status}"
}

entry_v2_locked_state() {
  local root="$1" deployment="$2" path
  path="$(entry_state_path "$root" "$deployment")" || return 2
  if [[ -e "$path" || -L "$path" ]]; then
    [[ -f "$path" && ! -L "$path" && "$(stat -c %a "$path")" == 600 ]] || return 4
    local state
    state="$(entry_read_state "$root" "$deployment")" || return 4
    printf '%s\n' "$state"
  else
    printf 'null\n'
  fi
}

entry_v2_reconcile() ( entry_v2_reconcile_locked "$@"; )

entry_v2_reconcile_locked() {
  local spec="$1" root="$2" deployment digest old state observation verified phase result probe_mode
  entry_validate_mutating_spec_file "$spec" && entry_v2_validate_spec "$spec" || {
    entry_v2_error invalid_spec preparing 'Invalid or untrusted v2 EntrySpec'; return 2;
  }
  entry_v2_adapter_available || { entry_v2_error unsupported_capability preparing 'No combined v2 Adapter is connected'; return 3; }
  deployment="$(jq -r '.deployment_id' "$spec")"
  digest="$(entry_v2_target_digest "$spec")"
  mkdir -p "$root" && chmod 0700 "$root" || return 12
  local lock_fd
  exec {lock_fd}>"$root/.lock" || return 12
  flock -x "$lock_fd" || return 12
  old="$(entry_v2_locked_state "$root" "$deployment")" || {
    entry_v2_error ownership_conflict preparing 'Journal ownership or format is invalid'; return 4;
  }
  if [[ "$old" != null ]]; then
    entry_v2_check_target "$spec" "$old" "$digest" || return $?
    phase="$(jq -r '.phase' <<<"$old")"
    if [[ "$phase" == stable && "$(jq -r '.health' <<<"$old")" == healthy && "$digest" == "$(jq -r '.committed_target.digest // ""' <<<"$old")" ]]; then
      if [[ "$(jq -r '.revision' "$spec")" != "$(jq -r '.desired_revision' <<<"$old")" ]]; then
        old="$(jq -c --argjson spec "$(jq -c . "$spec")" '.desired_revision=$spec.revision | .observed_revision=$spec.revision | .committed_target.spec=$spec' <<<"$old")"
        entry_v2_persist "$root" "$old" || return 12
      fi
      jq -c '. + {result:"unchanged"}' <<<"$old"
      return 0
    fi
  fi
  if [[ "$old" == null && "$(jq -r '.active_roles | length' "$spec")" != 2 ]]; then
    entry_v2_error invalid_spec preparing 'Initial combined deployment requires both roles'; return 2
  fi
  # Probe is read-only. Existing resources must retain exact journaled identity.
  observation="$(entry_v2_adapter_probe "$spec" "$old")" || {
    entry_v2_error ownership_conflict preparing 'Adapter probe rejected host ownership'; return 4;
  }
  probe_mode=probe
  if [[ "$old" != null && "$(jq -r '.phase' <<<"$old")" != stable && "$(jq -r '.committed_target == null' <<<"$old")" == false ]]; then
    probe_mode=recovery
  fi
  entry_v2_validate_observation "$observation" "$spec" "$probe_mode" "$old" || {
    entry_v2_error ownership_conflict preparing 'Adapter probe returned untrusted resource identity'; return 4;
  }
  if [[ "$old" == null ]]; then
    [[ "$(jq -c '.resources' <<<"$observation")" == '[]' ]] || {
      entry_v2_error ownership_conflict preparing 'Existing resources cannot be adopted without migration'; return 4;
    }
  else
    if [[ "$(jq -r '.phase' <<<"$old")" == stable ]]; then
      jq -e --argjson obs "$observation" '.resources == $obs.resources' <<<"$old" >/dev/null
    else
      # A crash may leave a changed digest for a known candidate identity. The
      # marker and resource coordinates remain pinned by the journal; unknown
      # identities still block rollback.
      jq -e --argjson obs "$observation" '
        (.previous_resources + .candidate_resources) as $known |
        def same_identity($a;$b):
          $a.kind == $b.kind and $a.id == $b.id and
          $a.owner == $b.owner and $a.deployment_id == $b.deployment_id and
          $a.scope == $b.scope and ($a.role // null) == ($b.role // null) and
          $a.identity.marker == $b.identity.marker and
          $a.identity.digest == $b.identity.digest;
        (all($obs.resources[]; . as $seen | any($known[]; same_identity($seen;.)))) and
        (all($obs.candidate_resources[]; . as $seen | any($known[]; same_identity($seen;.))))
      ' <<<"$old" >/dev/null
    fi || {
      entry_v2_error ownership_conflict preparing 'Observed resources differ from committed identities'; return 4;
    }
  fi
  if [[ "$old" != null && "$(jq -r '.phase' <<<"$old")" != stable ]]; then
    # The old journal, not the current request, selects the rollback target.
    state="$(jq -c --argjson obs "$observation" '.phase="rolling_back" | .health="unknown" | .candidate_resources=$obs.candidate_resources' <<<"$old")"
    entry_v2_persist "$root" "$state" || return 12
    if ! entry_v2_rollback "$spec" "$state" >/dev/null; then
      state="$(jq -c '.phase="failed" | .health="unhealthy"' <<<"$state")"
      entry_v2_persist "$root" "$state" || return 12
      entry_v2_error rollback_failed rolling_back 'Crash recovery failed' "$(jq -r '.deployment_id' <<<"$state")" true failed; return 8
    fi
    if [[ "$(jq -r '.committed_target == null' <<<"$state")" == true ]]; then
      rm -f -- "$(entry_state_path "$root" "$deployment")" || return 12
      old=null
    else
      old="$(jq -c '.phase="stable" | .health="degraded" | .candidate_target=null | .candidate_resources=[]' <<<"$state")"
      entry_v2_persist "$root" "$old" || return 12
    fi
  fi
  state="$(entry_v2_state "$spec" preparing unknown "$digest" "$old" "$observation")"
  entry_v2_persist "$root" "$state" || return 12
  if ! observation="$(entry_v2_adapter_prepare "$spec" "$state")"; then
    entry_v2_fail "$spec" "$root" "$state" preparing prepare_failed || return $?
    return 5
  fi
  entry_v2_validate_observation "$observation" "$spec" prepare || {
    entry_v2_fail "$spec" "$root" "$state" preparing ownership_conflict || return $?
    return 5;
  }
  state="$(jq -c --argjson obs "$observation" '.phase="prepared" | .candidate_resources=$obs.candidate_resources' <<<"$state")"
  entry_v2_persist "$root" "$state" || return 12
  state="$(jq -c '.phase="activating"' <<<"$state")"
  entry_v2_persist "$root" "$state" || return 12
  if ! entry_v2_adapter_activate "$spec" "$state" >/dev/null; then
    entry_v2_fail "$spec" "$root" "$state" activating activation_failed || return $?
    return 6
  fi
  state="$(jq -c '.phase="verifying"' <<<"$state")"
  entry_v2_persist "$root" "$state" || return 12
  if ! verified="$(entry_v2_adapter_verify "$spec" "$state")" ||
     ! entry_v2_validate_observation "$verified" "$spec" verify; then
    entry_v2_fail "$spec" "$root" "$state" verifying verification_failed || return $?
    return 7
  fi
  # Adapter must identify exactly the candidate resources committed by this run.
  jq -e --argjson obs "$verified" '.candidate_resources == $obs.resources' <<<"$state" >/dev/null || {
    entry_v2_fail "$spec" "$root" "$state" verifying ownership_conflict || return $?
    return 7;
  }
  result="$(jq -c --argjson spec "$(jq -c . "$spec")" --argjson obs "$verified" --arg digest "$digest" '
    .phase="stable" | .health="healthy" | .desired_revision=$spec.revision |
    .observed_revision=$spec.revision | .generation += 1 |
    .active_roles=$spec.active_roles | .committed_target={digest:$digest,spec:$spec} |
    .candidate_target=null | .resources=$obs.resources | .previous_resources=[] |
    .candidate_resources=[] | .certificates=$obs.certificates |
    .listeners=$obs.listeners | .capabilities=$obs.capabilities | del(.last_error)' <<<"$state")"
  entry_v2_persist "$root" "$result" || return 12
  printf '%s\n' "$result"
}

entry_v2_fail() {
  local spec="$1" root="$2" state="$3" failed_phase="$4" code="$5" error
  state="$(jq -c '.phase="rolling_back" | .health="unknown"' <<<"$state")"
  entry_v2_persist "$root" "$state" || return 12
  if entry_v2_rollback "$spec" "$state" >/dev/null; then
    error="$(jq -cn --arg code "$code" --arg phase "$failed_phase" '{code:$code,phase:$phase,retryable:true,rollback_status:"succeeded",message:"Combined Adapter transaction failed"}')"
    # First-install failure has no committed target; keep an explicit tombstone
    # rather than claiming stable ownership of an uncommitted deployment.
    if [[ "$(jq -r '.committed_target == null' <<<"$state")" == true ]]; then
      state="$(jq -c --argjson err "$error" '.phase="failed" | .health="unhealthy" | .last_error=$err' <<<"$state")"
    else
      state="$(jq -c --argjson err "$error" '.phase="stable" | .health="degraded" | .active_roles=.committed_target.spec.active_roles | .candidate_target=null | .candidate_resources=[] | .last_error=$err' <<<"$state")"
    fi
  else
    error="$(jq -cn --arg code "$code" --arg phase "$failed_phase" '{code:$code,phase:$phase,retryable:true,rollback_status:"failed",message:"Combined Adapter transaction and rollback failed"}')"
    state="$(jq -c --argjson err "$error" '.phase="failed" | .health="unhealthy" | .last_error=$err' <<<"$state")"
  fi
  entry_v2_persist "$root" "$state" || return 12
  local resource
  resource="$(jq -r '.deployment_id' <<<"$state")"
  if [[ "$(jq -r '.last_error.rollback_status' <<<"$state")" == succeeded ]]; then
    entry_v2_error "$code" "$failed_phase" 'Combined Adapter transaction failed' "$resource" true succeeded
  else
    entry_v2_error "$code" "$failed_phase" 'Combined Adapter transaction failed' "$resource" true failed
  fi
}

entry_v2_remove() ( entry_v2_remove_locked "$@"; )

entry_v2_remove_locked() {
  local spec="$1" root="$2" purge="$3" deployment old observation path
  entry_validate_mutating_spec_file "$spec" && entry_v2_validate_spec "$spec" || {
    entry_v2_error invalid_spec stable 'Invalid or untrusted v2 EntrySpec'; return 2;
  }
  entry_v2_adapter_available || { entry_v2_error unsupported_capability stable 'No combined v2 Adapter is connected'; return 3; }
  deployment="$(jq -r '.deployment_id' "$spec")"
  mkdir -p "$root" && chmod 0700 "$root" || return 12
  local lock_fd
  exec {lock_fd}>"$root/.lock" || return 12
  flock -x "$lock_fd" || return 12
  old="$(entry_v2_locked_state "$root" "$deployment")" || { entry_v2_error ownership_conflict stable 'Invalid journal'; return 4; }
  [[ "$old" != null ]] || { entry_v2_error ownership_conflict stable 'No committed v2 deployment to remove'; return 4; }
  entry_v2_check_target "$spec" "$old" "$(entry_v2_target_digest "$spec")" || return $?
  [[ "$(jq -r '.phase' <<<"$old")" == stable ]] || { entry_v2_error ownership_conflict stable 'Recover unfinished transaction before remove'; return 4; }
  [[ "$(jq -r '.active_roles | length' <<<"$old")" == 1 ]] || {
    entry_v2_error invalid_spec stable 'Remove is allowed only after one-role reconcile'; return 2;
  }
  [[ "$(entry_v2_target_digest "$spec")" == "$(jq -r '.committed_target.digest' <<<"$old")" ]] || {
    entry_v2_error invalid_spec stable 'Remove target must match the last committed role'; return 2;
  }
  observation="$(entry_v2_adapter_probe "$spec" "$old")" || { entry_v2_error ownership_conflict stable 'Remove ownership probe failed'; return 4; }
  entry_v2_validate_observation "$observation" "$spec" probe || return 4
  jq -e --argjson obs "$observation" '.resources == $obs.resources' <<<"$old" >/dev/null || {
    entry_v2_error ownership_conflict stable 'Resource identity changed'; return 4;
  }
  old="$(jq -c --argjson spec "$(jq -c . "$spec")" '.phase="activating" | .desired_revision=$spec.revision | .desired_digest=.committed_target.digest | .candidate_target=.committed_target | .previous_resources=.resources | .candidate_resources=[]' <<<"$old")"
  entry_v2_persist "$root" "$old" || return 12
  if ! entry_v2_adapter_remove "$spec" "$old" "$purge" >/dev/null; then
    old="$(jq -c '.phase="failed" | .health="unhealthy"' <<<"$old")"
    entry_v2_persist "$root" "$old" || return 12
    entry_v2_error driver_failed activating 'Combined Adapter remove failed'; return 5
  fi
  path="$(entry_state_path "$root" "$deployment")"
  rm -f -- "$path"
  jq -cn --arg deployment "$deployment" '{schema_version:2,deployment_id:$deployment,result:"removed"}'
}
