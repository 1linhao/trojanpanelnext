# TrojanPanel Next

简体中文 | [English](README_EN.md)

TrojanPanel Next 是支持 Xray、Hysteria2 和 NaiveProxy 的多用户代理管理平台。Web 主控提供账户、节点服务器、代理、流量和任务管理；Node Agent 管理各服务器上的代理内核。

当前预发布版本：**v1.0.2-rc.3（Pre-release）**。支持 Linux amd64、arm64，通过 GHCR 镜像部署。

## 快速安装

### 1. 在 Web 主机准备依赖

在 **root Bash 会话**执行。自动依赖安装支持 Debian 12/13、Ubuntu 22.04/24.04 的 amd64/arm64 主机，需要运行 systemd。

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) deps install
```

入口需要 Bash、curl、CA 证书、grep 和 coreutils；`deps` 还需要 util-linux 提供的 `flock`。缺少工具时先按[最小引导说明](docs/deployment.md#dependency-install)准备，其他 Linux 按[软件依赖](docs/deployment.md#dependency-manual)手动安装。准备 Web 域名解析和[网络访问](docs/deployment.md#network)。

### 2. 安装 Web 主控

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) web
```

按提示填写域名和证书邮箱。安装后访问该 HTTPS 域名，初始账户为 `sysadmin` / `123456`，首次登录后修改密码。

### 3. 在 Web 登记 Node 服务器

以系统管理员（`sysadmin` 角色）打开左侧 **服务器管理**，点击 **新增 Node 服务器**。填写服务器名称、Node 的 IP 或域名、gRPC 端口（默认 `8100`）和 gRPC 证书域名。

