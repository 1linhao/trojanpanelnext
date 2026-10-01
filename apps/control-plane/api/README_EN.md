# TrojanPanel Next API

[简体中文](README.md) | English

The Web backend provides account, node server, proxy, subscription, traffic, kernel task, and system configuration APIs. It also maintains the mTLS client identity used to manage Node servers.

## Deployment

Deploy Web through the [v1.0.2-rc.7 script entrypoint](../../../scripts/README_EN.md). See the [deployment guide](../../../docs/deployment_EN.md#web) for networking, databases, and certificates.

## Source layout

| Directory | Purpose |
| --- | --- |
| `router/`, `api/` | HTTP routing, request models, and responses |
| `middleware/` | JWT, permissions, and request handling |
| `service/`, `dao/` | Business logic, databases, and cache access |
| `core/` | Node gRPC clients and service configuration |
| `pki/` | Persistent mTLS identity, renewal, and CA rotation |

## Development

Run from this directory:

```bash
go test ./...
go build ./...
```

The [development guide](../../../docs/development_EN.md) covers builds and tests. The [API guide](../../../docs/api.md) describes authentication and current API entrypoints.
