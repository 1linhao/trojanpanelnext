# TrojanPanel Next

[简体中文](README.md) | English

TrojanPanel Next is a multi-user proxy management platform supporting Xray, Hysteria2, and NaiveProxy. The Web control plane manages accounts, node servers, proxies, traffic, and tasks. Node Agents manage proxy runtimes on each server.

Current pre-release: **v1.0.2-rc.6 (Pre-release)**. Deployment uses GHCR images for Linux amd64 and arm64.

## Start here

- [Quick installation: Web → download the Node package → install Node](#quick-install)
- [Configuration deployment (alternative)](#configuration-alternative)
- [Update product images](#product-update)
- [Remove the project and dependencies](#project-removal)
- [Complete deployment guide](docs/deployment_EN.md)

<a id="quick-install"></a>
## Quick installation

Follow this order: **prepare Web → sign in → register a server and download its Node package → install on Node → create proxies**.

### 1. Prepare dependencies on Web

Run in a **root Bash session**. Automatic dependency installation supports amd64/arm64 hosts running Debian 12/13 or Ubuntu 22.04/24.04 with systemd.

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.6/scripts/tp.sh) deps install
```

The entrypoint needs Bash, curl, CA certificates, grep, and coreutils; `deps` also needs `flock` from util-linux. Follow the [minimal bootstrap instructions](docs/deployment_EN.md#dependency-install) if tools are missing, or [prepare dependencies manually](docs/deployment_EN.md#dependency-manual) on other Linux distributions. Prepare Web DNS and [network access](docs/deployment_EN.md#network).

### 2. Install Web and sign in

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.6/scripts/tp.sh) web
```

Enter the Web domain and certificate email when prompted. Visit that HTTPS domain after installation, sign in with the initial account `sysadmin` / `123456`, and change its password.

### 3. Register Node in Server management

With the `sysadmin` role, open **Server management** in the sidebar and click **Add Node server**. Enter the server name, Node IP / domain reachable from Web, gRPC port (default `8100`), and gRPC certificate domain covered by Node's certificate. Traffic limits can retain their defaults.

Saving assigns a **numeric server ID greater than or equal to `1`** and automatically opens **Deploy Node**. ID and IP / domain are separate fields; the package uses the correct ID automatically. For an existing server, click its **Deploy Node** icon to reopen the guide. See [server registration](docs/deployment_EN.md#node-registration).

### 4. Select certificates and download the Node package

In **Deployment parameters**, select a certificate mode:

- **Caddy automatic issuance and renewal**: enter the certificate contact email and prepare DNS and ACME validation ports for the Node certificate domain.
- **Existing host certificates**: enter absolute paths to a full chain and matching unencrypted private key already present on Node. Host tools renew these certificates; this mode does not require email.

Confirm the prefilled **Web address reachable from Node**, changing it if necessary to a hostname or IP Node can reach, without protocol, port, or path. Check the displayed database and Redis addresses. Click **Next**, then **Download deployment package** under **Download and install**.

The filename is `tpnext-node-<server ID>.tar.gz`, containing:

| File | Purpose |
| --- | --- |
| `node.yaml` | Node configuration with the actual server ID, Web database / Redis connections, and credentials |
| `client-ca.crt` | Web's current public client CA |
| `install-node.sh` | Prepare the public CA and invoke matching-release installation |
| `README.md` | Package instructions |

This flow requires no manual ID or database-password editing or separate CA copy. Prepare external certificates and keys on Node beforehand. Packages and YAML contain sensitive credentials: save them with **`0600`** permissions, transfer them securely only to the target Node, and never commit them publicly. Packages contain no Web private key or Node TLS private key. See [package installation](docs/deployment_EN.md#node-deployment-package).

### 5. Transfer the package to Node and install

Prepare dependencies in a **root Bash session** on Node first:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.6/scripts/tp.sh) deps install
```

Securely transfer the package to a private working directory on Node. The example uses server ID `3`; replace the filename with the actual download and extract it in that directory:

```bash
tar -xzf ./tpnext-node-3.tar.gz
```

The four package files are extracted directly into the current directory. Run there:

```bash
bash ./install-node.sh
```

The entrypoint validates the release and configuration, installs using `node.yaml`, and initializes or rebinds management trust to the current Web public CA included in the package. It validates the CA and backs up replaced trust files; existing services are not forcibly recreated. Retain this YAML for future [updates](docs/deployment_EN.md#updates) and [removal](docs/deployment_EN.md#removal).

### 6. Return to Web and create proxies

Check Node is online in **Server management**, then create proxies in **Node management**. Offline status is expected before Node installation. Database, Redis, gRPC, and maintenance ports must be reachable according to the configuration; see [network requirements](docs/deployment_EN.md#network).

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

[Command-line Node installation](docs/deployment_EN.md#node-interactive) and [external certificate configuration](docs/deployment_EN.md#external-certificates) remain additional paths.

<a id="product-update"></a>
## Update product images

Use the actual deployment YAML on each host and explicitly select the target release:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.6/scripts/tp.sh) --version 1.0.2-rc.6 update --config ./web.yaml
```

Use `./node.yaml` on Node. Updates retain credentials, data, ports, PKI, and existing runtime configuration without upgrading database, Redis, or Caddy. Updates refuse configuration inconsistent with the running deployment. See [image updates](docs/deployment_EN.md#updates) for backups, brief interruption, and failure handling.

<a id="project-removal"></a>
## Removal

### Uninstall while retaining data

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.6/scripts/tp.sh) remove --config ./node.yaml --keep-data
```

### Uninstall the project and its data

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.6/scripts/tp.sh) remove --config ./node.yaml --purge-data
```

Use `./web.yaml` on Web. Both modes preserve external certificates, Nginx, and Certbot managed independently by the host. Web **Uninstall** and **Uninstall completely** require a connection to Node. **Delete** only clears Web records and associated data, including for an offline server, without stopping Node services. See [the full removal scope](docs/deployment_EN.md#web-removal).

### Remove added dependencies

Remove the project and other Docker services first, then remove dependencies added by this entrypoint:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.6/scripts/tp.sh) deps remove
```

Only recorded newly installed Docker packages and an unchanged yq installed by this command are removed. Basic tools, pre-existing dependencies, Docker data, and external certificates remain. Rerun `deps install` to repair an interrupted installation first. See [dependency removal](docs/deployment_EN.md#dependency-removal) for refusal conditions.

## Version selection

A single entrypoint can select the scripts, templates, and images of a specified release:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.6/scripts/tp.sh) --version 1.0.2-rc.6 web
```

Version `1.0.2-rc.6` maps to Git tag `v1.0.2-rc.6` and product image tag `:1.0.2-rc.6`. Installation and validation require matching configuration, scripts, and product images. Updates bind existing supported configuration to the target release. See [release binding](docs/deployment_EN.md#versions) for version rules and support scope.

## Documentation

| Topic | Guide |
| --- | --- |
| Complete deployment, configuration, certificates, and removal | [Deployment guide](docs/deployment_EN.md) |
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
