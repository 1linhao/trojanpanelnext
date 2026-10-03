#!/usr/bin/env bash
# Inline Python is deliberately literal; only environment fixtures are read.
# shellcheck disable=SC2016
set -Eeuo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../scripts/deploy" && pwd)"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf -- "${TEST_DIR}"' EXIT
export TEST_DIR
export MOCK_DOCKER_STATE="${TEST_DIR}/docker.json"
export MOCK_DOCKER_LOG="${TEST_DIR}/docker.log"
export MOCK_DOCKER_PROGRAM="${TEST_DIR}/docker.py"
export MOCK_TARGET_VERSION
MOCK_TARGET_VERSION="$("${SCRIPT_DIR}/install.sh" --version)"
MOCK_OLD_VERSION=1.0.1
if [[ "${MOCK_TARGET_VERSION}" == 1.0.1 ]]; then MOCK_OLD_VERSION=1.0; fi
export MOCK_OLD_VERSION
export TP_DATA="${TEST_DIR}/data" WEB_PATH="${TEST_DIR}/site"
export TP_PKI_BUNDLE_DIR="${TEST_DIR}/data/pki-bundle"
export KERNEL_RUNTIME_PATH="${TEST_DIR}/data/kernel-runtime"
export GRPC_CLIENT_CA_PATH="${TEST_DIR}/data/trojan-panel-core/pki/client-ca.crt"
export GRPC_CLIENT_CERT_PATH="${TEST_DIR}/data/trojan-panel/pki/client.crt"
export GRPC_CLIENT_KEY_PATH="${TEST_DIR}/data/trojan-panel/pki/client.key"
export MOCK_HOST_ROOT="${TEST_DIR}/host"
mkdir -p "${TEST_DIR}/library" "${MOCK_HOST_ROOT}/etc/systemd/system" "${MOCK_HOST_ROOT}/etc" "${MOCK_HOST_ROOT}/usr/local/lib" "${MOCK_HOST_ROOT}/run"
# Isolate host-helper writes while executing the actual update/install scripts.
for script in common.sh install.sh update.sh uninstall.sh; do
  sed "s|/etc/trojanpanelnext-host|${MOCK_HOST_ROOT}/etc/trojanpanelnext-host|g; s|/usr/local/lib/trojanpanelnext-host|${MOCK_HOST_ROOT}/usr/local/lib/trojanpanelnext-host|g; s|/etc/systemd/system/trojanpanelnext-host.service|${MOCK_HOST_ROOT}/etc/systemd/system/trojanpanelnext-host.service|g; s|/run/trojanpanelnext-update.lock|${MOCK_HOST_ROOT}/run/trojanpanelnext-update.lock|g" \
    "${SCRIPT_DIR}/${script}" >"${TEST_DIR}/library/${script}"
done
UPDATE="${TEST_DIR}/library/update.sh"
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

cat >"${MOCK_DOCKER_PROGRAM}" <<'PY'
import json, os, re, sys
args = sys.argv[1:]
path = os.environ['MOCK_DOCKER_STATE']
with open(path) as f: state = json.load(f)
containers = state['containers']
with open(os.environ['MOCK_DOCKER_LOG'], 'a') as f: f.write(json.dumps(args) + '\n')
def save():
    with open(path, 'w') as f: json.dump(state, f)
def die(): sys.exit(1)
def mount(source, dest, readonly=False):
    return {'Type': 'bind', 'Source': os.path.realpath(source), 'Destination': dest.rstrip('/'), 'RW': not readonly}
action = args.pop(0)
if action == 'info': pass
elif action == 'image':
    if args[0] != 'inspect': die()
elif action == 'pull':
    if os.environ.get('MOCK_FAIL_PULL') in (args[0], 'all'): die()
