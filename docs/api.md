# API 指南

[文档首页](README.md) · [开发指南](development.md)

## 目录

- [请求与鉴权](#authentication)
- [接口入口](#endpoints)
- [服务器删除](#server-removal)
- [Node 管理通道](#node-management)
- [接口模型](#models)

<a id="authentication"></a>
## 请求与鉴权

Web API 通过面板 HTTPS 下的 `/api` 提供服务。登录接口为 `POST /api/auth/login`，JSON 字段包含 `username` 和 `pass`；启用验证码时还需提交 `captchaId` 和 `captchaCode`。验证码来自 `GET /api/auth/generateCaptcha`。

登录返回 JWT。需要登录的接口使用 `Authorization: Bearer <token>`；权限由当前账户角色控制，内核任务和服务器维护等管理接口需要相应管理员权限。

```bash
curl -fsS https://panel.example.com/api/nodeServer/selectNodeServerList -H "Authorization: Bearer $TP_API_TOKEN"
```

JSON 响应包含 `code`、`type`、`message` 和 `data`。成功业务码为 `20000`；业务失败也可能使用 HTTP 200，因此客户端必须检查业务码与 `type`，不能只检查 HTTP 状态。订阅等返回原始内容的接口按对应接口处理。

<a id="endpoints"></a>
## 接口入口

| 功能 | 当前路径或路径前缀 | 源码 |
| --- | --- | --- |
| 登录、注册、验证码、订阅 | `/api/auth/` | [auth.go](../apps/control-plane/api/router/auth.go) |
| 账户 | `/api/account/` | [account.go](../apps/control-plane/api/router/account.go) |
| 角色 | `/api/role/` | [role.go](../apps/control-plane/api/router/role.go) |
| 节点服务器 | `/api/nodeServer/` | [node_server.go](../apps/control-plane/api/router/node_server.go) |
| 代理节点 | `/api/node/` | [node.go](../apps/control-plane/api/router/node.go) |
| 可用代理类型 | `/api/nodeType/selectNodeTypeList` | [node_type.go](../apps/control-plane/api/router/node_type.go) |
| 流量与首页 | `/api/dashboard/` | [dashboard.go](../apps/control-plane/api/router/dashboard.go) |
| 系统设置与订阅模板 | `/api/system/` | [system.go](../apps/control-plane/api/router/system.go) |
| 内核版本、清单与任务 | `/api/kernel/` | [kernel_upgrade.go](../apps/control-plane/api/router/kernel_upgrade.go) |
| 黑名单 | `/api/blackList/` | [black_list.go](../apps/control-plane/api/router/black_list.go) |
| 邮件记录 | `/api/emailRecord/` | [email_record.go](../apps/control-plane/api/router/email_record.go) |
| 文件任务 | `/api/fileTask/` | [file_task.go](../apps/control-plane/api/router/file_task.go) |

接口定义及模型随当前版本源码发布。完整路由参见 [router/](../apps/control-plane/api/router/)，Web 调用示例参见 [src/api/](../apps/control-plane/web/src/api/)。

<a id="server-removal"></a>
## 服务器删除

`POST /api/nodeServer/deleteNodeServerById` 接收服务器 ID 和清理模式：

```json
{
  "id": 12,
  "purge": false
}
```

`purge: false` 对应页面“删除”，卸载 Node 容器及镜像并保留项目数据；`purge: true` 对应“彻底删除”，同时清理项目数据以及此服务器对应的 Web 连接、流量与内核任务信息。该操作针对整台节点服务器，单个代理使用 `/api/node/deleteNodeById`。

Web 先请求目标 Node 的宿主机维护服务卸载，收到成功结果后再清理登记；卸载失败会保留服务器信息。维护结果确认与后续服务清理可能返回 `cleanupPending`，保持双向连接至完成。完整范围见[服务器删除说明](deployment.md#web-removal)。

<a id="node-management"></a>
## Node 管理通道

Web 到 Node 的 gRPC 与宿主机维护 HTTPS 均使用 mTLS，部署时配置 Node 公网证书及 Web 当前公开客户端 CA。gRPC 协议见 [grpc_api.proto](../apps/node-agent/api/grpc_api.proto)。这些接口应仅允许 Web 主机访问，不作为面向浏览器的公共 API。

`POST /api/nodeServer/completeHostRemoval` 是宿主机维护服务的结果确认入口，使用卸载流程签发的回执验证；它不使用普通用户 JWT。一般集成应使用 Web 服务器删除接口，不直接调用此内部回执端点。

<a id="models"></a>
## 接口模型

请求 DTO 位于 [model/dto/](../apps/control-plane/api/model/dto/)，响应模型位于 [model/vo/](../apps/control-plane/api/model/vo/)。创建服务器所需字段、流量限制与 TLS 服务名以 [node_server.go](../apps/control-plane/api/model/dto/node_server.go) 为准。代理参数按所选类型使用 [Web 代理表单](../apps/control-plane/web/src/views/node/list/components/) 与对应 DTO。

集成应用应锁定产品版本，按此版本的源码定义验证请求和响应。
