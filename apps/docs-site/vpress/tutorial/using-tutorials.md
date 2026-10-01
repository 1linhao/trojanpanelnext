# Web 使用指南

## 首次登录

访问部署时指定的 Web 域名。初始账户为 `sysadmin` / `123456`，首次登录后在个人中心修改密码。

## 操作顺序

1. 在服务器管理中登记 Node 主机，取得服务器 ID。
2. 按[Node 部署说明](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/deployment.md#node)安装 Agent，准备 Web 当前公开 CA，确认服务器在线。
3. 在节点管理中选择该服务器，创建 Xray、Hysteria2 或 NaiveProxy 代理实例。
4. 配置用户角色、额度和有效期，导入账户对应的节点或订阅。
5. 在首页查看流量，在内核管理中管理 Xray / Hysteria2 升级和回退任务。

## 服务器删除

服务器管理中的删除弹窗包含“取消”“删除”“彻底删除”。“删除”保留远端项目数据；“彻底删除”清理远端项目数据和此服务器对应的 Web 记录。远端卸载成功后才清理 Web 登记。

删除一个代理实例只影响该实例。完整删除范围、离线处理与重新接入见[服务器删除指南](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/deployment.md#web-removal)。

## 完整说明

[Web 使用指南](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/user-guide.md)覆盖账户、订阅、代理、内核任务、系统设置和界面偏好。