elif action == 'inspect':
    name = args[-1] if '--format' in args else args[0]
    if name not in containers: die()
    c = containers[name]
    if '--format' not in args: print(json.dumps([c]))
    else:
        fmt = args[args.index('--format')+1]
        if fmt == '{{.State.Running}}:{{.State.Restarting}}:{{.RestartCount}}':
            print(str(c['State']['Running']).lower() + ':false:0')
        else: die()
elif action == 'ps':
    pattern = next((v[5:] for v in args if v.startswith('name=')), '')
    running = 'status=running' in args
    for name, c in containers.items():
        if re.search(pattern, name) and (not running or c['State']['Running']): print(c['Id'])
elif action == 'stop':
    if args[0] not in containers: die()
    containers[args[0]]['State']['Running'] = False; save()
elif action == 'rename':
    old, new = args
    if os.environ.get('MOCK_FAIL_RENAME') == old: die()
    if old not in containers or new in containers: die()
    containers[new] = containers.pop(old); save()
elif action == 'start':
    if args[0] not in containers: die()
    containers[args[0]]['State']['Running'] = True; save()
elif action == 'rm':
    name = args[-1]
    if name not in containers: die()
    del containers[name]; save()
elif action == 'cp':
    if os.environ.get('MOCK_FAIL_HELPER') == '1': die()
    with open(args[-1], 'w') as f: f.write('host-agent-' + os.environ['MOCK_TARGET_VERSION'])
elif action == 'run':
    name = ''; env = []; mounts = []; image = ''
    while args:
        arg = args.pop(0)
        if arg == '-d': continue
        if arg in ('--name', '--restart', '-e', '-v', '--mount'):
            val = args.pop(0)
            if arg == '--name': name = val
            elif arg == '-e': env.append(val)
            elif arg == '-v':
                parts = val.split(':'); mounts.append(mount(parts[0], parts[1], len(parts) == 3 and parts[2] == 'ro'))
            elif arg == '--mount':
                entries = dict(part.split('=', 1) for part in val.split(',') if '=' in part)
                mounts.append(mount(entries['src'], entries['dst'], 'readonly' in val.split(',')))
        elif arg.startswith('--network='): continue
        else: image = arg; break
    if os.environ.get('MOCK_FAIL_RUN') == name: die()
    if name in containers: die()
    containers[name] = {'Id': 'id-' + name + '-' + image.split(':')[-1], 'Config': {'Image': image, 'Env': env},
        'State': {'Running': True}, 'RestartCount': 0, 'Mounts': mounts,
        'HostConfig': {'NetworkMode': 'host', 'RestartPolicy': {'Name': 'always'}, 'Privileged': False}}
    save(); print(containers[name]['Id'])
else: die()
PY

# Exported functions stand in for transport/systemd; YAML, scripts, transaction
# state, mount/env guards and filesystem operations execute unchanged.
# shellcheck disable=SC2317,SC2329
docker() { python3 "${MOCK_DOCKER_PROGRAM}" "$@"; }
# shellcheck disable=SC2317,SC2329
systemctl() {
  printf '%s\n' "$*" >>"${TEST_DIR}/systemctl.log"
  if [[ "${MOCK_PENDING_ON_STOP:-0}" == 1 && "$1" == stop ]]; then
    printf 'new-removal-snapshot' >"${MOCK_HOST_ROOT}/etc/trojanpanelnext-host/server.crt"
  fi
  if [[ "${MOCK_ASSERT_CLI_ADMISSION:-0}" == 1 && "$1" == restart ]]; then
    local marker="${KERNEL_RUNTIME_PATH}/container-update.json"
    [[ "$(yq -r '.owner' "${marker}")" == cli && "$(yq -r '.status' "${marker}")" == running ]] || return 88
    [[ "$(stat -c %a "${marker}")" == 600 && "$(stat -c %a "${KERNEL_RUNTIME_PATH}")" == 700 ]] || return 88
    # RecoverUpdate in the restarted service must be able to take this lock.
    flock -n "${KERNEL_RUNTIME_PATH}/maintenance.lock" -c true || return 88
    printf 'CLI marker present during restart\n' >>"${TEST_DIR}/cli-admission.log"
    if [[ "${MOCK_REPLACE_CLI_OWNER:-0}" == 1 ]]; then
      yq -i '.token = "ForeignOwner"' "${marker}"
    fi
  fi
  if [[ "${MOCK_FAIL_SERVICE:-0}" == 1 && "$1" == restart && ! -e "${TEST_DIR}/failed-service-once" ]]; then
    touch "${TEST_DIR}/failed-service-once"
    return 1
  fi
}
# shellcheck disable=SC2317,SC2329
curl() {
  local url="${*: -1}"
  if [[ "${MOCK_BAD_HEALTH:-0}" == 1 ]]; then return 1; fi
  case "${url}" in
  */api/auth/setting) printf '{"code":20000,"data":{}}' ;;
  */version) printf 'v%s' "${MOCK_TARGET_VERSION}" ;;
  */) printf '404' ;;
  *) return 1 ;;
  esac
}
# shellcheck disable=SC2317,SC2329
id() { if [[ "$*" == -u ]]; then printf '0'; else command id "$@"; fi; }
# shellcheck disable=SC2317,SC2329
sleep() { :; }
export -f docker systemctl curl id sleep

