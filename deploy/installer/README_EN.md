# TrojanPanel Next Installer

[简体中文](README.md) | English

The installer deploys a server non-interactively with one script, one YAML file, and one explicit deployment mode.

## Purpose modes

| Mode | Purpose | Services |
| --- | --- | --- |
| `web` | Web control plane | API, Web UI, MariaDB, Redis, and Caddy |
| `node` | Node Agent | Node Agent, proxy runtimes, certificates, and camouflage site |

Install the Web control plane first. Every Node Agent uses dedicated MariaDB and Redis identities
issued by the Web control plane; it never reuses the Web root or default users.

## External TLS mode

`tls_mode` decides who owns certificates and the public entry point:

| Value | Behaviour |
| --- | --- |
| `acme` (default) | The installer runs a Caddy container that listens on 80/443, obtains ACME certificates, and proxies |
| `external` | The installer creates **no reverse proxy container**; the external entry owns Web 80/443, ACME, and required plain fallbacks |

With `tls_mode: external` the installer still copies the certificates from `tls_cert_dir` into
`/tpdata/trojan-panel-core/cert` and mounts them read-only for the kernels, prepares the
`/tpdata/web` camouflage directory, removes leftover `*-caddy` containers from a previous
installation, and writes a host-specific contract summary to
`/tpdata/trojanpanelnext-external/README.md`.

Templates: [external-web.yaml](examples/external-web.yaml) and
[external-node.yaml](examples/external-node.yaml). The full list of behaviour the external entry
point has to implement is in [the external contract](../../docs/外部入口实现契约.md), with a short
form in [EXTERNAL.md](EXTERNAL.md).

Xray, NaiveProxy, and Hysteria2 listen directly on their Node protocol ports and terminate their own
TLS by default; they do not pass through a unified L4 ingress. The node agent records kernel
listeners in `/tpdata/trojan-panel-core/external/routes.json` for port/firewall audits and plain
fallback rendering. It is not an nginx `stream` configuration source. Only routes marked
`external_fallback_listener_required: true` need a plain-HTTP camouflage listener.

## Requirements

| Item | Requirement |
| --- | --- |
| OS | Debian 12 |
| Privileges | Installation and removal require `root` |
| CPU | `linux/amd64` (x86_64) |
| Memory | At least 1 GiB |
| Network | DNS points to the target host and configured ports are open |

## Install the Web control plane

```bash
cp examples/web.yaml ./web.yaml
chmod 600 ./web.yaml
```

Set `hostname` and `email`, then validate and install:

```bash
./install.sh validate --mode web --config ./web.yaml
sudo ./install.sh install --mode web --config ./web.yaml
```

The first installation generates random `sysadmin`, MariaDB, and Redis passwords, writes them back
to `web.yaml`, and changes its permissions to `600`. The terminal reports only where the secrets
were saved; it never prints them.
Sensitive writes use the Linux amd64 `secure-file` helper, which keeps the parent and original target
file descriptors open through commit. Existing targets use a verified `renameat2(RENAME_EXCHANGE)`
with rollback, while new targets use `RENAME_NOREPLACE`; a final-target or parent swap therefore
fails without overwriting the target. The installer also creates the control-plane mTLS identity in
`pki_bundle_dir`; the CA private key stays on the Web control plane.

Installation returns success only after the configured identity can access MariaDB, Redis, the public
HTTPS UI responds, and a read-only command inside the API container verifies the real `sysadmin`
credential. The credential probe exposes no HTTP path, starts no Redis client or limiter, and does not
issue a session, update login time, or increment login failures. Any failed probe returns non-zero with a secret-free diagnostic.
Replaying the same configuration reuses all three stored credentials.

## Install a Node Agent

After registering the Node on the Web control plane, use `node-bundle` from the same Release to create
an encrypted bootstrap bundle. Copy the Node template to a mode-`0600` working file and set the Node
domain, Web MariaDB/Redis addresses, and images. The identity ID, generation, and three dedicated
credentials are injected from the file produced by `node-identity register|rotate`:

```bash
cp ./config-node.yaml ./node-sg.yaml
chmod 600 ./node-sg.yaml
./node-bundle create \
  --credential-file /tpdata/trojan-panel/config/node-identities/node-sg.g1.json \
  --node-config ./node-sg.yaml \
  --client-ca /tpdata/trojanpanelnext-pki/client-ca.crt \
  --output ./node-sg.g1.age
```

The password is read interactively and confirmed by default. Automation may set
`TP_NODE_BUNDLE_PASSWORD`, but there is no password command-line option. The fixed inventory is only
`config-node.yaml`, `manifest.json`, and `pki/client-ca.crt`. The public CA is parsed and validated;
the bundle contains neither `client-ca.key`, the Web `client.key`, nor any other private key. Transfer
only the `.age` file to the Node VPS through a trusted channel.

Install on the Node with the same Release. After verifying the Release assets, the installer opens
plaintext only in a private directory under `/dev/shm` and removes it on every success or failure exit.
The default is an interactive password prompt; explicitly pass the environment through `sudo` for
non-interactive operation:

```bash
sudo ./install.sh validate --mode node --bundle ./node-sg.g1.age
sudo ./install.sh install --mode node --bundle ./node-sg.g1.age
# Non-interactive example: sudo env TP_NODE_BUNDLE_PASSWORD="$TP_NODE_BUNDLE_PASSWORD" \
#   ./install.sh install --mode node --bundle ./node-sg.g1.age
```

The installer first checks MariaDB, the Redis cache ACL, and the Redis auth ACL with the Node's own
identities, then waits for Web-to-Node mTLS/gRPC verification. While the Node install is waiting, run
this from another terminal on the Web control plane:

