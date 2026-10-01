---
home: true
heroImage: /logo.png
heroText: TrojanPanel Next
tagline: 支持 Xray、Hysteria2 和 NaiveProxy 的多用户 Web 管理面板
actionText: 快速上手 →
actionLink: ./install-tutorial/installation
features:
  - title: 一键部署
    details: 通过统一脚本入口部署 Web 与 Node，可指定产品版本
  - title: Web 主控
    details: 管理账户、节点服务器、代理、订阅和流量
  - title: Node Agent
    details: 管理代理运行时，使用 mTLS 连接主控
  - title: 证书管理
    details: 支持 Caddy 自动证书及 Node 外部证书模式
  - title: 响应式界面
    details: 管理员和普通用户页面适配桌面与手机
  - title: 内核任务
    details: 管理 Xray 与 Hysteria2 升级和回退
footer: TrojanPanel Next
---

简体中文 | [English](README_EN.md)

## 快速安装

当前版本为 **v1.0.2-rc.1**。在 root Bash 中先安装依赖，再准备域名和[网络访问](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#network)。自动依赖安装支持 Debian 12/13、Ubuntu 22.04/24.04（amd64/arm64，运行 systemd）：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) deps install
```

入口需要 Bash、curl、CA 证书、grep、coreutils；`deps` 还需 util-linux 的 `flock`。最小引导和其他 Linux 的手动安装见[依赖准备](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#dependencies)。

Web 主控：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) web
```

安装 Web 后，以系统管理员（`sysadmin` 角色）登录，点击首页的 **新增 Node 服务器**，或左侧 **服务器管理 → 新增 Node 服务器**，登记 Node 地址、gRPC 端口与 TLS 服务器名，记下真实服务器 ID，并复制 Web 当前公开 CA；详见[登记与接入](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#node-registration)。

Node Agent：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) node
```

入口按提示收集配置并部署同版本镜像，也支持指定版本、配置文件部署、外部证书、重建和卸载。完整操作以 [docs 部署指南](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md)为准。

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

卸载项目和其他 Docker 服务后，可按安装记录清理新增的 Docker 软件包和未修改的 yq：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.1/scripts/tp.sh) deps remove
```

原有依赖、基础工具、Docker 数据和外部证书保留；删除前检查 Docker 容器、共享 containerd 其他 namespace 的容器与维护服务。安装中断时先重跑 `deps install` 修复。完整范围见[依赖卸载](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md#dependency-removal)。

## 文档

- [完整部署与卸载](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/deployment.md)
- [Web 使用指南](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/user-guide.md)
- [证书管理](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/certificates.md)
- [API 指南](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/api.md)
- [开发指南](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.1/docs/development.md)