# shellcheck source=scripts/deploy/install.sh
source "${TEST_DIR}/library/install.sh"
setup_deployment() {
  local purpose="$1"
  export MOCK_PURPOSE="${purpose}"
  rm -rf -- "${TP_DATA}" "${WEB_PATH}" "${TEST_DIR}/before-data" "${MOCK_HOST_ROOT}/etc/trojanpanelnext-host" "${MOCK_HOST_ROOT}/usr/local/lib/trojanpanelnext-host"
  rm -f -- "${TEST_DIR}/failed-service-once"
  printf '{"containers":{}}' >"${MOCK_DOCKER_STATE}"
  : >"${MOCK_DOCKER_LOG}"
  : >"${TEST_DIR}/systemctl.log"
  unset MOCK_FAIL_PULL MOCK_FAIL_RUN MOCK_FAIL_RENAME MOCK_FAIL_HELPER MOCK_BAD_HEALTH MOCK_FAIL_SERVICE MOCK_PENDING_ON_STOP MOCK_ASSERT_CLI_ADMISSION MOCK_REPLACE_CLI_OWNER
  TP_PKI_BUNDLE_DIR="${TP_DATA}/pki-bundle"
  cp "${SCRIPT_DIR}/templates/${purpose}.yaml" "${TEST_DIR}/original.yaml"
  TP_TEST_PKI="${TP_PKI_BUNDLE_DIR}" TP_TEST_CA="${GRPC_CLIENT_CA_PATH}" TP_TEST_CERT="${GRPC_CLIENT_CERT_PATH}" TP_TEST_KEY="${GRPC_CLIENT_KEY_PATH}" TP_TEST_RUNTIME="${KERNEL_RUNTIME_PATH}" \
    yq -i '.trojanpanelnext.mariadb_password = "mock=p" | .trojanpanelnext.redis_password = "redis-secret" | .trojanpanelnext.pki_bundle_dir = strenv(TP_TEST_PKI) | .trojanpanelnext.grpc_client_ca_path = strenv(TP_TEST_CA) | .trojanpanelnext.grpc_client_cert_path = strenv(TP_TEST_CERT) | .trojanpanelnext.grpc_client_key_path = strenv(TP_TEST_KEY) | .trojanpanelnext.kernel_runtime_path = strenv(TP_TEST_RUNTIME)' "${TEST_DIR}/original.yaml"
  load_config "${TEST_DIR}/original.yaml"
  prepare_dirs
  if [[ "${purpose}" == web ]]; then
    PANEL_IMAGE="ghcr.io/1linhao/trojanpanelnext-api:${MOCK_OLD_VERSION}"
    UI_IMAGE="ghcr.io/1linhao/trojanpanelnext-web:${MOCK_OLD_VERSION}"
    write_panel_runtime_config
    deploy_panel_backend >/dev/null
    deploy_panel_ui >/dev/null
  else
    CORE_IMAGE="ghcr.io/1linhao/trojanpanelnext-node-agent:${MOCK_OLD_VERSION}"
    resolve_node_certificate_paths
    prepare_node_certificate
    OLD_NODE_CERTIFICATE_PATH=""; OLD_NODE_PRIVATE_KEY_PATH=""
    deploy_core >/dev/null
  fi
  TP_OLD_RELEASE="${MOCK_OLD_VERSION}" yq -i '.trojanpanelnext.release = strenv(TP_OLD_RELEASE)' "${TEST_DIR}/original.yaml"
  if [[ "${purpose}" == web ]]; then
    TP_TEST_API="${PANEL_IMAGE}" TP_TEST_UI="${UI_IMAGE}" yq -i '.trojanpanelnext.panel_image = strenv(TP_TEST_API) | .trojanpanelnext.ui_image = strenv(TP_TEST_UI)' "${TEST_DIR}/original.yaml"
  else
    TP_TEST_AGENT="${CORE_IMAGE}" yq -i '.trojanpanelnext.core_image = strenv(TP_TEST_AGENT)' "${TEST_DIR}/original.yaml"
    touch "${MOCK_HOST_ROOT}/etc/systemd/system/trojanpanelnext-host.service"
    install_host_removal_service >/dev/null
    printf 'old-helper' >"${MOCK_HOST_ROOT}/usr/local/lib/trojanpanelnext-host/tp-host-agent"
  fi
  chmod 600 "${TEST_DIR}/original.yaml"
  cp "${TEST_DIR}/original.yaml" "${TEST_DIR}/before.yaml"
  cp "${MOCK_DOCKER_STATE}" "${TEST_DIR}/before-docker.json"
  cp -a "${TP_DATA}" "${TEST_DIR}/before-data"
  : >"${MOCK_DOCKER_LOG}"
}

