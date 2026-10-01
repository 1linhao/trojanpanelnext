---
home: true
heroImage: /logo.png
heroText: TrojanPanel Next
tagline: 支持 Xray、Hysteria2 和 NaiveProxy 的多用户 Web 管理面板
actionText: 快速上手 →
actionLink: ./install-tutorial/installation
features:
  - title: 一键部署
    details: 通过统一脚本入口部署 Web 与 Node，可指定产品版本
  - title: Web 主控
    details: 管理账户、节点服务器、代理、订阅和流量
  - title: Node Agent
    details: 管理代理运行时，使用 mTLS 连接主控
  - title: 证书管理
    details: 支持 Caddy 自动证书及 Node 外部证书模式
  - title: 响应式界面
    details: 管理员和普通用户页面适配桌面与手机
  - title: 内核任务
    details: 管理 Xray 与 Hysteria2 升级和回退
footer: TrojanPanel Next
---

简体中文 | [English](README_EN.md)

## 快速安装

当前版本为 **v1.0**。先完成[依赖与网络准备](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/deployment.md#dependencies)，然后在 root Bash 中运行。

Web 主控：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0/scripts/tp.sh) web
```

Node Agent：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0/scripts/tp.sh) node
```

入口按提示收集配置并部署同版本镜像，也支持指定版本、配置文件部署、外部证书、重建和卸载。完整操作以 [docs 部署指南](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/deployment.md)为准。

## 文档

- [完整部署与卸载](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/deployment.md)
- [Web 使用指南](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/user-guide.md)
- [证书管理](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/certificates.md)
- [API 指南](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/api.md)
- [开发指南](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/development.md)
