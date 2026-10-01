# TrojanPanel Next script library

[简体中文](README.md) | English

`tp.sh` is the v1.0.1 entrypoint. It downloads commands, shared dependencies, and templates for the selected release. See the [deployment guide](../docs/deployment_EN.md) for complete instructions.

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.1/scripts/tp.sh) --help
```

| Command | Purpose |
| --- | --- |
| `deps install` | Install dependencies on supported Debian/Ubuntu hosts, reusing compatible Docker and yq |
| `deps remove` | Remove newly installed Docker packages and unchanged yq according to the installation record |
| `web` | Prompt for Web settings and install, or use an existing `--config` |
| `node` | Prompt for Node settings and install; supports `--certificate-mode external` |
| `config web\|node --output <file>` | Create a configuration template for the selected release |
| `validate --config <file>` | Validate YAML, version, and fields |
| `install --config <file> [--force]` | Deploy or recreate services using configuration |
| `remove --config <file> --keep-data` | Uninstall while retaining data |
| `remove --config <file> --purge-data` | Uninstall and delete project data |
| `--version <version>` | Select a release; accepted before or after the command |
| `--entry-version` | Show the entrypoint's default release |

Deployment implementations and templates live in `deploy/`. The entrypoint assembles and invokes their required files. Configuration, scripts, and product images must belong to the same release.

In a root Bash session, install dependencies separately before running `web`, `node`, or `install`; deployment commands only check dependencies.

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.1/scripts/tp.sh) deps install
```

Automatic dependency installation supports Debian 12/13 and Ubuntu 22.04/24.04 on amd64/arm64 with systemd. The entrypoint itself requires Bash, curl, CA certificates, grep, and coreutils; `deps` also requires `flock` from util-linux. See the [dependency guide](../docs/deployment_EN.md#dependencies) for minimal bootstrap instructions and manual preparation on other Linux distributions.

After removing the project and other Docker services, remove dependencies added by this command:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.1/scripts/tp.sh) deps remove
```

Pre-existing dependencies, basic tools, Docker data, and external certificates remain. Docker removal is refused while any Docker container exists (including stopped containers), shared containerd has containers in other namespaces, Node maintenance remains, or runtime state cannot be verified. If dependency installation was interrupted, rerun `deps install` to repair it before removal. See [dependency removal](../docs/deployment_EN.md#dependency-removal) for the full scope. `deps` also accepts `--version <version>`.

[Dependency installation](../docs/deployment_EN.md#dependency-install) · [One-command deployment](../docs/deployment_EN.md#web) · [Version selection](../docs/deployment_EN.md#versions) · [Configuration](../docs/deployment_EN.md#configuration) · [Project removal](../docs/deployment_EN.md#removal) · [Dependency removal](../docs/deployment_EN.md#dependency-removal)