run_update() { bash "${UPDATE}" --config "${TEST_DIR}/original.yaml" >"${TEST_DIR}/out" 2>"${TEST_DIR}/err"; }
assert_unchanged() {
  cmp "${TEST_DIR}/original.yaml" "${TEST_DIR}/before.yaml" || fail 'failed update changed original configuration'
  python3 -c 'import json,os; p=os.environ["TEST_DIR"]; assert json.load(open(p+"/docker.json")) == json.load(open(p+"/before-docker.json"))' || fail 'failed update did not restore original container identity/metadata'
  [[ ! -d "${MOCK_HOST_ROOT}/run/trojanpanelnext-update.lock" ]] || fail 'update lock leaked'
  [[ -z "$(find "${TEST_DIR}" -maxdepth 1 -name '.tpnext-update.*' -print -quit)" ]] || fail 'temporary update workspace leaked'
}
expect_failure() { if run_update; then fail 'update unexpectedly succeeded'; fi; }
assert_no_shared_mutations() {
  python3 -c 'import json,os; rows=[json.loads(x) for x in open(os.environ["MOCK_DOCKER_LOG"])]; shared={"trojan-panel-mariadb","trojan-panel-redis","trojan-panel-web-caddy","trojan-panel-node-caddy"}; assert not any(row[0] in ("rmi","prune") or (row[0] in ("stop","start","rename","rm") and any(x in shared for x in row)) or (row[0]=="pull" and not row[1].startswith("ghcr.io/1linhao/trojanpanelnext-")) or (row[0]=="run" and row[row.index("--name")+1] in shared) for row in rows)' || fail 'update mutated shared Docker resources or pruned images'
}

