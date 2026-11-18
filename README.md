# TrojanPanel Next

简体中文 | [English](README_EN.md)

TrojanPanel Next 是支持 Xray、Hysteria2 和 NaiveProxy 的多用户代理管理平台。Web 主控提供账户、节点服务器、代理、流量和任务管理；Node Agent 管理各服务器上的代理内核。

当前预发布版本：**v1.0.2-rc.13（Pre-release）**。支持 Linux amd64、arm64，通过 GHCR 镜像部署。

## 使用入口

- [快速安装：双机分开或单机一体](#quick-install)
- [配置文件部署（备选）](#configuration-alternative)
- [更新产品镜像](#product-update)
- [卸载项目与依赖](#project-removal)
- [完整部署指南](docs/deployment.md)

<a id="quick-install"></a>
## 快速安装

| 你准备了什么 | 选哪种 |
| --- | --- |
| 两台 VPS，分别运行 Web 和 Node，两个域名分别解析到各自 VPS | [方式 A · 双机分开](#quick-install-separate) |
| 一台 VPS 同时运行 Web 和 Node，两个域名解析到同一台 VPS，且 Node 域名已有宿主机证书 | [方式 B · 单机一体](#quick-install-colocated) |

两种方式都先安装 Web，然后从面板下载 Node 部署包。示例域名 `panel.example.com` 和 `node.example.com` 请换成自己的域名。下方命令用于全新安装，请在运行 systemd 的 Debian 12/13 或 Ubuntu 22.04/24.04（amd64/arm64）上，用 **root Bash** 执行。其他 Linux 可按完整指南安装。 [手动依赖](docs/deployment.md#dependency-manual)

### 先安装 Web

在 **Web VPS** 上将面板域名解析到公网 IP，开放 TCP `80`、`443`，然后粘贴整段命令。登录普通 SSH 账号时，先执行 `sudo -i` 进入 root 会话。

```bash
apt-get update &&
apt-get install -y bash curl ca-certificates grep coreutils util-linux tar gzip &&
install -d -m 700 /root/tpnext-web &&
cd /root/tpnext-web &&
umask 077 &&
curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.13/scripts/tp.sh -o ./tp.sh &&
bash ./tp.sh deps install &&
bash ./tp.sh web
```

安装器会提示填写面板域名和证书联系邮箱。随后打开对应的 HTTPS 地址，以 `sysadmin` / `123456` 登录并修改密码。保留 `/root/tpnext-web/web.yaml`，后续更新和卸载会用到。

上传 Node 部署包前，在运行 Node 的 VPS 上用 SSH 登录账号执行 `install -d -m 700 "$HOME/tpnext-node"`。用 SFTP 将文件上传到该目录，记下绝对路径。切换到 root Bash 后，用 `cd` 进入该目录。每台 Node 的部署包使用单独的解压目录。

<a id="quick-install-separate"></a>
### 方式 A · 双机分开

1. 把 Node 域名解析到 **Node VPS** 的公网 IP。Node 使用 Caddy 自动签证时，开放 Node 的公网 TCP `80`。Web 的 TCP `9507`、`6378` 仅允许 Node 访问，Node 的 TCP `8100`、`8101` 仅允许 Web 访问。在 VPS 防火墙或安全组里按来源配置。 [网络要求](docs/deployment.md#network)
2. 在面板打开 **服务器管理 → 新增 Node 服务器**。名称自定，**IP / 域名**填 Web 能访问的 Node 地址，**gRPC 端口**填 `8100`，**gRPC 证书域名**填 Node 域名。
3. 保存后进入 **部署 Node**。证书选 **Caddy 自动签发与续签** 并填邮箱，**Node 可连接的 Web 地址**填 Node 能访问的 Web 域名或私网 IP。确认数据库和 Redis 地址，点击 **下一步 → 下载部署包**。
4. 用 SSH 客户端的 SFTP 功能把下载的 `tpnext-node-*.tar.gz` 上传到 **Node VPS** 的私有目录。进入该目录并用 root Bash 粘贴下面整段，按提示输入实际文件名。

```bash
(
  set -e
  umask 077
  read -r -p '请输入已上传的部署包文件名 > ' bundle
  [[ "$bundle" == tpnext-node-*.tar.gz && -f "$bundle" ]] ||
    { printf '%s\n' '未找到部署包，请检查当前目录和文件名。' >&2; exit 1; }
  chmod 600 -- "$bundle"
  apt-get update
  apt-get install -y bash curl ca-certificates grep coreutils util-linux tar gzip
  tar -xzf "$bundle"
  bash ./tpnext/install-dependencies.sh
  bash ./tpnext/install-node.sh
)
```

<a id="quick-install-colocated"></a>
### 方式 B · 单机一体，也可再接外部 Node

Web 和 Node 都运行在 **同一台 VPS**。Node 使用已存在且可续期的证书，避免两套 Caddy 同时占用 TCP `80`。提前把 Node 域名的完整证书链和匹配的未加密私钥放在宿主机专用目录，例如由 Certbot 管理的 `/etc/letsencrypt/live/node.example.com/`。续签由宿主机证书工具负责。 [Node 外部证书](docs/deployment.md#external-certificates)

1. 在面板打开 **服务器管理 → 新增 Node 服务器**。名称可填「本机 Node」，**IP / 域名**填 `127.0.0.1`，**gRPC 端口**填 `8100`，**gRPC 证书域名**填证书覆盖的 Node 域名。
2. 在 **部署 Node** 中选 **使用宿主机现有证书**，填当前 VPS 上的完整证书链和私钥的绝对路径。**Node 可连接的 Web 地址**保留面板域名，点 **下一步 → 下载部署包**。下面的命令会把包内数据库和 Redis 地址改为回环地址，并为 Node 设置单独的 CA 信任目录。
3. 把部署包上传回这台 **Web VPS** 的另一个私有目录。在该目录用 root Bash 粘贴整段命令，按提示输入实际文件名。Web 安装步骤已经准备好 Docker 和 yq。

```bash
(
  set -e
  umask 077
  read -r -p '请输入已上传的部署包文件名 > ' bundle
  [[ "$bundle" == tpnext-node-*.tar.gz && -f "$bundle" ]] ||
    { printf '%s\n' '未找到部署包，请检查当前目录和文件名。' >&2; exit 1; }
  chmod 600 -- "$bundle"
  tar -xzf "$bundle"
  yq -i '
    .trojanpanelnext.mariadb_host = "127.0.0.1" |
    .trojanpanelnext.redis_host = "127.0.0.1" |
    .trojanpanelnext.pki_bundle_dir = "/tpdata/trojan-panel-core/pki-bootstrap"
  ' ./tpnext/node.yaml
  chmod 600 -- ./tpnext/node.yaml
  bash ./tpnext/install-node.sh
)
```

以后接入其他 VPS 时，继续在同一个面板新增服务器，为每台远程 Node 单独执行[方式 A](#quick-install-separate)。按需把 Web 的数据库和 Redis 端口仅开放给这些远程 Node。同机的 `8100`、`8101` 保持仅本机可访问。

### 验证安装

在 **服务器管理** 确认 Node 在线，再进入 **节点管理** 创建代理。代理域名填客户端能访问的 **Node 公网域名**，按代理协议开放对应的 TCP 或 UDP 端口，然后导出订阅或连接信息，在客户端连接验证。 [代理端口与转发](docs/port-forwarding.md)

部署包会自动带入服务器 ID、数据库与 Redis 凭据和 Web 公开 CA。将部署包和 `tpnext/node.yaml` 保存在管理员私有目录并保持 `0600` 权限。保留原 YAML 用于[更新](docs/deployment.md#updates)和[卸载](docs/deployment.md#removal)。

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

Node 可由系统管理员打开 **版本管理 → TPNext 容器管理**，选择服务器并更新到当前 Web 对应的发布版本。更新结果保存在 Node 宿主机，重新打开页面可继续查询；旧版 Node 需先按下方命令升级维护服务。网页更新要求 Web 能通过 mTLS 访问 Node 的维护端口（`grpc_port + 1`）。

Web 镜像以及无法远程更新的 Node，在对应主机使用实际部署 YAML，显式选择目标版本：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.13/scripts/tp.sh) --version 1.0.2-rc.13 update --config ./web.yaml
```

资源包部署的 Node 主机改用 `./tpnext/node.yaml`；手工配置使用原 YAML 路径。更新保留凭据、数据、端口、PKI 和已有运行配置，不升级数据库、Redis 或 Caddy；原 YAML 与运行配置不一致时拒绝更新。备份、短暂中断和失败处理见[镜像更新](docs/deployment.md#updates)。

<a id="project-removal"></a>
## 卸载

### 卸载项目并保留数据

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.13/scripts/tp.sh) remove --config ./tpnext/node.yaml --keep-data
```

### 彻底卸载项目与数据

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.13/scripts/tp.sh) remove --config ./tpnext/node.yaml --purge-data
```

Web 主机改用 `./web.yaml`。外部证书、Nginx 和 Certbot 由宿主机独立管理，两种模式都保留这些资源。Web 页面中的 **卸载** 和 **彻底卸载** 需联系 Node；**删除** 只清理 Web 记录和关联数据，可用于失联服务器，但不会停止 Node 上的服务。见[完整移除范围](docs/deployment.md#web-removal)。

### 卸载新增依赖

先卸载项目和其他 Docker 服务，再清理本入口新增的依赖：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.13/scripts/tp.sh) deps remove
```

仅删除安装记录中的新增 Docker 软件包和本命令安装且未被修改的 yq，保留基础工具、原有依赖、Docker 数据及外部证书。依赖安装中断时先重跑 `deps install` 修复；拒绝卸载的条件见[依赖卸载](docs/deployment.md#dependency-removal)。

## 版本选择

同一个入口可通过版本号选择对应发布的脚本、配置模板和镜像：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.13/scripts/tp.sh) --version 1.0.2-rc.13 web
```

版本号 `1.0.2-rc.13` 对应 Git 标签 `v1.0.2-rc.13` 和产品镜像标签 `:1.0.2-rc.13`。安装与校验要求配置、脚本和产品镜像匹配；更新使用现有配置并绑定到目标版本。支持范围与版本规则见[版本绑定](docs/deployment.md#versions)。

## 使用文档

| 内容 | 入口 |
| --- | --- |
| 完整部署、配置、证书和卸载说明 | [部署指南](docs/deployment.md) |
| 代理对外端口、Nginx SNI 与 UDP 转发 | [端口转发](docs/port-forwarding.md) |
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
