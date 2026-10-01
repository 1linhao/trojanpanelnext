# TrojanPanel Next Documentation

[简体中文](README.md) | English

TrojanPanel Next v1.0 provides a multi-user Web control plane for Xray, Hysteria2, and NaiveProxy.

## Quick installation

Prepare the [dependencies and networking](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/deployment_EN.md#dependencies), then run in a root Bash shell.

Web:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0/scripts/tp.sh) web
```

Node:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0/scripts/tp.sh) node
```

The entrypoint prompts for configuration and deploys images from the selected release. It also supports version selection, YAML deployment, external certificates, container recreation, and removal. Follow the [complete deployment guide](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/deployment_EN.md) for all commands.

## Documentation

- [Deployment and removal](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/deployment_EN.md)
- [Web user guide](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/user-guide.md)
- [Certificate management](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/certificates.md)
- [API reference](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/api.md)
- [Development guide](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/development_EN.md)