# Web success updates both products, leaves non-product resources intact and
# changes only the three release-bound fields in the original YAML.
setup_deployment web
sed -i 's/max_backups=5/max_backups=17/' "${TP_DATA}/trojan-panel/config/config.ini"
printf '\n# Retain locally maintained nginx tuning.\n' >>"${TP_DATA}/trojan-panel-ui/nginx/default.conf"
cp "${TP_DATA}/trojan-panel/config/config.ini" "${TEST_DIR}/custom-api.ini"
cp "${TP_DATA}/trojan-panel-ui/nginx/default.conf" "${TEST_DIR}/custom-nginx.conf"
run_update
grep -Fq 'Updated web' "${TEST_DIR}/out"
test "$(yq -r '.trojanpanelnext.release' "${TEST_DIR}/original.yaml")" = "${MOCK_TARGET_VERSION}"
yq 'del(.trojanpanelnext.release, .trojanpanelnext.panel_image, .trojanpanelnext.ui_image)' "${TEST_DIR}/original.yaml" >"${TEST_DIR}/after-fields"
yq 'del(.trojanpanelnext.release, .trojanpanelnext.panel_image, .trojanpanelnext.ui_image)' "${TEST_DIR}/before.yaml" >"${TEST_DIR}/before-fields"
cmp "${TEST_DIR}/after-fields" "${TEST_DIR}/before-fields"
backup="$(find "${TEST_DIR}" -maxdepth 1 -name 'original.yaml.backup-*' | head -n 1)"
test "$(stat -c %a "${backup}")" = 600
cmp "${backup}" "${TEST_DIR}/before.yaml"
test "$(stat -c %a "${TEST_DIR}/original.yaml")" = 600
cmp "${TP_DATA}/trojan-panel/config/config.ini" "${TEST_DIR}/custom-api.ini"
cmp "${TP_DATA}/trojan-panel-ui/nginx/default.conf" "${TEST_DIR}/custom-nginx.conf"
assert_no_shared_mutations
python3 -c 'import json,os; a=[json.loads(x) for x in open(os.environ["MOCK_DOCKER_LOG"])]; first_stop=next(i for i,x in enumerate(a) if x[0]=="stop"); assert first_stop>max(i for i,x in enumerate(a) if x[0]=="pull")' || fail 'products stopped before all target images were pulled'

# A failed second pull never stops the first old product or writes original.
setup_deployment web
export MOCK_FAIL_PULL="ghcr.io/1linhao/trojanpanelnext-web:${MOCK_TARGET_VERSION}"
expect_failure
assert_unchanged
if rg -q '"(stop|rename|run|rm)"' "${MOCK_DOCKER_LOG}"; then fail 'pull failure stopped old deployment'; fi

for failure in run rename; do
  setup_deployment web
  if [[ "${failure}" == run ]]; then export MOCK_FAIL_RUN=trojan-panel-ui; else export MOCK_FAIL_RENAME=trojan-panel-ui; fi
  expect_failure
  assert_unchanged
  cmp "${TP_DATA}/trojan-panel/config/config.ini" "${TEST_DIR}/before-data/trojan-panel/config/config.ini"
  cmp "${TP_DATA}/trojan-panel-ui/nginx/default.conf" "${TEST_DIR}/before-data/trojan-panel-ui/nginx/default.conf"
done

# Readiness failures run the real probe once, then force its deadline to expire.
setup_deployment web
export MOCK_BAD_HEALTH=1
if bash -c 'source "$1"; sleep() { SECONDS=100; }; main --config "$2"' test "${UPDATE}" "${TEST_DIR}/original.yaml" >"${TEST_DIR}/out" 2>"${TEST_DIR}/err"; then fail 'unhealthy new deployment accepted'; fi
assert_unchanged
grep -Fq 'readiness timed out' "${TEST_DIR}/err"

