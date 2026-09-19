# TrojanPanel Next Installer

[简体中文](README.md) | English

The installer deploys a server non-interactively with one script, one YAML file, and one explicit deployment mode.

## Purpose modes

| Mode | Purpose | Services |
| --- | --- | --- |
| `web` | Web control plane | API, Web UI, MariaDB, Redis, and Caddy |
| `node` | Node Agent | Node Agent, proxy runtimes, certificates, and camouflage site |

Install the Web control plane first. A Node Agent uses the MariaDB and Redis credentials generated in the Web configuration.

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
| OS | Ubuntu 20.04+, Debian 11+, or a comparable systemd Linux |
| Privileges | Installation and removal require `root` |
| CPU | `linux/amd64` or `linux/arm64` |
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

The first installation generates MariaDB and Redis passwords, writes them back to `web.yaml`, and changes its permissions to `600`. It also creates the control-plane mTLS identity in `pki_bundle_dir`; the CA private key stays on the Web control plane.

Before installing a Node Agent, transfer `/tpdata/trojanpanelnext-pki/client-ca.crt` from the Web control plane to the same path on the Node through a trusted file-transfer or secret-management channel. Copy only the public CA certificate; never copy `client-ca.key`, `client.key`, or `client.crt`.
The installer records the CA digest in the Core container environment. It recreates Core when adopting a legacy unmarked container or when the CA changes, so the new trust root takes effect immediately; unchanged replays do not restart Core.

## Install a Node Agent

```bash
cp examples/node-agent.yaml ./node-agent.yaml
chmod 600 ./node-agent.yaml
```

Set the node hostname, Web control-plane address, and the database and Redis passwords from the Web configuration. Confirm that the public CA certificate is present in `pki_bundle_dir`:

```bash
./install.sh validate --mode node --config ./node-agent.yaml
sudo ./install.sh install --mode node --config ./node-agent.yaml
sudo ./install.sh refresh-cert --mode node --config ./node-agent.yaml
```

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
`bootstrap.sh`, `install.sh`, the `web|node|combined` configuration templates,
`release-manifest.json`, and `SHA256SUMS`. Product and runtime images are pinned as
`name@sha256:<digest>`. Before invoking the installer, `bootstrap.sh` runs the bundled
`verify-assets.sh` to verify versions, asset digests, image references, and configuration. The
released `install.sh` runs the same preflight when called directly, before crossing the host
mutation boundary. The verifier only depends on Bash, awk, and coreutils from the Debian 12 base
system; it does not require a preinstalled `jq`. It checks `SHA256SUMS` against its fixed asset set
before sourcing or executing any other bundled program.

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
