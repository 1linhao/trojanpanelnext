#!/usr/bin/env bash

# A temp-directory-only simulator for candidate/active/backup state. It proves
# activation and rollback ordering without executing any host command listed in
# the plan.
nginx_certbot_fake_reconcile() {
  local plan="$1"
  local root="$2"
  nginx_certbot_validate_plan "${plan}" || return 2
  [[ "${root}" == /* ]] || return 2
  mkdir -p "${root}"
  chmod 0700 "${root}"
  printf '%s\n' "$(jq -r '.candidate.content' <<<"${plan}")" >"${root}/candidate.conf"
  chmod 0600 "${root}/candidate.conf"
  if [[ -f "${root}/active.conf" ]]; then
    cp "${root}/active.conf" "${root}/backup.conf"
  else
    rm -f "${root}/backup.conf"
  fi
  if [[ "${NGINX_CERTBOT_FAKE_FAIL_AT:-}" == activate ]]; then
    nginx_certbot_fake_rollback "${root}"
    return 6
  fi
  mv "${root}/candidate.conf" "${root}/active.conf"
  jq -cn --arg deployment_id "$(jq -r '.deployment_id' <<<"${plan}")" \
    --arg digest "$(sha256sum "${root}/active.conf" | awk '{print $1}')" '
      {schema_version:1,deployment_id:$deployment_id,phase:"stable",health:"healthy",config_digest:$digest}'
}

nginx_certbot_fake_rollback() {
  local root="$1"
  if [[ -f "${root}/backup.conf" ]]; then
    mv "${root}/backup.conf" "${root}/active.conf"
  fi
  rm -f "${root}/candidate.conf"
}
