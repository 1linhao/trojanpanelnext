# TrojanPanel Next

简体中文 | [English](README_EN.md)

TrojanPanel Next 是支持 Xray、Hysteria2 和 NaiveProxy 的多用户代理管理平台。Web 主控提供账户、节点服务器、代理、流量和任务管理；Node Agent 管理各服务器上的代理内核。

当前发布版本：**v1.0.1**。支持 Linux amd64、arm64，通过 GHCR 镜像部署。

## 快速安装

在对应服务器的 **root Bash 会话**执行。Debian 12/13、Ubuntu 22.04/24.04 的 amd64/arm64 主机可先一键准备依赖（需要 systemd）：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.1/scripts/tp.sh) deps install
```

入口需要 Bash、curl、CA 证书、grep 和 coreutils；`deps` 还需要 util-linux 提供的 `flock`。缺少工具时先按[最小引导说明](docs/deployment.md#dependency-install)准备。其他 Linux 按[软件依赖](docs/deployment.md#dependency-manual)手动安装。准备域名解析和[网络访问](docs/deployment.md#network)后，脚本按提示生成配置并安装。

Web 主控：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.1/scripts/tp.sh) web
```

安装后访问配置的 HTTPS 域名，初始账户为 `sysadmin` / `123456`，登录后修改密码。

先在 Web 登记节点服务器，取得服务器 ID，并将 Web 的公开 `client-ca.crt` 安全复制到 Node 服务器，然后安装 Node：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.1/scripts/tp.sh) node
```

宿主机已有证书，由 Nginx、Certbot 或其他工具管理时：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.1/scripts/tp.sh) node --certificate-mode external
```

该模式按提示读取完整证书链及私钥路径，跳过 Node Caddy 和证书申请。域名、证书和共享端口的准备见[外部证书说明](docs/deployment.md#external-certificates)。

## 配置文件部署与卸载

使用已填写的 Web 或 Node YAML 部署：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.1/scripts/tp.sh) install --config ./web.yaml
```

仅卸载容器与可清理的镜像，保留业务数据：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.1/scripts/tp.sh) remove --config ./node.yaml --keep-data
```

彻底卸载，包括项目数据、PKI、代理运行配置和部署 YAML：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.1/scripts/tp.sh) remove --config ./node.yaml --purge-data
```

替换配置路径即可操作对应主机上的 Web 或 Node。外部证书、Nginx 和 Certbot 由宿主机独立管理，两种卸载模式都保留这些外部资源。从 Web 删除整台节点服务器的行为见[远程卸载](docs/deployment.md#web-removal)。

卸载项目和其他 Docker 服务后，可清理本入口新增的依赖：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.1/scripts/tp.sh) deps remove
```

仅删除安装记录中的新增 Docker 软件包和本命令安装且未被修改的 yq；保留基础工具、原有依赖、Docker 数据及外部证书。存在任何 Docker 容器（含停止容器）、共享 containerd 中其他 namespace 的容器、Node 维护服务或无法确认运行状态时，拒绝删除 Docker。依赖安装中断时先重跑 `deps install` 修复，再卸载。见[依赖卸载](docs/deployment.md#dependency-removal)。

## 版本选择

同一个入口可通过版本号选择对应发布的脚本、配置模板和镜像：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.1/scripts/tp.sh) --version 1.0.1 web
```

版本号 `1.0.1` 对应 Git 标签 `v1.0.1` 和产品镜像标签 `:1.0.1`。配置、脚本和产品镜像必须匹配；支持范围与版本规则见[版本绑定](docs/deployment.md#versions)。

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
