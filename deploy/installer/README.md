# TrojanPanel Next 安装器

简体中文 | [English](README_EN.md)

使用独立下载的 `install.sh` 获取配置模板，编辑 YAML 后执行安装。`trojanpanelnext.purpose` 是部署类型的唯一来源：`web` 部署主控，`node` 部署节点 Agent。所有命令均无交互。

## 系统与软件依赖

支持 Ubuntu 20.04+、Debian 11+ 或同类 Linux，CPU 为 `linux/amd64` 或 `linux/arm64`，建议至少 1 GiB 内存。安装和卸载需要 root。

**安装器不会自动安装 yq、Docker 或其他软件。请先安装依赖，再执行安装命令。**

| 命令 | 预装依赖 |
| --- | --- |
| `config` | curl、coreutils（包含 mktemp、chmod、ln、dirname、rm），目标目录可写 |
| `validate` | [mikefarah/yq v4](https://github.com/mikefarah/yq)，与 Python 的同名 yq 不兼容 |
| `install` | Docker Engine、mikefarah/yq v4、curl、OpenSSL、tar、coreutils、findutils、awk |
| `remove` | Docker Engine、mikefarah/yq v4 |

Debian/Ubuntu 可先执行：

```bash
sudo apt-get update
sudo apt-get install -y curl ca-certificates openssl tar coreutils findutils gawk docker.io
sudo systemctl enable --now docker
```

然后手工安装经过 SHA-256 校验的 yq v4.53.6：

```bash
case "$(uname -m)" in
  x86_64 | amd64)
    yq_arch=amd64
    yq_sha=c5f056448f973ae7d39b5401949648a78f2dc1947d6a8eb65be60d5c504b9385
    ;;
  aarch64 | arm64)
    yq_arch=arm64
    yq_sha=88a1016bc1d657375a35864e4f44b6f333df8ff97b559f51bba0adcb2169df09
    ;;
  *) echo "Unsupported architecture" >&2; exit 1 ;;
esac
yq_download="$(mktemp)"
curl -fsSL --connect-timeout 10 --max-time 120 \
  "https://github.com/mikefarah/yq/releases/download/v4.53.6/yq_linux_${yq_arch}" \
  -o "$yq_download"
printf '%s  %s\n' "$yq_sha" "$yq_download" | sha256sum -c - && \
  sudo install -m 0755 "$yq_download" /usr/local/bin/yq
rm -f -- "$yq_download"
yq --version
sudo docker info >/dev/null
```

以 root 登录时可省略 `sudo`。其他发行版请使用对应包管理器安装依赖。Docker 服务必须已启动。

## 版本绑定与获取安装器

当前安装器版本为 `0.1.0-rc.4`。每台 Web 或 Node 服务器都执行：

```bash
curl -fsSL --connect-timeout 10 --max-time 60 \
  https://raw.githubusercontent.com/1linhao/trojanpanelnext/v0.1.0-rc.4/deploy/installer/install.sh \
  -o install.sh
chmod +x install.sh
./install.sh --version
```

安装器版本 `0.1.0-rc.4` 默认从 GitHub Raw 的 `v0.1.0-rc.4` 获取模板；模板中的 API、Web 和 Node Agent 镜像使用 `:0.1.0-rc.4`。Git 标签带 `v`，镜像标签不带 `v`。模板只通过远程下载获取，无需本地仓库。

`schema_version: 1` 表示 YAML 配置结构版本，与产品版本分别维护。此次 CLI 直接移除了旧部署类型参数，已有配置只要采用 `purpose` 字段即可使用新命令；使用旧参数会失败。

## 网络准备

域名的 A/AAAA 记录应解析到各自服务器，所用端口不能被其他服务占用。

| 方向 | 默认端口及用途 |
| --- | --- |
| 浏览器、证书签发服务 → Web | TCP 80、443：Caddy 与 HTTPS 面板 |
| Node → Web | TCP 9507：MariaDB；TCP 6378：Redis，只允许受信任 Node 来源 |
| 证书签发服务 → Node | TCP 80：默认 HTTP 域名验证 |
| Web → Node | TCP 8100：mTLS gRPC 控制通道 |
| 访问节点伪装站 → Node | TCP 8863：默认 Caddy HTTPS 端口 |
| 代理客户端 → Node | 面板中实际配置的代理 TCP/UDP 端口 |

API 8081、UI 8888、Node API 8082 使用主机网络监听，不应统一对公网放行。更改端口时同步调整防火墙和 YAML；AAAA 记录存在时也必须保证 IPv6 可达。

## 安装 Web 主控

```bash
./install.sh config web
nano web.yaml
./install.sh validate --config ./web.yaml
sudo ./install.sh install --config ./web.yaml
```

编辑 `hostname` 和 `email`。`config` 默认生成 `web.yaml`，权限为 `0600`；已有目标文件或符号链接会直接拒绝，下载中断不会留下半截目标配置。也可使用 `--output ./production-web.yaml`。

首次安装会生成 MariaDB 与 Redis 密码，写回 `web.yaml`，并在 `pki_bundle_dir` 生成主控 mTLS 身份。请保留配置和 PKI，实际配置含敏感凭据，不要提交到 Git。

安装后访问 `https://<hostname>`；首次登录用户名 `sysadmin`、默认密码 `123456`，登录后及时修改。数据库与 Redis 密码不是面板登录密码。

## 安装 Node Agent

先安装 Web，在面板中创建节点服务器：填写 Node 地址、gRPC 端口和 TLS 服务器名（Node 域名），记下实际服务器 ID，填入 Node YAML 的 `node_server_id`。

在 Node 的 `pki_bundle_dir`（默认 `/tpdata/trojanpanelnext-pki`）准备 Web 生成的 `client-ca.crt`。通过可信文件传输复制公开 CA，**不要复制 `client-ca.key`、`client.key` 或 `client.crt`**。

```bash
./install.sh config node
nano node.yaml
./install.sh validate --config ./node.yaml
sudo ./install.sh install --config ./node.yaml
```

填写 Node 的 `hostname`、`email`，Web 的 `mariadb_host`、`redis_host` 和连接密码；核对端口以及 `node_server_id` 与面板中实际服务器 ID 一致。模板中的 ID 和密码都是示例值。Node 需要连接 Web 的数据库和 Redis；安装器不自动注册服务器或获取主控 CA。

`validate` 只校验 YAML 和配置字段，不检查 DNS、证书文件、网络连通性或服务健康。通过校验不等于已经具备完整部署条件。

## 重建与卸载

修改配置中的应用镜像后，重新创建 API、UI、Agent 或 Caddy 容器：

```bash
sudo ./install.sh install --config ./web.yaml --force
sudo ./install.sh install --config ./node.yaml --force
```

`--force` 不重建已有 MariaDB、Redis 容器。安装器通常复用本地镜像，重建应用容器时会拉取配置指定的镜像。使用未发布测试镜像时请勿执行 `--force`。

保留数据卸载：

```bash
sudo ./install.sh remove --config ./web.yaml
sudo ./install.sh remove --config ./node.yaml
```

卸载并删除对应服务的数据目录：

```bash
sudo ./install.sh remove --config ./web.yaml --purge-data
sudo ./install.sh remove --config ./node.yaml --purge-data
```

Web 会删除 MariaDB、Redis、API、UI 和 Web Caddy 的数据；Node 会删除 Agent 和 Node Caddy 的数据。两者均保留 `pki_bundle_dir` 和伪装站 `WEB_PATH`；自定义到默认数据目录以外的运行时路径也需自行管理。卸载不会删除原始 YAML。`--force` 仅用于 `install`，`--purge-data` 仅用于 `remove`；YAML 的 `force`、`purge_data` 默认应保持 `0`。

## 开发验证

开发者可设置 `TP_CONFIG_REF` 指向 GitHub 上实际存在的分支或完整提交 SHA，用于获取尚未发布的模板。该覆盖只改变模板来源，不修改安装器版本或已编辑配置的镜像；测试时应使用本次源码构建的对应版本镜像。普通安装流程无需设置该变量。

发布前运行 `node scripts/check-installer-release.mjs`；发布工作流同时检查 Git 标签、安装器版本、模板默认镜像标签一致。实际发布镜像还要求远程版本标签存在，并指向当前构建提交；手工触发的未发布构建不要求标签已经存在。

## 支持

本项目来源：[TrojanPanel 原项目](https://github.com/trojanpanel)。
