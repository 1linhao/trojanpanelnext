# 部署 TrojanPanel Next v1.0.2-rc.8

## 目录

- [依赖与网络](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#dependencies)
- [一键安装依赖](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#dependency-install)
- [卸载依赖](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#dependency-removal)
- [选择版本](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#versions)
- [Web 部署](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#web)
- [Node 部署包](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#node-deployment-package)
- [Node 命令行备选](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#node-interactive)
- [外部证书](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#external-certificates)
- [登记节点服务器](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#node-registration)
- [模板下载](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#configuration-download)
- [Web 最少修改](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#configuration-web-minimum)
- [Node 最少修改](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#configuration-node-minimum)
- [准备 Node 公开 CA](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#configuration-node-ca)
- [配置校验与安装](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#configuration-validation)
- [更新产品镜像](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#updates)
- [卸载或只删除 Web 记录](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#web-removal)
- [重建与卸载](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#removal)
- [服务器重新接入](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#reconnect)
- [故障排查](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#troubleshooting)

## 一键安装

### 1. 在 Web 主机准备依赖

在 **root Bash 会话**执行。自动依赖安装支持 Debian 12/13、Ubuntu 22.04/24.04 的 amd64/arm64 主机，需要运行 systemd。

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.8/scripts/tp.sh) deps install
```

入口需要 Bash、curl、CA 证书、grep 和 coreutils；`deps` 还需要 util-linux 提供的 `flock`。缺少工具时先按[最小引导说明](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#dependency-install)准备，其他 Linux 按[软件依赖](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#dependency-manual)手动安装。准备 Web 域名解析和[网络访问](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#network)。

### 2. 安装 Web 主控

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.8/scripts/tp.sh) web
```

按提示填写域名和证书邮箱。安装后访问该 HTTPS 域名，初始账户为 `sysadmin` / `123456`，首次登录后修改密码。

### 3. 在 Web 登记 Node 服务器

以系统管理员（`sysadmin` 角色）打开左侧 **服务器管理**，点击 **新增 Node 服务器**。填写服务器名称、Node 的 IP 或域名、gRPC 端口（默认 `8100`）和 gRPC 证书域名。

保存后自动打开 **部署 Node**；之后可通过该服务器行的 **部署 Node** 重新打开。服务器列表的 **ID** 列显示 Web 数据库生成的数字 ID，例如 `3`。它必须是 **大于或等于 1 的整数**，与 **IP / 域名** 分列；它不是服务器地址，也不是某个代理节点的 ID。详见[登记节点服务器](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#node-registration)。

### 4. 下载 Node 部署包

在 **部署 Node** 中确认 Node 可访问的 Web 域名或 IP（例如 `panel.example.com`，不含 `https://` 和路径），选择证书模式：

- **Caddy**：填写证书邮箱，由 Node Caddy 申请与续签。
- **使用宿主机现有证书**：填写目标 Node 上已存在的 fullchain 与私钥绝对路径，由宿主机工具维护。

点击 **下一步** 进入 **下载与安装**，再点击 **下载部署包**。下载的 `.tar.gz` 包包含 `node.yaml`、公开 `client-ca.crt`、`install-node.sh` 和 `README.md`；配置已填写真实服务器 ID、Web 数据库 / Redis 凭据及连接参数。包不包含 Web 私钥或 Node TLS 私钥。已有证书模式的证书与私钥必须事先在 Node 主机准备，Web 不生成这些文件。

部署包和 YAML 含敏感凭据，应保存为 **`0600`**，只安全传输到目标 Node，不提交公开仓库。完整范围见[部署包安装](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#node-deployment-package)。

### 5. 在 Node 主机执行安装

先在 Node 的 root Bash 会话准备依赖：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.8/scripts/tp.sh) deps install
```

将部署包安全传到 Node 的私有工作目录。以下以 ID 为 `3` 的 `tpnext-node-3.tar.gz` 为例，按实际下载文件名替换：

```bash
tar -xzf ./tpnext-node-3.tar.gz
```

解压后四个文件位于 `tpnext/` 目录（权限 `0700`）。在解压目录运行包内入口：

```bash
bash ./tpnext/install-node.sh
```

入口自动准备公开 CA，按包内 `tpnext/node.yaml` 安装；安装器校验包内当前 Web 的公开 CA，在替换不同的旧信任文件前先备份；不会强制重建已有服务。安装后长期保留该 YAML，供[更新](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#updates)和[卸载](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#removal)使用。

### 6. 检查在线并创建代理

返回 **服务器管理** 检查 Node 在线，再在 **节点管理** 创建所需代理。Node 未安装时离线是正常状态。数据库、Redis、gRPC 和维护端口须按实际配置连通，详见[网络要求](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#network)。

## 配置文件部署

先在对应主机下载模板：

Web：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.8/scripts/tp.sh) config web --output ./web.yaml
```

Node：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.8/scripts/tp.sh) config node --output ./node.yaml
```

使用 `nano` 或 `vi` 编辑，保留 `trojanpanelnext:` 层级和当前版本的镜像字段。Web 至少修改 `hostname`、`email`；首次安装空数据库/Redis 密码会生成并写回 YAML。Node 至少修改域名、Caddy 邮箱、Web 数据库/Redis 地址与实际密码、整数服务器 ID（≥ `1`，不是 IP / 域名或代理 ID）、TLS 服务器名，并在安装前准备 Web 公开 CA。外部证书模式改 `node_certificate_mode: external` 与现存证书/私钥绝对路径，无需邮箱。模板权限为 `0600`，不覆盖已有文件。

[Web 最少修改](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#configuration-web-minimum) · [Node 最少修改](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#configuration-node-minimum) · [Node CA 准备](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#configuration-node-ca)

Web 校验：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.8/scripts/tp.sh) validate --config ./web.yaml
```

Web 安装：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.8/scripts/tp.sh) install --config ./web.yaml
```

Node 校验：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.8/scripts/tp.sh) validate --config ./node.yaml
```

Node 安装：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.8/scripts/tp.sh) install --config ./node.yaml --client-ca /root/client-ca.crt
```

## 更新产品镜像

在对应主机使用实际部署 YAML，并显式选择目标版本；资源包部署的 Node 主机使用 `./tpnext/node.yaml`；手工部署使用原 YAML 路径。保留凭据、数据与 PKI，不升级 MariaDB、Redis 或 Caddy。兼容范围、备份、短暂中断和恢复限制见[镜像更新](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#updates)。

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.8/scripts/tp.sh) --version 1.0.2-rc.8 update --config ./web.yaml
```

## 卸载依赖

先卸载项目及其他 Docker 服务并等待 Node 维护服务清理，然后执行：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.8/scripts/tp.sh) deps remove
```

仅清理安装记录中的新增 Docker 软件包与未修改的 yq。已有依赖、基础工具、Docker 数据和外部证书保留。存在任何 Docker 容器（含停止容器）、共享 containerd 的其他 namespace 中仍有容器、Node 维护服务或运行状态无法确认时拒绝删除 Docker。依赖安装中断时先重跑 `deps install` 修复，再卸载；完整范围见[依赖卸载](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md#dependency-removal)。

## 完整参考

仓库 [docs/deployment.md](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment.md) 是部署说明的维护源，包含依赖安装、所有参数、YAML 字段、端口、卸载和证书维护。英文版见 [docs/deployment_EN.md](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/deployment_EN.md)。选择指定版本时，入口从该版本标签加载命令、模板和镜像；文档与源码一同发布。
