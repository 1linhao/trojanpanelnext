# TrojanPanel Next Documentation

[简体中文](README.md) | English

TrojanPanel Next v1.0.2-rc.2 provides a multi-user Web control plane for Xray, Hysteria2, and NaiveProxy.

## Quick installation

In a root Bash session, install dependencies first, then prepare DNS and [network access](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.2/docs/deployment_EN.md#network). Automatic dependency installation supports Debian 12/13 and Ubuntu 22.04/24.04 on amd64/arm64 with systemd:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) deps install
```

The entrypoint needs Bash, curl, CA certificates, grep, and coreutils; `deps` also needs `flock` from util-linux. See [dependencies](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.2/docs/deployment_EN.md#dependencies) for minimal bootstrap instructions and manual installation on other Linux distributions.

Web:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) web
```

After installing Web, sign in with the `sysadmin` role and click **Add Node server** on the dashboard, or open **Server management** in the sidebar and click **Add Node server**. Register the Node address, gRPC port, and TLS server name; note its actual server ID and copy Web's current public CA. See [registration and connection](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.2/docs/deployment_EN.md#node-registration).

Node:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) node
```

The entrypoint prompts for configuration and deploys images from the selected release. It also supports version selection, YAML deployment, external certificates, image updates, container recreation, and removal. Follow the [complete deployment guide](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.2/docs/deployment_EN.md) for all commands.

## Configuration deployment

Download templates on the corresponding hosts first.

Web:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) config web --output ./web.yaml
```

Node:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) config node --output ./node.yaml
```

Edit with `nano` or `vi`, keeping the `trojanpanelnext:` mapping and current-release image fields. For Web, edit at least `hostname` and `email`; empty first-install database/Redis passwords are generated and written back. For Node, edit its domain, Caddy email, Web database/Redis addresses and actual passwords, actual server ID, and TLS server name; prepare Web's public CA before installation. External certificates require `node_certificate_mode: external` and existing absolute certificate/key paths, without an email. Templates use `0600` permissions and never overwrite existing files.

[Minimum Web edits](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.2/docs/deployment_EN.md#configuration-web-minimum) · [Minimum Node edits](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.2/docs/deployment_EN.md#configuration-node-minimum) · [Prepare Node CA](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.2/docs/deployment_EN.md#configuration-node-ca)

Validate Web:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) validate --config ./web.yaml
```

Install Web:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) install --config ./web.yaml
```

Validate Node:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) validate --config ./node.yaml
```

Install Node:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) install --config ./node.yaml
```

## Update product images

On each host, use the actual deployment YAML and explicitly select the target version. Use `./node.yaml` on Node. Credentials, data, and PKI remain; MariaDB, Redis, and Caddy are not upgraded. See [image updates](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.2/docs/deployment_EN.md#updates) for backups, compatibility, brief interruption, and recovery limits.

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) --version 1.0.2-rc.2 update --config ./web.yaml
```

## Remove dependencies

After removing the project and other Docker services, remove added Docker packages and unchanged yq according to the installation record:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) deps remove
```

Pre-existing dependencies, basic tools, Docker data, and external certificates remain; Docker containers, shared containerd's other namespaces, and maintenance are checked before removal. If installation was interrupted, rerun `deps install` to repair it first. See [dependency removal](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.2/docs/deployment_EN.md#dependency-removal) for the full scope.

## Documentation

- [Image updates](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.2/docs/deployment_EN.md#updates)
- [Deployment and removal](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.2/docs/deployment_EN.md)
- [Web user guide](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.2/docs/user-guide.md)
- [Certificate management](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.2/docs/certificates.md)
- [API reference](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.2/docs/api.md)
- [Development guide](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.2/docs/development_EN.md)
