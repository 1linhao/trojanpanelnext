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

## Versioned Releases and the candidate

Production deployments should use one matching Release archive, `release-manifest.json`, `SHA256SUMS`, and `bootstrap.sh`. The archive provides `config-web.yaml`, `config-node.yaml`, and `config-combined.yaml`; copy the matching template for each deployment mode. Do not substitute `latest` images or an unfixed script from a Git branch. The planned candidate tag is `v0.1.0-rc.1`; do not treat it as published until the tag and Release have actually been created and verified. Its asset version is `0.1.0-rc.1`, and its images and configuration are pinned to that version and their digests. An RC does not update Docker `latest`.

Pushing a version tag runs `.github/workflows/publish-images.yml` and creates the GitHub Release. Only an authorised release maintainer should create and push a candidate tag from an approved commit:

```bash
TAG=v0.1.0-rc.1
git tag -a "$TAG" -m "TrojanPanel Next $TAG"
git push origin "refs/tags/$TAG"
```

The current workflow does not set GitHub's prerelease flag automatically. After the workflow succeeds and before distributing or installing the candidate, the release maintainer must inspect and mark the Release as a prerelease:

```bash
gh release view "$TAG" --repo 1linhao/trojanpanelnext --json tagName,isPrerelease
gh release edit "$TAG" --repo 1linhao/trojanpanelnext --prerelease
gh release view "$TAG" --repo 1linhao/trojanpanelnext --json tagName,isPrerelease
```

Distribute only after `tagName` is `v0.1.0-rc.1` and `isPrerelease` is `true`. A candidate is for isolated-environment acceptance and is not production approval; the release maintainer decides whether to publish a stable version after acceptance and review.

From a trusted administration workstation, download and verify the three signed Release files, then extract and verify every asset digest:

```bash
TAG=v0.1.0-rc.1
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
(cd "$WORKDIR/assets" && sha256sum -c SHA256SUMS)
```

Copy a versioned template to a mode-`0600` configuration file and edit domains and deployment values. Run the read-only validation first, review its output and the target host, then explicitly run the root-required install:

```bash
cd "$WORKDIR/assets"
cp ./config-web.yaml ./web-site.yaml
chmod 600 ./web-site.yaml
# edit ./web-site.yaml
./bootstrap.sh validate --mode web --config ./web-site.yaml
sudo ./bootstrap.sh install --mode web --config ./web-site.yaml
```

`bootstrap.sh` verifies the version, fixed asset set, digests, image references, and configuration before invoking the installer. The installer verifies the secure configuration snapshot again before crossing the host mutation boundary. Attestations verify release provenance and SHA256 verifies archive contents; both must pass. Do not continue when the signer, version, or digest check fails.

## Security and manual gates

The following checks are human release points for every real deployment. The installer executes an approved local configuration and asset set; the operator owns these decisions:

- **Release gate**: confirm the candidate tag, the GitHub Release `isPrerelease` state, the attestation signer, image digests in `release-manifest.json`, and a passing `sha256sum -c SHA256SUMS`.
- **Host gate**: confirm a supported Debian 12 host with root access, no unauthorised existing containers or listeners, and operator-reviewed DNS, port 80/443, and Node-protocol firewall scope.
- **Entry gate**: standalone deployments use the installer-managed Caddy; confirm that DNS records are live and Caddy may request and renew ACME certificates before running install. An external entry and `--entry-spec` are a separate integration path.
- **Credential gate**: keep configuration and Node identity files at `0600`/root-only; transfer Node bootstrap bundles through a trusted channel, keep the password out of command arguments, and keep the CA and Web mTLS private keys on the Web control plane.
- **Change gate**: run `validate` first and retain its secret-free output, then manually check Web, Node, and mTLS/gRPC health after installation. Confirm `--force`, `remove`, `--purge-data`, and stable-version transitions separately.

Use a candidate only for isolated-environment acceptance. Complete acceptance, log review, certificate checks, and the production traffic decision before treating it as a stable release.

## Install the Web control plane

```bash
cp ./config-web.yaml ./web.yaml
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

Only when an external host manager has generated a Protocol v1 EntrySpec should you pass it explicitly; a standalone deployment does not need this option:

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

After removing one role from a combined deployment, replaying the original combined configuration does
not restore that role implicitly. Confirm that its domain, ports, and retained data still belong to this
deployment, then request the role explicitly:

```bash
sudo ./install.sh install --mode combined --config ./combined-site.yaml --restore-role web
```

`--restore-role` accepts only the currently inactive `web` or `node` role. A normal replay or certificate
refresh never restores a removed role implicitly.

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

## Configuration files

The Release `config-web.yaml` contains the hostname, images, ports, mTLS identity directory, and internal credentials.

The Release `config-node.yaml` contains the node hostname, control-plane database and Redis connections, gRPC settings, public CA directory, and certificate paths.

The Release `config-combined.yaml` contains both domains, the local Node public IP, its
dedicated identity credential path, and shared Entry ports. `node_identity_credential_file` must remain
under `/tpdata/trojan-panel/config/node-identities/` and root-only.

For external TLS, use a copied `config-web.yaml` or `config-node.yaml` and set these keys:

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
