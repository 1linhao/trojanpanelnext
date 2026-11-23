# TrojanPanel Next

[简体中文](README.md) | English

TrojanPanel Next is a multi-user proxy management platform supporting Xray, Hysteria2, and NaiveProxy. The Web control plane manages accounts, node servers, proxies, traffic, and tasks. Node Agents manage proxy runtimes on each server.

Current pre-release: **v1.0.2-rc.13 (Pre-release)**. Deployment uses GHCR images for Linux amd64 and arm64.

## Start here

- [Quick installation on one or two servers](#quick-install)
- [Configuration deployment (alternative)](#configuration-alternative)
- [Update product images](#product-update)
- [Remove the project and dependencies](#project-removal)
- [Complete deployment guide](docs/deployment_EN.md)

<a id="quick-install"></a>
## Quick installation

| What you have | Choose |
| --- | --- |
| Two VPSs, one for Web and one for Node, with a domain pointing to each | [Option A · Separate servers](#quick-install-separate) |
| One VPS for both Web and Node, with two domains pointing to it | [Option B · One server](#quick-install-colocated) |

For either option, install Web first and download the Node deployment package from the panel. Replace `panel.example.com` and `node.example.com` with your domains. These commands are for a fresh installation in **root Bash** on Debian 12/13 or Ubuntu 22.04/24.04 (amd64 or arm64) running systemd. Follow the full guide for other Linux distributions. [Manual dependencies](docs/deployment_EN.md#dependency-manual)

### Install Web first

Point the panel domain at the **Web VPS** public IP and allow TCP `80` and `443`. If you signed in over SSH as a regular user, run `sudo -i` first. Paste the entire block into a root Bash session on that VPS.

The v1.0.2-rc.13 entrypoint omits `validate.sh` from the quick install downloads. The block below verifies the official entrypoint's SHA-256 and adds the missing download to a temporary copy. The script library, templates, and application images all come from v1.0.2-rc.13.

```bash
(
  set -e -o pipefail
  apt-get update
  apt-get install -y bash curl ca-certificates grep coreutils util-linux tar gzip
  install -d -m 700 /root/tpnext-web
  cd /root/tpnext-web
  umask 077
  entry="$(mktemp -p . .tpnext-rc13.XXXXXX)"
  trap 'unlink "$entry"' EXIT
  curl -fsSL --proto '=https' --proto-redir '=https' --retry 2 --connect-timeout 10 --max-time 60 \
    https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.13/scripts/tp.sh -o "$entry"
  printf '%s  %s\n' d90249d6281087aeb1701e338215078ccdadb18b094137448f06229358952f15 "$entry" | sha256sum -c
  sed -i 's/for dependency in quick\.sh config\.sh install\.sh uninstall\.sh; do/for dependency in quick.sh config.sh validate.sh install.sh uninstall.sh; do/' "$entry"
  bash -n "$entry"
  bash "$entry" deps install
  bash "$entry" web
)
```

Enter the panel domain and certificate contact email when prompted. Open the HTTPS address, sign in with `sysadmin` / `123456`, and change the password. Keep `/root/tpnext-web/web.yaml` for updates and removal.

For a later release that includes this fix, save `scripts/tp.sh` from that release tag as `tp.sh`. Run `bash ./tp.sh deps install` and then `bash ./tp.sh web` from the installation directory.

Before uploading the Node deployment package, SSH into the VPS that will run Node and run `install -d -m 700 "$HOME/tpnext-node"` under your SSH account. Upload the file there with SFTP and note the absolute path. After switching to root Bash, use `cd` to enter that directory. Use a separate extraction directory for each Node package.

<a id="quick-install-separate"></a>
### Option A · Separate servers

1. Point the Node domain at the **Node VPS** public IP. Allow public TCP `80` on Node for Caddy certificate issuance. On Web, allow TCP `9507` and `6378` from Node only. On Node, allow TCP `8100` and `8101` from Web only. Configure these source restrictions in your VPS firewall or security group. [Network requirements](docs/deployment_EN.md#network)
2. In the panel, open **Server management → Add Node server**. Enter a name, put the Node address reachable from Web in **Node connection address**, set **gRPC port** to `8100`, and enter the Node domain under **gRPC certificate domain**.
3. In **Deploy Node**, choose **Caddy automatic issuance and renewal** and enter the certificate email. For **Web address reachable from Node**, use a Web domain or private IP that Node can reach. Confirm the database and Redis addresses and select **Next → Download package**.
4. Use your SSH client's SFTP feature to upload the downloaded `tpnext-node-*.tar.gz` into a private directory on the **Node VPS**. Enter that directory in root Bash, paste this block, and enter the exact filename when prompted.

```bash
(
  set -e
  umask 077
  read -r -p 'Enter the uploaded package filename > ' bundle
  [[ "$bundle" == tpnext-node-*.tar.gz && -f "$bundle" ]] ||
    { printf '%s\n' 'Package not found. Check the current directory and filename.' >&2; exit 1; }
  chmod 600 -- "$bundle"
  apt-get update
  apt-get install -y bash curl ca-certificates grep coreutils util-linux tar gzip
  tar -xzf "$bundle"
  bash ./tpnext/install-dependencies.sh
  bash ./tpnext/install-node.sh
)
```

<a id="quick-install-colocated"></a>
### Option B · One server, with room to add remote Nodes

Web and Node both run on the **same VPS**. Node uses a host certificate. If you already have a certificate covering the Node domain, use its full chain and matching unencrypted private key. [External Node certificates](docs/deployment_EN.md#external-certificates)

On a fresh VPS, use Certbot's DNS-01 challenge to issue a separate certificate for Node. You need permission to add a DNS TXT record for the Node domain. DNS validation works while Web Caddy is running. Replace both occurrences of `node.example.com` with your Node domain and run this block in root Bash on that VPS.

```bash
apt-get install -y certbot &&
certbot certonly --manual --preferred-challenges dns \
  --cert-name tpnext-node-node.example.com -d node.example.com
```

Add the `_acme-challenge.node.example.com` TXT record requested by Certbot. Wait until public DNS returns the displayed value before continuing. After issuance, the full chain and private key are `fullchain.pem` and `privkey.pem` under `/etc/letsencrypt/live/tpnext-node-node.example.com/`. For manual DNS renewal, repeat the Certbot command and TXT verification before the certificate expires. Automatic renewal requires your DNS provider's Certbot plugin and API credentials that can edit the challenge record. [Certbot DNS plugins and manual validation](https://eff-certbot.readthedocs.io/en/stable/using.html#getting-certificates)

**If you can only point the domain at the VPS**, use Certbot's HTTP-01 challenge. Point the Node domain at this VPS and allow public TCP `80`. If the domain has an AAAA record, it must also point at this VPS and accept validation requests. You can issue the certificate before installing Web. If Web already runs here, the block briefly stops Web Caddy during issuance and attempts to restart it on success, failure, or cancellation. Run it during a maintenance window. The command uses the email you enter to register and accept the Let's Encrypt terms of service. [Certbot standalone](https://eff-certbot.readthedocs.io/en/stable/using.html#standalone)

```bash
(
  set -e
  read -r -p 'Node domain > ' node_domain
  read -r -p 'Certificate contact email > ' email
  [[ -n "$node_domain" && -n "$email" ]] ||
    { printf '%s\n' 'Enter the Node domain and certificate contact email.' >&2; exit 1; }
  apt-get update
  apt-get install -y certbot

  caddy=trojan-panel-web-caddy
  restart_caddy=0
  restore_caddy() {
    if [[ "$restart_caddy" == 1 ]]; then
      docker start "$caddy" >/dev/null ||
        { printf '%s\n' "Run docker start $caddy to restore Web Caddy." >&2; return 1; }
    fi
  }
  trap restore_caddy EXIT
  trap 'exit 129' HUP
  trap 'exit 130' INT
  trap 'exit 143' TERM
  if [[ "$(docker inspect --format '{{.State.Running}}' "$caddy" 2>/dev/null || true)" == true ]]; then
    restart_caddy=1
    docker stop "$caddy" >/dev/null
  fi

  certbot certonly --standalone --preferred-challenges http --non-interactive \
    --agree-tos --email "$email" --keep-until-expiring \
    --cert-name "tpnext-node-$node_domain" -d "$node_domain"
  printf '%s\n' "/etc/letsencrypt/live/tpnext-node-$node_domain/"
)
```

Use `fullchain.pem` and `privkey.pem` from the displayed directory in the Node deployment package. HTTP-01 renewal also requires TCP `80`. Rerun the same block during a maintenance window before expiry. Use your DNS provider's plugin and API credentials for automatic renewal.

1. In **Server management → Add Node server**, enter a name such as “Local Node”. Set **Node connection address** to `127.0.0.1`, **gRPC port** to `8100`, and **gRPC certificate domain** to the Node domain covered by the certificate.
2. In **Deploy Node**, choose **Existing host certificates** and enter the absolute paths to the full chain and key on this VPS. Keep the panel domain under **Web address reachable from Node** and choose **Next → Download package**. The command below configures loopback addresses for the database and Redis and gives Node a separate CA trust directory.
3. Upload the package to a separate private directory on the **Web VPS**. Enter that directory in root Bash, paste this block, and enter the exact filename when prompted. The Web installation has already prepared Docker and yq.

```bash
(
  set -e
  umask 077
  read -r -p 'Enter the uploaded package filename > ' bundle
  [[ "$bundle" == tpnext-node-*.tar.gz && -f "$bundle" ]] ||
    { printf '%s\n' 'Package not found. Check the current directory and filename.' >&2; exit 1; }
  chmod 600 -- "$bundle"
  tar -xzf "$bundle"
  yq -i '
    .trojanpanelnext.mariadb_host = "127.0.0.1" |
    .trojanpanelnext.redis_host = "127.0.0.1" |
    .trojanpanelnext.pki_bundle_dir = "/tpdata/trojan-panel-core/pki-bootstrap"
  ' ./tpnext/node.yaml
  chmod 600 -- ./tpnext/node.yaml
  bash ./tpnext/install-node.sh
)
```

To add another VPS later, register it in the same panel and follow [Option A](#quick-install-separate) for each remote Node. Open Web's database and Redis ports only to those remote Nodes as needed. Keep the colocated Node's ports `8100` and `8101` reachable only from the same host.

### Verify the installation

Check that Node is online in **Server management**, then create a proxy in **Node management**. Use the **public Node domain** for the proxy. Open its TCP or UDP port as required by the protocol, then export the subscription or connection details and connect from a client. [Proxy ports and forwarding](docs/port-forwarding_EN.md)

The deployment package includes the server ID, database and Redis credentials, and the Web public CA. Store the archive and `tpnext/node.yaml` privately at mode `0600`. Keep the original YAML for [updates](docs/deployment_EN.md#updates) and [removal](docs/deployment_EN.md#removal).

<a id="configuration-alternative"></a>
## Configuration deployment (alternative)

For manual YAML editing or automated deployment, follow the supplementary instructions in the full guide:

| Step | Guide |
| --- | --- |
| Download templates | [Configuration downloads](docs/deployment_EN.md#configuration-download) |
| Configure Web | [Minimum Web edits](docs/deployment_EN.md#configuration-web-minimum) |
| Configure Node | [Minimum Node edits](docs/deployment_EN.md#configuration-node-minimum) |
| Prepare public CA | [CA for manual configuration](docs/deployment_EN.md#configuration-node-ca) |
| Validate and install | [Configuration deployment](docs/deployment_EN.md#configuration) |

For v1.0.2-rc.13 Node installs, use a deployment package or the configuration commands. For releases that include the fix above, you can also use the [interactive Node command](docs/deployment_EN.md#node-interactive).

<a id="product-update"></a>
## Update product images

A system administrator can select a Node in **Version Management → TPNext Containers** and update it to the release used by Web. Results are stored on the Node host and remain available when the page is reopened. Upgrade an older Node maintenance service once using the command below. Web updates require mTLS access to the Node maintenance port (`grpc_port + 1`).

For Web images or a Node that cannot be updated remotely, use the actual deployment YAML on its host and explicitly select the target release:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.13/scripts/tp.sh) --version 1.0.2-rc.13 update --config ./web.yaml
```

For package-deployed Node, use `./tpnext/node.yaml`; use the original YAML path for manual deployments. Updates retain credentials, data, ports, PKI, and existing runtime configuration without upgrading database, Redis, or Caddy. Updates refuse configuration inconsistent with the running deployment. See [image updates](docs/deployment_EN.md#updates) for backups, brief interruption, and failure handling.

<a id="project-removal"></a>
## Removal

### Uninstall while retaining data

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.13/scripts/tp.sh) remove --config ./tpnext/node.yaml --keep-data
```

### Uninstall the project and its data

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.13/scripts/tp.sh) remove --config ./tpnext/node.yaml --purge-data
```

Use `./web.yaml` on Web. Both modes preserve external certificates, Nginx, and Certbot managed independently by the host. Web **Uninstall** and **Uninstall completely** require a connection to Node. **Delete** only clears Web records and associated data, including for an offline server, without stopping Node services. See [the full removal scope](docs/deployment_EN.md#web-removal).

### Remove added dependencies

Remove the project and other Docker services first, then remove dependencies added by this entrypoint:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.13/scripts/tp.sh) deps remove
```

Only recorded newly installed Docker packages and an unchanged yq installed by this command are removed. Basic tools, pre-existing dependencies, Docker data, and external certificates remain. Rerun `deps install` to repair an interrupted installation first. See [dependency removal](docs/deployment_EN.md#dependency-removal) for refusal conditions.

## Version selection

A single entrypoint can select the scripts, templates, and images of a specified release:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.13/scripts/tp.sh) --version 1.0.2-rc.13 validate --config ./web.yaml
```

Version `1.0.2-rc.13` maps to Git tag `v1.0.2-rc.13` and product image tag `:1.0.2-rc.13`. Installation and validation require matching configuration, scripts, and product images. Updates bind existing supported configuration to the target release. See [release binding](docs/deployment_EN.md#versions) for version rules and support scope.

## Documentation

| Topic | Guide |
| --- | --- |
| Complete deployment, configuration, certificates, and removal | [Deployment guide](docs/deployment_EN.md) |
| Proxy external ports, Nginx SNI, and UDP forwarding | [Port forwarding](docs/port-forwarding_EN.md) |
| Current documentation index | [docs](docs/README_EN.md) |
| Commands and script library | [scripts](scripts/README_EN.md) |
| Certificate renewal and internal mTLS trust | [Certificate maintenance](docs/certificates.md) |

## Source layout

| Directory | Contents |
| --- | --- |
| `apps/control-plane/api` | Web control-plane API |
| `apps/control-plane/web` | Web administration interface |
| `apps/node-agent` | Node Agent and proxy runtime management |
| `apps/docs-site` | Documentation site |
| `scripts/deploy` | Release-bound deployment scripts and templates |
| `tests` | Automated tests |
| `tools` | Repository checks |
| `docs` | User and architecture documentation |

## Project origin

The project is independently maintained from [TrojanPanel](https://github.com/trojanpanel). See [NOTICE.md](NOTICE.md) for attribution.
