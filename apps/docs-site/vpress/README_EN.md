# TrojanPanel Next Documentation

[简体中文](README.md) | English

TrojanPanel Next v1.0.2-rc.13 provides a multi-user Web control plane for Xray, Hysteria2, and NaiveProxy.

## Quick installation

### 1. Prepare dependencies on Web

Run in a **root Bash session**. Automatic dependency installation supports amd64/arm64 hosts running Debian 12/13 or Ubuntu 22.04/24.04 with systemd.

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.13/scripts/tp.sh) deps install
```

The entrypoint needs Bash, curl, CA certificates, grep, and coreutils; `deps` also needs `flock` from util-linux. Follow the [minimal bootstrap instructions](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment_EN.md#dependency-bootstrap) if tools are missing, or [prepare dependencies manually](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment_EN.md#dependency-manual) on other Linux distributions. Prepare Web DNS and [network access](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment_EN.md#network).

### 2. Install Web

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.13/scripts/tp.sh) web
```

Enter the domain and certificate email when prompted. Visit that HTTPS domain after installation. The initial account is `sysadmin` / `123456`; change its password after the first sign-in.

### 3. Register a Node server in Web

With the `sysadmin` role, open **Server management** in the sidebar and click **Add Node server**. Enter the server name, Node IP or domain, gRPC port (default `8100`), and gRPC certificate domain.

Saving opens **Deploy Node** automatically; the same action on each server row opens it again. The **ID** column shows a numeric ID assigned by the Web database, such as `3`. It must be an **integer greater than or equal to 1**, displayed separately from **IP / domain**. It is neither the server address nor an individual proxy node ID. See [server registration](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment_EN.md#node-registration).

### 4. Download the Node deployment package

In **Deploy Node**, confirm the Web hostname or IP reachable from Node (such as `panel.example.com`, without `https://` or a path) and select a certificate mode:

- **Caddy**: enter a certificate email; Node Caddy issues and renews certificates.
- **Existing host certificates**: enter fullchain and private-key absolute paths already present on the target Node; host tools maintain them.

Click **Next** to open **Download and install**, then click **Download deployment package**. The `.tar.gz` package contains `node.yaml`, public `client-ca.crt`, `install-dependencies.sh`, `install-node.sh`, and `README.md`. Configuration includes the actual server ID, Web database / Redis credentials, and connection settings. The package contains no Web private keys or Node TLS private key. Prepare certificates and keys on Node before using external mode; Web does not generate them.

Deployment packages and YAML contain sensitive credentials. Save them with **`0600`** permissions, transfer them securely only to the target Node, and never commit them to a public repository. See [package installation](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment_EN.md#node-deployment-package).

### 5. Install on the Node host

In root Bash on Node, follow the [minimal bootstrap instructions](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment_EN.md#dependency-bootstrap) to prepare basic tools and confirm systemd is running. Securely transfer the package to a private working directory on Node. The example uses server ID `3`; substitute the downloaded filename:

```bash
umask 077
chmod 600 ./tpnext-node-3.tar.gz
tar -xzf ./tpnext-node-3.tar.gz
```

All five files extract into `tpnext/` (mode `0700`); both scripts have mode `0700`. Run the dependency entrypoint first:

```bash
bash ./tpnext/install-dependencies.sh
```

This script invokes the package release's `deps install` to prepare Docker, mikefarah/yq v4, OpenSSL, and other deployment dependencies, reusing compatible tools and dependency management records. It does not install Node or read credential-bearing YAML. Automatic installation supports Debian 12/13 and Ubuntu 22.04/24.04 on amd64/arm64 with running systemd. On other Linux distributions, [prepare dependencies manually](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment_EN.md#dependency-manual) and skip this step.

Then run the installation entrypoint:

```bash
bash ./tpnext/install-node.sh
```

The entrypoint prepares the public CA and installs using the included `tpnext/node.yaml`. The installer validates the current Web public CA and backs up different retained trust files before replacing them; it does not force recreation of existing services. Retain this YAML for future [updates](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment_EN.md#updates) and [removal](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment_EN.md#removal).

### 6. Check connectivity and create proxies

Check Node is online in **Server management**, then create proxies in **Node management**. Offline status is expected before installation. Database, Redis, gRPC, and maintenance ports must be reachable according to the actual configuration; see [network requirements](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment_EN.md#network).

## Configuration deployment

Download templates on the corresponding hosts first.

Web:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.13/scripts/tp.sh) config web --output ./web.yaml
```

Node:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.13/scripts/tp.sh) config node --output ./node.yaml
```

Edit with `nano` or `vi`, keeping the `trojanpanelnext:` mapping and current-release image fields. For Web, edit at least `hostname` and `email`; empty first-install database/Redis passwords are generated and written back. For Node, edit its domain, Caddy email, Web database/Redis addresses and actual passwords, integer server ID (≥ `1`, not an IP / domain or proxy ID), and TLS server name; prepare Web's public CA before installation. External certificates require `node_certificate_mode: external` and existing absolute certificate/key paths, without an email. Templates use `0600` permissions and never overwrite existing files.

[Minimum Web edits](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment_EN.md#configuration-web-minimum) · [Minimum Node edits](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment_EN.md#configuration-node-minimum) · [Prepare Node CA](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment_EN.md#configuration-node-ca)

Validate Web:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.13/scripts/tp.sh) validate --config ./web.yaml
```

Install Web:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.13/scripts/tp.sh) install --config ./web.yaml
```

Validate Node:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.13/scripts/tp.sh) validate --config ./node.yaml
```

Install Node:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.13/scripts/tp.sh) install --config ./node.yaml --client-ca /root/client-ca.crt
```

## Update product images

Node can be updated in **Version Management → TPNext Containers** to the current Web release. Its maintenance service must support updates and allow Web mTLS access on `grpc_port + 1`. Upgrade older Nodes once using the CLI; Web images still use the CLI.

On each host, use the actual deployment YAML and explicitly select the target version. For package-deployed Node use `./tpnext/node.yaml`; use the original YAML path for manual deployments. Credentials, data, and PKI remain; MariaDB, Redis, and Caddy are not upgraded. See [image updates](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment_EN.md#updates) for backups, compatibility, brief interruption, and recovery limits.

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.13/scripts/tp.sh) --version 1.0.2-rc.13 update --config ./web.yaml
```

## Remove dependencies

After removing the project and other Docker services, remove added Docker packages and unchanged yq according to the installation record:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.13/scripts/tp.sh) deps remove
```

Pre-existing dependencies, basic tools, Docker data, and external certificates remain; Docker containers, shared containerd's other namespaces, and maintenance are checked before removal. If installation was interrupted, rerun `deps install` to repair it first. See [dependency removal](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment_EN.md#dependency-removal) for the full scope.

## Documentation

- [Image updates](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment_EN.md#updates)
- [Deployment and removal](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment_EN.md)
- [Web user guide](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/user-guide.md)
- [Certificate management](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/certificates.md)
- [API reference](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/api.md)
- [Development guide](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/development_EN.md)