# Wrong source credentials, mounts and tags must fail before pulling or stopping.
for mismatch in credential mount image schema downgrade runtime; do
  setup_deployment web
  case "${mismatch}" in
  credential) yq -i '.trojanpanelnext.redis_password = "incorrect"' "${TEST_DIR}/original.yaml" ;;
  mount) yq -i '.containers.trojan-panel.Mounts[0].Source = "/wrong/source"' "${MOCK_DOCKER_STATE}" ;;
  image) yq -i '.containers.trojan-panel.Config.Image = "unrelated:latest"' "${MOCK_DOCKER_STATE}" ;;
  schema) yq -i '.trojanpanelnext.schema_version = 2' "${TEST_DIR}/original.yaml" ;;
  downgrade) yq -i '.trojanpanelnext.release = "99.0"' "${TEST_DIR}/original.yaml" ;;
  runtime) sed -i 's/^port=8081$/port=8088/' "${TP_DATA}/trojan-panel/config/config.ini" ;;
  esac
  expect_failure
  if rg -q '"(pull|stop|rename|run|rm)"' "${MOCK_DOCKER_LOG}"; then fail "${mismatch} changed Docker state"; fi
done

# A queued kernel upgrade already persisted by the Agent must prevent manual
# container switching before images are pulled or old products are stopped.
setup_deployment node
mkdir -p "${KERNEL_RUNTIME_PATH}/operations"
printf '{"stage":"queued"}\n' >"${KERNEL_RUNTIME_PATH}/operations/active.json"
expect_failure
assert_unchanged
if rg -q '"(stop|rename|run|rm|pull)"' "${MOCK_DOCKER_LOG}"; then fail 'active kernel allowed container switching'; fi

# An active host worker keeps its executable mapped while maintenance is
# refreshed. Docker must stage and atomically replace the path, never truncate it.
setup_deployment node
live_helper="${MOCK_HOST_ROOT}/usr/local/lib/trojanpanelnext-host/tp-host-agent"
cp -- "$(type -P sleep)" "${live_helper}"
chmod 700 "${live_helper}"
"${live_helper}" 60 &
live_helper_pid=$!
if ! run_update; then
  kill "${live_helper_pid}" 2>/dev/null || true
  wait "${live_helper_pid}" 2>/dev/null || true
  cat "${TEST_DIR}/err" >&2
  fail 'Node update overwrote an executing host worker binary'
fi
kill -0 "${live_helper_pid}" || fail 'maintenance refresh interrupted the independent worker'
kill "${live_helper_pid}"
wait "${live_helper_pid}" 2>/dev/null || true
test -z "$(find "${MOCK_HOST_ROOT}/usr/local/lib/trojanpanelnext-host" -name '.tp-host-agent.*' -print -quit)"

# The CLI marker remains visible across helper restart without holding the
# admission flock. Success and completed rollback clear only their own marker.
for outcome in success rollback; do
  setup_deployment node
  export MOCK_ASSERT_CLI_ADMISSION=1
  if [[ "${outcome}" == rollback ]]; then export MOCK_FAIL_SERVICE=1; expect_failure; assert_unchanged; else run_update; fi
  grep -q 'CLI marker present during restart' "${TEST_DIR}/cli-admission.log"
  test ! -e "${KERNEL_RUNTIME_PATH}/container-update.json"
  test ! -d "${MOCK_HOST_ROOT}/run/trojanpanelnext-update.lock"
done

# A superseding owner's marker is never removed by an old CLI completion.
setup_deployment node
export MOCK_ASSERT_CLI_ADMISSION=1 MOCK_REPLACE_CLI_OWNER=1
run_update
test "$(yq -r '.token' "${KERNEL_RUNTIME_PATH}/container-update.json")" = ForeignOwner

