# 部署 TrojanPanel Next v1.0.2-rc.1

## 目录

- [依赖与网络](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#dependencies)
- [一键安装依赖](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#dependency-install)
- [卸载依赖](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#dependency-removal)
- [选择版本](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#versions)
- [Web 部署](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#web)
- [Node 部署](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#node)
- [外部证书](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#external-certificates)
- [登记节点服务器](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#node-registration)
- [模板下载](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#configuration-download)
- [Web 最少修改](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#configuration-web-minimum)
- [Node 最少修改](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#configuration-node-minimum)
- [准备 Node 公开 CA](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#configuration-node-ca)
- [配置校验与安装](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#configuration-validation)
- [重建与卸载](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#removal)
- [服务器重新接入](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#reconnect)
- [故障排查](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#troubleshooting)

## 一键安装

在 root Bash 中先安装依赖。自动安装支持 Debian 12/13、Ubuntu 22.04/24.04（amd64/arm64，运行 systemd）；`deps` 还需要 util-linux 的 `flock`。入口最小工具与其他 Linux 的手动准备见[依赖说明](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#dependencies)。

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) deps install
```

准备域名 DNS 与所需端口后，执行对应部署命令。

Web 主控：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) web
```

Node Agent：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) node
```

Node 使用已有证书：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) node --certificate-mode external
```

部署脚本提示输入配置；Node 部署前以系统管理员（`sysadmin` 角色）登录 Web，通过首页的 **新增 Node 服务器**，或左侧 **服务器管理 → 新增 Node 服务器** 登记服务器，取得真实服务器 ID，并复制当前公开客户端 CA。入口详情见[登记节点服务器](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#node-registration)。`web`、`node`、`install` 只检查依赖，也不会替已有 Nginx 设置端口分流。

## 配置文件部署

先在对应主机下载模板：

Web：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) config web --output ./web.yaml
```

Node：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) config node --output ./node.yaml
```

使用 `nano` 或 `vi` 编辑，保留 `trojanpanelnext:` 层级和当前版本的镜像字段。Web 至少修改 `hostname`、`email`；首次安装空数据库/Redis 密码会生成并写回 YAML。Node 至少修改域名、Caddy 邮箱、Web 数据库/Redis 地址与实际密码、真实服务器 ID、TLS 服务器名，并在安装前准备 Web 公开 CA。外部证书模式改 `node_certificate_mode: external` 与现存证书/私钥绝对路径，无需邮箱。模板权限为 `0600`，不覆盖已有文件。

[Web 最少修改](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#configuration-web-minimum) · [Node 最少修改](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#configuration-node-minimum) · [Node CA 准备](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#configuration-node-ca)

Web 校验：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) validate --config ./web.yaml
```

Web 安装：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) install --config ./web.yaml
```

Node 校验：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) validate --config ./node.yaml
```

Node 安装：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) install --config ./node.yaml
```

## 卸载依赖

先卸载项目及其他 Docker 服务并等待 Node 维护服务清理，然后执行：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) deps remove
```

仅清理安装记录中的新增 Docker 软件包与未修改的 yq。已有依赖、基础工具、Docker 数据和外部证书保留。存在任何 Docker 容器（含停止容器）、共享 containerd 的其他 namespace 中仍有容器、Node 维护服务或运行状态无法确认时拒绝删除 Docker。依赖安装中断时先重跑 `deps install` 修复，再卸载；完整范围见[依赖卸载](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#dependency-removal)。

## 完整参考

仓库 [docs/deployment.md](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md) 是部署说明的维护源，包含依赖安装、所有参数、YAML 字段、端口、卸载和证书维护。英文版见 [docs/deployment_EN.md](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment_EN.md)。选择指定版本时，入口从该版本标签加载命令、模板和镜像；文档与源码一同发布。