```bash
sudo docker exec trojan-panel /tpdata/trojan-panel/trojan-panel \
  node-identity verify --id <node-identity-id> --challenge <installer-printed-challenge>
```

The challenge is random for this installation and must be copied from that installation's output.
The call uses the client certificate retained by Web and verifies the Node server certificate. Node
`/healthz` becomes ready only after the identity ID, generation, server ID, and challenge all match,
and installation succeeds only after all four checks pass. The Node rechecks all three data identities
over fresh connections at a fixed production cadence and exits within 10 seconds after invalidation.
After rotation or revocation, an old bundle cannot pass installation and an already-running
old-generation Node stops. Generate and install a new bundle for the new generation.

A mode-`0600` `--config` remains available for development, removal, and certificate refresh, but use
the encrypted `--bundle` for a production Node's initial install.

When a host manager has generated a Protocol v1 EntrySpec, pass it explicitly:

```bash
sudo ./install.sh install --mode node --config ./node-agent.yaml \
  --entry-spec /var/lib/vps-factory/service-specs/trojanpanelnext-node.json
```

After the application install succeeds, the installer calls the adjacent
`entry/entryctl.sh reconcile`. Removal calls `remove` before deleting application
containers. The EntrySpec purpose and domain must match the installer config;
mutating actions require a root-owned regular file with mode 0600.

`--mode` must match `trojanpanelnext.deployment_mode`; the installer exits immediately when they
differ. Legacy `purpose` remains accepted only as compatibility input.

## Recreate or remove

```bash
sudo ./install.sh install --mode web --config ./web.yaml --force
sudo ./install.sh remove --mode web --config ./web.yaml --keep-data
sudo ./install.sh remove --mode node --config ./node-agent.yaml --purge-data
```

`--keep-data` overrides `purge_data` in the config for a recoverable removal;
`--purge-data` explicitly deletes generated data.

## Configuration files

[Web control-plane template](examples/web.yaml) contains the hostname, images, ports, mTLS identity directory, and internal credentials.

[Node Agent template](examples/node-agent.yaml) contains the node hostname, control-plane database and Redis connections, gRPC settings, public CA directory, and certificate paths.

External TLS mode uses [external-web.yaml](examples/external-web.yaml) and
[external-node.yaml](examples/external-node.yaml), which add these keys:

| Key | Default | Meaning |
| --- | --- | --- |
| `tls_mode` | `acme` | `acme` or `external` |
| `tls_cert_dir` | empty | External certificate directory; required for `external` + `node` |
| `tls_cert_file` / `tls_key_file` | empty | Explicit file names when the directory holds several pairs |
| `bind_address` | `0.0.0.0` | Panel UI listen address; prefer `127.0.0.1` in external mode |
| `managed_cert_dir` | `/tpdata/trojan-panel-core/cert` | Managed certificate copy; kernels read only this directory |
| `external_managed_dir` | `/tpdata/trojanpanelnext-external` | Contract directory |
| `external_routes_dir` | `/tpdata/trojan-panel-core/external` | Directory where the node agent writes `routes.json` |

Certificate discovery order: the files named by `tls_cert_file`/`tls_key_file`, then
`fullchain.pem` with `privkey.pem` (the certd layout), then a same-stem `.crt`/`.key` pair
searched three levels deep. Several pairs without an explicit choice is an error.

Passwords are never printed. Treat populated configuration files as secrets and do not commit them to Git.

## Versioned release assets

The release workflow uses `release/generate-assets.sh` to produce matching versions of
`bootstrap.sh`, `install.sh`, `node-bundle`, the `secure-file` helper for descriptor-safe reads and atomic sensitive writes, the `web|node|combined` configuration templates,
`release-manifest.json`, and `SHA256SUMS`. Product and runtime images are pinned as
`name@sha256:<digest>`. Before invoking the installer, `bootstrap.sh` runs the bundled
`verify-assets.sh` to verify versions, asset digests, image references, and configuration. The
released `install.sh` runs the same preflight when called directly, before crossing the host
mutation boundary. The verifier only depends on Bash, awk, grep, and coreutils from the Debian 12
base system; it does not require a preinstalled `jq`. It checks `SHA256SUMS` against its fixed asset
set, rejects symlink components in bundle paths, and accepts only the generator's printable ASCII +
LF manifest bytes before sourcing or executing any other bundled program. Both entrypoints verify all
13 assets, including `secure-file` and `node-bundle`, before a helper can first execute, then verify the configuration
contract again from the descriptor-safe snapshot before crossing the host mutation boundary.

The release configuration contract uses `deployment_mode`, `api_image`, `web_image`, and
`node_agent_image`. The legacy `purpose`, `panel_image`, `ui_image`, and `core_image` keys are
accepted only when reading existing installer configurations and are not emitted in new templates.

`combined` is valid in the unified configuration contract, but its container orchestration is
outside this ticket. Release validation accepts a combined configuration while the existing
installer continues to execute only `web` and `node`. See
[example-release-manifest.json](release/example-release-manifest.json) for a secret-free manifest
example. The Release tar.gz preserves executable modes and is accompanied by the manifest and
SHA256SUMS. Verify its attestation before extracting and checking the bundled digests:

```bash
archive=trojanpanelnext-installer-<version>.tar.gz
gh attestation verify "${archive}" --repo 1linhao/trojanpanelnext
mkdir trojanpanelnext-installer
tar -xzf "${archive}" -C trojanpanelnext-installer
cd trojanpanelnext-installer
sha256sum -c SHA256SUMS
```

Copy and edit a template instead of changing the digest-protected file in the release bundle,
then validate the deployment configuration:

```bash
cp ./config-web.yaml ./deployment.yaml
./verify-assets.sh --assets-dir . --config ./deployment.yaml
```

## Support

Project origin: [the original TrojanPanel project](https://github.com/trojanpanel).
