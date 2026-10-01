# TrojanPanel Next

简体中文 | [English](README_EN.md)

TrojanPanel Next 是支持 Xray、Hysteria2 和 NaiveProxy 的多用户代理管理平台。Web 主控提供账户、节点服务器、代理、流量和任务管理；Node Agent 管理各服务器上的代理内核。

当前发布版本：**v1.0**。支持 Linux amd64、arm64，通过 GHCR 镜像部署。

## 快速安装

在对应服务器的 **root Bash 会话**执行。请先按[软件依赖](docs/deployment.md#dependencies)安装并启动 Docker Engine、mikefarah/yq v4 等工具，准备域名解析和[网络访问](docs/deployment.md#network)。脚本按提示生成配置并安装。

Web 主控：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0/scripts/tp.sh) web
```

安装后访问配置的 HTTPS 域名，初始账户为 `sysadmin` / `123456`，登录后修改密码。

先在 Web 登记节点服务器，取得服务器 ID，并将 Web 的公开 `client-ca.crt` 安全复制到 Node 服务器，然后安装 Node：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0/scripts/tp.sh) node
```

宿主机已有证书，由 Nginx、Certbot 或其他工具管理时：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0/scripts/tp.sh) node --certificate-mode external
```

该模式按提示读取完整证书链及私钥路径，跳过 Node Caddy 和证书申请。域名、证书和共享端口的准备见[外部证书说明](docs/deployment.md#external-certificates)。

## 配置文件部署与卸载

使用已填写的 Web 或 Node YAML 部署：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0/scripts/tp.sh) install --config ./web.yaml
```

仅卸载容器与可清理的镜像，保留业务数据：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0/scripts/tp.sh) remove --config ./node.yaml --keep-data
```

彻底卸载，包括项目数据、PKI、代理运行配置和部署 YAML：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0/scripts/tp.sh) remove --config ./node.yaml --purge-data
```

替换配置路径即可操作对应主机上的 Web 或 Node。外部证书、Nginx 和 Certbot 由宿主机独立管理，两种卸载模式都保留这些外部资源。从 Web 删除整台节点服务器的行为见[远程卸载](docs/deployment.md#web-removal)。

## 版本选择

同一个入口可通过版本号选择对应发布的脚本、配置模板和镜像：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0/scripts/tp.sh) --version 1.0 web
```

版本号 `1.0` 对应 Git 标签 `v1.0` 和产品镜像标签 `:1.0`。配置、脚本和产品镜像必须匹配；支持范围与版本规则见[版本绑定](docs/deployment.md#versions)。

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
