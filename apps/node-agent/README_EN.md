# TrojanPanel Next Node Agent

[简体中文](README.md) | English

The Node Agent runs on a node server and receives management requests from the Web control plane over mTLS. It manages Xray, Hysteria2, and NaiveProxy processes, configuration, accounts, and traffic statistics.

## Deployment

Install Node through the [v1.0 script entrypoint](../../scripts/README_EN.md). See the [deployment guide](../../docs/deployment_EN.md#node) for network, database, server registration, and public CA preparation.

Certificates can be managed automatically by Caddy or supplied in [external certificate mode](../../docs/deployment_EN.md#external-certificates). The host maintenance service handles server removal; the Agent does not need a Docker socket mount.

## Source layout

| Directory | Purpose |
| --- | --- |
| `app/` | Proxy kernels, processes, and certificate loading |
| `api/` | gRPC management and Hysteria2 authentication |
| `core/` | Initialization, configuration, and shared state |
| `dao/`, `service/` | Accounts, node configuration, and runtime state |
| `hostagent/`, `cmd/host-agent/` | Host maintenance service |
| `scripts/` | Build tools for the NaiveProxy traffic extension |

## Development

Run from this directory:

```bash
go test ./...
go build ./...
```

See the [development guide](../../docs/development_EN.md) for dependencies, workflow, and test commands.

## Proxy projects

- [Xray-core](https://github.com/XTLS/Xray-core)
- [Hysteria2](https://github.com/apernet/hysteria)
- [NaiveProxy](https://github.com/klzgrad/naiveproxy)
