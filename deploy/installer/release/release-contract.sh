#!/usr/bin/env bash

# Shared release contract used by generation, verification, and the release
# workflow. Keep the independently maintained expected set in
# tests/release_assets_test.sh so a coordinated omission still fails tests.
TP_RELEASE_ASSET_PATHS=(
  bootstrap.sh
  release-contract.sh
  verify-assets.sh
  install.sh
  secure-file
  node-bundle
  config-web.yaml
  config-node.yaml
  config-combined.yaml
  entry/entryctl.sh
  entry/controller.sh
  entry/v2.sh
  entry/adapters/external.sh
  entry/adapters/nginx_certbot.sh
  entry/adapters/caddy.sh
)

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
