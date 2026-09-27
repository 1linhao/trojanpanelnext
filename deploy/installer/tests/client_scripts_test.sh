#!/usr/bin/env bash
set -Eeuo pipefail

installer_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
download="${installer_dir}/client/download-assets.sh"
upload="${installer_dir}/client/upload-assets.sh"
work="$(mktemp -d)"
trap 'rm -rf -- "${work}"' EXIT

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

mkdir -p "${work}/source"
for mode in web node combined; do
  printf 'trojanpanelnext:\n  deployment_mode: %s\n' "${mode}" >"${work}/source/config-${mode}.yaml"
done
(cd "${work}/source" && sha256sum config-*.yaml >SHA256SUMS)
tar -C "${work}/source" -czf "${work}/fixture.tar.gz" .
TP_CLIENT_TEST_TAR="${work}/fixture.tar.gz"
export TP_CLIENT_TEST_TAR
archive_sha="$(sha256sum "${work}/fixture.tar.gz" | awk '{print $1}')"

curl() {
  local previous="" argument destination=""
  for argument in "$@"; do
    if [[ "${previous}" == -o ]]; then destination="${argument}"; fi
    previous="${argument}"
  done
  [[ -n "${destination}" ]] || return 2
  cp "${TP_CLIENT_TEST_TAR}" "${destination}"
}
export -f curl

bash "${download}" --tag v9.8.7 --sha256 "${archive_sha}" --work-dir "${work}/client" >"${work}/download.out"
for mode in web node combined; do
  cmp "${work}/source/config-${mode}.yaml" "${work}/client/config/${mode}.yaml" ||
    fail "${mode} configuration copy differs from verified asset"
done
printf 'locally edited\n' >"${work}/client/config/web.yaml"
bash "${download}" --tag v9.8.7 --sha256 "${archive_sha}" --work-dir "${work}/client" >"${work}/download-again.out"
grep -Fxq 'locally edited' "${work}/client/config/web.yaml" || fail 'download overwrote an edited configuration'
if bash "${download}" --tag v9.8.7 --sha256 "$(printf '%064d' 0)" --work-dir "${work}/bad" >"${work}/bad.out" 2>&1; then
  fail 'download accepted a mismatched archive checksum'
fi
test ! -e "${work}/bad/config/web.yaml" || fail 'bad checksum produced a configuration'

trace="${work}/upload.trace"
TP_CLIENT_TEST_TRACE="${trace}"
export TP_CLIENT_TEST_TRACE
ssh() { printf 'ssh %s\n' "$*" >>"${TP_CLIENT_TEST_TRACE}"; }
scp() { printf 'scp %s\n' "$*" >>"${TP_CLIENT_TEST_TRACE}"; }
export -f ssh scp
printf 'encrypted bundle\n' >"${work}/client/node-1.g1.age"
for mode in web node-config combined; do
  bash "${upload}" --mode "${mode}" --host root@example.test --work-dir "${work}/client" >"${work}/upload-${mode}.out"
done
bash "${upload}" --mode node --host root@example.test --work-dir "${work}/client" \
  --bundle "${work}/client/node-1.g1.age" >"${work}/upload-node.out"
test "$(grep -c '^scp ' "${trace}")" = 4 || fail 'upload did not perform each transfer'
grep -Fq 'config/web.yaml' "${trace}" || fail 'web upload omitted its configuration'
grep -Fq 'config/node.yaml' "${trace}" || fail 'node-config upload omitted its configuration'
grep -Fq 'config/combined.yaml' "${trace}" || fail 'combined upload omitted its configuration'
grep -Fq 'node-1.g1.age' "${trace}" || fail 'node upload omitted its bundle'
if bash "${upload}" --mode web --host '-unsafe' --work-dir "${work}/client" >/dev/null 2>&1; then
  fail 'upload accepted an unsafe SSH target'
fi

printf 'PASS local download and upload scripts\n'
