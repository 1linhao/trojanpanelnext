# TrojanPanel Next Documentation

[简体中文](README.md) | English

TrojanPanel Next is a multi-user Web administration panel for Xray, Hysteria2, and NaiveProxy.

## Installation

Clone the project and enter the installer directory:

```bash
git clone https://github.com/1linhao/trojanpanelnext.git
cd trojanpanelnext/deploy/installer
```

Install the Web control plane:

```bash
cp examples/web.yaml ./web.yaml
./install.sh validate --mode web --config ./web.yaml
sudo ./install.sh install --mode web --config ./web.yaml
```

Install a Node Agent:

```bash
# After registering the identity on Web and creating node-sg.age with node-bundle
sudo ./install.sh validate --mode node --bundle ./node-sg.age
sudo ./install.sh install --mode node --bundle ./node-sg.age
```

## Support

[Original TrojanPanel project on GitHub](https://github.com/trojanpanel)
