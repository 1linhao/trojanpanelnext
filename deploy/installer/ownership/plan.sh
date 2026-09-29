#!/usr/bin/env bash

# Source this file from install.sh after its read-only configuration parse.
# The plan contains identities and paths only; never configuration secrets.
ownership_plan_json() {
  local manifest="${INSTALLER_DIR}/release-manifest.json"
  if [[ "${INSTALLER_ASSET_VERSION:-}" == development && ! -f "$manifest" ]]; then
    manifest="${INSTALLER_DIR}/release/example-release-manifest.json"
  fi
  [[ -f "$manifest" && ! -L "$manifest" ]] || return 1
  local images
  images="$(jq -c '.images | with_entries(.value = {reference:.value.reference})' "$manifest")" || return 1
  jq -cn \
    --arg id "$OWNERSHIP_DEPLOYMENT_ID" --arg root "$TP_DATA" --arg mode "$TP_DEPLOYMENT_MODE" \
    --arg tls "$TLS_MODE" --arg webroot "$WEB_PATH" \
    --arg web_domain "${TP_WEB_DOMAIN:-}" --arg node_domain "${TP_NODE_DOMAIN:-}" \
    --arg mariadb "$MARIADB_CONTAINER" --arg redis "$REDIS_CONTAINER" \
    --arg panel "$PANEL_CONTAINER" --arg ui "$UI_CONTAINER" --arg core "$CORE_CONTAINER" \
    --arg web_caddy "$WEB_CADDY_CONTAINER" --arg node_caddy "$NODE_CADDY_CONTAINER" \
    --arg entry_caddy "$COMBINED_ENTRY_CONTAINER" \
    --arg external_routes "$EXTERNAL_ROUTES_DIR" --arg kernel_runtime "$KERNEL_RUNTIME_PATH" \
    --arg managed_cert "$MANAGED_CERT_DIR" \
    --argjson mariadb_port "$MARIADB_PORT" --argjson redis_port "$REDIS_PORT" \
    --argjson panel_port "$PANEL_PORT" --argjson ui_port "$UI_PORT" \
    --argjson core_port "$CORE_PORT" --argjson grpc_port "$GRPC_PORT" \
    --argjson node_http "$NODE_CADDY_HTTP_PORT" --argjson node_https "$NODE_CADDY_HTTPS_PORT" \
    --argjson images "$images" '
      def m($src;$dst;$ro): {source:$src,target:$dst,read_only:$ro};
      def inside($path): "/tpdata" + ($path | ltrimstr($root));
      def p($port): {protocol:"tcp",port:$port};
      def item($name;$role;$image;$mounts;$ports):
        {name:$name,role:$role,image:$image,mounts:$mounts,ports:$ports};
      def caddy($name;$root;$role;$ports):
        item($name;$role;"caddy";
          [m($root+"/Caddyfile";"/etc/caddy/Caddyfile";true),
           m($root+"/data";"/data";false),m($root+"/config";"/config";false),
           m($webroot;"/srv";true)];$ports);
      def db:
        [item($mariadb;(if $mode == "combined" then "shared" else "web" end);"mariadb";
          [m($root+"/mariadb/data";"/var/lib/mysql";false)];[p($mariadb_port)]),
         item($redis;(if $mode == "combined" then "shared" else "web" end);"redis";
          [m($root+"/redis/data";"/data";false)];[p($redis_port)])];
      def web:
        [item($panel;"web";"api";
          [m($webroot;"/tpdata/trojan-panel/webfile";false),
           m($root+"/trojan-panel/logs";"/tpdata/trojan-panel/logs";false),
           m($root+"/trojan-panel/config";"/tpdata/trojan-panel/config";false),
           m($root+"/trojan-panel/pki";"/tpdata/trojan-panel/pki";true),
           m("/etc/localtime";"/etc/localtime";true)];[p($panel_port)]),
         item($ui;"web";"web";
          [m($root+"/trojan-panel-ui/nginx/default.conf";"/etc/nginx/conf.d/default.conf";true)];
          [p($ui_port)])];
      def core_cert:
        if $tls == "external" or $mode == "combined" then
          m($managed_cert;inside($managed_cert);true)
        else
          ($root+"/custom/node-caddy/data/caddy/certificates/acme-v02.api.letsencrypt.org-directory/"+$node_domain) as $path |
          m($path;inside($path);true)
        end;
      def node:
        [item($core;"node";"node_agent";
          [m($root+"/trojan-panel-core/bin/xray/config";"/tpdata/trojan-panel-core/bin/xray/config";false),
           m($root+"/trojan-panel-core/bin/naiveproxy/config";"/tpdata/trojan-panel-core/bin/naiveproxy/config";false),
           m($root+"/trojan-panel-core/bin/hysteria2/config";"/tpdata/trojan-panel-core/bin/hysteria2/config";false),
           m($root+"/trojan-panel-core/logs";"/tpdata/trojan-panel-core/logs";false),
           m($root+"/trojan-panel-core/config";"/tpdata/trojan-panel-core/config";false),
           m($root+"/trojan-panel-core/config/config.ini";"/tpdata/trojan-panel-core/config/config.ini";true),
           m($root+"/trojan-panel-core/pki";"/tpdata/trojan-panel-core/pki";true),
           m($kernel_runtime;"/tpdata/trojan-panel-core/runtime";false),core_cert,
           m($external_routes;"/tpdata/trojan-panel-core/external";false),
           m($webroot;"/tpdata/web";true),m("/etc/localtime";"/etc/localtime";true)];
          [p($core_port),p($grpc_port)])];
      (if $mode == "combined" then ["web","node"] else [$mode] end) as $roles |
      (if $mode == "web" then db + web +
        (if $tls == "acme" then [caddy($web_caddy;$root+"/custom/web-caddy";"web";[p(80),p(443)])] else [] end)
       elif $mode == "node" then node +
        (if $tls == "acme" then [caddy($node_caddy;$root+"/custom/node-caddy";"node";[p($node_http),p($node_https)])] else [] end)
       else db + web + node +
        [caddy($entry_caddy;$root+"/custom/web-caddy";"shared";[p(80),p(443)])] end) as $containers |
      {schema_version:1,deployment_id:$id,root:$root,mode:$mode,roles:$roles,
       images:($images | with_entries(select(.key as $key | $containers | any(.[]; .image == $key)))),
       containers:$containers}
    '
}
