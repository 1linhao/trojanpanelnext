# API 指南

Web API 位于面板 HTTPS 下的 `/api`。管理接口使用 `Authorization: Bearer <token>`，并根据账户角色限制权限；Node 管理通道使用 mTLS。

[当前 API 参考](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/api.md)包括登录与验证码、响应业务码、接口入口、服务器卸载及对应请求模型源码。

## 主要模块

- 账户与角色
- 节点服务器与代理节点
- 订阅及客户端模板
- 流量统计
- Xray / Hysteria2 内核任务
- 系统设置、黑名单、邮件与文件任务

集成应用应绑定产品版本，并以该版本的 [router 源码](https://github.com/1linhao/trojanpanelnext/tree/v1.0/apps/control-plane/api/router)及 DTO 为准。
