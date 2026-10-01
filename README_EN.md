# TrojanPanel Next

[简体中文](README.md) | English

TrojanPanel Next is a multi-user proxy management platform supporting Xray, Hysteria2, and NaiveProxy. The Web control plane manages accounts, node servers, proxies, traffic, and tasks. Node Agents manage proxy runtimes on each server.

Current pre-release: **v1.0.2-rc.1 (Pre-release)**. Deployment uses GHCR images for Linux amd64 and arm64.

## Quick installation

Run these commands in a **root Bash session** on the target server. On amd64/arm64 hosts running Debian 12/13 or Ubuntu 22.04/24.04 with systemd, install dependencies first:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) deps install
```

The entrypoint needs Bash, curl, CA certificates, grep, and coreutils; `deps` also needs `flock` from util-linux. If tools are missing, follow the [minimal bootstrap instructions](docs/deployment_EN.md#dependency-install). Prepare dependencies [manually](docs/deployment_EN.md#dependency-manual) on other Linux distributions. After preparing DNS and [network access](docs/deployment_EN.md#network), the deployment scripts prompt for configuration and install services.

Web control plane:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) web
```

Visit the configured HTTPS domain after installation. The initial account is `sysadmin` / `123456`; change its password after signing in.

Sign into Web with the `sysadmin` role and click **Add Node server** on the dashboard, or open **Server management** in the sidebar and click **Add Node server**. Enter the Node address, gRPC port (default `8100`), and TLS server name; save and note the **Server ID** shown below the server name in each row. Securely copy Web's public `client-ca.crt` to the Node host, then install Node. See [registration and connection](docs/deployment_EN.md#node-registration):

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) node
```

For certificates already managed on the host by Nginx, Certbot, or another tool:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) node --certificate-mode external
```

This mode prompts for the full certificate chain and private key paths, skipping Node Caddy and certificate issuance. See [external certificates](docs/deployment_EN.md#external-certificates) for DNS, certificate, and shared-port preparation.

## Configuration deployment and removal

Download the corresponding configuration template on each host first.

Web host:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) config web --output ./web.yaml
```

Node host:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) config node --output ./node.yaml
```

Edit the downloaded file with `nano` or `vi`. Keep the `trojanpanelnext:` mapping, release, images, and other default fields; quote YAML strings and passwords. Templates use `0600` permissions and never overwrite existing files.

| Configuration | Minimum edits or preparation |
| --- | --- |
| Web | `hostname`, `email`. On the first installation, empty database and Redis passwords are generated and written back to YAML; retain the actual credentials afterwards. See [minimum Web edits](docs/deployment_EN.md#configuration-web-minimum) |
| Node | `hostname`, `email` (Caddy mode), `mariadb_host` / `mariadb_password`, `redis_host` / `redis_password`, actual `node_server_id`, and `grpc_tls_server_name`; connection settings must match Web. See [minimum Node edits](docs/deployment_EN.md#configuration-node-minimum) |
| Node public CA | Before installation, place Web's public CA at `/tpdata/trojanpanelnext-pki/client-ca.crt` on Node, or `pki_bundle_dir/client-ca.crt` for a custom directory. See [CA preparation](docs/deployment_EN.md#configuration-node-ca) |
| Existing Node certificates | Set `node_certificate_mode: external` and existing absolute fullchain/private-key paths; `email` is not required. See [external certificates](docs/deployment_EN.md#external-certificates) |

Validate Web:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) validate --config ./web.yaml
```

Install Web:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) install --config ./web.yaml
```

Validate Node:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) validate --config ./node.yaml
```

Install Node:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) install --config ./node.yaml
```

See [configuration deployment](docs/deployment_EN.md#configuration) for complete editing examples, non-default ports, and field descriptions.

Remove containers and eligible images while retaining service data:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) remove --config ./node.yaml --keep-data
```

Remove the project completely, including service data, PKI, proxy runtime configuration, and deployment YAML:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) remove --config ./node.yaml --purge-data
```

Choose the configuration path for the Web or Node deployment on that host. External certificates, Nginx, and Certbot remain managed independently by the host and are preserved in both modes. See [remote removal](docs/deployment_EN.md#web-removal) for deleting an entire node server from Web.

After removing the project and other Docker services, remove dependencies added by this entrypoint:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) deps remove
```

Only newly installed Docker packages in the installation record and an unchanged yq installed by this command are removed. Basic tools, pre-existing dependencies, Docker data, and external certificates remain. Docker removal is refused if any Docker container exists (including stopped containers), shared containerd has containers in other namespaces, Node maintenance remains, or runtime state cannot be verified. If dependency installation was interrupted, rerun `deps install` to repair it before removal. See [dependency removal](docs/deployment_EN.md#dependency-removal).

## Version selection

A single entrypoint can select the scripts, templates, and images of a specified release:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) --version 1.0.2-rc.1 web
```

Version `1.0.2-rc.1` maps to Git tag `v1.0.2-rc.1` and product image tag `:1.0.2-rc.1`. Configuration, scripts, and product images must match. See [release binding](docs/deployment_EN.md#versions) for version rules and support scope.

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
