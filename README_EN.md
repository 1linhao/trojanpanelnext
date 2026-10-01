# TrojanPanel Next

[简体中文](README.md) | English

TrojanPanel Next is a multi-user proxy management platform supporting Xray, Hysteria2, and NaiveProxy. The Web control plane manages accounts, node servers, proxies, traffic, and tasks. Node Agents manage proxy runtimes on each server.

Current pre-release: **v1.0.2-rc.3 (Pre-release)**. Deployment uses GHCR images for Linux amd64 and arm64.

## Quick installation

### 1. Prepare dependencies on Web

Run in a **root Bash session**. Automatic dependency installation supports amd64/arm64 hosts running Debian 12/13 or Ubuntu 22.04/24.04 with systemd.

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) deps install
```

The entrypoint needs Bash, curl, CA certificates, grep, and coreutils; `deps` also needs `flock` from util-linux. Follow the [minimal bootstrap instructions](docs/deployment_EN.md#dependency-install) if tools are missing, or [prepare dependencies manually](docs/deployment_EN.md#dependency-manual) on other Linux distributions. Prepare Web DNS and [network access](docs/deployment_EN.md#network).

### 2. Install Web

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) web
```

Enter the domain and certificate email when prompted. Visit that HTTPS domain after installation. The initial account is `sysadmin` / `123456`; change its password after the first sign-in.

### 3. Register a Node server in Web

With the `sysadmin` role, open **Server management** in the sidebar and click **Add Node server**. Enter the server name, Node IP or domain, gRPC port (default `8100`), and gRPC certificate domain.

Saving opens **Deploy Node** automatically; the same action on each server row opens it again. The **ID** column shows a numeric ID assigned by the Web database, such as `3`. It must be an **integer greater than or equal to 1**, displayed separately from **IP / domain**. It is neither the server address nor an individual proxy node ID. See [server registration](docs/deployment_EN.md#node-registration).

### 4. Download the Node deployment package

In **Deploy Node**, confirm the Web hostname or IP reachable from Node (such as `panel.example.com`, without `https://` or a path) and select a certificate mode:

- **Caddy**: enter a certificate email; Node Caddy issues and renews certificates.
- **Existing host certificates**: enter fullchain and private-key absolute paths already present on the target Node; host tools maintain them.

Click **Next** to open **Download and install**, then click **Download deployment package**. The `.tar.gz` package contains `node.yaml`, public `client-ca.crt`, `install-node.sh`, and `README.md`. Configuration includes the actual server ID, Web database / Redis credentials, and connection settings. The package contains no Web private keys or Node TLS private key. Prepare certificates and keys on Node before using external mode; Web does not generate them.

Deployment packages and YAML contain sensitive credentials. Save them with **`0600`** permissions, transfer them securely only to the target Node, and never commit them to a public repository. See [package installation](docs/deployment_EN.md#node-deployment-package).

### 5. Install on the Node host

First prepare dependencies in a root Bash session on Node:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) deps install
```

Securely transfer the package to a private working directory on Node. The example uses `tpnext-node-3.tar.gz` for server ID `3`; substitute the downloaded filename:

```bash
tar -xzf ./tpnext-node-3.tar.gz
```

Run the included entrypoint from the extracted directory:

```bash
bash ./install-node.sh
```

The entrypoint prepares the public CA and installs using the included `node.yaml`. It refuses to overwrite a different existing trust file and does not force recreation of existing services. Retain this YAML for future [updates](docs/deployment_EN.md#updates) and [removal](docs/deployment_EN.md#removal).

### 6. Check connectivity and create proxies

Check Node is online in **Server management**, then create proxies in **Node management**. Offline status is expected before installation. Database, Redis, gRPC, and maintenance ports must be reachable according to the actual configuration; see [network requirements](docs/deployment_EN.md#network).

## Configuration deployment (alternative)

### Download Web configuration and make minimum edits

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) config web --output ./web.yaml
```

Edit with `nano` or `vi`, changing at least `hostname` and `email`. Empty first-install database and Redis passwords are generated and written back to YAML; retain the actual credentials afterwards. Keep the `trojanpanelnext:` mapping, release, and images. Templates use `0600` permissions and never overwrite existing files. See [minimum Web edits](docs/deployment_EN.md#configuration-web-minimum).

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) install --config ./web.yaml
```

### Download Node configuration and make minimum edits

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) config node --output ./node.yaml
```

Set the Node domain, Caddy email, Web database / Redis addresses and actual passwords, `node_server_id`, and `grpc_tls_server_name`. Use the integer greater than or equal to `1` from the server management **ID** column for `node_server_id`, never an IP, domain, or proxy ID.

Prepare Web's public CA before installation. External mode requires `node_certificate_mode: external` and fullchain / private-key absolute paths already present on Node; no email is needed. See [minimum Node edits](docs/deployment_EN.md#configuration-node-minimum), [CA preparation](docs/deployment_EN.md#configuration-node-ca), and [external certificates](docs/deployment_EN.md#external-certificates).

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) install --config ./node.yaml
```

See [configuration deployment](docs/deployment_EN.md#configuration) for validation commands, non-default ports, and all fields. The interactive `node` entrypoint can also prompt for configuration; see [command-line Node installation](docs/deployment_EN.md#node-interactive).

## Update product images

Use the actual deployment YAML on each host and explicitly select the target release:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) --version 1.0.2-rc.3 update --config ./web.yaml
```

Use `./node.yaml` on Node. Updates retain credentials, data, ports, PKI, and existing runtime configuration without upgrading database, Redis, or Caddy. Updates refuse configuration inconsistent with the running deployment. See [image updates](docs/deployment_EN.md#updates) for backups, brief interruption, and failure handling.

## Removal

### Uninstall while retaining data

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) remove --config ./node.yaml --keep-data
```

### Uninstall the project and its data

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) remove --config ./node.yaml --purge-data
```

Use `./web.yaml` on Web. Both modes preserve external certificates, Nginx, and Certbot managed independently by the host. Web **Uninstall** and **Uninstall completely** require a connection to Node. **Delete** only clears Web records and associated data, including for an offline server, without stopping Node services. See [the full removal scope](docs/deployment_EN.md#web-removal).

### Remove added dependencies

Remove the project and other Docker services first, then remove dependencies added by this entrypoint:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) deps remove
```

Only recorded newly installed Docker packages and an unchanged yq installed by this command are removed. Basic tools, pre-existing dependencies, Docker data, and external certificates remain. Rerun `deps install` to repair an interrupted installation first. See [dependency removal](docs/deployment_EN.md#dependency-removal) for refusal conditions.

## Version selection

A single entrypoint can select the scripts, templates, and images of a specified release:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) --version 1.0.2-rc.3 web
```

Version `1.0.2-rc.3` maps to Git tag `v1.0.2-rc.3` and product image tag `:1.0.2-rc.3`. Installation and validation require matching configuration, scripts, and product images. Updates bind existing supported configuration to the target release. See [release binding](docs/deployment_EN.md#versions) for version rules and support scope.

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
