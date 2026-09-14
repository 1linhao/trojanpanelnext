#!/usr/bin/env bash

# Pure candidate renderers for the nginx-certbot Adapter. They print config to
# stdout and never inspect or mutate nginx, certbot, systemd, ports, or files.

nginx_certbot_safe_domain() {
  [[ "$1" =~ ^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$ ]]
}

nginx_certbot_safe_path() {
  [[ "$1" == /* && "$1" != *[$'\n\r\t {};']* ]]
}

nginx_certbot_safe_endpoint() {
  if [[ "$1" =~ ^(127\.0\.0\.1|\[::1\]):([1-9][0-9]{0,4})$ ]]; then
    ((10#${BASH_REMATCH[2]} <= 65535))
    return
  fi
  return 1
}

nginx_certbot_render_web() {
  local domain="$1"
  local upstream="$2"
  local webroot="$3"
  local cert="$4"
  local key="$5"
  nginx_certbot_safe_domain "${domain}" || return 2
  nginx_certbot_safe_endpoint "${upstream}" || return 2
  nginx_certbot_safe_path "${webroot}" || return 2
  nginx_certbot_safe_path "${cert}" || return 2
  nginx_certbot_safe_path "${key}" || return 2

  cat <<EOF
# Managed candidate: trojanpanelnext/${domain}/web
server {
    listen 80;
    server_name ${domain};
    location ^~ /.well-known/acme-challenge/ { root ${webroot}; }
    location / { return 308 https://\$host\$request_uri; }
}
server {
    listen 443 ssl;
    server_name ${domain};
    ssl_certificate ${cert};
    ssl_certificate_key ${key};
    location / {
        proxy_set_header Host \$host;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
        proxy_pass http://${upstream};
    }
}
EOF
}

nginx_certbot_render_node() {
  local domain="$1"
  local webroot="$2"
  local fallbacks="${3:-}"
  nginx_certbot_safe_domain "${domain}" || return 2
  nginx_certbot_safe_path "${webroot}" || return 2

  cat <<EOF
# Managed candidate: trojanpanelnext/${domain}/node-acme
server {
    listen 80;
    server_name ${domain};
    location ^~ /.well-known/acme-challenge/ { root ${webroot}; }
    location / { return 404; }
}
EOF

  local listen root
  while IFS='|' read -r listen root; do
    [[ -z "${listen}" ]] && continue
    nginx_certbot_safe_endpoint "${listen}" || return 2
    nginx_certbot_safe_path "${root}" || return 2
    cat <<EOF
server {
    listen ${listen};
    server_name _;
    root ${root};
}
EOF
  done <<<"${fallbacks}"
}

# Build a deterministic, side-effect-free description of the candidate files,
# argv vectors and verification intent. Nothing in this function invokes nginx,
# certbot, systemd, a package manager, or a network client.
nginx_certbot_plan_spec() {
  local spec="$1"
  entry_validate_spec "${spec}" || return $?
  [[ "$(jq -r '.provider' "${spec}")" == nginx-certbot ]] || return 2

  local deployment purpose domain webroot config_path cert_path key_path candidate fallbacks upstream email
  deployment="$(jq -r '.deployment_id' "${spec}")"
  purpose="$(jq -r '.purpose' "${spec}")"
  domain="$(jq -r '.domain' "${spec}")"
  webroot="/var/lib/trojanpanelnext-entry/acme/${deployment}"
  config_path="/etc/nginx/conf.d/trojanpanelnext-${deployment}.conf"
  cert_path="/etc/letsencrypt/live/${domain}/fullchain.pem"
  key_path="/etc/letsencrypt/live/${domain}/privkey.pem"
  email="$(jq -r '.certificate.email // empty' "${spec}")"
  if [[ "${purpose}" == web ]]; then
    upstream="$(jq -r '.ingress.web_upstream' "${spec}")"
    candidate="$(nginx_certbot_render_web "${domain}" "${upstream}" "${webroot}" "${cert_path}" "${key_path}")" || return $?
  else
    fallbacks="$(jq -r '(.ingress.fallbacks // [])[] | [.listen, .root] | join("|")' "${spec}")"
    candidate="$(nginx_certbot_render_node "${domain}" "${webroot}" "${fallbacks}")" || return $?
  fi

  jq -cn \
    --arg deployment_id "${deployment}" \
    --argjson desired_revision "$(jq -r '.revision' "${spec}")" \
    --arg purpose "${purpose}" \
    --arg domain "${domain}" \
    --arg webroot "${webroot}" \
    --arg config_path "${config_path}" \
    --arg cert_path "${cert_path}" \
    --arg key_path "${key_path}" \
    --arg email "${email}" \
    --arg candidate "${candidate}" '
      {
        schema_version: 1,
        plan_kind: "nginx-certbot-candidate",
        executable: false,
        deployment_id: $deployment_id,
        desired_revision: $desired_revision,
        provider: "nginx-certbot",
        purpose: $purpose,
        domain: $domain,
        candidate: {path: $config_path, content: $candidate},
        certificate: {webroot: $webroot, cert_path: $cert_path, key_path: $key_path},
        commands: {
          issue: (["certbot", "certonly", "--non-interactive", "--agree-tos", "--webroot", "-w", $webroot, "-d", $domain]
                  + (if ($email | length) > 0 then ["--email", $email] else ["--register-unsafely-without-email"] end)),
          validate: ["nginx", "-t"],
          reload: ["systemctl", "reload", "nginx"]
        },
        checks: (["candidate.syntax", "certificate.material", "nginx.configtest"]
                 + (if $purpose == "web" then ["listener.tcp.80", "listener.tcp.443", "web.upstream"]
                    else ["listener.tcp.80", "node.direct", "fallback.loopback"] end))
      }'
}

nginx_certbot_validate_plan() {
  local plan="$1"
  jq -e '
    .schema_version == 1 and
    .plan_kind == "nginx-certbot-candidate" and
    .executable == false and
    .provider == "nginx-certbot" and
    (.deployment_id | test("^[a-z][a-z0-9-]{0,62}$")) and
    (.desired_revision | type == "number" and floor == . and . >= 1) and
    (.domain | test("^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$")) and
    (.candidate.path == ("/etc/nginx/conf.d/trojanpanelnext-" + .deployment_id + ".conf")) and
    (.candidate.content | type == "string" and length > 0) and
    (.certificate.webroot | startswith("/var/lib/trojanpanelnext-entry/acme/")) and
    (.certificate.cert_path | startswith("/etc/letsencrypt/live/")) and
    (.certificate.key_path | startswith("/etc/letsencrypt/live/")) and
    (.commands.issue | type == "array" and .[0] == "certbot") and
    (.commands.validate == ["nginx", "-t"]) and
    (.commands.reload == ["systemctl", "reload", "nginx"]) and
    (if .purpose == "web" then
       (.candidate.content | contains("listen 443 ssl;") and contains("proxy_pass http://127.0.0.1:"))
     elif .purpose == "node" then
       (.candidate.content | contains("listen 80;") and (contains("listen 443") | not) and (contains("stream") | not) and (contains("ssl_certificate") | not))
     else false end)
  ' <<<"${plan}" >/dev/null 2>&1
}
