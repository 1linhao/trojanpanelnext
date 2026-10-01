# TrojanPanel Next

简体中文 | [English](README_EN.md)

TrojanPanel Next 是支持 Xray、Hysteria2 和 NaiveProxy 的多用户代理管理平台。Web 主控提供账户、节点服务器、代理、流量和任务管理；Node Agent 管理各服务器上的代理内核。

当前预发布版本：**v1.0.2-rc.2（Pre-release）**。支持 Linux amd64、arm64，通过 GHCR 镜像部署。

## 快速安装

在对应服务器的 **root Bash 会话**执行。Debian 12/13、Ubuntu 22.04/24.04 的 amd64/arm64 主机可先一键准备依赖（需要 systemd）：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) deps install
```

入口需要 Bash、curl、CA 证书、grep 和 coreutils；`deps` 还需要 util-linux 提供的 `flock`。缺少工具时先按[最小引导说明](docs/deployment.md#dependency-install)准备。其他 Linux 按[软件依赖](docs/deployment.md#dependency-manual)手动安装。准备域名解析和[网络访问](docs/deployment.md#network)后，脚本按提示生成配置并安装。

Web 主控：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) web
```

安装后访问配置的 HTTPS 域名，初始账户为 `sysadmin` / `123456`，登录后修改密码。

以系统管理员（`sysadmin` 角色）登录 Web，点击首页的 **新增 Node 服务器**，或左侧 **服务器管理 → 新增 Node 服务器**，填写 Node 地址、gRPC 端口（默认 `8100`）和 TLS 服务器名，保存后取得各行服务器名称下方标为“服务器 ID”的 ID。将 Web 的公开 `client-ca.crt` 安全复制到 Node 服务器，然后安装 Node；详见[登记与接入](docs/deployment.md#node-registration)：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) node
```

宿主机已有证书，由 Nginx、Certbot 或其他工具管理时：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) node --certificate-mode external
```

该模式按提示读取完整证书链及私钥路径，跳过 Node Caddy 和证书申请。域名、证书和共享端口的准备见[外部证书说明](docs/deployment.md#external-certificates)。

## 配置文件部署、更新与卸载

先在对应主机下载配置模板。

Web 主机：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) config web --output ./web.yaml
```

Node 主机：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) config node --output ./node.yaml
```

使用 `nano` 或 `vi` 编辑下载的文件，保留 `trojanpanelnext:` 层级、发布版本、镜像和其他默认字段；字符串与密码使用 YAML 引号。模板以 `0600` 保存，不覆盖已有文件。

| 配置 | 至少修改或准备 |
| --- | --- |
| Web | `hostname`、`email`。首次安装可保留数据库和 Redis 空密码，安装后生成并写回 YAML，后续沿用实际密码。详见[Web 最少修改](docs/deployment.md#configuration-web-minimum) |
| Node | `hostname`、`email`（Caddy 模式）、`mariadb_host` / `mariadb_password`、`redis_host` / `redis_password`、真实 `node_server_id`、`grpc_tls_server_name`；连接信息须匹配 Web。详见[Node 最少修改](docs/deployment.md#configuration-node-minimum) |
| Node 公开 CA | 安装前将 Web 公开 CA 放在 Node 的 `/tpdata/trojanpanelnext-pki/client-ca.crt`，自定义目录使用 `pki_bundle_dir/client-ca.crt`。见[CA 准备](docs/deployment.md#configuration-node-ca) |
| Node 已有证书 | 改 `node_certificate_mode: external`，填写已存在的 fullchain / 私钥绝对路径；无需 `email`。见[外部证书](docs/deployment.md#external-certificates) |

Web 校验：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) validate --config ./web.yaml
```

Web 安装：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) install --config ./web.yaml
```

Node 校验：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) validate --config ./node.yaml
```

Node 安装：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) install --config ./node.yaml
```

完整编辑示例、非默认端口与字段说明见[配置文件部署](docs/deployment.md#configuration)。

在对应主机更新 Web 产品镜像：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) --version 1.0.2-rc.2 update --config ./web.yaml
```

Node 主机将配置路径改为 `./node.yaml`。保留实际部署 YAML，更新沿用凭据、数据、端口和 PKI；数据库、Redis 与 Caddy 不随该命令升级。目标版本、备份和失败处理见[镜像更新](docs/deployment.md#updates)。

仅卸载容器与可清理的镜像，保留业务数据：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) remove --config ./node.yaml --keep-data
```

彻底卸载，包括项目数据、PKI、代理运行配置和部署 YAML：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) remove --config ./node.yaml --purge-data
```

替换配置路径即可操作对应主机上的 Web 或 Node。外部证书、Nginx 和 Certbot 由宿主机独立管理，两种卸载模式都保留这些外部资源。Web 的“卸载”和“彻底卸载”需联系 Node；“删除”只清理 Web 记录和关联数据，可用于失联服务器，但不会停止 Node 上的服务。范围见[服务器移除](docs/deployment.md#web-removal)。

卸载项目和其他 Docker 服务后，可清理本入口新增的依赖：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) deps remove
```

仅删除安装记录中的新增 Docker 软件包和本命令安装且未被修改的 yq；保留基础工具、原有依赖、Docker 数据及外部证书。存在任何 Docker 容器（含停止容器）、共享 containerd 中其他 namespace 的容器、Node 维护服务或无法确认运行状态时，拒绝删除 Docker。依赖安装中断时先重跑 `deps install` 修复，再卸载。见[依赖卸载](docs/deployment.md#dependency-removal)。

## 版本选择

同一个入口可通过版本号选择对应发布的脚本、配置模板和镜像：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) --version 1.0.2-rc.2 web
```

版本号 `1.0.2-rc.2` 对应 Git 标签 `v1.0.2-rc.2` 和产品镜像标签 `:1.0.2-rc.2`。安装与校验要求配置、脚本和产品镜像匹配；更新使用现有配置并绑定到目标版本。支持范围与版本规则见[版本绑定](docs/deployment.md#versions)。

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