保存后自动打开 **部署 Node**；之后可通过该服务器行的 **部署 Node** 重新打开。服务器列表的 **ID** 列显示 Web 数据库生成的数字 ID，例如 `3`。它必须是 **大于或等于 1 的整数**，与 **IP / 域名** 分列；它不是服务器地址，也不是某个代理节点的 ID。详见[登记节点服务器](docs/deployment.md#node-registration)。

### 4. 下载 Node 部署包

在 **部署 Node** 中确认 Node 可访问的 Web 域名或 IP（例如 `panel.example.com`，不含 `https://` 和路径），选择证书模式：

- **Caddy**：填写证书邮箱，由 Node Caddy 申请与续签。
- **使用宿主机现有证书**：填写目标 Node 上已存在的 fullchain 与私钥绝对路径，由宿主机工具维护。

点击 **下一步** 进入 **下载与安装**，再点击 **下载部署包**。下载的 `.tar.gz` 包包含 `node.yaml`、公开 `client-ca.crt`、`install-node.sh` 和 `README.md`；配置已填写真实服务器 ID、Web 数据库 / Redis 凭据及连接参数。包不包含 Web 私钥或 Node TLS 私钥。已有证书模式的证书与私钥必须事先在 Node 主机准备，Web 不生成这些文件。

部署包和 YAML 含敏感凭据，应保存为 **`0600`**，只安全传输到目标 Node，不提交公开仓库。完整范围见[部署包安装](docs/deployment.md#node-deployment-package)。

### 5. 在 Node 主机执行安装

先在 Node 的 root Bash 会话准备依赖：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) deps install
```

将部署包安全传到 Node 的私有工作目录。以下以 ID 为 `3` 的 `tpnext-node-3.tar.gz` 为例，按实际下载文件名替换：

```bash
tar -xzf ./tpnext-node-3.tar.gz
```

在解压目录运行包内入口：

```bash
bash ./install-node.sh
```

入口自动准备公开 CA，按包内 `node.yaml` 安装；已有不同的信任文件时拒绝覆盖，也不会强制重建已有服务。安装后长期保留该 YAML，供[更新](docs/deployment.md#updates)和[卸载](docs/deployment.md#removal)使用。

### 6. 检查在线并创建代理

返回 **服务器管理** 检查 Node 在线，再在 **节点管理** 创建所需代理。Node 未安装时离线是正常状态。数据库、Redis、gRPC 和维护端口须按实际配置连通，详见[网络要求](docs/deployment.md#network)。

## 配置文件部署（备选）

### Web 配置下载与最少编辑

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) config web --output ./web.yaml
```

用 `nano` 或 `vi` 编辑，至少修改 `hostname`、`email`。首次安装可保留数据库和 Redis 空密码，安装器生成后写回 YAML，后续沿用实际密码。保留 `trojanpanelnext:` 层级、发布版本及镜像；模板权限为 `0600`，不覆盖已有文件。详见[Web 最少修改](docs/deployment.md#configuration-web-minimum)。

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) install --config ./web.yaml
```

### Node 配置下载与最少编辑

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) config node --output ./node.yaml
```

至少填写 Node 域名、Caddy 邮箱、Web 数据库 / Redis 地址和实际密码、`node_server_id`、`grpc_tls_server_name`。`node_server_id` 填服务器管理 **ID** 列的大于或等于 `1` 的整数，不填 IP、域名或代理 ID。

安装前准备 Web 公开 CA；已有证书模式填写 `node_certificate_mode: external` 和 Node 上已存在的 fullchain / 私钥绝对路径，无需邮箱。详见[Node 最少修改](docs/deployment.md#configuration-node-minimum)、[CA 准备](docs/deployment.md#configuration-node-ca)和[外部证书](docs/deployment.md#external-certificates)。

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) install --config ./node.yaml
```

校验命令、非默认端口和完整字段见[配置文件部署](docs/deployment.md#configuration)。交互式 `node` 入口仍可按提示生成配置，见[命令行 Node 安装](docs/deployment.md#node-interactive)。

## 更新产品镜像

在对应主机使用实际部署 YAML，显式选择目标版本：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) --version 1.0.2-rc.3 update --config ./web.yaml
```

Node 主机改用 `./node.yaml`。更新保留凭据、数据、端口、PKI 和已有运行配置，不升级数据库、Redis 或 Caddy；原 YAML 与运行配置不一致时拒绝更新。备份、短暂中断和失败处理见[镜像更新](docs/deployment.md#updates)。

## 卸载

### 卸载项目并保留数据

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) remove --config ./node.yaml --keep-data
```

### 彻底卸载项目与数据

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) remove --config ./node.yaml --purge-data
```

Web 主机改用 `./web.yaml`。外部证书、Nginx 和 Certbot 由宿主机独立管理，两种模式都保留这些资源。Web 页面中的 **卸载** 和 **彻底卸载** 需联系 Node；**删除** 只清理 Web 记录和关联数据，可用于失联服务器，但不会停止 Node 上的服务。见[完整移除范围](docs/deployment.md#web-removal)。

### 卸载新增依赖

先卸载项目和其他 Docker 服务，再清理本入口新增的依赖：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) deps remove
```

仅删除安装记录中的新增 Docker 软件包和本命令安装且未被修改的 yq，保留基础工具、原有依赖、Docker 数据及外部证书。依赖安装中断时先重跑 `deps install` 修复；拒绝卸载的条件见[依赖卸载](docs/deployment.md#dependency-removal)。

## 版本选择

同一个入口可通过版本号选择对应发布的脚本、配置模板和镜像：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) --version 1.0.2-rc.3 web
```

版本号 `1.0.2-rc.3` 对应 Git 标签 `v1.0.2-rc.3` 和产品镜像标签 `:1.0.2-rc.3`。安装与校验要求配置、脚本和产品镜像匹配；更新使用现有配置并绑定到目标版本。支持范围与版本规则见[版本绑定](docs/deployment.md#versions)。

## 使用文档

| 内容 | 入口 |
| --- | --- |
| 完整部署、配置、证书和卸载说明 | [部署指南](docs/deployment.md) |
| 当前版本文档索引 | [docs](docs/README.md) |
| 命令与脚本库 | [scripts](scripts/README.md) |
| 自动续签与内部 mTLS 信任 | [证书维护](docs/certificates.md) |

## 源码目录

| 目录 | 内容 |
| --- | --- |
| `apps/control-plane/api` | Web 主控 API |
| `apps/control-plane/web` | Web 管理界面 |
| `apps/node-agent` | Node Agent 与代理内核管理 |
| `apps/docs-site` | 文档站 |
| `scripts/deploy` | 按发布版本绑定的部署脚本与模板 |
| `tests` | 自动化测试 |
| `tools` | 仓库检查工具 |
| `docs` | 使用与架构文档 |

## 项目来源

本项目基于 [TrojanPanel](https://github.com/trojanpanel) 独立维护。来源说明见 [NOTICE.md](NOTICE.md)。
