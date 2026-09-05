# 架构设计

- 所有运行服务使用 Docker 容器承载，降低平台差异。
- Web 主控与 Node Agent 分离，可以按需部署多个节点服务器。
- Node Agent 统一管理 Xray、Hysteria2 和 NaiveProxy 运行时。
- 安装脚本读取 YAML 配置，无需交互输入。

这是占内存最小而且不需要自己手动申请/续签证书的轻量级方案。实测，把所有服务器都部署完，1H1G的服务器足够用，参考[性能检测](/tutorial/performance-testing)。
