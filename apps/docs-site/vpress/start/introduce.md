# 介绍

TrojanPanel Next v1.0.2-rc.4 是支持 Xray、Hysteria2 和 NaiveProxy 的多用户 Web 管理面板。

系统由 Web 主控与 Node Agent 组成。Web 提供管理界面、业务 API、数据库和缓存；Node 运行代理实例，通过 mTLS 接收管理请求。宿主机维护服务负责整台节点服务器的卸载。

通过统一脚本入口可以一键部署，或者使用 YAML 配置无交互安装。Node 证书支持 Caddy 自动管理与外部证书路径两种模式。

- [快速部署](/install-tutorial/installation)
- [完整社区使用文档](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.4/docs/README.md)
- [系统架构](/start/system-structure)
