# TrojanPanel Next Documentation

[简体中文](README.md) | English

TrojanPanel Next v1.0.1 provides a multi-user Web control plane for Xray, Hysteria2, and NaiveProxy.

## Quick installation

In a root Bash session, install dependencies first, then prepare DNS and [network access](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/deployment_EN.md#network). Automatic dependency installation supports Debian 12/13 and Ubuntu 22.04/24.04 on amd64/arm64 with systemd:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.1/scripts/tp.sh) deps install
```

The entrypoint needs Bash, curl, CA certificates, grep, and coreutils; `deps` also needs `flock` from util-linux. See [dependencies](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/deployment_EN.md#dependencies) for minimal bootstrap instructions and manual installation on other Linux distributions.

Web:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.1/scripts/tp.sh) web
```

Node:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.1/scripts/tp.sh) node
```

The entrypoint prompts for configuration and deploys images from the selected release. It also supports version selection, YAML deployment, external certificates, container recreation, and removal. Follow the [complete deployment guide](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/deployment_EN.md) for all commands.

After removing the project and other Docker services, remove added Docker packages and unchanged yq according to the installation record:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.1/scripts/tp.sh) deps remove
```

Pre-existing dependencies, basic tools, Docker data, and external certificates remain; Docker containers, shared containerd's other namespaces, and maintenance are checked before removal. If installation was interrupted, rerun `deps install` to repair it first. See [dependency removal](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/deployment_EN.md#dependency-removal) for the full scope.

## Documentation

- [Deployment and removal](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/deployment_EN.md)
- [Web user guide](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/user-guide.md)
- [Certificate management](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/certificates.md)
- [API reference](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/api.md)
- [Development guide](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/development_EN.md)