# Agent success keeps the original source path in the maintained host helper.
setup_deployment node
sed -i 's/max_age=30/max_age=37/' "${TP_DATA}/trojan-panel-core/config/config.ini"
cp "${TP_DATA}/trojan-panel-core/config/config.ini" "${TEST_DIR}/custom-agent.ini"
run_update
grep -Fq 'Updated node' "${TEST_DIR}/out"
test "$(yq -r '.originalConfig' "${MOCK_HOST_ROOT}/etc/trojanpanelnext-host/config.json")" = "${TEST_DIR}/original.yaml"
test "$(yq -r '.trojanpanelnext.release' "${MOCK_HOST_ROOT}/etc/trojanpanelnext-host/node.yaml")" = "${MOCK_TARGET_VERSION}"
grep -Fxq "SCRIPT_VERSION=\"${MOCK_TARGET_VERSION}\"" "${MOCK_HOST_ROOT}/usr/local/lib/trojanpanelnext-host/uninstall.sh"
grep -Fxq "host-agent-${MOCK_TARGET_VERSION}" "${MOCK_HOST_ROOT}/usr/local/lib/trojanpanelnext-host/tp-host-agent"
cmp "${TP_DATA}/trojan-panel-core/config/config.ini" "${TEST_DIR}/custom-agent.ini"
assert_no_shared_mutations

# Failure copying the target helper restores the original old helper + Agent.
setup_deployment node
export MOCK_FAIL_HELPER=1
expect_failure
assert_unchanged
grep -Fxq old-helper "${MOCK_HOST_ROOT}/usr/local/lib/trojanpanelnext-host/tp-host-agent"
cmp "${MOCK_HOST_ROOT}/etc/trojanpanelnext-host/node.yaml" "${TEST_DIR}/before.yaml"

# A remote uninstall accepted just before helper stop must retain its fresh
# TLS snapshot, and the update may not stop/recreate the Agent afterward.
setup_deployment node
export MOCK_PENDING_ON_STOP=1
expect_failure
assert_unchanged
grep -Fq 'maintenance removal is pending' "${TEST_DIR}/err"
grep -Fxq new-removal-snapshot "${MOCK_HOST_ROOT}/etc/trojanpanelnext-host/server.crt"
if rg -q '"(stop|rename|run|rm)"' "${MOCK_DOCKER_LOG}"; then fail 'update changed Agent while host removal was pending'; fi

# A new service startup failure occurs after YAML replacement; both it and the
# target helper must still roll back to the saved deployment.
setup_deployment node
export MOCK_FAIL_SERVICE=1
expect_failure
assert_unchanged
grep -Fxq old-helper "${MOCK_HOST_ROOT}/usr/local/lib/trojanpanelnext-host/tp-host-agent"
cmp "${MOCK_HOST_ROOT}/etc/trojanpanelnext-host/node.yaml" "${TEST_DIR}/before.yaml"

# Only the persisted job's own worker may enter update while its queued/running
# journal is active. Ordinary CLI updates cannot substitute a boolean bypass.
setup_deployment node
host_job="${MOCK_HOST_ROOT}/etc/trojanpanelnext-host/update.json"
host_job_id="$(printf '%064d' 0)"
printf '{"nodeId":%s,"job":{"id":"%s","status":"running","targetVersion":"%s"}}\n' \
  "${NODE_SERVER_ID}" "${host_job_id}" "${MOCK_TARGET_VERSION}" >"${host_job}"
chmod 600 "${host_job}"
printf '{"jobId":"%s","status":"running"}\n' "${host_job_id}" >"${KERNEL_RUNTIME_PATH}/container-update.json"
cp "${host_job}" "${TEST_DIR}/before-job.json"
expect_failure
assert_unchanged
export TP_HOST_UPDATE_JOB=1
expect_failure
assert_unchanged
export TP_HOST_UPDATE_JOB="${host_job_id}"
run_update
cmp "${host_job}" "${TEST_DIR}/before-job.json"
unset TP_HOST_UPDATE_JOB

# Rollback restores maintenance files without rewinding the independent worker's
# journal or overwriting the executable mapped by that worker.
setup_deployment node
printf '{"nodeId":%s,"job":{"id":"%s","status":"running","targetVersion":"%s"}}\n' \
  "${NODE_SERVER_ID}" "${host_job_id}" "${MOCK_TARGET_VERSION}" >"${host_job}"
