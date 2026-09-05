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

克隆项目并进入安装器目录：

```bash
git clone https://github.com/1linhao/trojanpanelnext.git
cd trojanpanelnext/deploy/installer
```

Web 主控：

```bash
cp examples/web.yaml ./web.yaml
./install.sh validate --mode web --config ./web.yaml
sudo ./install.sh install --mode web --config ./web.yaml
```

Node Agent：

```bash
cp examples/node-agent.yaml ./node-agent.yaml
./install.sh validate --mode node --config ./node-agent.yaml
sudo ./install.sh install --mode node --config ./node-agent.yaml
```

查看[完整安装说明](./install-tutorial/installation.md)。

## 支持

[TrojanPanel 原项目 GitHub](https://github.com/trojanpanel)
