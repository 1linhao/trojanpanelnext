# TrojanPanel Next script library

[简体中文](README.md) | English

`tp.sh` is the v1.0 entrypoint. It downloads commands, shared dependencies, and templates for the selected release. See the [deployment guide](../docs/deployment_EN.md) for complete instructions.

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0/scripts/tp.sh) --help
```

| Command | Purpose |
| --- | --- |
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

[One-command deployment](../docs/deployment_EN.md#web) · [Version selection](../docs/deployment_EN.md#versions) · [Configuration](../docs/deployment_EN.md#configuration) · [Removal](../docs/deployment_EN.md#removal)
