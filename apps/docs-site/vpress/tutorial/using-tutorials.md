# Web 使用指南

## 首次登录

访问部署时指定的 Web 域名。初始账户为 `sysadmin` / `123456`，首次登录后在个人中心修改密码。

## 操作顺序

1. 在服务器管理中登记 Node 主机；**ID** 列显示 Web 数据库生成的整数（≥ `1`），与 IP / 域名分列，也不是代理 ID。
2. 保存后在 **部署 Node** 下载包，安全传到目标 Node，准备依赖后解压并运行 `bash ./tpnext/install-node.sh`；它自动准备公开 CA，按已填真实 ID 的 YAML 安装。步骤见[部署包安装](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.10/docs/deployment.md#node-deployment-package)，安装后确认服务器在线。
3. 在节点管理中选择该服务器，创建 Xray、Hysteria2 或 NaiveProxy 代理实例。使用外部转发入口时开启 **端口转发**：对外端口用于客户端，实际端口用于 Node 监听；转发规则自行维护。
4. 配置用户角色、额度和有效期，导入账户对应的节点或订阅。
5. 在首页查看流量，在内核管理中管理 Xray / Hysteria2 升级和回退任务。

## 服务器卸载与删除

服务器管理中的移除弹窗包含“取消”“删除”“卸载”“彻底卸载”。“卸载”远程卸载项目并保留 Node 数据；“彻底卸载”还清理 Node 项目数据与对应 Web 数据。远程卸载成功后才清理 Web 登记，失联或卸载失败会报错并保留记录。

“删除”不联系 Node，只清理 Web 中该服务器及关联代理、协议配置、流量、任务和连接记录，失联时也可执行。Node 主机上仍运行的项目服务不会停止，需要时在该主机执行本地卸载。

删除一个代理实例只影响该实例。完整删除范围、离线处理与重新接入见[服务器删除指南](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.10/docs/deployment.md#web-removal)。

## 完整说明

[Web 使用指南](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.10/docs/user-guide.md)覆盖账户、订阅、代理、内核任务、系统设置和界面偏好。

## 用户备注

只有 `sysadmin` 可在用户列表查看 **备注**，并在账户 **编辑** 弹窗中设置。备注为最多 500 个字符的纯文本，清空后保存即可删除。`admin` 和普通用户的页面、个人资料和接口响应不包含备注。详见[用户备注](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.10/docs/user-guide.md#account-remarks)。

## 重置账号登录失败次数

`sysadmin` 可在用户列表点击所选账号的 **重置登录失败次数** 图标，确认后清除该账号的失败计数和临时锁定。重置不改变密码或禁用状态，也不影响其他账号。详见[登录失败次数重置](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.10/docs/user-guide.md#account-login-reset)。

## 代理端口转发

使用 Nginx / NAT 入口时，在原节点表单中配置对外和实际端口，按 TCP / UDP 协议维护外部转发。TCP TLS 分流要求客户端 SNI 与 Node 提供的证书域名一致；Hysteria2 的 UDP 和公开跳跃范围需独立规划。见[端口转发](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.10/docs/port-forwarding.md)。
