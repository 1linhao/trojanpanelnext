# TrojanPanel Next

简体中文 | [English](README_EN.md)

TrojanPanel Next 是一个支持 Xray、Hysteria2 和 NaiveProxy 的多用户 Web
管理面板，提供用户与节点管理、系统看板、证书管理和分布式部署能力。

本仓库统一维护以下产品组件：

| 组件 | 说明 |
| --- | --- |
| `apps/control-plane/api` | 控制面后端服务 |
| `apps/control-plane/web` | Web 管理界面 |
| `apps/node-agent` | 节点 Agent 与代理内核运行管理 |
| `deploy/installer` | 安装和部署工具 |
| `apps/docs-site` | 使用与安装文档 |

## 开始使用

当前版本为 `0.1.0-rc.6`。先按[安装说明](deploy/installer/README.md#系统与软件依赖)预装 Docker Engine、mikefarah/yq v4 等依赖并启动 Docker；安装器不会自动安装依赖。Node 主机还需要 systemd。

在每台 Web 或 Node 服务器下载命令入口：

```bash
curl -fsSL --connect-timeout 10 --max-time 60 \
  https://raw.githubusercontent.com/1linhao/trojanpanelnext/v0.1.0-rc.6/deploy/installer/tp.sh \
  -o tp.sh
chmod +x tp.sh
./tp.sh --version
```

入口自动下载同版本的所需脚本。先部署 Web 主控：

```bash
./tp.sh config web --output ./web.yaml
nano web.yaml
./tp.sh validate --config ./web.yaml
sudo ./tp.sh install --config ./web.yaml
```

编辑域名和邮箱，准备 DNS 及防火墙。首次生成的数据库和 Redis 密码会写回 YAML。访问 `https://<hostname>`，首次账户为 `sysadmin` / `123456`，登录后修改密码。

在 Web 创建节点服务器，记下实际 ID（至少为 `1`），并把 Web 的公开 `client-ca.crt` 安全复制到 Node 的 `pki_bundle_dir`（默认 `/tpdata/trojanpanelnext-pki`）。然后在 Node 主机部署：

```bash
./tp.sh config node --output ./node.yaml
nano node.yaml
./tp.sh validate --config ./node.yaml
sudo ./tp.sh install --config ./node.yaml
```

填写 Node 域名、Web 数据库和 Redis 地址/凭据、实际 `node_server_id`。只向 Node 复制公开 CA，私钥与 Web 客户端身份留在 Web。Web 到 Node 的 gRPC 端口（默认 `8100`）及宿主机维护 HTTPS 端口（固定为 `grpc_port + 1`，默认 `8101`）只允许 Web 来源。Node 还需能访问 Web 的 HTTPS（TCP 443），以完成卸载结果确认。详细网络准备见[完整安装说明](deploy/installer/README.md)。`validate` 仅检查 YAML 与字段。

修改应用镜像配置后重建容器；升级到此版本时先执行所有 Node，再执行 Web：

```bash
sudo ./tp.sh install --config ./node.yaml --force
sudo ./tp.sh install --config ./web.yaml --force
```

保留数据卸载（在对应主机执行）：

```bash
sudo ./tp.sh remove --config ./node.yaml --keep-data
sudo ./tp.sh remove --config ./web.yaml --keep-data
```

彻底卸载（删除数据、PKI、伪装站、自定义内核运行目录和部署 YAML；`--purge` 是 `--purge-data` 的别名）：

```bash
sudo ./tp.sh remove --config ./node.yaml --purge-data
sudo ./tp.sh remove --config ./web.yaml --purge-data
```

两种方式都删除对应容器、匿名 volume 及其镜像仓库的全部本地 tag（含默认 GHCR 仓库的历史 tag）；被其他容器使用的共享镜像会保留并提示，不执行全局 prune。彻底卸载不能在仍存在另一用途容器的同一主机执行。两种方式最终都清理安装的宿主机维护服务，手工下载的 `tp.sh` 与 Docker Engine 保留。

本地卸载不会同步删除 Web 登记；需要同时卸载 Node 并清理登记时，请直接在 Web 的节点服务器页面发起删除。

普通删除保留数据和 Web 历史，无痕删除还清理数据及对应历史；节点离线、旧版本缺少维护服务或卸载失败时保留登记并报错。删除单个代理节点仅删除代理。完整行为见[卸载说明](deploy/installer/README.md#卸载)。

[Web 管理界面](apps/control-plane/web/README.md)

[Node Agent](apps/node-agent/README.md)

[文档站源码](apps/docs-site/vpress)

## 项目来源

TrojanPanel Next 是基于 [TrojanPanel](https://github.com/trojanpanel) 原始项目演进的
独立维护版本，不是原组织的官方发布。项目来源与历史说明见 [NOTICE.md](NOTICE.md)。
