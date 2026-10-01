# TrojanPanel Next API

简体中文 | [English](README_EN.md)

Web 主控后端，提供账户、节点服务器、代理节点、订阅、流量、内核任务和系统设置接口，并维护 Web 到 Node 的 mTLS 客户端身份。

## 部署

通过 [v1.0.2-rc.6 脚本入口](../../../scripts/README.md)部署 Web。网络、数据库和证书准备见[部署指南](../../../docs/deployment.md#web)。

## 源码

| 目录 | 用途 |
| --- | --- |
| `router/`、`api/` | HTTP 路由、请求模型与响应 |
| `middleware/` | JWT、权限与请求处理 |
| `service/`、`dao/` | 业务逻辑、数据库及缓存访问 |
| `core/` | Node gRPC 客户端与服务配置 |
| `pki/` | 持久化 mTLS 身份、续签与 CA 轮换 |

## 开发

在本目录执行：

```bash
go test ./...
go build ./...
```

[开发指南](../../../docs/development.md)包含构建和测试命令；[API 指南](../../../docs/api.md)说明鉴权方式与当前接口入口。
