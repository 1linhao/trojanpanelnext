#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
WEB_SMOKE="${ROOT_DIR}/deploy/installer/tests/release_web_docker_smoke_test.sh"
NODE_SMOKE="${ROOT_DIR}/apps/control-plane/api/testing/node_identity_cli_test.sh"

fail() {
  printf 'FAIL Docker smoke: %s\n' "$1" >&2
  exit 1
}

go_wrapper_dir=""
cleanup() {
  [[ -z "${go_wrapper_dir}" ]] || rm -rf -- "${go_wrapper_dir}"
}
trap cleanup EXIT

# GitHub-hosted runners expose Docker directly; the local development runner
# may require passwordless sudo. Do not silently turn a required integration
# gate into a unit-test-only run when neither daemon is available.
if ! docker info >/dev/null 2>&1; then
  docker() { sudo -n /usr/bin/docker "$@"; }
  export -f docker
fi
docker info >/dev/null 2>&1 || fail 'Docker daemon is required for the Web/Node smoke gate'

printf '%s\n' '== Web smoke: released bundle + installer entry + HTTPS/DB/Redis/admin probes =='
bash "${WEB_SMOKE}"

printf '%s\n' '== Node smoke: released bundle + installer entry + identity DB/Redis/mTLS/gRPC probes =='
if ! go version | grep -Eq 'go1\.(2[3-9]|[3-9][0-9])\.'; then
  # context.WithoutCancel is part of the Node Agent's supported Go contract.
  # The smoke still uses the real host Docker daemon for the installed Node;
  # this wrapper only supplies the compiler version CI already pins.
  go_wrapper_dir="$(mktemp -d)"
  cat >"${go_wrapper_dir}/go" <<'EOF'
#!/usr/bin/env bash
set -Eeuo pipefail
repo_root="${TP_SMOKE_REPO_ROOT:?}"
workdir="${PWD/#${repo_root}/\/repo}"
exec sudo -n /usr/bin/docker run --rm --network host \
  --user "$(id -u):$(id -g)" \
  -e GOCACHE=/tmp/trojanpanelnext-go-build \
  -e GOMODCACHE=/tmp/trojanpanelnext-go-mod \
  -v "${repo_root}:/repo" -v /tmp:/tmp -w "${workdir}" \
  golang:1.23-alpine go "$@"
EOF
  chmod 0755 "${go_wrapper_dir}/go"
  TP_SMOKE_REPO_ROOT="${ROOT_DIR}" PATH="${go_wrapper_dir}:${PATH}" bash "${NODE_SMOKE}"
else
  bash "${NODE_SMOKE}"
fi

printf '%s\n' 'PASS Web/Node Docker smoke (formal release installer, TLS, identities, and mTLS/gRPC)'
