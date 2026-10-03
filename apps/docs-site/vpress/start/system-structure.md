# 系统架构

| 组件 | 职责 |
| --- | --- |
| Web UI | 账户、服务器、代理、订阅、流量和版本管理的操作界面 |
| Web API | 业务数据、权限、Node 控制与 mTLS 客户端身份维护 |
| MariaDB / Redis | 数据存储、会话与协作数据 |
| Node Agent | 管理 Xray、Hysteria2、NaiveProxy 的配置、进程和流量 |
| 宿主机维护服务 | systemd 服务，执行 Node 项目卸载并确认结果 |
| Caddy | Web HTTPS 与默认 Node 公网证书；Node 可使用外部证书 |

应用服务由 Docker 承载，宿主机维护服务由 systemd 承载。Web 与 Node 可部署到独立主机，Web 到 Node 的 gRPC 和维护 HTTPS 使用 mTLS；Agent 不挂载 Docker socket。

一台节点服务器可以运行多个代理实例。完整组件关系与领域术语见[架构文档](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.12/docs/architecture/domain-context.md)，网络准备见[部署指南](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.12/docs/deployment.md#network)。
