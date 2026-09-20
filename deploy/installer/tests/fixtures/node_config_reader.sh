#!/bin/sh
set -eu

config_path="$(find / -path '*/trojan-panel-core/config/config.ini' -type f -print -quit 2>/dev/null)"
test -n "${config_path}"
sha256sum "${config_path}" | awk '{print $1}' >/tmp/observed-node-config-sha256
exec sleep 300
