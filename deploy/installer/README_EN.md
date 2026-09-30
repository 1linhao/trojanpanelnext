# TrojanPanel Next Installer

[简体中文](README.md) | English

Download a standalone `install.sh`, fetch a configuration template, edit the YAML, and install. `trojanpanelnext.purpose` is the only source of deployment type: `web` or `node`. Commands are non-interactive.

## System and software dependencies

Use Ubuntu 20.04+, Debian 11+, or an equivalent Linux system, on `linux/amd64` or `linux/arm64`. At least 1 GiB RAM is recommended. Installation and removal require root.

**The installer does not install yq, Docker, or other software tools. Install dependencies before running installation commands.**

| Command | Required software |
| --- | --- |
| `config` | curl, coreutils (mktemp, chmod, ln, dirname, rm), and a writable destination directory |
| `validate` | [mikefarah/yq v4](https://github.com/mikefarah/yq), not the Python package named yq |
| `install` | Docker Engine, mikefarah/yq v4, curl, OpenSSL, tar, coreutils, findutils, awk |
| `remove` | Docker Engine and mikefarah/yq v4 |

On Debian/Ubuntu:

```bash
sudo apt-get update
sudo apt-get install -y curl ca-certificates openssl tar coreutils findutils gawk docker.io
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

## Release binding and installer download

The installer version is `0.1.0-rc.5`. Run on each Web or Node server:

```bash
curl -fsSL --connect-timeout 10 --max-time 60 \
  https://raw.githubusercontent.com/1linhao/trojanpanelnext/v0.1.0-rc.5/deploy/installer/install.sh \
  -o install.sh
chmod +x install.sh
./install.sh --version
```

Installer `0.1.0-rc.5` downloads templates only from GitHub Raw at `v0.1.0-rc.5`. Product images use `:0.1.0-rc.5`, without the Git tag's `v` prefix. No local checkout is needed. `schema_version: 1` describes the YAML structure and is independent of the product version. The previous deployment-type CLI option has been removed; existing YAML using `purpose` can use the new commands.

## Network preparation

Point each domain's A/AAAA records at its server, and keep configured ports available.

| Direction | Default ports |
| --- | --- |
| Browsers and certificate authority → Web | TCP 80 and 443 for Caddy and the HTTPS panel |
| Node → Web | TCP 9507 for MariaDB and 6378 for Redis; restrict access to trusted Node sources |
| Certificate authority → Node | TCP 80 for the default HTTP domain validation |
| Web → Node | TCP 8100 for mTLS gRPC |
| Visitors → Node camouflage site | TCP 8863 for Caddy HTTPS |
| Proxy clients → Node | The actual TCP/UDP proxy ports configured in the panel |

API 8081, UI 8888, and Node API 8082 listen on host networking; avoid exposing all internal ports publicly. Update firewall rules when changing ports. Published AAAA records also require working IPv6 connectivity.

## Install Web

```bash
./install.sh config web
nano web.yaml
./install.sh validate --config ./web.yaml
sudo ./install.sh install --config ./web.yaml
```

Set `hostname` and `email`. `config` creates `web.yaml` with mode `0600`, rejects existing files and symlinks, and never publishes a partial download. Use `--output ./production-web.yaml` to select another path.

The first installation generates MariaDB and Redis passwords, writes them back to `web.yaml`, and creates the control-plane mTLS identity in `pki_bundle_dir`. Keep the configuration and PKI secure; never commit deployment credentials to Git.

Visit `https://<hostname>`. The initial account is `sysadmin`, with default password `123456`; change it after signing in. Database and Redis credentials are separate from panel login credentials.

## Install Node

Install Web first, then create a node server in the panel with the Node address, gRPC port, and TLS server name (the Node domain). Record its actual server ID in the Node YAML's `node_server_id`.

Securely copy Web's public `client-ca.crt` to the Node's `pki_bundle_dir`, defaulting to `/tpdata/trojanpanelnext-pki`. Do not copy `client-ca.key`, `client.key`, or `client.crt`.

```bash
./install.sh config node
nano node.yaml
./install.sh validate --config ./node.yaml
sudo ./install.sh install --config ./node.yaml
```

Set the Node's `hostname` and `email`, Web's database/Redis addresses and passwords, and matching ports. Match `node_server_id` to the actual server ID in the panel; the template ID and passwords are examples. Node must reach Web's database and Redis. Enrollment, server registration, and CA transfer remain manual.

`validate` checks YAML and fields; it does not verify DNS, certificate files, network connectivity, or service health.

## Recreate containers and remove services

Recreate API, UI, Agent, and Caddy containers after updating application images:

```bash
sudo ./install.sh install --config ./web.yaml --force
sudo ./install.sh install --config ./node.yaml --force
```

Existing MariaDB and Redis containers are retained. Installation normally reuses local images; forced application recreation pulls the configured images. Do not use `--force` with unpublished test images.

Remove services while retaining data:

```bash
sudo ./install.sh remove --config ./web.yaml
sudo ./install.sh remove --config ./node.yaml
```

Remove services and their corresponding data directories:

```bash
sudo ./install.sh remove --config ./web.yaml --purge-data
sudo ./install.sh remove --config ./node.yaml --purge-data
```

Web removes MariaDB, Redis, API, UI, and Web Caddy data. Node removes Agent and Node Caddy data. Both retain `pki_bundle_dir` and the camouflage site at `WEB_PATH`. Manage custom runtime paths outside the default data directories separately. The original YAML remains. `--force` is only valid with `install`; `--purge-data` is only valid with `remove`. Keep YAML `force` and `purge_data` at `0` unless intended.

## Development verification

Developers can set `TP_CONFIG_REF` to a branch or full commit SHA that exists on GitHub. This changes only the template source; it does not change the installer version or images in edited configurations. Use corresponding images built from the tested source. Normal installation does not need this override.

Run `node scripts/check-installer-release.mjs` before release. Publishing verifies that the Git tag, installer version, and default template image tags agree. Pushing images also requires the remote version tag to exist and point to the checked-out commit; manual builds without publication do not require an existing tag.

## Support

[Original TrojanPanel project](https://github.com/trojanpanel).

## Automatic certificate maintenance

- Caddy renews public Web and Node certificates. Preserve its data directories and keep DNS and ACME validation reachable.
- Every minute, the Agent checks NaiveProxy certificates. It validates the certificate/key pair, saves the live configuration including users, validates that configuration, and restarts only affected instances. Existing connections disconnect briefly and clients must reconnect. Failures are logged and retried on the next pass.
- At startup and every 5 minutes, the Web API renews its internal mTLS client certificate when fewer than 90 days remain. Certificates last at most 825 days and never outlive their CA. CA private keys stay on Web.
- With fewer than 365 days remaining, CA rotation distributes both CAs over existing authenticated mTLS connections. The client identity switches only after every registered mTLS Node acknowledges. The previous identity is retained for at least 24 hours, then retired after all nodes acknowledge removal of the old CA. Offline or older unsupported nodes block progress; retries run every 5 minutes.
- Initial bootstrap still requires copying Web's current public `client-ca.crt`. Register new nodes before rotation and use the current bundle; unregistered nodes are outside the rotation inventory. Upgrade all Nodes before Web, update existing YAML image tags to `0.1.0-rc.5`, and run `install --config ... --force` to update mounts. Node reinstalls preserve the live CA file rather than replacing it with an old bootstrap copy. The Agent also updates the public bootstrap bundle for reinstalls after runtime data removal.
- The API mounts `pki_bundle_dir` with signing access; Nodes can write only their public trust file. Back up the entire Web PKI directory, including `state.json` and `generations`, rather than only the top-level symlinks. Inspect API / Agent logs for rotation failures. Nodes offline past the previous CA's expiry, or restored from expired backups, require manual trust bootstrap.
