# TrojanPanel Next 安装器

简体中文 | [English](README_EN.md)

安装器使用一个脚本、一份 YAML 配置和一个明确的服务器用途完成部署，全程无交互。

## 用途模式

| 模式 | 用途 | 部署内容 |
| --- | --- | --- |
| `web` | Web 主控 | API、Web UI、MariaDB、Redis、Caddy |
| `node` | Node Agent | 节点 Agent、代理内核运行环境、证书与伪装站 |

Web 主控应先安装。Node Agent 使用 Web 主控配置文件中生成的 MariaDB 和 Redis 密码。

## 系统要求

| 项目 | 要求 |
| --- | --- |
| 操作系统 | Ubuntu 20.04+、Debian 11+ 或同类 systemd Linux |
| 权限 | 安装和卸载需要 `root` |
| CPU | `linux/amd64` 或 `linux/arm64` |
| 内存 | 至少 1 GiB |
| 网络 | 域名已解析到目标服务器，防火墙放行所配置端口 |

## Web 主控安装

复制配置模板：

```bash
cp examples/web.yaml ./web.yaml
chmod 600 ./web.yaml
```

编辑 `hostname` 和 `email`，然后先校验再安装：

```bash
./install.sh validate --mode web --config ./web.yaml
sudo ./install.sh install --mode web --config ./web.yaml
```

首次安装会生成 MariaDB 与 Redis 密码，写回 `web.yaml` 并将文件权限设为 `600`。
安装器还会在 `pki_bundle_dir` 自动生成主控 mTLS 身份；CA 私钥只保留在 Web 主控。

Node Agent 安装前，通过可信的文件传输或密钥管理系统，将 Web 主控中的
`/tpdata/trojanpanelnext-pki/client-ca.crt` 复制到 Node 的同一路径。只复制公开 CA
证书，不要复制 `client-ca.key`、`client.key` 或 `client.crt`。

## Node Agent 安装

复制配置模板：

```bash
cp examples/node-agent.yaml ./node-agent.yaml
chmod 600 ./node-agent.yaml
```

填写节点域名、Web 主控地址以及 Web 配置中的数据库和 Redis 密码，并确认公开 CA
证书已放入 `pki_bundle_dir`：

```bash
./install.sh validate --mode node --config ./node-agent.yaml
sudo ./install.sh install --mode node --config ./node-agent.yaml
```

`--mode` 必须和配置中的 `trojanpanelnext.purpose` 一致，模式不匹配时安装器会立即退出。

## 重建与卸载

重新创建已有容器：

```bash
sudo ./install.sh install --mode web --config ./web.yaml --force
```

保留数据卸载服务：

```bash
sudo ./install.sh remove --mode web --config ./web.yaml
```

删除服务及生成数据：

```bash
sudo ./install.sh remove --mode node --config ./node-agent.yaml --purge-data
```

## 配置文件

[Web 主控模板](examples/web.yaml)包含域名、镜像、服务端口、mTLS 身份目录以及主控内部凭据。

[Node Agent 模板](examples/node-agent.yaml)包含节点域名、主控数据库连接、Redis 连接、gRPC、公开 CA 目录与证书路径。

配置中的密码不会打印到终端。请将实际配置作为敏感文件保存，不要提交到 Git。

## 支持

本项目来源：[TrojanPanel 原项目](https://github.com/trojanpanel)。
