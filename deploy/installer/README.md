# TrojanPanel Next 安装器

简体中文 | [English](README_EN.md)

安装器使用一个脚本、一份 YAML 配置和一个明确的服务器用途完成部署，全程无交互。

## 用途模式

| 模式 | 用途 | 部署内容 |
| --- | --- | --- |
| `web` | Web 主控 | API、Web UI、MariaDB、Redis、Caddy |
| `node` | Node Agent | 节点 Agent、代理内核运行环境、证书与伪装站 |

Web 主控应先安装。Node Agent 使用 Web 主控配置文件中生成的 MariaDB 和 Redis 密码。

## 外部 TLS 模式

`tls_mode` 决定证书与入口归谁负责：

| 取值 | 行为 |
| --- | --- |
| `acme`（默认） | 安装器创建 Caddy 容器，由它监听 80/443、申请 ACME 证书并反代 |
| `external` | 安装器**不创建任何反代容器**；外部入口负责 Web 80/443、ACME 和所需的明文回落 |

`tls_mode: external` 时安装器仍然做这些事：复制 `tls_cert_dir` 中的证书到
`/tpdata/trojan-panel-core/cert` 并以只读方式挂给内核、准备 `/tpdata/web` 伪装站目录、
删除上一次安装残留的 `*-caddy` 容器、写出本机契约摘要到
`/tpdata/trojanpanelnext-external/README.md`。

示例配置：[external-web.yaml](examples/external-web.yaml)、
[external-node.yaml](examples/external-node.yaml)。外部入口需要实现的完整功能清单见
[外部入口实现契约](../../docs/外部入口实现契约.md)，简版见 [EXTERNAL.md](EXTERNAL.md)。

Node 的 Xray、NaiveProxy 和 Hysteria2 默认直接监听协议端口并自行终止 TLS，不经过统一 L4
入口。节点 agent 会把当前内核 listener 写入
`/tpdata/trojan-panel-core/external/routes.json`，供端口/防火墙检查和明文回落渲染；它不是
nginx `stream` 动态配置源。只有标记 `external_fallback_listener_required: true` 的路由需要
外部入口提供明文 HTTP 伪装站。

## 系统要求

| 项目 | 要求 |
| --- | --- |
| 操作系统 | Debian 12 |
| 权限 | 安装和卸载需要 `root` |
| CPU | `linux/amd64`（x86_64） |
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

首次安装会生成 `sysadmin`、MariaDB 与 Redis 的随机密码，写回 `web.yaml`
并将文件权限设为 `600`。终端只显示密码保存位置，不显示密码。
敏感写入由 Linux amd64 `secure-file` helper 完成：父目录与原目标 fd 在提交期间保持打开；
已有目标使用 `renameat2(RENAME_EXCHANGE)` 交换、核验并在竞态时回滚，新目标使用
`RENAME_NOREPLACE`，因此最终文件或父目录在提交窗口被替换时安装会失败且不覆盖目标。
安装器还会在 `pki_bundle_dir` 自动生成主控 mTLS 身份；CA 私钥只保留在 Web 主控。

安装命令只有在当前配置身份访问 MariaDB、Redis、公网 HTTPS UI 和容器内只读 `sysadmin` 凭据验证全部通过后才返回
成功。任一探测失败都返回非零，并输出不含秘密的定位建议。使用同一配置重跑会复用已保存的三组凭据。
管理员凭据探测不暴露 HTTP 路径，也不会启动 Redis/限流、签发会话、更新登录时间或累计登录失败次数。

Node Agent 安装前，通过可信的文件传输或密钥管理系统，将 Web 主控中的
`/tpdata/trojanpanelnext-pki/client-ca.crt` 复制到 Node 的同一路径。只复制公开 CA
证书，不要复制 `client-ca.key`、`client.key` 或 `client.crt`。
安装器会把 CA 摘要写入 Core 容器环境；首次接管旧容器或 CA 内容变化时会自动重建
Core，使新的信任根立即生效。CA 未变化时重复安装不会重启 Core。

## Node Agent 安装

复制配置模板：

```bash
cp examples/node-agent.yaml ./node-agent.yaml
chmod 600 ./node-agent.yaml
```

填写节点域名和 Web 主控地址，并把 Web 主控 `node-identity register|rotate` 产生的受限凭据文件中
`mariadb.username`、`mariadb.password`、`redis.username`、`redis.password` 和 `node_server_id`
写入配置；不要复用 Web 的 root/默认用户密码。确认公开 CA 证书已放入 `pki_bundle_dir`：

```bash
./install.sh validate --mode node --config ./node-agent.yaml
sudo ./install.sh install --mode node --config ./node-agent.yaml
sudo ./install.sh refresh-cert --mode node --config ./node-agent.yaml
```

当宿主管理系统已经生成 Protocol v1 EntrySpec 时，通过 `--entry-spec` 把它交给安装器：

```bash
sudo ./install.sh install --mode node --config ./node-agent.yaml \
  --entry-spec /var/lib/vps-factory/service-specs/trojanpanelnext-node.json
```

