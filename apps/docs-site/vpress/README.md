---
home: true
heroImage: /logo.png
heroText: TrojanPanel Next
tagline: 支持 Xray、Hysteria2 和 NaiveProxy 的多用户 Web 管理面板
actionText: 快速上手 →
actionLink: ./start/introduce
features:
  - title: 配置驱动
    details: 使用 YAML 配置和明确用途模式完成无交互部署
  - title: Web 主控
    details: 统一管理用户、节点、流量、证书和系统配置
  - title: Node Agent
    details: 将代理内核与主控分离，可按需扩展多个节点服务器
  - title: 多代理支持
    details: 支持 Xray、Hysteria2 和 NaiveProxy
  - title: 响应式界面
    details: 管理员与普通用户页面均适配桌面和手机浏览器
  - title: 多语言
    details: Web 管理界面支持多种语言和主题
footer: TrojanPanel Next
---

简体中文 | [English](README_EN.md)

## 安装

当前版本为 `0.1.0-rc.8`。先预装 Docker Engine、[mikefarah/yq v4](https://github.com/mikefarah/yq)、Bash、curl、CA 证书、grep、OpenSSL、tar、coreutils、findutils 和 awk，并启动 Docker。Node 主机必须运行 systemd。安装器不会自动安装依赖；[安装器说明](https://github.com/1linhao/trojanpanelnext/blob/main/deploy/installer/README.md)保留完整 Debian/Ubuntu 命令与 yq 手工校验安装步骤。

在每台服务器下载命令入口：

```bash
curl -fsSL --connect-timeout 10 --max-time 60 \
  https://raw.githubusercontent.com/1linhao/trojanpanelnext/v0.1.0-rc.8/deploy/installer/tp.sh \
  -o tp.sh
chmod +x tp.sh
./tp.sh --version
```

入口从同一版本下载 `common.sh` 和对应命令脚本，安装时还下载 `uninstall.sh`，不需要本地仓库。先部署 Web 主控：

```bash
./tp.sh config web --output ./web.yaml
nano web.yaml
./tp.sh validate --config ./web.yaml
sudo ./tp.sh install --config ./web.yaml
```

设置域名和邮箱，准备 DNS 与端口；首次生成的数据库、Redis 密码写回 `web.yaml`。访问 `https://<hostname>`，初始账户为 `sysadmin` / `123456`，登录后修改密码。

先在 Web 创建节点服务器，取得真实 ID（至少为 `1`），把 Web 的公开 `client-ca.crt` 安全复制到 Node 的 `pki_bundle_dir`（默认 `/tpdata/trojanpanelnext-pki`），再部署 Node Agent：

```bash
./tp.sh config node --output ./node.yaml
nano node.yaml
./tp.sh validate --config ./node.yaml
sudo ./tp.sh install --config ./node.yaml
```

填入 Node 域名、Web 数据库/Redis 地址和凭据及真实 `node_server_id`；私钥和 Web 客户端身份留在 Web。Node 安装同时部署 `trojanpanelnext-host.service`，维护接口使用 mTLS HTTPS，固定端口为 `grpc_port + 1`（默认 `8101`）。gRPC（默认 `8100`）及维护接口仅允许 Web 来源；Node 还需能访问 Web HTTPS（TCP 443）以完成卸载结果确认。Agent 不挂载 `docker.sock`。`validate` 仅检查 YAML 与字段。完整网络和配置准备见[安装教程](./install-tutorial/installation.md)。

## 重建与卸载

升级时修改现有 YAML 的应用镜像标签为 `0.1.0-rc.8`，先更新所有 Node，再更新 Web；Node 的 `--force` 安装会部署宿主机维护服务：

```bash
sudo ./tp.sh install --config ./node.yaml --force
sudo ./tp.sh install --config ./web.yaml --force
```

`--force` 重建应用和 Caddy 容器，已有 MariaDB、Redis 容器保留。

在对应主机保留数据卸载：

```bash
sudo ./tp.sh remove --config ./node.yaml --keep-data
sudo ./tp.sh remove --config ./web.yaml --keep-data
```

彻底卸载（`--purge` 是 `--purge-data` 的别名）：

```bash
sudo ./tp.sh remove --config ./node.yaml --purge-data
sudo ./tp.sh remove --config ./web.yaml --purge-data
```

两种模式都删除对应容器、匿名 volume 和所用镜像仓库的全部本地 tag（含默认 GHCR 仓库的历史 tag）；仍被其他容器引用的镜像保留并提示，不执行全局 prune。普通卸载保留业务数据、PKI、伪装站、运行目录及 YAML；彻底卸载还删除所有项目服务目录、这些路径和 YAML。另一用途的项目容器仍在同一主机时会拒绝彻底卸载。两种模式最终都清理安装的宿主机维护服务与工作副本，手工下载的 `tp.sh` 和 Docker Engine 保留。

Web 节点服务器的“删除”先远程卸载、成功后事务删除服务器及关联代理配置，并保留业务数据和流量/内核任务历史；“无痕删除”还清理数据及相应 Web 历史。离线、旧版服务缺失或卸载失败时保留登记并报错。远程维护服务的 TLS 暂存直到双方完成结果确认；结果确认和辅助服务清理失败会后台重试，支持重启恢复，并显示 `cleanupPending`。保持双向维护通道及 Node 到 Web HTTPS（TCP 443）可达，直到清理完成。删除单个代理节点仍只删除代理。本地 `remove` 不同步清理 Web 登记；需要联动清理时直接在 Web 删除服务器。

脚本来源由 `TP_SCRIPT_REF` 控制，默认 `v0.1.0-rc.8`，模板跟随该 ref；`TP_CONFIG_REF` 可仅覆盖模板来源。直接使用子脚本需同目录、同版本 `common.sh`，且不再加命令前缀。完整参数见[安装器说明](https://github.com/1linhao/trojanpanelnext/blob/main/deploy/installer/README.md)。

## 支持

[TrojanPanel 原项目 GitHub](https://github.com/trojanpanel)
