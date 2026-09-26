# TrojanPanel Next Installer

[简体中文](README.md) | English

The installer deploys TrojanPanel Next directly on a supported Linux host without an external VPS management system, control panel, or orchestration backend. Deployment uses one versioned Release, one YAML file, and one explicit deployment mode.

## Purpose modes

| Mode | Purpose | Services |
| --- | --- | --- |
| `web` | Web control plane | API, Web UI, MariaDB, Redis, and public entry |
| `node` | Node Agent | Node Agent, proxy runtimes, certificates, and camouflage site |
| `combined` | Control plane and Node on one host | Web control plane, Node Agent, one shared entry, and local data services |

Deploy `web` and `node` on separate hosts by installing Web first, then registering each Node and issuing dedicated MariaDB and Redis identities; a Node never reuses the Web root or default users. `combined` runs both roles on one host and uses two different domains that resolve to that host's public IP. Standalone deployments use the installer-managed Caddy and ACME entry by default; use `tls_mode: external` only when an operator already owns and manages the entry. `--entry-spec` is an optional external-entry integration, not a standalone prerequisite.

## Download the Release and example configurations

The current VPS acceptance candidate is [v0.1.0-rc.2](https://github.com/1linhao/trojanpanelnext/releases/tag/v0.1.0-rc.2). Run these commands on a trusted workstation or a target Debian 12 x86_64 host. The archive already contains matching `config-web.yaml`, `config-node.yaml`, and `config-combined.yaml`; do not fetch examples from a source branch. For separate Web and Node hosts, use the same Release on both.

```bash
set -euo pipefail
TAG=v0.1.0-rc.2
VERSION="${TAG#v}"
WORKDIR="./trojanpanelnext-${VERSION}"
ARCHIVE="trojanpanelnext-installer-${VERSION}.tar.gz"
mkdir -m 700 "$WORKDIR"
gh release download "$TAG" --repo 1linhao/trojanpanelnext \
  --pattern "$ARCHIVE" --pattern release-manifest.json --pattern SHA256SUMS \
  --dir "$WORKDIR"
for file in "$ARCHIVE" release-manifest.json SHA256SUMS; do
  gh attestation verify "$WORKDIR/$file" \
    --repo 1linhao/trojanpanelnext \
    --signer-workflow 1linhao/trojanpanelnext/.github/workflows/publish-images.yml
done
mkdir "$WORKDIR/assets"
tar -xzf "$WORKDIR/$ARCHIVE" -C "$WORKDIR/assets"
cmp "$WORKDIR/release-manifest.json" "$WORKDIR/assets/release-manifest.json"
cmp "$WORKDIR/SHA256SUMS" "$WORKDIR/assets/SHA256SUMS"
(cd "$WORKDIR/assets" && sha256sum -c SHA256SUMS)
cd "$WORKDIR/assets"
```

Stop if verification fails. Leave the scripts, manifest, and original templates unchanged. Copy the required template to a mode `0600` working configuration and edit only that copy. `gh` is used for download and verification; running `bootstrap.sh` on the target host does not need a Git repository.

## Choose a deployment mode

Run the following commands from the verified `assets` directory. Check DNS, ports 80/443, and any Node protocol ports before `validate`; `install` modifies the host and requires root. See [Configuration options](#configuration-options) for every template field.

### Web control plane

```bash
cp ./config-web.yaml ./web.yaml
chmod 600 ./web.yaml
# Edit web.yaml: at least hostname and email; keep the pinned images and asset_version.
./bootstrap.sh validate --mode web --config ./web.yaml
sudo ./bootstrap.sh install --mode web --config ./web.yaml
```

### Separate Node

Install Web first. On the Web host, in the verified assets directory, create a Node identity and encrypted bootstrap bundle. Replace the example domain, public IP, and Web data-service addresses with real values. The control plane issues the identity ID, generation, and three dedicated credentials; do not manually use the template placeholders.

```bash
sudo install -d -m 0700 /tpdata/trojan-panel/config/node-identities
sudo docker exec trojan-panel /tpdata/trojan-panel/trojan-panel node-identity register \
  --name node-sg --domain node.example.com --public-ip 203.0.113.10 \
  --credential-file /tpdata/trojan-panel/config/node-identities/node-sg.g1.json
cp ./config-node.yaml ./node-sg.yaml
chmod 600 ./node-sg.yaml
# Edit node-sg.yaml: hostname, email, mariadb_host, redis_host, grpc_tls_server_name;
# Set control_plane_public_ip for a precise network allowlist;
# also set mariadb_port and redis_port if Web uses non-default ports.
sudo ./node-bundle create \
  --credential-file /tpdata/trojan-panel/config/node-identities/node-sg.g1.json \
  --node-config ./node-sg.yaml \
  --client-ca /tpdata/trojanpanelnext-pki/client-ca.crt \
  --output ./node-sg.g1.age
```

The credential file is root-managed, so the bundle command above also uses `sudo`. Transfer only `node-sg.g1.age` over a trusted channel into the Node host's verified `assets` directory. Download and verify the same Release on Node, then run:

```bash
sudo ./bootstrap.sh validate --mode node --bundle ./node-sg.g1.age
sudo ./bootstrap.sh install --mode node --bundle ./node-sg.g1.age
```

Installation waits for Web-to-Node mTLS verification. In another terminal on Web, use the identity ID and challenge printed by this Node installation:

```bash
sudo docker exec trojan-panel /tpdata/trojan-panel/trojan-panel \
  node-identity verify --id "<node-identity-id>" --challenge "<installer-printed-challenge>"
```

### Single-host combined

Run Web and Node on one host. The domains must differ and both resolve to that host's public IP. Shared Caddy owns ports 80/443; the installer registers the local Node identity.

```bash
cp ./config-combined.yaml ./combined.yaml
chmod 600 ./combined.yaml
# Edit combined.yaml: web_hostname, node_hostname, node_name, node_public_ip, email.
./bootstrap.sh validate --mode combined --config ./combined.yaml
sudo ./bootstrap.sh install --mode combined --config ./combined.yaml
```

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

The Release archive contains `config-web.yaml`, `config-node.yaml`, and `config-combined.yaml`; copy
the matching template into a working file before editing. The full list of behaviour the external entry
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

The installer checks configuration, dependencies, and deployment health, but it does not configure DNS, cloud security groups, or the host firewall. The operator must confirm DNS, open ports, and the target host first; the default ACME entry needs public access to ports 80 and 443. Install, removal, and certificate changes modify the target host directly and require operator review before execution in production.

After a successful install, the current topology's network allowlist is written to
`/tpdata/trojanpanelnext-network/allowlist.json` and `allowlist.md`. Review it first, then have the
host or cloud-security operator apply the rules; the installer never changes the firewall:

```bash
sudo sed -n '1,240p' /tpdata/trojanpanelnext-network/allowlist.md
sudo jq . /tpdata/trojanpanelnext-network/allowlist.json
sudo test ! -e /tpdata/trojan-panel-core/external/routes.json || \
  sudo jq '.routes[] | {network,port,external_fallback_listener_required}' \
    /tpdata/trojan-panel-core/external/routes.json
```

`ingress` entries are host inbound rules and `egress` lists DNS/HTTPS required for installation;
never expose MariaDB, Redis, panel API/UI, Core API, or gRPC to `0.0.0.0/0`. Node protocol ports
are direct kernel listeners: open only ports declared by `routes.json`. Fill missing or invalid
sources and rerun `validate` before applying any rule.

## Version and verification

`v0.1.0-rc.2` is the current prerelease candidate. It pins installation assets and image digests and does not update Docker `latest`. The old `v0.1.0-rc.1` never produced a complete Release and is unsuitable for this VPS test. The download commands above verify provenance for all three Release files, equality of the manifests inside and outside the archive, and every asset digest. `bootstrap.sh` also checks asset versions, image references, and the configuration before running the installer. Stop if any check fails.

## Security and manual gates

The following checks are human release points for every real deployment. The installer executes an approved local configuration and asset set; the operator owns these decisions:

- **Release gate**: confirm the candidate tag, the GitHub Release `isPrerelease` state, the attestation signer, image digests in `release-manifest.json`, and a passing `sha256sum -c SHA256SUMS`.
- **Host gate**: confirm a supported Debian 12 host with root access, no unauthorised existing containers or listeners, and operator-reviewed DNS, port 80/443, and Node-protocol firewall scope.
- **Entry gate**: standalone deployments use the installer-managed Caddy; confirm that DNS records are live and Caddy may request and renew ACME certificates before running install. An external entry and `--entry-spec` are a separate integration path.
- **Credential gate**: keep configuration and Node identity files at `0600`/root-only; transfer Node bootstrap bundles through a trusted channel, keep the password out of command arguments, and keep the CA and Web mTLS private keys on the Web control plane.
- **Change gate**: run `validate` first and retain its secret-free output, then manually check Web, Node, and mTLS/gRPC health after installation. Confirm `--force`, `remove`, `--purge-data`, and stable-version transitions separately.

Use a candidate only for isolated-environment acceptance. Complete acceptance, log review, certificate checks, and the production traffic decision before treating it as a stable release.

## After installing Web

For Web download, configuration, validation, and installation commands, see [Web control plane](#web-control-plane) above.

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

Each successful install writes `allowlist.json` and `allowlist.md` under
`${TP_DATA}/trojanpanelnext-network`. The plan separates public entry ports from restricted
MariaDB, Redis, Core API, gRPC, and panel ports. A Node plan uses `control_plane_public_ip` when
provided, while a Web plan reads the public IPs recorded in Node identity files. The installer only
generates this plan: it never invokes `nftables`, `ufw`, `iptables`, or a cloud security-group API.
Apply and review the rules with the host or cloud operator, and add only direct Node listener ports
declared in `routes.json`.

## Install a Node Agent

After registering the Node on the Web control plane, use `node-bundle` from the same Release to create
an encrypted bootstrap bundle. Copy the Node template to a mode-`0600` working file and set the Node
domain and Web MariaDB/Redis addresses; retain the archive's pinned image digests. The identity ID, generation, and three dedicated
credentials are injected from the file produced by `node-identity register|rotate`:

```bash
cp ./config-node.yaml ./node-sg.yaml
chmod 600 ./node-sg.yaml
sudo ./node-bundle create \
  --credential-file /tpdata/trojan-panel/config/node-identities/node-sg.g1.json \
  --node-config ./node-sg.yaml \
  --client-ca /tpdata/trojanpanelnext-pki/client-ca.crt \
  --output ./node-sg.g1.age
```

The password is read interactively and confirmed by default. Automation may set
`TP_NODE_BUNDLE_PASSWORD`, but there is no password command-line option. The fixed inventory is only
`config-node.yaml`, `manifest.json`, and `pki/client-ca.crt`. The public CA is parsed and validated;
the bundle contains neither `client-ca.key`, the Web `client.key`, nor any other private key. Transfer
only the `.age` file to the Node host through a trusted channel.

Install on the Node with the same Release. After verifying the Release assets, the installer opens
plaintext only in a private directory under `/dev/shm` and removes it on every success or failure exit.
The default is an interactive password prompt; explicitly pass the environment through `sudo` for
non-interactive operation:

```bash
sudo ./bootstrap.sh validate --mode node --bundle ./node-sg.g1.age
sudo ./bootstrap.sh install --mode node --bundle ./node-sg.g1.age
# Non-interactive example: sudo env TP_NODE_BUNDLE_PASSWORD="$TP_NODE_BUNDLE_PASSWORD" \
#   ./bootstrap.sh install --mode node --bundle ./node-sg.g1.age
```

The installer first checks MariaDB, the Redis cache ACL, and the Redis auth ACL with the Node's own
identities, then waits for Web-to-Node mTLS/gRPC verification. While the Node install is waiting, run
this from another terminal on the Web control plane:

```bash
sudo docker exec trojan-panel /tpdata/trojan-panel/trojan-panel \
  node-identity verify --id "<node-identity-id>" --challenge "<installer-printed-challenge>"
```

The challenge is random for this installation and must be copied from that installation's output.
The call uses the client certificate retained by Web and verifies the Node server certificate. Node
`/healthz` becomes ready only after the identity ID, generation, server ID, and challenge all match,
and installation succeeds only after all four checks pass. The Node rechecks all three data identities
over fresh connections at a fixed production cadence and exits within 10 seconds after invalidation.
After rotation or revocation, an old bundle cannot pass installation and an already-running
old-generation Node stops. Generate and install a new bundle for the new generation.

For a later removal, retain one complete configuration extracted from the bundle on the Node host
(it contains the dedicated credentials); the template `node-sg.yaml` alone is not sufficient. Restrict
the extracted file to mode `0600`, and remove the temporary extraction directory after copying it:

```bash
mkdir -m 700 ./node-bundle-extracted
./node-bundle extract --bundle ./node-sg.g1.age --directory ./node-bundle-extracted
install -m 600 ./node-bundle-extracted/config-node.yaml ./node-installed.yaml
rm -rf ./node-bundle-extracted
```

A mode-`0600` `--config` remains available for development, removal, and certificate refresh, but use
the encrypted `--bundle` for a production Node's initial install.

Only when an external host manager has generated a Protocol v1 EntrySpec should you pass it explicitly; a standalone deployment does not need this option:

```bash
sudo ./bootstrap.sh install --mode node --config ./node-sg.yaml \
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
sudo ./bootstrap.sh install --mode web --config ./web.yaml --force
sudo ./bootstrap.sh remove --mode web --config ./web.yaml --keep-data
sudo ./bootstrap.sh remove --mode node --config ./node-installed.yaml --purge-data
```

`--keep-data` overrides `purge_data` in the config for a recoverable removal;
`--purge-data` explicitly deletes generated data.

After removing one role from a combined deployment, replaying the original combined configuration does
not restore that role implicitly. Confirm that its domain, ports, and retained data still belong to this
deployment, then request the role explicitly:

```bash
sudo ./bootstrap.sh install --mode combined --config ./combined-site.yaml --restore-role web
```

`--restore-role` accepts only the currently inactive `web` or `node` role. Before restoring, the Entry
journal must still contain a matching 64-hex `committed_target.digest`; otherwise perform an explicit
migration first. A normal replay or certificate refresh never restores a removed role implicitly.

Combined mode requires two different domains that both resolve to the host's public IP. One shared
Entry owns ports 80/443, while Node kernel protocol listeners remain direct. The local Node uses its
own MariaDB and Redis identities issued by the Web control plane. Copy the Release archive's
`config-combined.yaml` to a mode-`0600` working file before editing and running:

```bash
cp ./config-combined.yaml ./combined-site.yaml
chmod 600 ./combined-site.yaml
# edit ./combined-site.yaml
./bootstrap.sh validate --mode combined --config ./combined-site.yaml
sudo ./bootstrap.sh install --mode combined --config ./combined-site.yaml
sudo ./bootstrap.sh refresh-cert --mode combined --config ./combined-site.yaml
```

`refresh-cert` does not take over ACME; the shared Entry remains the certificate producer. It restarts
Core only after the Node certificate generation changes. With a combined config,
`remove --mode web|node` removes one role, rewrites the shared Entry for the surviving domain, and
preserves shared MariaDB/Redis resources. Node removal revokes its control-plane identity first.
If Web was removed first, a one-shot control-plane CLI revokes the remaining Node identity.
Combined installation rejects an existing standalone Node Entry and verifies Web-to-Node mTLS/gRPC before success.

## Configuration options

All keys live under `trojanpanelnext:` in the YAML file. Use the templates from the Release archive. The `example.com` domains and `203.0.113.10` IP are placeholders to replace before installation. The archive already pins image fields to digests; retain them. The repository's `examples/` files use `latest` and are not candidate-release configurations.

### Shared by all three modes

| Key | Purpose and value |
| --- | --- |
| `schema_version` | Configuration format version; currently fixed at `1`. |
| `asset_version` | Must match the installer assets; already set by the rc.2 archive. Leave unchanged. |
| `deployment_mode` | Host role: `web`, `node`, or `combined`; must match `--mode`. |
| `email` | Contact address for Caddy/ACME; use an address that receives notifications. |
| `caddy_image`, `mariadb_image`, `redis_image` | Entry and data-service images, pinned to digests in the archive. |
| `api_image`, `web_image`, `node_agent_image` | Product images, pinned to digests. Every template keeps all three keys, though a mode may run only some services. |
| `force` | `0` for normal reconciliation; `1` to recreate existing containers and pull images again. A single install may also use `--force`. |
| `purge_data` | `0` retains generated data on removal; `1` removes it. `--keep-data` or `--purge-data` overrides it for one removal. |

### Web template `config-web.yaml`

| Key | Purpose and value |
| --- | --- |
| `hostname` | Public Web domain resolving to the Web host; Caddy serves HTTPS for it. |
| `mariadb_port`, `redis_port` | Web data-service ports; a separate Node must use the same ports, with access restricted to authorised Nodes. |
| `panel_port`, `ui_port` | Local API and Web UI ports proxied by the Entry; do not expose them to all sources. |
| `mariadb_password`, `redis_password`, `sysadmin_password` | May stay empty on first install; the installer generates and writes them back. Retain the written values on replay and protect the file. |
| `grpc_client_cert_path`, `grpc_client_key_path` | Web's client certificate and private-key paths for Node gRPC; the private key stays on Web. |
| `grpc_server_ca_path` | CA path for verifying the Node gRPC server certificate; the installer PKI flow handles the empty default. |
| `pki_bundle_dir` | Web mTLS/CA material, default `/tpdata/trojanpanelnext-pki`; Node bundling reads `client-ca.crt` here. |

### Node template `config-node.yaml`

| Key | Purpose and value |
| --- | --- |
| `hostname` | Node domain; it must match the registered `--domain` and certificate name. |
| `node_caddy_http_port`, `node_caddy_https_port` | Node HTTP/ACME and HTTPS Entry ports; template values are `80` and `8863`. Review the resulting allowlist. |
| `mariadb_host`, `mariadb_port`, `database`, `account_table` | Web database address, port, database, and account table. Set the actual Web host/port; keep the default database/table. |
| `mariadb_user`, `mariadb_password` | Dedicated Node database identity injected from registration into the encrypted bundle. Placeholders are unusable; never use Web root. |
| `redis_host`, `redis_port` | Web Redis address and port; set actual Web values. |
| `redis_username`, `redis_password` | Dedicated Node cache ACL identity injected from registration; never use Redis `default`. |
| `redis_auth_username`, `redis_auth_password` | Separate read-only auth ACL identity injected from registration; distinct from the cache identity. |
| `control_plane_public_ip` | Optional public Web IP for the data-plane inbound source plan; review the generated network plan if left empty. |
| `grpc_port`, `core_port` | Node gRPC management and internal Core ports; restrict gRPC to the Web control plane. |
| `node_server_id`, `node_identity_id`, `node_identity_generation` | Web-issued server ID, identity UUID, and credential generation; injected by `node-bundle`. Do not use template placeholders. |
| `grpc_tls_mode`, `grpc_tls_server_name` | Currently `mtls` only; server name must match the Node domain. |
| `grpc_client_ca_path` | Public CA file on the Node that verifies the Web client certificate. The bundle includes no Web private key. |
| `pki_bundle_dir`, `kernel_runtime_path` | Node PKI and proxy-kernel runtime directories; defaults suit standalone deployment. |

### Combined template `config-combined.yaml`

| Key | Purpose and value |
| --- | --- |
| `web_hostname`, `node_hostname` | Two distinct domains resolving to this host's public IP; shared Caddy owns 80/443. |
| `node_name`, `node_public_ip` | Stable local Node name and real public IP; the installer registers its dedicated identity. |
| `node_identity_credential_file` | Root-only local identity file under `/tpdata/trojan-panel/config/node-identities/`; the default directory is suitable. |
| `mariadb_port`, `redis_port`, `panel_port`, `ui_port` | Local data-service, API, and UI ports; they must not conflict with 80/443. |
| `core_port`, `grpc_port`, `grpc_tls_mode` | Local Core and gRPC ports; TLS mode is `mtls`. |
| `node_caddy_http_port`, `node_caddy_https_port` | Shared Entry ports; combined requires exactly `80` and `443`. |
| `pki_bundle_dir` | Local Web/Node PKI material, default `/tpdata/trojanpanelnext-pki`. |

### Optional external TLS keys

Add these keys to a copied `web` or separate `node` working configuration only when you manage the Entry yourself. Combined currently requires installer-managed Caddy.

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

## Release asset contents and trust boundary

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
15 assets, including `secure-file` and `node-bundle`, before a helper can first execute, then verify the configuration
contract again from the descriptor-safe snapshot before crossing the host mutation boundary.

The publishing workflow retains an SBOM and maximum provenance for every product image, and creates
GitHub artifact attestations for the three product images and the three Release files (archive,
manifest, and SHA256SUMS). It re-verifies each subject/digest against the fixed publishing-workflow
signer; downloaded assets should do the same rather than trusting the repository name alone:

The release configuration contract uses `deployment_mode`, `api_image`, `web_image`, and
`node_agent_image`. The legacy `purpose`, `panel_image`, `ui_image`, and `core_image` keys are
accepted only when reading existing installer configurations and are not emitted in new templates.

The installer fully orchestrates `combined`: Web and Node share local data services and one shared
Entry with separate certificates for both domains. Node kernel listeners are never forwarded through
a unified L4 entry. See
[example-release-manifest.json](release/example-release-manifest.json) for a secret-free manifest
example. The Release tar.gz preserves executable modes and is accompanied by the manifest and
SHA256SUMS. As in the candidate procedure above, verify the three Release-file attestations before extracting and checking the bundled digests. Download RC and stable versions by their explicit tag; do not select a candidate through `latest`:

```bash
archive=trojanpanelnext-installer-<version>.tar.gz
mkdir trojanpanelnext-installer
tar -xzf "${archive}" -C trojanpanelnext-installer
cd trojanpanelnext-installer
sha256sum -c SHA256SUMS
```

Copy and edit a template instead of changing the digest-protected file in the release bundle,
then validate the deployment configuration:

```bash
cp ./config-web.yaml ./deployment.yaml
./bootstrap.sh validate --mode web --config ./deployment.yaml
```

`verify-assets.sh` can also inspect a downloaded bundle and configuration; use `bootstrap.sh` for normal installation so verification runs before the installer.

## Support

Project origin: [the original TrojanPanel project](https://github.com/trojanpanel).
