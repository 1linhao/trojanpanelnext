# 常见问题

## Web 和 Node 有什么区别？

Web 提供面板、API、数据库与缓存。Node 运行代理实例；一台 Node 可以运行多个代理节点，Web 可以管理多台 Node。[架构说明](/start/system-structure)介绍各组件关系。

## 如何选择安装版本？

统一脚本入口的 `--version` 指定目标版本。入口下载该版本的脚本与模板，并使用绑定的产品镜像。所有参数见[版本说明](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment.md#versions)。

## 安装前需要哪些软件？

Docker Engine、Bash、curl、mikefarah/yq v4、OpenSSL 和常用系统工具；Node 还需要 systemd。在支持的 Debian/Ubuntu 主机上可先通过统一入口执行 `deps install`，该命令还需 util-linux 的 `flock`，其他 Linux 手动准备；`web`、`node`、`install` 部署命令只检查依赖。[依赖说明](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment.md#dependencies)包含完整清单、最小引导与安装命令。

## 配置文件从哪里下载，至少要改什么？

通过脚本入口执行 `config web --output ./web.yaml` 或 `config node --output ./node.yaml`，模板权限为 `0600`，不覆盖已有文件。Web 至少改域名和证书邮箱；Node 还需填写 Web 数据库/Redis 地址及实际密码、整数服务器 ID（≥ `1`，不是 IP / 域名或代理 ID）、TLS 服务器名，并在安装前准备 Web 公开 CA。所有字段保留在 `trojanpanelnext` 映射内，配置版本和镜像沿用下载模板。完整命令见[模板下载](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment.md#configuration-download)，字段见[Web 最少修改](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment.md#configuration-web-minimum)和[Node 最少修改](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment.md#configuration-node-minimum)。

## Web 部署后在哪里登记新 Node？

使用系统管理员（`sysadmin` 角色）登录，打开 **服务器管理 → 新增 Node 服务器**，填写 Node 地址、gRPC 端口和 TLS 服务器名。保存后自动打开 **部署 Node**，可下载填有连接参数、真实整数 ID 和公开 CA 的部署包；也可在服务器行重新打开。**ID** 列是 Web 数据库生成的整数（≥ `1`），不是 IP / 域名或代理实例 ID。Node 未安装时离线是正常状态。步骤见[登记节点服务器](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment.md#node-registration)和[部署包安装](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment.md#node-deployment-package)。

## 如何更新 Web 或 Node 镜像？

在对应主机使用统一入口，显式指定目标版本并执行 `update --config <实际部署 YAML>`；Web 使用 `web.yaml`，Node 使用 `node.yaml`。更新保留凭据、数据、端口和 PKI，不升级数据库、Redis 或 Caddy。切换会短暂中断服务，执行前备份业务数据；脚本的部署恢复不会回滚新版本的数据库迁移或写入。命令与限制见[镜像更新](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment.md#updates)。

## 卸载项目会卸载依赖吗？

不会。先完成项目和其他 Docker 服务卸载，再通过统一入口单独执行 `deps remove`。它仅清理记录中的新增 Docker 软件包与未修改的 yq；基础工具、原有依赖和 Docker 数据保留。存在 Docker 容器（含停止容器）、共享 containerd 其他 namespace 的容器、Node 维护服务或无法确认运行状态时拒绝删除 Docker。依赖安装中断时先重跑 `deps install` 修复，再卸载。见[依赖卸载](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment.md#dependency-removal)。

## 登录提示 timeout of 5000ms exceeded？

先检查 Web API、MariaDB 和 Redis 容器是否运行，浏览器 `/api` 请求是否正确转发，数据库和缓存地址是否可达，以及服务日志的具体错误。按实际端口放行防火墙；排查步骤见[故障处理](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment.md#troubleshooting)。

## Node 能使用现有证书吗？

可以。外部模式接收 fullchain PEM 和私钥路径，跳过 Node Caddy 和证书申请，续签由 Certbot 等外部工具负责。见[外部证书部署](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment.md#external-certificates)。

## Node 失联时如何移除服务器？

选择“删除”，只清理 Web 中该服务器的记录和关联数据，不连接 Node。“卸载”保留 Node 数据并远程移除项目，“彻底卸载”还清理 Node 项目数据；这两种操作都需要连通 Node，失败时保留 Web 记录。“删除”不会停止 Node 上仍运行的服务，需要时自行在该主机卸载。完整范围见[服务器移除](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment.md#web-removal)。

## 移除后能重新接入吗？

可以。保留数据“卸载”后，在 Web 重新创建服务器，更新 Node 配置里的新服务器 ID 与当前公开 CA，重新安装后创建所需代理。只“删除”Web 记录时，先在 Node 主机保留数据卸载，避免沿用已删除的服务器 ID。见[重新接入说明](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment.md#reconnect)。

## 同一端口能运行多个服务吗？

需要按实际协议配置反向代理或 TLS SNI 分流。脚本不自动更改已有 Nginx 配置；Node 可使用外部证书与已有证书管理方案协作。TCP 与 UDP 分别监听，端口规划见[网络说明](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/deployment.md#network)。

## 证书是否自动续签？

默认公网证书由 Caddy 维护。外部模式由外部证书工具维护。内部 mTLS 身份与 CA 由 Web API 自动维护；NaiveProxy 加载变更证书时可能短暂中断连接。详见[证书管理](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/certificates.md)。
