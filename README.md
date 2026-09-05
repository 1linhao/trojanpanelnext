# TrojanPanel Next

简体中文 | [English](README_EN.md)

TrojanPanel Next 是一个支持 Xray、Hysteria2 和 NaiveProxy 的多用户 Web
管理面板，提供用户与节点管理、系统看板、证书管理和分布式部署能力。

本仓库统一维护以下产品组件：

| 组件 | 说明 |
| --- | --- |
| `apps/control-plane/api` | 控制面后端服务 |
| `apps/control-plane/web` | Web 管理界面 |
| `apps/node-agent` | 节点 Agent 与代理内核运行管理 |
| `deploy/installer` | 安装和部署工具 |
| `apps/docs-site` | 使用与安装文档 |

## 开始使用

[安装说明](deploy/installer/README.md)

[Web 管理界面](apps/control-plane/web/README.md)

[Node Agent](apps/node-agent/README.md)

[文档站源码](apps/docs-site/vpress)

## 项目来源

TrojanPanel Next 是基于 [TrojanPanel](https://github.com/trojanpanel) 原始项目演进的
独立维护版本，不是原组织的官方发布。项目来源与历史说明见 [NOTICE.md](NOTICE.md)。
