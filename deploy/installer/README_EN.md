# TrojanPanel Next Installer

[简体中文](README.md) | English

Download the `tp.sh` command entrypoint, fetch a configuration template, edit the YAML, and install. The entrypoint downloads `common.sh` and the selected command script; installation also downloads the removal script for the Node host maintenance service. `trojanpanelnext.purpose` is the only source of deployment type: `web` or `node`. Commands are non-interactive.

## System and software dependencies

Use Ubuntu 20.04+, Debian 11+, or an equivalent Linux system, on `linux/amd64` or `linux/arm64`. At least 1 GiB RAM is recommended. Installation and removal require root. Node hosts must run systemd.

**The installer does not install yq, Docker, or other software tools. Install dependencies before running installation commands.**

| Command | Required software |
| --- | --- |
| All entrypoint commands | Bash, curl, CA certificates, grep, coreutils (including mktemp, chmod, rm) |
| `config` | The above tools, coreutils ln/dirname, and a writable destination directory |
| `validate` | The above tools and [mikefarah/yq v4](https://github.com/mikefarah/yq), not the Python package named yq |
| `install` | The above tools, Docker Engine, mikefarah/yq v4, OpenSSL, tar, coreutils, findutils, awk; Node also requires systemd |
| `remove` | The above tools, Docker Engine, mikefarah/yq v4, coreutils (including realpath, rmdir); Node maintenance service cleanup requires systemctl |

On Debian/Ubuntu:

```bash
sudo apt-get update
sudo apt-get install -y bash grep curl ca-certificates openssl tar coreutils findutils gawk docker.io
sudo systemctl enable --now docker
```

Manually install checksum-verified yq v4.53.6:

```bash
case "$(uname -m)" in
  x86_64 | amd64)
    yq_arch=amd64
    yq_sha=c5f056448f973ae7d39b5401949648a78f2dc1947d6a8eb65be60d5c504b9385
    ;;
  aarch64 | arm64)
    yq_arch=arm64
    yq_sha=88a1016bc1d657375a35864e4f44b6f333df8ff97b559f51bba0adcb2169df09
    ;;
  *) echo "Unsupported architecture" >&2; exit 1 ;;
esac
yq_download="$(mktemp)"
curl -fsSL --connect-timeout 10 --max-time 120 \
  "https://github.com/mikefarah/yq/releases/download/v4.53.6/yq_linux_${yq_arch}" \
  -o "$yq_download"
printf '%s  %s\n' "$yq_sha" "$yq_download" | sha256sum -c - && \
  sudo install -m 0755 "$yq_download" /usr/local/bin/yq
rm -f -- "$yq_download"
yq --version
sudo docker info >/dev/null
```

Omit `sudo` when logged in as root. Use the appropriate package manager on other distributions. Docker must already be running.

## Release binding and entrypoint download

The current version is `0.1.0-rc.6`. Run on each Web or Node server:

```bash
curl -fsSL --connect-timeout 10 --max-time 60 \
  https://raw.githubusercontent.com/1linhao/trojanpanelnext/v0.1.0-rc.6/deploy/installer/tp.sh \
  -o tp.sh
chmod +x tp.sh
./tp.sh --version
```

By default, the entrypoint downloads the selected command and `common.sh` from GitHub Raw at `v0.1.0-rc.6`; installation additionally downloads `uninstall.sh`. Downloaded files undergo version matching and Bash syntax checks, run in a temporary directory, and are cleaned up afterwards. Templates default to the same tag. API, Web, and Node Agent images use `:0.1.0-rc.6`, without the Git tag's `v` prefix. No local checkout is needed.

`schema_version: 1` describes the YAML structure and is independent of the product version. Existing configurations should use `trojanpanelnext.purpose: web` or `node`; old deployment-type options are no longer supported.

## Entrypoint and command scripts

```text
./tp.sh config web|node [--output <file>]
./tp.sh validate --config <file>
./tp.sh install --config <file> [--force]
./tp.sh remove --config <file> [--keep-data | --purge-data]
```

`--purge` is an alias for `--purge-data`; it cannot be combined with `--keep-data`. `--force` applies only to installation; data options apply only to removal. Use `./tp.sh <command> --help` for command help.

| Entrypoint command | Script and direct invocation |
| --- | --- |
| `config` | `bash config.sh web\|node [--output <file>]` |
| `validate` | `bash validate.sh --config <file>` |
| `install` | `bash install.sh --config <file> [--force]` |
| `remove` | `bash uninstall.sh --config <file> [--keep-data \| --purge-data]` |

Direct script invocation requires a matching `common.sh` in the same directory. Node installation also requires `uninstall.sh` there. Command scripts no longer accept the `config`, `validate`, `install`, or `remove` prefix. Downloading `install.sh` alone is insufficient; use the entrypoint to obtain matching files.

## Network preparation

Point each domain's A/AAAA records at its server, and keep configured ports available.

| Direction | Default ports |
| --- | --- |
| Browsers and certificate authority → Web | TCP 80 and 443 for Caddy and the HTTPS panel |
| Node → Web | TCP 9507 for MariaDB and 6378 for Redis; restrict access to trusted Node sources |
| Node host maintenance service → Web | TCP 443 for the HTTPS removal result confirmation callback |
| Certificate authority → Node | TCP 80 for the default HTTP domain validation |
| Web → Node | TCP 8100 for mTLS gRPC; allow only Web sources |
| Web → Node host maintenance service | TCP 8101 for mTLS HTTPS removal; always `grpc_port + 1`, allow only Web sources |
| Visitors → Node camouflage site | TCP 8863 for Caddy HTTPS |
| Proxy clients → Node | The actual TCP/UDP proxy ports configured in the panel |

API 8081, UI 8888, and Node API 8082 listen on host networking; avoid exposing all internal ports publicly. Update firewall rules and YAML when changing ports. The maintenance port cannot be configured independently; `grpc_port` must be at most `65534`. Published AAAA records also require working IPv6 connectivity.

## Install Web

```bash
./tp.sh config web
nano web.yaml
./tp.sh validate --config ./web.yaml
sudo ./tp.sh install --config ./web.yaml
```

Set `hostname` and `email`. `config` creates `web.yaml` with mode `0600`, rejects existing files and symlinks, and never publishes a partial download. Use `--output ./production-web.yaml` to select another path.

The first installation generates MariaDB and Redis passwords, writes them back to `web.yaml`, and creates the control-plane mTLS identity in `pki_bundle_dir`. Keep the configuration and PKI secure; never commit deployment credentials to Git.

Visit `https://<hostname>`. The initial account is `sysadmin`, with default password `123456`; change it after signing in. Database and Redis credentials are separate from panel login credentials.

## Install Node

Install Web first, then create a node server in the panel with the Node address, gRPC port, and TLS server name (the Node domain). Obtain its actual server ID and set the Node YAML's `node_server_id`. The ID must be at least `1`; do not copy the template's example ID.

Securely copy Web's public `client-ca.crt` to the Node's `pki_bundle_dir`, defaulting to `/tpdata/trojanpanelnext-pki`. **Do not copy `client-ca.key`, `client.key`, or `client.crt`.**

```bash
./tp.sh config node
nano node.yaml
./tp.sh validate --config ./node.yaml
sudo ./tp.sh install --config ./node.yaml
```

Set the Node's `hostname` and `email`, Web's database/Redis addresses and passwords, and matching ports. Match `node_server_id` to the actual server ID in the panel; the template ID and passwords are examples. Node must reach Web's database and Redis; the host maintenance service also needs Web HTTPS (TCP 443) to confirm removal results. Server registration and CA transfer remain manual.

Node installation provisions `trojanpanelnext-host.service` on the host. It provides an mTLS HTTPS maintenance endpoint at `grpc_port + 1` (default `8101`). Restrict it to Web sources with the firewall. It executes removal using the saved local configuration and script. The Agent container has no `docker.sock` mount. Check the service with `sudo systemctl status trojanpanelnext-host.service` after installation.

`validate` checks only YAML and fields; it does not verify DNS, certificate files, connectivity, systemd status, or service health. Passing validation does not establish that the host is ready for deployment.

## Recreate containers and upgrade

Use `--force` to recreate API, UI, Agent, and Caddy containers after updating application images. To upgrade to `0.1.0-rc.6`, first set `core_image` to `:0.1.0-rc.6` in every Node YAML, then update `panel_image` and `ui_image` in Web YAML. Download this release's entrypoint on each host and run in this order:

```bash
# Run on every Node host first to provision the host maintenance service
sudo ./tp.sh install --config ./node.yaml --force
# Run on Web after all Nodes have been updated successfully
sudo ./tp.sh install --config ./web.yaml --force
```

Before upgrading, set the actual `node_server_id` in existing Node YAML and permit Web access to the maintenance port. Older Nodes without this service cannot perform host removal through Web.

Existing MariaDB and Redis containers are retained. Installation normally reuses local images; forced application recreation pulls the configured images. Do not use `--force` with unpublished test images.

## Removal

Run on the corresponding server. Uninstall while retaining data:

```bash
sudo ./tp.sh remove --config ./web.yaml --keep-data
sudo ./tp.sh remove --config ./node.yaml --keep-data
```

Uninstall completely:

```bash
sudo ./tp.sh remove --config ./web.yaml --purge-data
sudo ./tp.sh remove --config ./node.yaml --purge-data
```

| Behavior | `--keep-data` | `--purge-data` / `--purge` |
| --- | --- | --- |
| Corresponding containers and anonymous volumes | Removed | Removed |
| All local tags in repositories used by containers and configuration | Remove images unused by other containers | Remove images unused by other containers |
| Service data, PKI, camouflage site, custom kernel runtime | Retained | All project service directories and these paths removed |
| Original deployment YAML | Retained | Removed |
| Installed host maintenance service and its working copies | Removed | Removed |
| Manually downloaded `tp.sh` and Docker Engine itself | Retained | Retained |

Web removal targets MariaDB, Redis, API, UI, and Web Caddy containers; Node removal targets Agent and Node Caddy containers. Removal covers repositories named in the configuration and those actually used by containers, including all local historical tags in the default GHCR repositories for the corresponding purpose. Shared images still referenced by other containers are retained with a message. No `docker system prune` or global image cleanup runs.

Full removal deletes all project service data directories (including remnants of an inactive deployment purpose), `pki_bundle_dir`, camouflage content at `WEB_PATH`, custom `kernel_runtime_path`, configured identity files, and original YAML. Node maintenance files reside at `/etc/trojanpanelnext-host`, `/usr/local/lib/trojanpanelnext-host`, and `/etc/systemd/system/trojanpanelnext-host.service`. Full removal is refused before deletion while containers for the other purpose remain on the same host, to protect shared data.

Without a data option, YAML `purge_data` selects the mode; templates default to `0`. `--keep-data` overrides YAML `purge_data: 1`; `--purge-data` explicitly enables full removal. Usually keep YAML `force` and `purge_data` at `0` and choose actions on the command line.

### Delete a node server from Web

Ordinary deletion on the node server page first asks Node's host maintenance service to uninstall containers and images while retaining data. After the host reports success, Web transactionally deletes the server and associated proxy configurations, retaining that server's traffic and kernel task history. Deletion with data purge also clears the corresponding Web traffic and kernel task records. Shared task records involving other servers are retained.

When a node is offline, an older Node has no maintenance service, mTLS/connectivity fails, or local removal fails, Web returns an error and retains the server registration and associated proxy configurations. Connection failure is never treated as successful removal. Deleting one proxy node still deletes only that proxy, without uninstalling the host.

During remote removal, the maintenance service temporarily retains TLS identity and the removal receipt until Web commits its deletion transaction and both sides confirm the result. It then removes the maintenance service and files. Result confirmation and maintenance service cleanup retry persisted pending work in the background on failure, and resume after a service or host restart. When the UI reports pending cleanup (`cleanupPending`), the main host services and Web records have been removed, but temporary maintenance files still await cleanup. Keep Web access to the Node maintenance port and Node access to Web HTTPS (TCP 443) available until cleanup completes.

Local `tp.sh remove` only affects the host and does not remove Web registration. To coordinate Node removal and registration cleanup, initiate deletion directly on Web's node server page.

## Development verification

`TP_SCRIPT_REF` defaults to `v0.1.0-rc.6`. Set it to a branch or full commit SHA that exists on GitHub to fetch scripts and templates from the same ref. The entrypoint still requires command versions to match its own version; when testing branch code, also download `tp.sh` from that branch. For example:

```bash
TP_SCRIPT_REF=feat/installer-entrypoint ./tp.sh config web --output ./test-web.yaml
TP_SCRIPT_REF=feat/installer-entrypoint ./tp.sh validate --config ./test-web.yaml
sudo env TP_SCRIPT_REF=feat/installer-entrypoint ./tp.sh install --config ./test-web.yaml
```

`TP_CONFIG_REF` takes precedence and can override only the template source. Otherwise, templates follow `TP_SCRIPT_REF`. These overrides do not change edited configuration image tags or the entrypoint version. Use corresponding images built from the tested source. Normal installation does not need these variables.

Run `node scripts/check-installer-release.mjs` before release. Publishing verifies that the Git tag, entrypoint/command/common script versions, and default template image tags agree. Pushing images also requires the remote version tag to exist and point to the checked-out commit; manual builds without publication do not require an existing tag.

## Support

[Original TrojanPanel project](https://github.com/trojanpanel).

## Automatic certificate maintenance

- Caddy renews public Web and Node certificates. Preserve its data directories and keep DNS and ACME validation reachable.
- Every minute, the Agent checks NaiveProxy certificates. It validates the certificate/key pair, saves the live configuration including users, validates that configuration, and restarts only affected instances. Existing connections disconnect briefly and clients must reconnect. Failures are logged and retried on the next pass.
- At startup and every 5 minutes, the Web API renews its internal mTLS client certificate when fewer than 90 days remain. Certificates last at most 825 days and never outlive their CA. CA private keys stay on Web.
- With fewer than 365 days remaining, CA rotation distributes both CAs over existing authenticated mTLS connections. The client identity switches only after every registered mTLS Node acknowledges. The previous identity is retained for at least 24 hours, then retired after all nodes acknowledge removal of the old CA. Offline or older unsupported nodes block progress; retries run every 5 minutes.
- Initial bootstrap still requires copying Web's current public `client-ca.crt`. Register new nodes before rotation and use the current bundle; unregistered nodes are outside the rotation inventory. Upgrade all Nodes before Web, update existing YAML image tags to `0.1.0-rc.6`, and run `./tp.sh install --config ... --force` to update mounts and provision Node host maintenance. Node reinstalls preserve the live CA file rather than replacing it with an old bootstrap copy. The Agent also updates the public bootstrap bundle for reinstalls after runtime data removal.
- The API mounts `pki_bundle_dir` with signing access; Nodes can write only their public trust file. Back up the entire Web PKI directory, including `state.json` and `generations`, rather than only the top-level symlinks. Inspect API / Agent logs for rotation failures. Nodes offline past the previous CA's expiry, or restored from expired backups, require manual trust bootstrap.
