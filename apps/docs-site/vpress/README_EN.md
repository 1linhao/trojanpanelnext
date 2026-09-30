# TrojanPanel Next Documentation

[简体中文](README.md) | English

TrojanPanel Next is a multi-user Web administration panel for Xray, Hysteria2, and NaiveProxy.

## Installation

Preinstall Docker, [mikefarah/yq v4](https://github.com/mikefarah/yq), curl, OpenSSL, tar, coreutils, findutils, and awk. The installer does not install dependencies. See the [installer guide](https://github.com/1linhao/trojanpanelnext/blob/main/deploy/installer/README_EN.md) for Debian/Ubuntu and verified yq installation instructions.

Download the matching `0.1.0-rc.5` installer on each server:

```bash
curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v0.1.0-rc.5/deploy/installer/install.sh -o install.sh
chmod +x install.sh
```

Web control plane:

```bash
./install.sh config web
nano web.yaml
./install.sh validate --config ./web.yaml
sudo ./install.sh install --config ./web.yaml
```

Node Agent:

```bash
./install.sh config node
nano node.yaml
./install.sh validate --config ./node.yaml
sudo ./install.sh install --config ./node.yaml
```

Before installing Node, copy Web's public CA, fill in database/Redis credentials, and match the actual panel server ID.

## Support

[Original TrojanPanel project on GitHub](https://github.com/trojanpanel)