安装成功后 installer 调用相邻的 `entry/entryctl.sh reconcile`；移除时先调用 `remove`，入口回收
成功后才删除应用容器。EntrySpec 必须与配置的 purpose、domain 一致，且变更动作要求 root 所有的
0600 普通文件。

`--mode` 必须和配置中的 `trojanpanelnext.deployment_mode` 一致，模式不匹配时安装器会立即退出。
旧配置的 `purpose` 仅作为兼容输入继续接受。

## 重建与卸载

重新创建已有容器：

```bash
sudo ./install.sh install --mode web --config ./web.yaml --force
```

保留数据卸载服务：

```bash
sudo ./install.sh remove --mode web --config ./web.yaml --keep-data
```

`--keep-data` 会覆盖配置中的 `purge_data`，适合由外部管理系统执行可恢复卸载。

删除服务及生成数据：

```bash
sudo ./install.sh remove --mode node --config ./node-agent.yaml --purge-data
```

## 配置文件

[Web 主控模板](examples/web.yaml)包含域名、镜像、服务端口、mTLS 身份目录以及主控内部凭据。

[Node Agent 模板](examples/node-agent.yaml)包含节点域名、主控数据库连接、Redis 连接、gRPC、公开 CA 目录与证书路径。

外部 TLS 模式使用 [external-web.yaml](examples/external-web.yaml) 与
[external-node.yaml](examples/external-node.yaml)，它们在模板基础上增加以下键：

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `tls_mode` | `acme` | `acme` 或 `external` |
| `tls_cert_dir` | 空 | 外部证书目录；`external` + `node` 必填 |
| `tls_cert_file` / `tls_key_file` | 空 | 目录内存在多对证书时显式指定文件名 |
| `bind_address` | `0.0.0.0` | 面板 UI 的监听地址，建议外部模式设为 `127.0.0.1` |
| `managed_cert_dir` | `/tpdata/trojan-panel-core/cert` | 托管证书副本目录，内核只读该目录 |
| `external_managed_dir` | `/tpdata/trojanpanelnext-external` | 契约文件目录 |
| `external_routes_dir` | `/tpdata/trojan-panel-core/external` | Node Agent 写入 `routes.json` 的目录 |

证书目录的识别顺序：`tls_cert_file`/`tls_key_file` 指定的文件 → `fullchain.pem`+`privkey.pem`
（certd 布局）→ 同词干的 `.crt`/`.key` 配对（递归三层）。存在多对且未显式指定时报错。

配置中的密码不会打印到终端。请将实际配置作为敏感文件保存，不要提交到 Git。

## 版本化发布资产

正式发布工作流使用 `release/generate-assets.sh` 生成同一版本的 `bootstrap.sh`、
`install.sh`、用于安全打开/原子写入敏感配置的 `secure-file`、`web|node|combined` 配置模板、`release-manifest.json` 和
`SHA256SUMS`。产品镜像和运行时镜像都以 `name@sha256:<digest>` 固定；
`bootstrap.sh` 会先调用同包内的 `verify-assets.sh` 校验版本、资产摘要、镜像引用和配置；
发布包内的 `install.sh` 在被直接调用时也会执行同一预检。验证器只依赖 Debian 12
基础系统提供的 Bash、awk、grep 与 coreutils，不要求宿主预装 `jq`；它先依据固定资产集合校验
`SHA256SUMS`，拒绝 bundle 路径中的符号链接，并只接受生成器输出的 printable ASCII + LF manifest；
且不会在此之前 source 或执行其他随包程序。两条入口只有在完整校验 12 个资产（包括
`secure-file`）后才首次执行 helper，并在安全快照后再次验证配置契约；全部通过后才越过宿主变更边界。

发布配置契约使用 `deployment_mode`、`api_image`、`web_image` 和 `node_agent_image`；旧的
`purpose`、`panel_image`、`ui_image` 和 `core_image` 只供既有安装配置兼容读取，不会出现在新模板中。

`combined` 是统一配置契约中的有效部署模式，但本工单不提供其容器编排；发布资产校验可以验证
combined 配置，现有安装器仍只执行 `web` 与 `node`。无秘密的 manifest 结构示例见
[example-release-manifest.json](release/example-release-manifest.json)。Release 使用 tar.gz 保留脚本可执行位，
并同时附带 manifest 与 SHA256SUMS。下载后先验证归档 attestation，再解包并验证包内摘要：

```bash
archive=trojanpanelnext-installer-<version>.tar.gz
gh attestation verify "${archive}" --repo 1linhao/trojanpanelnext
mkdir trojanpanelnext-installer
tar -xzf "${archive}" -C trojanpanelnext-installer
cd trojanpanelnext-installer
sha256sum -c SHA256SUMS
```

复制对应模板并编辑副本，不要修改发布包中受摘要保护的模板；然后验证实际配置：

```bash
cp ./config-web.yaml ./deployment.yaml
./verify-assets.sh --assets-dir . --config ./deployment.yaml
```

## 支持

本项目来源：[TrojanPanel 原项目](https://github.com/trojanpanel)。
