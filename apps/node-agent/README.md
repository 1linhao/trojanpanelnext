# TrojanPanel Next Node Agent

简体中文 | [English](README_EN.md)

Node Agent 运行在节点服务器上，接受 Web 主控的 mTLS 管理请求，管理 Xray、Hysteria2 和 NaiveProxy 的运行配置、账户及流量统计。

## 部署

使用 [v1.0.2-rc.5 脚本入口](../../scripts/README.md)安装 Node，具体网络、数据库、节点登记和公开 CA 准备见[部署指南](../../docs/deployment.md#node)。

证书支持 Caddy 自动管理和[外部证书模式](../../docs/deployment.md#external-certificates)。宿主机维护服务负责服务器卸载；Agent 无需挂载 Docker socket。

## 源码

| 目录 | 用途 |
| --- | --- |
| `app/` | 代理内核、进程管理与证书加载 |
| `api/` | gRPC 管理接口与 Hysteria2 认证接口 |
| `core/` | 服务初始化、配置与公共状态 |
| `dao/`、`service/` | 账户、节点配置和运行状态 |
| `hostagent/`、`cmd/host-agent/` | 宿主机维护服务 |
| `scripts/` | NaiveProxy 流量统计扩展的构建工具 |

## 开发

在本目录执行：

```bash
go test ./...
go build ./...
```

依赖、完整开发流程与测试说明见[开发指南](../../docs/development.md)。

## 代理项目

- [Xray-core](https://github.com/XTLS/Xray-core)
- [Hysteria2](https://github.com/apernet/hysteria)
- [NaiveProxy](https://github.com/klzgrad/naiveproxy)