cp "${host_job}" "${TEST_DIR}/before-job.json"
printf '{"jobId":"%s","status":"running"}\n' "${host_job_id}" >"${KERNEL_RUNTIME_PATH}/container-update.json"
export TP_HOST_UPDATE_JOB="${host_job_id}" MOCK_FAIL_SERVICE=1
expect_failure
assert_unchanged
cmp "${host_job}" "${TEST_DIR}/before-job.json"
unset TP_HOST_UPDATE_JOB MOCK_FAIL_SERVICE

# Certificate paths are carried through an external-certificate update without
# invoking either signer, including the Certbot live -> archive directory binds.
setup_deployment node
mkdir -p "${TEST_DIR}/certificates/live/node.example.com" "${TEST_DIR}/certificates/archive/node.example.com"
openssl req -x509 -newkey rsa:2048 -nodes -days 2 -subj /CN=node.example.com \
  -addext subjectAltName=DNS:node.example.com \
  -keyout "${TEST_DIR}/certificates/archive/node.example.com/privkey1.pem" \
  -out "${TEST_DIR}/certificates/archive/node.example.com/fullchain1.pem" >/dev/null 2>&1
ln -s ../../archive/node.example.com/fullchain1.pem "${TEST_DIR}/certificates/live/node.example.com/fullchain.pem"
ln -s ../../archive/node.example.com/privkey1.pem "${TEST_DIR}/certificates/live/node.example.com/privkey.pem"
NODE_CERTIFICATE_MODE=external
NODE_CERTIFICATE_PATH="${TEST_DIR}/certificates/live/node.example.com/fullchain.pem"
NODE_PRIVATE_KEY_PATH="${TEST_DIR}/certificates/live/node.example.com/privkey.pem"
TP_TEST_CERT="${NODE_CERTIFICATE_PATH}" TP_TEST_KEY="${NODE_PRIVATE_KEY_PATH}" \
  yq -i '.trojanpanelnext.node_certificate_mode = "external" | .trojanpanelnext.node_certificate_path = strenv(TP_TEST_CERT) | .trojanpanelnext.node_private_key_path = strenv(TP_TEST_KEY)' "${TEST_DIR}/original.yaml"
docker rm -f "${CORE_CONTAINER}"
prepare_node_certificate
deploy_core >/dev/null
install_host_removal_service >/dev/null
cp "${TEST_DIR}/original.yaml" "${TEST_DIR}/before.yaml"
certificate_before="$(sha256sum "${NODE_CERTIFICATE_PATH}" "${NODE_PRIVATE_KEY_PATH}")"
: >"${MOCK_DOCKER_LOG}"
run_update
test "$(sha256sum "${NODE_CERTIFICATE_PATH}" "${NODE_PRIVATE_KEY_PATH}")" = "${certificate_before}"
assert_no_shared_mutations
test "$(yq -r '.trojanpanelnext.node_certificate_mode' "${TEST_DIR}/original.yaml")" = external
test "$(yq -r '.trojanpanelnext.node_certificate_path' "${TEST_DIR}/original.yaml")" = "${NODE_CERTIFICATE_PATH}"

# Version ordering includes RC -> RC, RC -> stable and rejects stable -> RC.
bash -c 'source "$1"; version_can_update 1.0 1.0.1; version_can_update 1.0.2-rc.1 1.0.2-rc.3; version_can_update 1.0.2-rc.12 1.0.2-rc.12; version_can_update 1.0.2-rc.3 1.0.2; ! version_can_update 1.0.2 1.0.2-rc.3; ! version_can_update 1.1 1.0.2; ! version_can_update 0.9 1.0' test "${UPDATE}"
printf 'PASS product-only image updates, protected config backup, source guards, pull safety, readiness/rename/create recovery and Node helper refresh\n'
