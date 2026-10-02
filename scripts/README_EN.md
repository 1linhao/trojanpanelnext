# TrojanPanel Next script library

[简体中文](README.md) | English

`tp.sh` is the v1.0.2-rc.9 entrypoint. It downloads commands, shared dependencies, and templates for the selected release. See the [deployment guide](../docs/deployment_EN.md) for complete instructions.

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.9/scripts/tp.sh) --help
```

| Command | Purpose |
| --- | --- |
| `deps install` | Install dependencies on supported Debian/Ubuntu hosts, reusing compatible Docker and yq |
| `deps remove` | Remove newly installed Docker packages and unchanged yq according to the installation record |
| `web` | Prompt for Web settings and install, or use an existing `--config` |
| `node` | Prompt for Node settings and install; supports `--certificate-mode external` |
| `config web\|node --output <file>` | Create a configuration template for the selected release |
| `validate --config <file>` | Validate YAML, version, and fields |
| `install --config <file> [--force] [--client-ca <file>]` | Deploy or recreate services; Node can explicitly import the current Web public CA |
| `--version <target> update --config <file>` | Update Web or Node product images, retaining configuration and data |
| `remove --config <file> --keep-data` | Uninstall while retaining data |
| `remove --config <file> --purge-data` | Uninstall and delete project data |
| `--version <version>` | Select a release; accepted before or after the command |
| `--entry-version` | Show the entrypoint's default release |

Deployment implementations and templates live in `deploy/`. The entrypoint assembles and invokes their required files. Installation and validation require configuration, scripts, and product images from the same release. The update command binds existing supported configuration to the selected target release.

In a root Bash session, install dependencies separately before running `web`, `node`, or `install`; deployment commands only check dependencies.

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.9/scripts/tp.sh) deps install
```

Automatic dependency installation supports Debian 12/13 and Ubuntu 22.04/24.04 on amd64/arm64 with systemd. The entrypoint itself requires Bash, curl, CA certificates, grep, and coreutils; `deps` also requires `flock` from util-linux. See the [dependency guide](../docs/deployment_EN.md#dependencies) for minimal bootstrap instructions and manual preparation on other Linux distributions.

## Web deployment package for Node

In Web **Server management**, save a server to open **Deploy Node**, or reopen it from the corresponding row. The package contains `node.yaml` with the actual numeric server ID and database / Redis credentials, public `client-ca.crt`, `install-node.sh`, and instructions. A server ID is an integer ≥ `1`, separate from its IP / domain and proxy IDs.

On Node, prepare dependencies, securely transfer and extract the package. All four files reside in `tpnext/` (mode `0700`); run from the extraction directory:

```bash
bash ./tpnext/install-node.sh
```

The included entrypoint invokes release-bound `install --config --client-ca`, validates the current Web public CA, and backs up and replaces existing trust to reconnect. It does not use `--force`. Keep the package and YAML private with `0600` permissions; retain the actual YAML for updates and removal. See [package installation](../docs/deployment_EN.md#node-deployment-package).

## Configuration deployment

Download a template, edit it, then validate and install. Templates use `0600` permissions and existing files are never overwritten.

Web template:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.9/scripts/tp.sh) config web --output ./web.yaml
```

Node template:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.9/scripts/tp.sh) config node --output ./node.yaml
```

Edit with `nano` or `vi`; all fields belong to the `trojanpanelnext` mapping. For Web, edit at least `hostname` and `email`; empty first-install database/Redis passwords are generated and written back. For Node, edit its domain, Caddy email, Web database/Redis addresses and actual passwords, integer server ID (≥ `1`, not its IP / domain or proxy ID), and TLS server name. Prepare Web's public CA before installation. For external certificates, set `node_certificate_mode: external` and existing absolute certificate/key paths; email is not required.

[Minimum Web edits](../docs/deployment_EN.md#configuration-web-minimum) · [Register a server](../docs/deployment_EN.md#node-registration) · [Minimum Node edits](../docs/deployment_EN.md#configuration-node-minimum) · [Prepare Node CA](../docs/deployment_EN.md#configuration-node-ca)

Validate Web:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.9/scripts/tp.sh) validate --config ./web.yaml
```

Install Web:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.9/scripts/tp.sh) install --config ./web.yaml
```

Validate Node:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.9/scripts/tp.sh) validate --config ./node.yaml
```

Install Node:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.9/scripts/tp.sh) install --config ./node.yaml --client-ca /root/client-ca.crt
```

`install --client-ca` only supports Node installation. It initializes or rebinds management trust using the securely supplied current Web public CA, validating and backing up old files before replacement. Installation without this option and image updates preserve live trust; use [certificate maintenance](../docs/certificates.md) for CA rotation.

## Update product images

Run on the corresponding Web or Node host using the actual deployment YAML. The target version is required; use `./tpnext/node.yaml` for package-deployed Node, or the original YAML path for manual deployments. Updates retain credentials, data, ports, and PKI, and do not upgrade MariaDB, Redis, or Caddy. See [image updates](../docs/deployment_EN.md#updates) for configuration compatibility, backups, service interruption, and recovery limits.

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.9/scripts/tp.sh) --version 1.0.2-rc.9 update --config ./web.yaml
```

## Remove dependencies

After removing the project and other Docker services, remove dependencies added by this command:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.9/scripts/tp.sh) deps remove
```

Pre-existing dependencies, basic tools, Docker data, and external certificates remain. Docker removal is refused while any Docker container exists (including stopped containers), shared containerd has containers in other namespaces, Node maintenance remains, or runtime state cannot be verified. If dependency installation was interrupted, rerun `deps install` to repair it before removal. See [dependency removal](../docs/deployment_EN.md#dependency-removal) for the full scope. `deps` also accepts `--version <version>`.

[Dependency installation](../docs/deployment_EN.md#dependency-install) · [One-command deployment](../docs/deployment_EN.md#web) · [Version selection](../docs/deployment_EN.md#versions) · [Configuration](../docs/deployment_EN.md#configuration) · [Image updates](../docs/deployment_EN.md#updates) · [Project removal](../docs/deployment_EN.md#removal) · [Dependency removal](../docs/deployment_EN.md#dependency-removal)
