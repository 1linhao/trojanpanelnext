#!/usr/bin/env bash
set -Eeuo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
work="$(mktemp -d)"
trap 'rm -r -- "${work}"' EXIT
fail() { printf 'FAIL local release bundle: %s\n' "$1" >&2; exit 1; }
digest() { printf 'sha256:%064d' "$1"; }
generate_args=(
  --version 1.2.3 --source-commit 0123456789abcdef0123456789abcdef01234567
  --output "${work}/generated"
  --api-image "example/api@$(digest 1)" --web-image "example/web@$(digest 2)"
  --node-agent-image "example/node@$(digest 3)" --caddy-image "caddy@$(digest 4)"
  --mariadb-image "mariadb@$(digest 5)" --redis-image "redis@$(digest 6)"
)
"${root}/deploy/installer/release/generate-assets.sh" "${generate_args[@]}" >/dev/null
archive="${work}/trojanpanelnext-installer-1.2.3.tar.gz"
"${root}/deploy/installer/release/package-assets.sh" --assets-dir "${work}/generated" --output "${archive}" >/dev/null

for path in client/tpnext.sh client/topology.sh client/verify-release.sh \
  client/templates/unified-ssh.yaml client/templates/unified-local.yaml \
  config-web.yaml config-node.yaml config-combined.yaml; do
  [[ -f "${work}/generated/${path}" ]] || fail "missing ${path}"
  grep -Fq "  ${path}" "${work}/generated/SHA256SUMS" || fail "unchecked ${path}"
  jq -e --arg path "${path}" '.assets[] | select(.path == $path and (.sha256 | length == 64))' \
    "${work}/generated/release-manifest.json" >/dev/null || fail "unmanifested ${path}"
done

mkdir "${work}/clean"
tar -xzf "${archive}" -C "${work}/clean"
archive_sha="$(sha256sum "${archive}" | awk '{print $1}')"
(cd "${work}/clean" && printf '%s  %s\n' "${archive_sha}" "${archive}" | sha256sum -c - >/dev/null)
"${work}/clean/verify-assets.sh" --assets-dir "${work}/clean" --assets-only >/dev/null
cli="${work}/clean/client/tpnext.sh"
bash "${cli}" init --help | grep -Fq 'init'
bash "${cli}" plan --help | grep -Fq -- '--config FILE'

for transport in ssh local; do
  config="${work}/${transport}.yaml"
  bash "${cli}" init --config "${config}" --tag v1.2.3 --sha256 "${archive_sha}" \
    --archive "${archive}" --web-transport "${transport}" --work-dir "${work}/${transport}.local" >/dev/null
  [[ "$(stat -c %a "${config}" 2>/dev/null || stat -f %Lp "${config}")" == 600 ]] || fail 'configuration permissions'
  [[ "$(stat -c %a "${work}/${transport}.local" 2>/dev/null || stat -f %Lp "${work}/${transport}.local")" == 700 ]] || fail 'work directory permissions'
  plan="$(bash "${cli}" plan --config "${config}")"
  if [[ "${transport}" == ssh ]]; then
    [[ "${plan}" == *$'web-host\tssh\tweb\t'* && "${plan}" == *$'node-host\tssh\tnode\tnode-one\t'* ]] || fail 'separate SSH plan'
  else
    [[ "${plan}" == *$'web-host\tlocal\tcombined\tnode-one\t'* ]] || fail 'local combined plan'
  fi
done

cp "${work}/ssh.yaml" "${work}/remote-combined.yaml"
yq -i '.nodes[0].host = "web-host" | del(.hosts."node-host") | .nodes[0].settings.node_caddy_https_port = 443' "${work}/remote-combined.yaml"
bash "${cli}" plan --config "${work}/remote-combined.yaml" | grep -Fq $'web-host\tssh\tcombined\tnode-one\t' || fail 'remote combined plan'
cp "${work}/remote-combined.yaml" "${work}/append.yaml"
yq -i '.hosts."node-two-host" = {"transport":"ssh","ssh":{"user":"root","hostname":"203.0.113.30","port":22,"identity_file":"","config_file":""}} | .nodes += [{"node_key":"node-two","host":"node-two-host","name":"node-two","domain":"node-two.example.com","public_ip":"203.0.113.30","settings":{"grpc_port":8100,"core_port":8082,"node_caddy_http_port":80,"node_caddy_https_port":8863},"passwords":{"bundle":""}}]' "${work}/append.yaml"
plan="$(bash "${cli}" plan --config "${work}/append.yaml")"
[[ "${plan}" == *$'web-host\tssh\tcombined\tnode-one\t'* && "${plan}" == *$'node-two-host\tssh\tnode\tnode-two\t'* ]] || fail 'combined with appended Node plan'

cp "${work}/append.yaml" "${work}/invalid-host.yaml"
yq -i '.nodes[1].host = "missing-host"' "${work}/invalid-host.yaml"
if bash "${cli}" plan --config "${work}/invalid-host.yaml" >"${work}/invalid.out" 2>&1; then fail 'accepted invalid host reference'; fi
grep -Fq 'undeclared host' "${work}/invalid.out" || fail 'host rejection reason'
cp "${work}/append.yaml" "${work}/second-local-node.yaml"
yq -i '.nodes[1].host = "web-host" | del(.hosts."node-two-host")' "${work}/second-local-node.yaml"
if bash "${cli}" plan --config "${work}/second-local-node.yaml" >"${work}/invalid.out" 2>&1; then fail 'accepted second combined Node'; fi

cp -a "${work}/clean" "${work}/tampered"
printf '\n# tampered\n' >>"${work}/tampered/client/topology.sh"
if "${work}/tampered/verify-assets.sh" --assets-dir "${work}/tampered" --assets-only >"${work}/invalid.out" 2>&1; then fail 'accepted tampered CLI'; fi
cp -a "${work}/clean" "${work}/tampered-template"
printf '\n# tampered\n' >>"${work}/tampered-template/client/templates/unified-ssh.yaml"
if "${work}/tampered-template/verify-assets.sh" --assets-dir "${work}/tampered-template" --assets-only >"${work}/invalid.out" 2>&1; then fail 'accepted tampered template'; fi

printf 'PASS local Release bundle CLI, topology, and integrity\n'
