---
home: true
heroImage: /logo.png
heroText: TrojanPanel Next
tagline: 支持 Xray、Hysteria2 和 NaiveProxy 的多用户 Web 管理面板
actionText: 快速上手 →
actionLink: ./start/introduce
features:
  - title: 配置驱动
    details: 使用 YAML 配置和明确用途模式完成无交互部署
  - title: Web 主控
    details: 统一管理用户、节点、流量、证书和系统配置
  - title: Node Agent
    details: 将代理内核与主控分离，可按需扩展多个节点服务器
  - title: 多代理支持
    details: 支持 Xray、Hysteria2 和 NaiveProxy
  - title: 响应式界面
    details: 管理员与普通用户页面均适配桌面和手机浏览器
  - title: 多语言
    details: Web 管理界面支持多种语言和主题
footer: TrojanPanel Next
---

简体中文 | [English](README_EN.md)

## 安装

安装前请预装 Docker、[mikefarah/yq v4](https://github.com/mikefarah/yq)、curl、OpenSSL、tar、coreutils、findutils 和 awk。安装器不会自动安装依赖；[安装器说明](https://github.com/1linhao/trojanpanelnext/blob/main/deploy/installer/README.md)提供 Debian/Ubuntu 命令和 yq 校验安装步骤。

在每台服务器下载 `0.1.0-rc.5` 版本安装器：

```bash
curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v0.1.0-rc.5/deploy/installer/install.sh -o install.sh
chmod +x install.sh
```

Web 主控：

```bash
./install.sh config web
nano web.yaml
./install.sh validate --config ./web.yaml
sudo ./install.sh install --config ./web.yaml
```

Node Agent：

```bash
./install.sh config node
nano node.yaml
./install.sh validate --config ./node.yaml
sudo ./install.sh install --config ./node.yaml
```

Node 安装前需复制 Web 的公开 CA，填写数据库、Redis 凭据和实际服务器 ID。查看[完整安装说明](./install-tutorial/installation.md)。

## 支持

[TrojanPanel 原项目 GitHub](https://github.com/trojanpanel)
