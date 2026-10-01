# Web 使用指南

[文档首页](README.md) · [部署指南](deployment.md)

## 目录

- [首次登录](#login)
- [节点服务器](#servers)
- [代理节点](#proxies)
- [账户与订阅](#accounts)
- [流量与内核管理](#traffic-and-kernels)
- [产品镜像更新](#image-updates)
- [系统设置](#settings)
- [界面偏好](#appearance)

<a id="login"></a>
## 首次登录

打开安装时指定的 Web 域名。初始管理员为 `sysadmin`，初始密码为 `123456`，首次登录后在个人中心修改密码。可在系统设置中控制注册与验证码；普通账户只查看和使用具有访问权限的资源。

<a id="servers"></a>
## 节点服务器

使用具有 `sysadmin` 角色的系统管理员账号登录，点击首页的“新增 Node 服务器”，或左侧“服务器管理”中的同名按钮。首页入口会直接打开登记表单，也可访问 `https://你的Web域名/#/server-manage/server-list`。

登记时至少填写以下内容：

| 字段 | 要求 |
| --- | --- |
| 服务器名称 | 2–20 个字符，用于识别服务器 |
| 服务器 IP | Web 可访问的 Node IP 地址或域名 |
| gRPC端口 | 默认 `8100`，与 Node 配置中的 `grpc_port` 一致 |
| gRPC 证书域名 | 与 Node 配置中的 `grpc_tls_server_name` 一致，并被 Node 证书覆盖 |

确认后在服务器名称下方查看“服务器 ID”，将真实 ID 填入 Node 配置的 `node_server_id`。Node 尚未安装时显示离线是正常的。将 Web 当前公开 `client-ca.crt` 复制到 Node，再按[配置文件下载](deployment.md#configuration-download)、[Node 最少编辑项](deployment.md#configuration-node-minimum)和[CA 准备](deployment.md#configuration-node-ca)完成校验与安装。详细步骤见[登记 Node 服务器](deployment.md#node-registration)。

一台节点服务器可承载多个代理节点。服务器在线后再添加代理；地址、证书服务名、CA 与防火墙配置必须一致。

服务器移除弹窗包含“取消”“删除”“卸载”“彻底卸载”：

- **删除**：不联系 Node，只清理 Web 中该服务器的登记、关联代理与协议配置、流量、任务和连接记录，失联服务器也能删除。Node 主机上仍运行的服务不会停止；需要清理时在该主机执行[本地卸载](deployment.md#removal)。
- **卸载**：远程卸载 Node 项目容器与可清理的镜像，保留 Node 数据，再移除 Web 登记与关联代理配置。
- **彻底卸载**：远程卸载并清理 Node 项目数据，同时清理对应 Web 数据。

同一服务器已有移除请求正在处理时会提示忙碌，待操作结束后重试；其他服务器的失联卸载不会阻塞纯 Web 删除。两种远程卸载只有成功后才移除 Web 登记；Node 失联或卸载失败会报错并保留记录。完整范围见[服务器移除](deployment.md#web-removal)，重新接入见[重新接入](deployment.md#reconnect)。

<a id="proxies"></a>
## 代理节点

节点管理中选择服务器和代理类型，当前支持 Xray、Hysteria2 与 NaiveProxy。填写监听端口、域名及对应协议参数，确认端口没有被已有服务占用，并按协议开放 TCP 或 UDP。

TLS 代理所使用的域名须被 Node 证书覆盖。多个域名共用 Node 证书时需使用匹配的 SAN 或通配符证书。现有 Nginx、Certbot 环境可使用[外部证书模式](deployment.md#external-certificates)，按实际网络设计配置 SNI 分流。

创建后检查状态，可复制对应连接 URL、查看二维码或导出客户端配置。删除单个代理只移除该实例；删除整台节点服务器通过服务器管理完成。

<a id="accounts"></a>
## 账户与订阅

用户管理支持额度、有效期、角色和速率等设置。账户的访问范围受角色及代理设置控制。普通用户在“我的”页面查看有效期、剩余流量及可用订阅。

订阅按目标客户端生成，支持 Xray、sing-box 和 Clash 系列模板。选择与代理协议兼容的客户端，使用订阅地址或单节点配置导入。含账户凭据的订阅 URL、二维码和导出配置应作为私密信息保存。

<a id="traffic-and-kernels"></a>
## 流量与内核管理

首页展示账户及服务器统计，管理员可按日期范围查询流量。服务器流量周期、总量或上下行限制在服务器配置中设置。

内核管理提供 Xray 与 Hysteria2 上游版本、节点库存、升级和回退任务。选择正式或预发布通道后确认目标服务器，关注每个任务项的结果；执行升级或回退可能重启对应代理连接。NaiveProxy 通过产品 Node 镜像提供。

<a id="image-updates"></a>
## 产品镜像更新

Web 和 Node 的 TrojanPanel Next 镜像在各自主机通过脚本入口的 `update --config` 更新，使用实际部署 YAML 和目标发布版本。Web 页面中的内核升级只更新代理内核，不会更新 Web 或 Node 产品镜像。命令与保留范围见[镜像更新](deployment.md#updates)。

<a id="settings"></a>
## 系统设置

系统设置包含注册、账户默认值、验证码、邮箱、界面名称与标识、伪装网站和客户端订阅模板。配置系统邮箱后可发送账户到期提醒。模板编辑时选择正确目标客户端与格式，保存前校验语法并导出检查。

<a id="appearance"></a>
## 界面偏好

页面支持亮色和暗色主题、四套调色板、多语言与响应式导航。调色板保存在当前浏览器，系统主题变化可同步到页面。浏览器工具栏采用页面主题色的效果取决于浏览器和操作系统支持。
