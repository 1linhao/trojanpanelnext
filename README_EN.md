# TrojanPanel Next

[简体中文](README.md) | English

TrojanPanel Next is a multi-user proxy management platform supporting Xray, Hysteria2, and NaiveProxy. The Web control plane manages accounts, node servers, proxies, traffic, and tasks. Node Agents manage proxy runtimes on each server.

Current release: **v1.0**. Deployment uses GHCR images for Linux amd64 and arm64.

## Quick installation

Run these commands in a **root Bash session** on the target server. First install and start Docker Engine, mikefarah/yq v4, and the other [dependencies](docs/deployment_EN.md#dependencies), then prepare DNS and [network access](docs/deployment_EN.md#network). The scripts prompt for configuration and install the services.

Web control plane:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0/scripts/tp.sh) web
```

Visit the configured HTTPS domain after installation. The initial account is `sysadmin` / `123456`; change its password after signing in.

Register a node server in Web, obtain its server ID, and securely copy Web's public `client-ca.crt` to the Node host. Then install Node:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0/scripts/tp.sh) node
```

For certificates already managed on the host by Nginx, Certbot, or another tool:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0/scripts/tp.sh) node --certificate-mode external
```

This mode prompts for the full certificate chain and private key paths, skipping Node Caddy and certificate issuance. See [external certificates](docs/deployment_EN.md#external-certificates) for DNS, certificate, and shared-port preparation.

## Configuration deployment and removal

Deploy using a completed Web or Node YAML file:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0/scripts/tp.sh) install --config ./web.yaml
```

Remove containers and eligible images while retaining service data:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0/scripts/tp.sh) remove --config ./node.yaml --keep-data
```

Remove the project completely, including service data, PKI, proxy runtime configuration, and deployment YAML:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0/scripts/tp.sh) remove --config ./node.yaml --purge-data
```

Choose the configuration path for the Web or Node deployment on that host. External certificates, Nginx, and Certbot remain managed independently by the host and are preserved in both modes. See [remote removal](docs/deployment_EN.md#web-removal) for deleting an entire node server from Web.

## Version selection

A single entrypoint can select the scripts, templates, and images of a specified release:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0/scripts/tp.sh) --version 1.0 web
```

Version `1.0` maps to Git tag `v1.0` and product image tag `:1.0`. Configuration, scripts, and product images must match. See [release binding](docs/deployment_EN.md#versions) for version rules and support scope.

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
