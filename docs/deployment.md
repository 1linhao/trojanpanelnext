# TrojanPanel Next v1.0.2-rc.3 部署指南

简体中文 | [English](deployment_EN.md)

## 目录

- [系统与软件依赖](#dependencies)
  - [一键安装依赖](#dependency-install)
  - [手动准备依赖](#dependency-manual)
  - [卸载依赖](#dependency-removal)
- [版本绑定与命令入口](#versions)
- [域名、端口与网络](#network)
- [一键安装 Web](#web)
- [登记节点服务器](#node-registration)
- [安装 Node](#node)
  - [Web 部署包](#node-deployment-package)
  - [命令行交互安装](#node-interactive)
- [Node 使用已有证书](#external-certificates)
- [配置文件部署与字段](#configuration)
  - [下载并编辑模板](#configuration-download)
  - [Web 最少修改](#configuration-web-minimum)
  - [Node 最少修改](#configuration-node-minimum)
  - [准备 Node 公开 CA](#configuration-node-ca)
  - [校验并安装](#configuration-validation)
  - [配置字段参考](#configuration-fields)
- [重建当前版本服务](#recreate)
- [更新 Web 与 Node 镜像](#updates)
- [本地卸载](#removal)
- [从 Web 卸载或删除节点服务器](#web-removal)
- [移除后重新接入](#reconnect)
- [证书自动维护](#certificate-maintenance)
- [日常维护](#operations)
- [故障排查](#troubleshooting)

<a id="dependencies"></a>
## 系统与软件依赖

支持 Linux `amd64` 和 `arm64`。使用 Bash 执行命令，安装、更新、重建和卸载需要 root；Node 宿主机必须运行 systemd。建议每台服务器至少有 1 GiB 内存。Web 与 Node 可部署在不同服务器，通过受控网络连接。

`web`、`node` 和 `install` 部署命令只检查依赖。可先单独执行 `deps install` 一键安装所需软件，或手动准备：

| 操作 | 依赖 |
| --- | --- |
| 入口、帮助、模板下载 | Bash、curl、CA 证书、grep、coreutils |
| 自动依赖安装与卸载 | 上述入口依赖、util-linux（`flock`）、root、受支持的 Debian/Ubuntu、systemd、apt-get、dpkg、dpkg-query |
| 一键部署与配置校验 | 入口依赖及 **mikefarah/yq v4** |
| 安装、更新与重建 | 入口依赖、yq、运行中的 Docker Engine、OpenSSL、tar、findutils、awk；Node 还需 systemd |
| 卸载 | Bash、curl、CA 证书、grep、coreutils（含 realpath、rmdir）、Docker Engine、yq；Node 维护服务清理需要 systemctl |

<a id="dependency-install"></a>
### 一键安装依赖

支持 **Debian 12/13、Ubuntu 22.04/24.04**，CPU 架构为 `amd64` 或 `arm64`，需要 root 和运行中的 systemd。Web 与 Node 主机都可以使用；其他 Linux 发行版按[手动说明](#dependency-manual)准备。

在 root Bash 会话中执行：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) deps install
```

命令准备 Bash、curl、CA 证书、grep、coreutils、OpenSSL、tar、findutils、awk（缺少时安装 gawk），以及发行版提供的 Docker Engine 软件包（`docker.io`、`containerd`、`runc`；Debian 13 另需 `docker-cli`），并启动 Docker。缺少兼容 yq 时，下载对应架构的 **mikefarah/yq v4.53.6** 二进制，通过 SHA256 校验后安装。已有兼容的 Docker 和 mikefarah/yq v4 会复用，不替换原有工具。

入口本身需要 Bash、curl、CA 证书、grep 和 coreutils；`deps` 还需 util-linux 提供的 `flock` 用于防止并发操作。若主机缺少这些最小工具，先在 root Bash 中通过 apt 引导：

```bash
apt-get update
apt-get install -y bash curl ca-certificates grep coreutils util-linux
```

依赖命令与部署命令使用同一个版本入口，也可显式选择版本：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) --version 1.0.2-rc.3 deps install
```

安装记录保存在 `/var/lib/trojanpanelnext-dependencies`（目录权限 `0700`），记录本命令新增的 Docker 软件包以及所安装 yq 文件的 SHA256，供[依赖卸载](#dependency-removal)使用。重复执行会检查并补齐缺少的依赖。安装中断时先重跑 `deps install` 修复，再执行卸载。

<a id="dependency-manual"></a>
### 手动准备依赖

其他 Linux 发行版需通过自己的包管理器准备依赖。Debian/Ubuntu 也可手动安装系统工具并启动 Docker：

```bash
apt-get update
apt-get install -y bash curl ca-certificates grep coreutils openssl tar findutils gawk docker.io
systemctl enable --now docker
```

Debian 13 还需安装 `docker-cli`。也可按 [Docker Engine 官方说明](https://docs.docker.com/engine/install/)安装。按 [mikefarah/yq 官方安装说明](https://github.com/mikefarah/yq#install)获取与 CPU 架构对应的 v4 二进制并校验发布文件；Python 的同名 `yq` 不兼容。手动安装的软件不会纳入 `deps remove` 的删除范围。

部署前检查：

```bash
yq --version
docker info
openssl version
```

`yq --version` 应显示 mikefarah/yq v4；`docker info` 应能连接 Docker 服务。本指南的一键命令均在 root 的 Bash 会话运行，远程脚本从固定发布标签获取。

<a id="dependency-removal"></a>
### 卸载依赖

先在对应主机完成[项目卸载](#removal)，并移除其他 Docker 服务的容器；从 Web 卸载 Node 后还需等待宿主机维护服务清理完成；只删除 Web 记录时，Node 主机仍需自行本地卸载。然后执行：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) deps remove
```

| 对象 | 处理 |
| --- | --- |
| 安装记录中由 `deps install` 新增的 Docker 软件包 | 卸载；已有 Docker 容器（含停止容器）、共享 containerd 的其他 namespace 中仍有容器、Node 维护服务仍存在或运行状态无法确认时拒绝删除 Docker |
| 本命令安装的 yq | 仅在当前文件 SHA256 与记录一致时删除；文件被替换或修改时保留 |
| 原有 Docker、yq，以及 Bash、curl、CA、OpenSSL 等基础工具 | 保留 |
| `/var/lib/docker` 等 Docker 数据、保留的项目数据及 YAML | 保留 |
| 外部证书、Nginx、Certbot | 保留 |

无安装记录时不执行删除。依赖安装中断时先重跑 `deps install` 修复，再执行 `deps remove`。命令不运行 `apt autoremove` 或 Docker 全局 prune，也不删除其他来源安装的软件。卸载 Docker 软件包不会清除 Docker 数据；需要删除项目数据时，应在 Docker 可用时先执行 `remove --purge-data`。`deps remove` 需要与自动安装相同的受支持系统及入口最小依赖。

<a id="versions"></a>
## 版本绑定与命令入口

本指南对应预发布版本 **v1.0.2-rc.3（Pre-release）**。版本号 `1.0.2-rc.3` 对应 Git 标签 `v1.0.2-rc.3`、配置 `trojanpanelnext.release: "1.0.2-rc.3"`，以及以下产品镜像：

| 组件 | 镜像 |
| --- | --- |
| Web API | `ghcr.io/1linhao/trojanpanelnext-api:1.0.2-rc.3` |
| Web 界面 | `ghcr.io/1linhao/trojanpanelnext-web:1.0.2-rc.3` |
| Node Agent | `ghcr.io/1linhao/trojanpanelnext-node-agent:1.0.2-rc.3` |

所有产品镜像提供 `linux/amd64` 和 `linux/arm64`。Caddy、MariaDB、Redis 使用配置模板中的独立上游版本。

统一入口 `scripts/tp.sh` 按选定版本获取该标签下的脚本库及模板，检查脚本版本并执行对应命令；下载的临时脚本在命令结束后清理。安装与校验要求配置和产品镜像匹配所选版本；[更新命令](#updates)使用现有受支持配置，并将发布版本与产品镜像绑定到目标版本。脚本不推断其他配置规范的字段迁移，仅维护当前发布版本。

显式指定版本安装 Web：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) --version 1.0.2-rc.3 web
```

`--version <版本号>` 可放在命令前或后；`--entry-version` 显示入口的默认版本；`--help` 或 `<命令> --help` 查看用法。只传 `--version` 不用于查询版本。

| 命令 | 行为 |
| --- | --- |
| `deps install` | 一键准备支持系统上的依赖 |
| `deps remove` | 按安装记录清理新增依赖 |
| `web [选项]` | 按提示生成 Web YAML、校验并安装 |
| `node [选项]` | 按提示生成 Node YAML、校验并安装 |
| `config web\|node [--output <文件>]` | 下载对应版本配置模板 |
| `validate --config <文件>` | 校验 YAML、发布版本和字段 |
| `install --config <文件> [--force]` | 按 YAML 部署对应用途 |
| `--version <目标版本> update --config <文件>` | 使用现有配置更新 Web 或 Node 产品镜像 |
| `remove --config <文件> [--keep-data\|--purge-data]` | 按 YAML 卸载对应用途 |

一键部署读取终端输入，数据库和 Redis 密码不会作为命令行参数传入。`--config` 使用已有配置立即安装，不再提示填写。配置模板、生成配置均以 `0600` 权限保存；目标文件已存在或是符号链接时不会覆盖。`--config` 仅能与 `--force` 组合，其他值在 YAML 中设置。

| 一键部署选项 | 用途 |
| --- | --- |
| `--hostname <域名>` | 当前服务器域名；省略时提示输入 |
| `--email <邮箱>` | Web 或 Node Caddy 的 ACME 联系邮箱 |
| `--output <文件>` | 新配置路径，默认 `./web.yaml` 或 `./node.yaml` |
| `--config <文件>` | 使用已填写配置安装 |
| `--force` | 重建应用容器，保留业务数据 |
| `--web-host <地址>` | Node 的 Web 数据库和 Redis 地址；不同地址使用 YAML 设置 |
| `--node-id <ID>` | Web 服务器记录的整数 ID（≥ `1`），不是 IP / 域名或代理 ID |
| `--client-ca <文件>` | Node 使用的本机 Web 公开 CA 文件 |
| `--certificate-mode caddy\|external` | Node 证书模式，默认 `caddy` |
| `--certificate <文件>`、`--private-key <文件>` | Node 外部证书模式的 fullchain 和私钥路径 |

<a id="network"></a>
## 域名、端口与网络

为 Web 和 Node 分别准备域名，例如 `panel.example.com` 和 `node.example.com`，将 A/AAAA 记录解析到对应服务器。存在 AAAA 记录时，IPv6 也必须可达。自动签证模式要求验证端口可以从公网访问，并且没有其他服务占用。

| 方向 | 默认端口和用途 |
| --- | --- |
| 浏览器、证书签发服务 → Web | TCP 80、443：Caddy 和 HTTPS 面板 |
| Node → Web | TCP 9507：MariaDB；TCP 6378：Redis，仅允许受信任 Node 来源 |
| Node 宿主机维护服务 → Web | TCP 443：卸载结果确认 |
| 证书签发服务 → Node（Caddy 模式） | TCP 80：HTTP 域名验证 |
| Web → Node | TCP 8100：mTLS gRPC 控制，只允许 Web 来源 |
| Web → Node 宿主机维护服务 | TCP 8101：mTLS HTTPS 卸载，固定为 `grpc_port + 1`，只允许 Web 来源 |
| 访问 Node 伪装站 → Node（Caddy 模式） | TCP 8863：Caddy HTTPS |
| 代理客户端 → Node | 面板中实际配置的代理 TCP/UDP 端口 |

API 8081、UI 8888 和 Node API 8082 使用宿主机网络监听。按来源控制这些内部服务的访问，不要统一向公网放行。MariaDB、Redis 应通过防火墙或可信私网连接；配置不会自动建立 VPN 或防火墙规则。

更改端口时同步更新配置、面板登记和防火墙。维护端口不能独立配置，`grpc_port` 最大为 `65534`。外部证书模式不启动 Node Caddy，其 HTTP/HTTPS 端口不使用；代理自身仍需可用的监听端口。

<a id="web"></a>
## 一键安装 Web

在 Web 主机执行：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) web
```

按提示输入 Web 域名和证书邮箱。也可直接提供公开参数：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) web --hostname panel.example.com --email admin@example.com --output ./web.yaml
```

默认输出 `./web.yaml`。首次安装生成 MariaDB 与 Redis 密码并写入配置，创建 Web 内部 mTLS 身份，部署 MariaDB、Redis、API、UI 和 Web Caddy。

访问 `https://panel.example.com`。初始用户名为 `sysadmin`，密码为 `123456`，登录后及时修改。面板登录密码独立于 YAML 中的数据库、Redis 密码。现有部署的登录密码以账户设置为准。

备份 `web.yaml` 和完整 `pki_bundle_dir`。配置包含敏感凭据，应仅允许管理员读取，不要提交到公开仓库。

<a id="node-registration"></a>
## 登记节点服务器

### 1. 打开服务器管理

以系统管理员（`sysadmin` 角色）登录 Web，打开左侧 **服务器管理**，点击 **新增 Node 服务器**。也可访问 `https://panel.example.com/#/server-manage/server-list`，将示例域名替换为实际 Web 域名。只有 `sysadmin` 可新增服务器或生成含凭据的部署包。

这里登记的是整台 Node 宿主机。代理实例在 Node 安装完成后于节点管理中另行创建。

### 2. 填写服务器连接参数

| 字段 | 要求 |
| --- | --- |
| 服务器名称 | 2–20 个字符，用于识别主机 |
| Node 地址 | Web 可访问的 IP 或域名，例如 `203.0.113.10` 或 `node.example.com` |
| gRPC 端口 | 默认 `8100`，与 Node 配置及防火墙一致 |
| gRPC 证书域名 | Node 证书覆盖的域名，例如 `node.example.com`，与 `grpc_tls_server_name` 一致 |

### 3. 区分数字 ID 与服务器地址

保存后，Web 数据库生成 **大于或等于 `1` 的整数 ID**。列表的 **ID** 列与 **IP / 域名** 地址分别显示。例如 ID 为 `3`、地址为 `node.example.com` 时，Node 配置使用 `node_server_id: 3`。

服务器 ID 不是 IP、域名、名称，也不是该主机上某个代理实例的 ID。不要将地址填进 `node_server_id`，也不要沿用模板的示例 ID。

保存会自动打开 **部署 Node**；以后可通过对应服务器行的同名按钮重新打开。部署包会自动填写该服务器的数字 ID。Node 尚未安装时显示离线属于正常情况。

<a id="node"></a>
## 安装 Node

<a id="node-deployment-package"></a>
### 方式一：从 Web 下载部署包

先按[登记节点服务器](#node-registration)创建记录，再在 **部署 Node** 弹窗中完成以下操作。

#### 1. 确认连接与证书模式

Web 域名或 IP 必须可从 Node 访问，例如 `panel.example.com`，不含 `https://` 和路径；生成的配置使用 Web 的实际数据库和 Redis 端口及凭据。选择 **Caddy** 时填写证书邮箱；选择 **使用宿主机现有证书** 时填写 Node 主机上已存在的 fullchain 和私钥绝对路径。证书须覆盖 Node 域名和 gRPC 证书域名，外部工具负责续签。

Web 不生成 Node 外部 TLS 证书或私钥。路径与目录要求见[外部证书模式](#external-certificates)。安装前确认数据库、Redis、gRPC 和 `grpc_port + 1` 维护端口可按[网络规则](#network)访问。

#### 2. 下载并安全传输部署包

在 **部署参数** 确认后点击 **下一步**，进入 **下载与安装** 并点击 **下载部署包**。文件名为 `tpnext-node-<服务器 ID>.tar.gz`，包含以下文件：

| 文件 | 内容 |
| --- | --- |
| `node.yaml` | 当前发布的 Node 配置，填入真实数字服务器 ID、数据库 / Redis 连接和实际凭据 |
| `client-ca.crt` | Web 当前公开客户端 CA，用于 Node 验证 Web 管理连接 |
| `install-node.sh` | 准备公开 CA，再按原 YAML 调用对应版本安装入口 |
| `README.md` | 包内文件与安装使用说明 |

只有 `sysadmin` 能生成该含凭据包。部署包及 YAML 应以 `0600` 权限保存在管理员私有目录，只安全传给目标 Node，不提交公开仓库。包不包含 CA 私钥、Web 客户端私钥，也不包含 Node TLS 私钥。

#### 3. 在 Node 准备依赖

在 Node 的 root Bash 会话执行：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) deps install
```

支持范围与手动准备见[软件依赖](#dependencies)。

#### 4. 解压并运行包内入口

以下以服务器 ID 为 `3` 的 `tpnext-node-3.tar.gz` 为例，使用实际文件名替换。在 Node 的私有工作目录执行：

```bash
tar -xzf ./tpnext-node-3.tar.gz
```

在包含 `node.yaml` 和 `install-node.sh` 的解压目录执行：

```bash
bash ./install-node.sh
```

脚本将包内公开 CA 准备到配置指定的引导信任位置，再按包内 `node.yaml` 安装。现有不同 CA 信任文件会导致拒绝，不能通过部署包覆盖；信任轮换见[证书维护](certificates.md)。该入口不使用 `--force` 重建已有服务。

长期保留原部署 `node.yaml`，后续在同一主机用于[镜像更新](#updates)、[重建](#recreate)和[卸载](#removal)。返回 Web 检查服务器在线后，再创建代理实例。

<a id="node-interactive"></a>
### 方式二：命令行交互安装

先完成 Web 部署，按[登记节点服务器](#node-registration)创建服务器并记下 **ID** 列的大于或等于 `1` 的整数。此处不是 Node 地址或代理 ID。

将 Web 的当前公开 `client-ca.crt` 安全传输到 Node 主机，例如 `/root/client-ca.crt`。该文件默认位于 Web 的 `/tpdata/trojanpanelnext-pki/client-ca.crt`。仅传输公开 CA；`client-ca.key`、`client.key` 和 `client.crt` 留在 Web。

在 Node 主机执行：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) node
```

按提示输入 Node 域名、Web 地址、节点服务器 ID、公开 CA 路径，以及 Web 的数据库和 Redis 凭据；Caddy 模式还需输入证书邮箱。这些连接密码可由管理员读取 Web YAML 获取，敏感输入不显示在终端。

也可提供公开参数，剩余字段仍按提示填写：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) node --hostname node.example.com --email admin@example.com --web-host panel.example.com --node-id 1 --client-ca /root/client-ca.crt
```

`--client-ca` 指向已安全传输到本机的 PEM 公开 CA。文件只应包含未过期的 CA 证书，可包含轮换期间的新旧 CA；叶证书、过期 CA、私钥和其他混入内容会被拒绝，校验通过前不会保存新 YAML 或发布 CA 文件。脚本校验后将其复制到 `/tpdata/trojanpanelnext-pki/client-ca.crt`；该默认文件已存在时可省略此选项。同一份 CA 可重复使用，不会覆盖不同的现有信任文件；信任更新遵循[证书维护](certificates.md)流程。

默认输出 `./node.yaml`。Node 需要持续访问 Web 的 MariaDB、Redis 和 HTTPS；Web 需要访问 Node 的控制及维护端口。脚本不会自动登记节点服务器或从 Web 获取 CA。

安装会部署 Agent、默认证书模式下的 Node Caddy，以及宿主机 `trojanpanelnext-host.service`。该维护服务使用 mTLS HTTPS，仅接受获授权的 Web 客户端，用于远程卸载；Agent 容器不挂载 `docker.sock`。

完成后在 Web 检查服务器在线，再创建需要的 Xray、Hysteria2 或 NaiveProxy 代理。

<a id="external-certificates"></a>
## Node 使用已有证书

宿主机由 Nginx＋Certbot 或其他工具管理证书时，先签发有效证书，再使用外部模式：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) node --certificate-mode external
```

按提示输入 fullchain 和未加密 PEM 私钥的绝对路径。也可直接提供路径：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) node --certificate-mode external --certificate /etc/letsencrypt/live/node.example.com/fullchain.pem --private-key /etc/letsencrypt/live/node.example.com/privkey.pem
```

| 字段 | 含义 |
| --- | --- |
| `node_certificate_mode: caddy` | 默认方式，启动 Node Caddy 申请和续签证书 |
| `node_certificate_mode: external` | 读取现有 PEM，跳过 Node Caddy 与证书申请 |
| `node_certificate_path` | 完整证书链的宿主机绝对路径 |
| `node_private_key_path` | 匹配的未加密 PEM 私钥绝对路径 |

外部模式安装前检查文件可读、证书有效期、域名覆盖和私钥配对。Agent、宿主机维护服务和使用默认路径的代理读取这对证书；代理使用其他域名时，证书也必须覆盖这些域名，可使用多域名 SAN 或通配符证书。

证书所在目录和符号链接目标目录只读挂载，支持 Certbot 的 `live/<域名>` → `archive/<域名>` 文件链接更新。使用独立证书目录，并与可写的 mTLS PKI 分开。不能将外部证书放在项目数据、PKI、伪装站、内核运行或维护目录中；直接挂载 `/`、`/etc`、`/root` 等宽泛目录也会被拒绝。

证书申请、续签及 Nginx reload 由宿主机维护。可通过以下命令检查 Certbot webroot 续签链路：

```bash
certbot renew --cert-name node.example.com --dry-run --no-random-sleep-on-renew
```

续签的 pre/post hook 应与验证方式相符；使用 Nginx webroot 时保持 Nginx 运行。若 Nginx 读取该证书，应设置相应 deploy hook 检查配置并 reload Nginx。

一台主机上的多个 TLS 服务可由宿主机 Nginx stream 根据 SNI 分流到不同的本地 TCP 端口，各服务使用独立域名。Nginx stream 模块、监听端口、TLS 终止方式及 PROXY protocol 必须与后端匹配。脚本不自动设置 SNI 分流；TCP 和 UDP 是独立监听，SNI TCP 分流不能代替 UDP 代理的端口规划。

改变已安装 Node 的证书模式、路径或符号链接目标目录时，用 `--force` 重建挂载。对应原证书、私钥的代理配置引用会同步更新，代理账户与其他参数保留。两种本地卸载和 Web 远程卸载均保留外部证书、Nginx、Certbot 及其配置。

<a id="configuration"></a>
## 配置文件部署与字段

配置式部署按“下载模板 → 编辑最少字段 → 准备 Node CA → 校验 → 安装”的顺序执行。依赖与域名、端口准备仍按本指南完成。

<a id="configuration-download"></a>
### 下载并编辑配置模板

在 Web 主机下载 Web 模板：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) config web --output ./web.yaml
```

在 Node 主机下载 Node 模板：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) config node --output ./node.yaml
```

模板以 `0600` 权限保存；目标文件已存在或是符号链接时不会覆盖。已有部署请直接编辑原配置，首次部署需要另一个文件时使用新的 `--output` 路径。

使用已安装的编辑器，例如 `nano ./web.yaml`、`nano ./node.yaml`，或 `vi ./web.yaml`、`vi ./node.yaml`。下文 YAML 是要修改的字段片段，不能替换整个模板。保留 `trojanpanelnext:` 层级和两个空格的字段缩进；字符串使用引号，密码中的单引号在 YAML 单引号字符串内写成两个单引号，例如 `'pass''word'`。不要将配置提交到公开仓库。

<a id="configuration-web-minimum"></a>
### Web 至少修改哪些字段

使用默认端口和目录首次安装时，至少修改以下字段：

| 字段（位于 `trojanpanelnext` 内） | 必须填写的内容 |
| --- | --- |
| `hostname` | 指向 Web 主机的真实域名，如 `panel.example.com` |
| `email` | 接收证书通知的真实邮箱 |

编辑原模板中的对应值：

```yaml
trojanpanelnext:
  hostname: 'panel.example.com'
  email: 'admin@example.com'
```

保留模板的 `release`、`schema_version`、`purpose`、镜像、端口和路径字段。首次安装可将 `mariadb_password`、`redis_password` 保持为空，安装器会生成密码并写回 `web.yaml`。安装后备份该文件；已初始化的数据库和 Redis 密码不能仅通过改 YAML 更换，也不能清空后重新生成。Node 连接必须使用 Web 当前实际凭据。

Web 安装与登录后，先按[登记节点服务器](#node-registration)创建真实服务器记录，再填写 Node 配置。

<a id="configuration-node-minimum"></a>
### Node 至少修改哪些字段

在默认 Caddy 证书模式下，至少修改或确认以下字段；示例域名、占位密码和 ID `1` 不能直接当作真实配置：

| 字段（位于 `trojanpanelnext` 内） | 必须填写的内容 |
| --- | --- |
| `hostname` | 指向 Node 主机的真实域名 |
| `email` | Caddy 申请证书的真实联系邮箱 |
| `mariadb_host`、`redis_host` | Node 可连接的 Web 数据库与 Redis 地址；可以是同一域名或可信私网地址 |
| `mariadb_password`、`redis_password` | Web `web.yaml` 中当前实际密码，分别对应数据库和 Redis |
| `node_server_id` | [Web 登记](#node-registration)后 ID 列中的整数（≥ `1`），不是 IP / 域名或代理 ID |
| `grpc_tls_server_name` | Node 服务端证书覆盖的域名，与 Web 登记的 TLS 服务器名一致 |

编辑原模板中的对应值：

```yaml
trojanpanelnext:
  hostname: 'node.example.com'
  email: 'admin@example.com'
  mariadb_host: 'panel.example.com'
  mariadb_password: 'Web 当前实际数据库密码'
  redis_host: 'panel.example.com'
  redis_password: 'Web 当前实际 Redis 密码'
  node_server_id: 3
  grpc_tls_server_name: 'node.example.com'
```

示例 ID `3` 也必须替换成 Web 中本次登记的 ID。`mariadb_user`、`database`、`account_table` 保持默认，除非 Web 数据库实际采用其他设置。Web 数据库或 Redis 使用非默认端口时，同步修改 Node 的 `mariadb_port`、`redis_port`；修改 `grpc_port` 时，同步更新 Web 登记的 gRPC 端口和防火墙，宿主机维护端口固定为 `grpc_port + 1`。

**使用已有证书**时，在 Node 模板内另修改：

```yaml
trojanpanelnext:
  node_certificate_mode: 'external'
  node_certificate_path: '/etc/letsencrypt/live/node.example.com/fullchain.pem'
  node_private_key_path: '/etc/letsencrypt/live/node.example.com/privkey.pem'
```

这些绝对路径必须已经存在、可读，证书覆盖 `hostname` 与 `grpc_tls_server_name`，私钥与证书匹配且未加密。外部模式不需要填写 `email`，不会运行 Node Caddy；其他 Node 必填项和公开 CA 准备仍需完成。目录要求与续签说明见[外部证书模式](#external-certificates)。

<a id="configuration-node-ca"></a>
### 手工配置安装前准备 Web 公开 CA

在 Web 主机，从当前配置的 `pki_bundle_dir` 安全复制公开 `client-ca.crt` 到 Node。默认源文件为 `/tpdata/trojanpanelnext-pki/client-ca.crt`；例如在 Web 主机执行（替换 Node SSH 地址、用户和端口）：

```bash
scp /tpdata/trojanpanelnext-pki/client-ca.crt root@node.example.com:/root/client-ca.crt
```

仅复制公开 `client-ca.crt`，不要复制 `client-ca.key`、`client.key` 或 `client.crt`。然后在 Node 主机准备默认引导位置：

```bash
install -d -m 0700 /tpdata/trojanpanelnext-pki
install -m 0644 /root/client-ca.crt /tpdata/trojanpanelnext-pki/client-ca.crt
```

若 Node 使用自定义 `pki_bundle_dir`，目标必须是该目录下的 `client-ca.crt`。引导位置与 `grpc_client_ca_path` 不同：默认运行时文件为 `/tpdata/trojan-panel-core/pki/client-ca.crt`，安装器会从引导目录复制到这里，无需改模板的运行路径。已有部署会优先复用运行中的信任文件；CA 轮换请遵循[证书维护](certificates.md)，不要随意覆盖已有信任。

<a id="configuration-validation"></a>
### 校验并安装

在对应主机编辑和准备完成后，先校验。

Web 校验：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) validate --config ./web.yaml
```

Web 安装：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) install --config ./web.yaml
```

Node 校验：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) validate --config ./node.yaml
```

Node 安装：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) install --config ./node.yaml
```

也可使用 `web --config ./web.yaml`、`node --config ./node.yaml`，命令会检查配置用途。`validate` 仅校验 YAML、版本与字段，不验证 DNS、证书内容、网络连接或服务健康。安装完成后在[服务器管理](#node-registration)检查 Node 状态。

<a id="configuration-fields"></a>
### 配置字段参考

完整可编辑模板：[Web](../scripts/deploy/templates/web.yaml) · [Node](../scripts/deploy/templates/node.yaml)。所有字段位于 `trojanpanelnext` 映射内：

| 通用字段 | 说明 |
| --- | --- |
| `release` | 必填字符串，当前为 `"1.0.2-rc.3"`，必须匹配选定脚本版本 |
| `schema_version` | 配置结构版本，当前为 `1` |
| `purpose` | `web` 或 `node`，决定部署和卸载对象 |
| `hostname`、`email` | 当前服务器域名与证书联系邮箱；Node 外部模式不要求邮箱 |
| `pki_bundle_dir` | PKI 目录，默认 `/tpdata/trojanpanelnext-pki` |
| `caddy_image` | Caddy 镜像；Node 外部证书模式不启动 Caddy |
| `force` | `0` 或 `1`，是否重建应用容器；通常保持 `0` 并使用 `--force` |
| `purge_data` | `0` 或 `1`，无卸载数据参数时的默认模式；通常保持 `0` |

| Web 字段 | 说明 |
| --- | --- |
| `panel_image`、`ui_image` | 对应当前发布的 API、Web 镜像 |
| `mariadb_image`、`redis_image` | 数据库及缓存上游镜像 |
| `mariadb_port`、`redis_port` | 数据库及 Redis 宿主机端口，默认 `9507`、`6378` |
| `panel_port`、`ui_port` | API、UI 内部监听端口，默认 `8081`、`8888` |
| `mariadb_password`、`redis_password` | 首次安装可留空以生成；后续安装沿用现有凭据 |
| `grpc_client_cert_path`、`grpc_client_key_path` | Web mTLS 客户端身份的运行路径 |
| `grpc_server_ca_path` | Node 服务端证书的可选 CA 文件；留空使用系统信任 |

| Node 字段 | 说明 |
| --- | --- |
| `core_image` | 对应当前发布的 Node Agent 镜像 |
| `mariadb_host`、`mariadb_port`、`mariadb_user`、`mariadb_password` | Web 数据库连接，须与 Web 实际设置一致 |
| `database`、`account_table` | 数据库及账户表，默认 `trojan_panel_db`、`account` |
| `redis_host`、`redis_port`、`redis_password` | Web Redis 连接 |
| `node_server_id` | Web 服务器数据库记录的整数 ID（≥ `1`），不是 IP / 域名或代理 ID |
| `grpc_port`、`core_port` | Agent gRPC 与 API 端口，默认 `8100`、`8082` |
| `grpc_tls_mode` | 必须为 `mtls` |
| `grpc_tls_server_name` | Node 证书域名，需与 Web 登记一致 |
| `grpc_client_ca_path` | Node 运行时公开 CA 信任文件的绝对路径 |
| `kernel_runtime_path` | 代理内核运行目录，默认 `/tpdata/trojan-panel-core/runtime` |
| `node_certificate_mode`、`node_certificate_path`、`node_private_key_path` | [证书模式与路径](#external-certificates) |
| `node_caddy_http_port`、`node_caddy_https_port` | Caddy 模式端口，默认 `80`、`8863` |

Node 模板中的密码和服务器 ID 是待填写示例，不能直接作为实际部署凭据。使用配置式部署时，在 Node 的 `pki_bundle_dir` 准备 Web 当前公开 `client-ca.crt`。

<a id="recreate"></a>
## 重建当前版本服务

修改当前版本的域名、应用端口或证书挂载后，用已有配置重建相应主机：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) install --config ./node.yaml --force
```

Web 使用 `./web.yaml`。`--force` 会拉取配置指定的镜像并重建 API、UI、Agent 或 Caddy 容器，不重建已有 MariaDB、Redis 容器。调整已初始化的数据库密码需要同时正确更新数据库本身及所有连接方，不能仅改 YAML。

跨版本更新使用[镜像更新命令](#updates)和目标发布版本。安装、校验与重建使用匹配版本的配置和镜像；维护前备份数据与 PKI。

<a id="updates"></a>
## 更新 Web 与 Node 镜像

在对应主机的 root Bash 会话执行，使用**现有的实际部署 YAML**。`--version` 必须显式指定目标发布版本；同一入口会获取目标版本的更新脚本和产品镜像。

Web：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) --version 1.0.2-rc.3 update --config ./web.yaml
```

Node：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) --version 1.0.2-rc.3 update --config ./node.yaml
```

YAML 中的 `purpose` 决定更新对象。无需先手工替换 `release` 或镜像，也不要用空白模板覆盖已有配置。更新接受 `1.0` 及以上版本、`schema_version: 1` 且采用受支持容器布局的配置；仅升级或同版本修复，不提供自动降级，也不使用 `latest`。其他配置规范需按目标版本说明准备。

| 内容 | 更新行为 |
| --- | --- |
| Web 产品镜像 | 更新 API 和 Web UI |
| Node 产品镜像 | 更新 Node Agent，并更新配套宿主机维护服务 |
| 部署 YAML | 仅修改 `release` 和对应用途的官方产品镜像字段；凭据、服务器 ID、域名、端口、路径沿用原配置 |
| 数据、PKI、证书、代理运行配置 | 保留；不重新申请证书 |
| API / Agent `config.ini`、UI Nginx 配置 | 保留原文件内容，包括不属于 YAML 的自定义设置；关键连接、端口和身份须与原 YAML 一致 |
| MariaDB、Redis、Caddy | 保留已有容器与版本，不随产品更新升级 |
| 旧产品镜像 | 保留，便于恢复；不执行 Docker prune |

执行前先备份业务数据和 PKI，并确认原 YAML 与运行中的容器设置一致。原配置的 `release` 必须与正在运行的官方 GHCR 产品镜像标签一致；不一致时先恢复实际部署配置，不能用新模板代替。更新脚本检查容器环境、挂载、网络与端口后预拉取全部目标产品镜像；预拉取失败不停止旧容器，也不修改原配置。

切换前在原 YAML 旁生成权限为 `0600` 的备份，名称为 `<配置文件>.backup-<原版本>-<时间>-<随机>`。切换产品容器会短暂中断 Web 或 Node 服务。切换或健康检查失败时，脚本尝试恢复原配置、旧产品容器、运行配置及 Node 维护服务；恢复过程失败时保留恢复文件并输出目录，需按错误修复部署。该恢复只针对部署；新版本写入或迁移的业务数据库不会自动回滚，恢复数据库需使用升级前的数据备份。

Web 与 Node 分别在各自主机更新。命令不替换宿主机 Nginx、Certbot，也不更新 Xray / Hysteria2 的独立内核修订；内核升级在 Web 内核管理中操作。

<a id="removal"></a>
## 本地卸载

在对应主机执行。保留数据：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) remove --config ./node.yaml --keep-data
```

删除项目数据：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.3/scripts/tp.sh) remove --config ./node.yaml --purge-data
```

Web 使用 `./web.yaml`。`--keep-data` 和 `--purge-data` 不能同时使用；不传选项时使用 YAML `purge_data`，模板默认保留数据。

| 对象 | `--keep-data` | `--purge-data` |
| --- | --- | --- |
| 对应用途容器、匿名 volume | 删除 | 删除 |
| 对应镜像仓库中可清理的本地镜像 | 删除 | 删除 |
| 业务数据、PKI、伪装站、内核运行配置 | 保留 | 删除 |
| 原始部署 YAML | 保留 | 删除 |
| 安装的宿主机维护服务及工作副本 | 删除 | 删除 |
| 外部证书、宿主机 Nginx 和 Certbot | 保留 | 保留 |
| Docker Engine、管理员自行保存的入口脚本 | 保留 | 保留 |

项目卸载后，如需移除本入口安装的 Docker 和 yq，另执行 [`deps remove`](#dependency-removal)。

Web 卸载 MariaDB、Redis、API、UI、Web Caddy；Node 卸载 Agent 和 Caddy 模式的 Node Caddy。镜像清理覆盖配置及实际容器所用仓库的全部本地标签，被其他容器引用的共享镜像保留并提示。不执行全局 prune。

彻底卸载清理项目服务目录、`pki_bundle_dir`、伪装站、自定义 `kernel_runtime_path`、配置的身份文件和原始 YAML。维护服务目录为 `/etc/trojanpanelnext-host`、`/usr/local/lib/trojanpanelnext-host`，systemd 单元为 `/etc/systemd/system/trojanpanelnext-host.service`。同一主机仍运行另一用途的项目容器时，脚本会拒绝彻底卸载以保护共享数据。外部证书路径与清理范围重叠时也会拒绝操作。

本地卸载不删除 Web 的服务器登记；需要协调卸载和登记清理时，在 Web 选择“卸载”或“彻底卸载”。主机已自行清理或失联时，可仅“删除”Web 记录。

<a id="web-removal"></a>
## 从 Web 卸载或删除节点服务器

节点服务器页面的移除弹窗提供“取消”“删除”“卸载”“彻底卸载”：

| 操作 | Node 主机 | Web |
| --- | --- | --- |
| 取消 | 不执行 | 不执行 |
| 删除 | 不联系 Node，不停止或清理主机服务 | 清理该服务器及关联代理、协议配置、流量、任务和连接记录 |
| 卸载 | 保留数据模式卸载项目容器和可清理的镜像 | 删除服务器及关联代理配置，保留流量与任务历史 |
| 彻底卸载 | 删除项目数据模式卸载 | 清理该服务器及关联代理、协议配置、流量、任务和连接记录 |

**删除**不需要 Node 在线，可用于主机失联或不再管理的服务器。这只清理 Web 数据，Node 上仍运行的 Agent、代理、容器、镜像和宿主机维护服务不会停止或删除。需要清理主机时，在 Node 执行[本地卸载](#removal)。同一服务器已有卸载或删除请求正在处理时，会明确提示忙碌，请等待该操作结束后重试；其他服务器的失联卸载不会阻塞此服务器的纯 Web 删除。

**卸载**和**彻底卸载**先调用 Node 宿主机维护服务，收到成功结果后再清理 Web 登记。Node 离线、mTLS/网络连接失败或卸载失败时返回错误，保留服务器及关联代理配置；可排查连接后重试，或选择仅删除 Web 记录。

清理 Web 数据按服务器 ID 定向执行：共享任务保留其他服务器的任务项，空的该服务器专有任务会清理，指向该服务器的灰度目标会清除。账户全局流量累计、日排名、JWT、系统设置和共享缓存保留，因为这些数据属于跨服务器的账户或系统状态。删除单个代理节点仅删除该代理。

远程卸载会暂存 TLS 身份和卸载回执，待 Web 提交完成并确认结果后，再清理维护服务及文件。界面显示“清理待完成”（`cleanupPending`）时，主服务和 Web 记录已清理，维护文件仍等待确认。失败的确认和清理在后台重试，并能在重启后恢复；保持 Web 到 Node 维护端口及 Node 到 Web HTTPS 连通，直至完成。

<a id="reconnect"></a>
## 移除后重新接入

使用“卸载”后，Node 数据、PKI、证书和 YAML 保留，服务器与代理登记已移除。重新接入步骤：

1. 等待宿主机维护服务清理完成，在 Web 重新添加服务器，可复用名称和地址，取得新 ID。
2. 在保留的 `node.yaml` 更新 `node_server_id`，并核对数据库、Redis、TLS 设置。
3. 使用 `install --config ./node.yaml` 重新部署 Node。
4. 在 Web 检查服务器在线，并重新创建所需代理节点。

保留的历史仍属于原服务器 ID，不会自动迁移到新 ID。使用“彻底卸载”后，按首次安装步骤重新准备 YAML 和公开 CA；外部证书仍由宿主机保留和维护。

如果只使用“删除”，Node 主机可能仍在运行旧配置，Web 中的关联记录与服务器专属历史已清理。重新接入前先在主机保留数据卸载，再登记新 ID 并按上述步骤重新部署；不要继续沿用已删除的服务器 ID。

<a id="certificate-maintenance"></a>
## 证书自动维护

Caddy 模式下，Web 和 Node 的公网证书由各自 Caddy 申请和续签，需保留证书数据并保证 DNS、ACME 验证可达。Node 外部模式由宿主机工具申请、续签。

| 服务 | 更新证书后的加载行为 |
| --- | --- |
| Agent gRPC、宿主机维护服务、Hysteria2 | 新 TLS 握手读取证书文件 |
| NaiveProxy | 每分钟检查证书，验证后短暂重启受影响实例，保存用户及运行配置 |
| Xray | 文件证书默认每小时加载，可在面板重启该代理立即加载 |
| 宿主机 Nginx | 由管理员配置的续签 deploy hook 检查并 reload |

Web API 每 5 分钟检查内部 mTLS 身份，客户端证书剩余不足 90 天时重新签发。CA 剩余不足 365 天时开始分发新旧信任，在全部已登记 mTLS Node 确认后切换身份；旧身份至少保留 24 小时。离线 Node 会阻止轮换推进，连接恢复后重试。

首次接入需将当前公开 CA 安全传到 Node；[Web 部署包](#node-deployment-package)包含公开 CA 并由包内脚本准备，手工配置安装按[CA 准备](#configuration-node-ca)操作。备份完整 Web PKI 目录，包括 `state.json`、`generations` 和文件链接。Node 离线超过原信任有效期或从过期备份恢复时，需要重新引导信任。完整机制见[证书维护](certificates.md)。

<a id="operations"></a>
## 日常维护

主机数据默认位于 `/tpdata`，部署 YAML 保存在生成时选择的路径。备份应包含数据库、Redis、API/Agent 配置、代理运行配置及完整 PKI；保存外部证书时遵守对应证书工具的备份要求。数据库备份应使用一致性导出或在停止写入后执行。

查看容器与维护服务：

```bash
docker ps
docker logs --tail 100 trojan-panel
docker logs --tail 100 trojan-panel-core
systemctl status trojanpanelnext-host.service
journalctl -u trojanpanelnext-host.service -n 100 --no-pager
```

在 Web 主机查看 API 容器，在 Node 主机查看 Agent 和维护服务。公开日志前删除密码、访问令牌、私钥及用户连接信息。

<a id="troubleshooting"></a>
## 故障排查

| 现象 | 检查 |
| --- | --- |
| 依赖检查失败 | 支持系统先执行 `deps install`；其他系统手动准备。确认 mikefarah/yq v4、Docker 已启动，Node 运行 systemd |
| 依赖卸载拒绝删除 Docker | 检查全部 Docker 容器（`docker ps -a`，包括停止容器）、共享 containerd 的其他 namespace、Node 维护服务及运行状态，先完成项目和其他服务卸载；安装中断时先重跑 `deps install` 修复 |
| 版本或镜像不匹配 | 使用所选发布的模板，核对 `release` 与产品镜像标签 |
| Web 证书签发失败 | 检查 A/AAAA、80/443 可达与端口占用、Caddy 日志 |
| 登录请求超时 | 检查 API、MariaDB、Redis 容器与 API 日志，确认 Caddy 到 API 的连接；超时并不代表密码错误 |
| 登录凭据错误 | 初次部署使用初始账户；现有部署使用管理员已设置的密码 |
| Node 不在线 | 核对实际服务器 ID、Web 数据库/Redis连接、gRPC 防火墙、证书域名与公开 CA |
| 外部证书安装失败 | 检查绝对路径、完整链、未加密私钥、有效期、域名覆盖和独立目录 |
| 续签后仍显示原证书 | 按服务加载周期等待或重启相应代理，检查证书链接目标与 Nginx reload |
| Web 卸载服务器失败 | 检查 Node 维护服务、Web 到 `grpc_port + 1` 的 mTLS 连接和 Node 到 Web HTTPS 回调；失联且只需清 Web 数据时选“删除” |
| 卸载显示共享镜像保留 | 该镜像仍由其他容器使用，确认使用方后再自行处理 |

提交问题时提供所用发布版本、操作命令（移除敏感参数）、错误信息和已脱敏日志。不要上传实际部署 YAML、私钥、数据库密码或访问令牌。
