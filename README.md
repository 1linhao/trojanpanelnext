# TrojanPanel Next

简体中文 | [English](README_EN.md)

TrojanPanel Next 是支持 Xray、Hysteria2 和 NaiveProxy 的多用户代理管理平台。Web 主控提供账户、节点服务器、代理、流量和任务管理；Node Agent 管理各服务器上的代理内核。

当前预发布版本：**v1.0.2-rc.5（Pre-release）**。支持 Linux amd64、arm64，通过 GHCR 镜像部署。

## 使用入口

- [快速安装：Web → 下载 Node 资源包 → 安装 Node](#quick-install)
- [配置文件部署（备选）](#configuration-alternative)
- [更新产品镜像](#product-update)
- [卸载项目与依赖](#project-removal)
- [完整部署指南](docs/deployment.md)

<a id="quick-install"></a>
## 快速安装

按以下顺序部署：**准备 Web → 登录管理页面 → 登记服务器并下载 Node 部署资源包 → 在 Node 安装 → 创建代理**。

### 1. 在 Web 主机准备依赖

在 **root Bash 会话**执行。自动依赖安装支持 Debian 12/13、Ubuntu 22.04/24.04 的 amd64/arm64 主机，需要运行 systemd。

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.5/scripts/tp.sh) deps install
```

入口需要 Bash、curl、CA 证书、grep 和 coreutils；`deps` 还需要 util-linux 提供的 `flock`。缺少工具时先按[最小引导说明](docs/deployment.md#dependency-install)准备，其他 Linux 按[软件依赖](docs/deployment.md#dependency-manual)手动安装。准备 Web 域名解析和[网络访问](docs/deployment.md#network)。

### 2. 安装 Web 并登录

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.5/scripts/tp.sh) web
```

按提示填写 Web 域名和证书邮箱。完成后访问该 HTTPS 域名，使用初始账户 `sysadmin` / `123456` 登录并修改密码。

### 3. 在服务器管理登记 Node

以系统管理员（`sysadmin` 角色）打开左侧 **服务器管理**，点击 **新增 Node 服务器**，填写服务器名称、Web 可连接的 Node IP / 域名、gRPC 端口（默认 `8100`）和被 Node 证书覆盖的 gRPC 证书域名。流量限制可保持默认。

保存后，Web 自动生成 **大于或等于 `1` 的数值服务器 ID**，并打开 **部署 Node** 引导。ID 与 IP / 域名是两个字段；资源包自动使用正确 ID。已有服务器可点击该行的 **部署 Node** 图标重新打开引导，详见[登记节点服务器](docs/deployment.md#node-registration)。

### 4. 选择证书方式并下载 Node 资源包

在 **部署参数** 中选择证书方式：

- **Caddy 自动签发与续签**：填写证书联系邮箱，并为 Node 证书域名准备解析和 ACME 验证端口。
- **使用宿主机现有证书**：填写目标 Node 上已有完整证书链（fullchain）及匹配的未加密私钥绝对路径，由宿主机工具续签；此模式不要求邮箱。

确认预填的 **Node 可连接的 Web 地址**，必要时调整为 Node 可访问的域名或 IP，不含协议、端口或路径。同时检查显示的数据库和 Redis 地址。点击 **下一步**，在 **下载与安装** 中点击 **下载部署包**。

下载文件名为 `tpnext-node-<服务器 ID>.tar.gz`，包含：

| 文件 | 用途 |
| --- | --- |
| `node.yaml` | 已填真实服务器 ID、Web 数据库 / Redis 连接参数与凭据的 Node 配置 |
| `client-ca.crt` | Web 当前公开客户端 CA |
| `install-node.sh` | 准备公开 CA 并执行匹配版本的安装 |
| `README.md` | 资源包使用说明 |

按此流程无需手工编辑 ID、数据库密码或复制 CA 文件。外部证书和私钥仍需在 Node 主机预先准备。资源包和 YAML 含敏感凭据，应保存为 **`0600`**，仅安全传输到目标 Node，不提交公开仓库；包不含 Web 私钥或 Node TLS 私钥。完整说明见[资源包安装](docs/deployment.md#node-deployment-package)。

### 5. 将资源包传到 Node 并安装

先在 Node 主机的 **root Bash 会话**准备依赖：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.5/scripts/tp.sh) deps install
```

将资源包安全传到 Node 的私有工作目录。以下以服务器 ID `3` 的文件为例，按实际下载名称替换，在该目录解包：

```bash
tar -xzf ./tpnext-node-3.tar.gz
```

资源包中的四个文件直接解压到当前目录。在同一目录运行：

```bash
bash ./install-node.sh
```

入口会验证版本和配置、准备公开 CA，再按 `node.yaml` 安装。不同的现有 CA 信任不会被覆盖，已有服务也不会被强制重建。长期保留该 YAML，供[更新](docs/deployment.md#updates)和[卸载](docs/deployment.md#removal)使用。

### 6. 返回 Web 创建代理节点

在 **服务器管理** 检查 Node 在线，再进入 **节点管理** 创建所需代理。Node 安装前显示离线是正常状态。数据库、Redis、gRPC 和维护端口须按配置连通，详见[网络要求](docs/deployment.md#network)。

<a id="configuration-alternative"></a>
## 配置文件部署（备选）

需要自行编辑 YAML 或进行自动化部署时，使用详细指南中的补充流程：

| 步骤 | 说明 |
| --- | --- |
| 下载模板 | [配置文件下载](docs/deployment.md#configuration-download) |
| 设置 Web | [Web 最少编辑项](docs/deployment.md#configuration-web-minimum) |
| 设置 Node | [Node 最少编辑项](docs/deployment.md#configuration-node-minimum) |
| 准备公开 CA | [手工配置的 CA 准备](docs/deployment.md#configuration-node-ca) |
| 校验与安装 | [配置文件部署](docs/deployment.md#configuration) |

[命令行 Node 引导](docs/deployment.md#node-interactive)和[外部证书配置](docs/deployment.md#external-certificates)也保留为补充路径。

<a id="product-update"></a>
## 更新产品镜像

在对应主机使用实际部署 YAML，显式选择目标版本：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.5/scripts/tp.sh) --version 1.0.2-rc.5 update --config ./web.yaml
```

Node 主机改用 `./node.yaml`。更新保留凭据、数据、端口、PKI 和已有运行配置，不升级数据库、Redis 或 Caddy；原 YAML 与运行配置不一致时拒绝更新。备份、短暂中断和失败处理见[镜像更新](docs/deployment.md#updates)。

<a id="project-removal"></a>
## 卸载

### 卸载项目并保留数据

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.5/scripts/tp.sh) remove --config ./node.yaml --keep-data
```

### 彻底卸载项目与数据

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.5/scripts/tp.sh) remove --config ./node.yaml --purge-data
```

Web 主机改用 `./web.yaml`。外部证书、Nginx 和 Certbot 由宿主机独立管理，两种模式都保留这些资源。Web 页面中的 **卸载** 和 **彻底卸载** 需联系 Node；**删除** 只清理 Web 记录和关联数据，可用于失联服务器，但不会停止 Node 上的服务。见[完整移除范围](docs/deployment.md#web-removal)。

### 卸载新增依赖

先卸载项目和其他 Docker 服务，再清理本入口新增的依赖：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.5/scripts/tp.sh) deps remove
```

仅删除安装记录中的新增 Docker 软件包和本命令安装且未被修改的 yq，保留基础工具、原有依赖、Docker 数据及外部证书。依赖安装中断时先重跑 `deps install` 修复；拒绝卸载的条件见[依赖卸载](docs/deployment.md#dependency-removal)。

## 版本选择

同一个入口可通过版本号选择对应发布的脚本、配置模板和镜像：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.5/scripts/tp.sh) --version 1.0.2-rc.5 web
```

版本号 `1.0.2-rc.5` 对应 Git 标签 `v1.0.2-rc.5` 和产品镜像标签 `:1.0.2-rc.5`。安装与校验要求配置、脚本和产品镜像匹配；更新使用现有配置并绑定到目标版本。支持范围与版本规则见[版本绑定](docs/deployment.md#versions)。

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
