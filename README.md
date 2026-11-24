# TrojanPanel Next

简体中文 | [English](README_EN.md)

<a id="quick-install"></a>
## 快速部署

在运行 systemd 的 Debian 12/13 或 Ubuntu 22.04/24.04（amd64、arm64）上，用 root Bash 执行。普通 SSH 用户先运行 `sudo -i`。以下命令安装 Web 与 Node `v1.0.2-rc.13`。

准备一个面板域名和一个 Node 域名，分别解析到对应 VPS 的公网 IP。单机部署时，两个域名解析到同一台 VPS。如有 AAAA 记录，也需指向能提供对应服务的 VPS。

- **双机部署。** Web 是面板机，Node 是代理机。按顺序完成步骤 1、[步骤 2A](#quick-install-separate)和步骤 3。
- **单机部署。** Web 和 Node 在同一台 VPS。先准备 Node 域名的证书，再完成步骤 1、[步骤 2B](#quick-install-colocated)和步骤 3。已有匹配的完整证书链与未加密私钥时，可以直接使用。

### 单机部署先准备 Node 证书

如果暂无证书，先确认 TCP `80` 已对公网开放且未被占用，再执行以下命令。按提示输入 Node 域名和联系邮箱。证书链和私钥位于 `/etc/letsencrypt/live/tpnext-node-实际域名/` 下，分别为 `fullchain.pem` 和 `privkey.pem`。已安装 Web 时的签发和续期见[单机证书说明](docs/deployment.md#colocated-node-certificates)。

```bash
(
  set -e
  read -r -p 'Node 域名 > ' node_domain
  read -r -p '证书联系邮箱 > ' email
  [[ -n "$node_domain" && -n "$email" ]] ||
    { printf '%s\n' '请填写 Node 域名和证书联系邮箱。' >&2; exit 1; }
  apt-get update
  apt-get install -y certbot
  certbot certonly --standalone --preferred-challenges http \
    --agree-tos --email "$email" --keep-until-expiring \
    --cert-name "tpnext-node-$node_domain" -d "$node_domain"
)
```

### 1. 安装 Web

在 Web VPS 对公网开放 TCP `80` 和 `443`，粘贴整段命令。按提示输入面板域名和证书联系邮箱。

```bash
(
  set -e
  apt-get update
  apt-get install -y bash curl ca-certificates grep coreutils util-linux tar gzip
  install -d -m 700 /root/tpnext-web
  cd /root/tpnext-web
  umask 077
  entry="$(mktemp -p . .tpnext.XXXXXX)"
  trap 'unlink "$entry"' EXIT
  curl -fsSL --proto '=https' --proto-redir '=https' --retry 2 \
    https://raw.githubusercontent.com/1linhao/trojanpanelnext/579316408f46a28247231e90dcaf80d8b8e4f41a/scripts/tp.sh \
    -o "$entry"
  printf '%s  %s\n' 5e9de5018f6ca8fdac4055669c9edaee2d512983ca44e657ae945a8b9f75256c "$entry" | sha256sum -c
  install -m 700 "$entry" ./tp.sh
  bash ./tp.sh deps install
  bash ./tp.sh web
)
```

打开面板域名的 HTTPS 地址，用初始账户 `sysadmin`、密码 `123456` 登录并修改密码。

<a id="quick-install-separate"></a>
### 2A. 双机部署，在面板登记远程 Node

1. 在防火墙中允许 Node VPS 访问 Web VPS 的 TCP `9507`、`6378`，允许 Web VPS 访问 Node VPS 的 TCP `8100`、`8101`。Node 使用 Caddy 签证时对公网开放 TCP `80`。 [网络要求](docs/deployment.md#network)
2. 在面板打开 **服务器管理 → 新增 Node 服务器**。填写 2–20 个字符的**服务器名称**，**Node 连接地址**填写 Web 能访问的 Node 地址，**gRPC 端口**填 `8100`，**gRPC 证书域名**填 Node 域名。
3. 保存后，在 **部署 Node** 向导选择 **Caddy 自动签发与续签** 并填邮箱。若向导已关闭，在该服务器行点击 **部署 Node**，按钮折叠时先展开 **操作**。**Node 可连接的 Web 地址**填写 Node 能访问的面板域名或 Web 私网地址。点 **下一步 → 下载部署包**，上传到 Node VPS 的管理员私有目录。

<a id="quick-install-colocated"></a>
### 2B. 单机部署，在面板登记本机 Node

1. 在防火墙中将 TCP `9507`、`6378`、`8100`、`8101` 的访问范围限制为本机。
2. 在面板打开 **服务器管理 → 新增 Node 服务器**。**服务器名称**可填「本机 Node」，**Node 连接地址**填 `127.0.0.1`，**gRPC 端口**填 `8100`，**gRPC 证书域名**填 Node 的公网域名。
3. 保存后，在 **部署 Node** 向导选择 **使用宿主机现有证书**，填写完整证书链和私钥的绝对路径。**Node 可连接的 Web 地址**保留面板域名。点 **下一步 → 下载部署包**，上传回这台 VPS 的管理员私有目录。

### 3. 安装 Node 并验证连接

在存放部署包的 VPS 上，粘贴整段命令。输入部署包的绝对路径。同机安装输入 `y`，独立 Node 按 Enter。

```bash
(
  set -e
  umask 077
  read -r -p '部署包的绝对路径 > ' bundle
  [[ "$bundle" == /* && "${bundle##*/}" == tpnext-node-*.tar.gz && -f "$bundle" ]] ||
    { printf '%s\n' '请填写已上传的 tpnext-node-*.tar.gz 绝对路径。' >&2; exit 1; }
  read -r -p 'Node 是否与 Web 在同一台 VPS？[y/N] > ' colocated
  apt-get update
  apt-get install -y bash curl ca-certificates grep coreutils util-linux tar gzip
  install -d -m 700 /root/tpnext-node
  cd /root/tpnext-node
  chmod 600 -- "$bundle"
  tar -xzf "$bundle"
  bash ./tpnext/install-dependencies.sh
  case "$colocated" in
    y|Y)
      yq -i '
        .trojanpanelnext.mariadb_host = "127.0.0.1" |
        .trojanpanelnext.redis_host = "127.0.0.1" |
        .trojanpanelnext.pki_bundle_dir = "/tpdata/trojan-panel-core/pki-bootstrap"
      ' ./tpnext/node.yaml
      ;;
  esac
  chmod 600 -- ./tpnext/node.yaml
  bash ./tpnext/install-node.sh
)
```

在 **服务器管理** 确认 Node 在线，再到 **节点管理** 创建代理。代理地址填写客户端能访问的 Node 公网域名，开放所选协议的端口，然后导出订阅，在客户端连接验证。 [代理端口说明](docs/port-forwarding.md)

后续接入其他 VPS 时，在同一面板为每台 Node 重复步骤 2A 和步骤 3。保留 `/root/tpnext-web/web.yaml` 和各节点的 `/root/tpnext-node/tpnext/node.yaml`，并将部署包和 YAML 存在管理员私有目录。 [卸载步骤](docs/deployment.md#removal)

## 项目来源

本项目基于 [TrojanPanel](https://github.com/trojanpanel) 独立维护。来源说明见 [NOTICE.md](NOTICE.md)。
