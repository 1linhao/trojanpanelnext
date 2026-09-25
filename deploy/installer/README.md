# TrojanPanel Next 安装器

简体中文 | [English](README_EN.md)

安装器可直接在受支持的 Linux 主机上部署 TrojanPanel Next，不依赖外部 VPS 管理系统、控制台或编排后台。部署使用一个版本化 Release、一份 YAML 配置和一个明确的部署模式，全程无交互。

## 用途模式

| 模式 | 用途 | 部署内容 |
| --- | --- | --- |
| `web` | Web 主控 | API、Web UI、MariaDB、Redis、公开入口 |
| `node` | Node Agent | 节点 Agent、代理内核运行环境、证书与伪装站 |
| `combined` | 单机主控与节点 | Web 主控、Node Agent、共享入口及本机数据服务 |

`web` 与 `node` 可部署在不同主机，先部署 Web 主控，再为每个 Node 登记并签发独立 MariaDB/Redis 身份；Node 不复用 Web 的 root 或 default 用户。`combined` 在同一主机运行两种职责，使用两个不同且解析到该机公网 IP 的域名。独立部署默认由安装器管理 Caddy 与 ACME 入口；`tls_mode: external` 仅在操作者已有并自行管理入口时使用。`--entry-spec` 是可选的外部入口集成，不是安装前提。

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

Release 归档中的配置模板为 `config-web.yaml`、`config-node.yaml` 和
`config-combined.yaml`；外部入口流程请复制归档内对应模板后再编辑。外部入口需要实现的完整功能清单见
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

安装器会检查配置、依赖和部署健康状态，但不配置 DNS、云安全组或宿主防火墙。操作者须先确认域名解析、端口放行和目标主机；默认 ACME 入口需要可从公网访问的 80/443。安装、卸载及证书变更都直接修改目标主机，生产操作须由操作者审核并执行。

安装成功后，当前拓扑的网络放行清单会写入 `/tpdata/trojanpanelnext-network/allowlist.json`
和 `allowlist.md`。先审阅清单，再由主机或云安全组操作者实施规则；安装器不会自动修改防火墙：

```bash
sudo sed -n '1,240p' /tpdata/trojanpanelnext-network/allowlist.md
sudo jq . /tpdata/trojanpanelnext-network/allowlist.json
sudo test ! -e /tpdata/trojan-panel-core/external/routes.json || \
  sudo jq '.routes[] | {network,port,external_fallback_listener_required}' \
    /tpdata/trojan-panel-core/external/routes.json
```

`ingress` 是主机入站规则，`egress` 是安装所需的 DNS/HTTPS 出站范围；MariaDB、Redis、面板
API/UI、Core API 和 gRPC 不得对 `0.0.0.0/0` 开放。Node 协议端口是内核直连，仅按
`routes.json` 中实际声明的端口放行。来源为空或解析异常时，先补齐配置并重新执行 `validate`。

## 版本化 Release 与候选版本

生产部署应使用同一个版本的 Release 归档、`release-manifest.json`、`SHA256SUMS` 和 `bootstrap.sh`。归档提供 `config-web.yaml`、`config-node.yaml` 和 `config-combined.yaml`；三种部署模式都从对应模板复制配置副本。不要用 `latest` 镜像或从 Git 分支取未固定版本的安装脚本替代 Release。计划中的候选版本标签为 `v0.1.0-rc.1`；在标签和 Release 实际创建并验证前，不应把它当作已发布版本。标签中的版本号为 `0.1.0-rc.1`，此版本生成的镜像和配置会固定到该版本及其摘要，RC 不会更新 Docker `latest`。

推送版本标签会触发 `.github/workflows/publish-images.yml` 并自动创建 GitHub Release。只有获准的发布维护者才应在审核过的提交上创建并推送候选标签：

```bash
TAG=v0.1.0-rc.1
git tag -a "$TAG" -m "TrojanPanel Next $TAG"
git push origin "refs/tags/$TAG"
```

当前发布工作流创建 Release 时未自动设置 GitHub 的 prerelease 标记。流水线成功后、分发或安装候选版前，发布维护者必须检查并将 Release 标记为预发布：

```bash
TAG=v0.1.0-rc.1
gh release view "$TAG" --repo 1linhao/trojanpanelnext --json tagName,isPrerelease
gh release edit "$TAG" --repo 1linhao/trojanpanelnext --prerelease
gh release view "$TAG" --repo 1linhao/trojanpanelnext --json tagName,isPrerelease
```

确认 `tagName` 为 `v0.1.0-rc.1` 且 `isPrerelease` 为 `true` 后再分发。候选版本用于隔离环境验收，不代表已获生产批准；候选验收及人工审核通过后，才由发布维护者决定是否发布稳定版本。

在可信的管理工作站下载并验证三个已签名 Release 文件，然后解包并检查归档内全部资产摘要：

```bash
TAG=v0.1.0-rc.1
VERSION="${TAG#v}"
WORKDIR="./trojanpanelnext-${VERSION}"
ARCHIVE="trojanpanelnext-installer-${VERSION}.tar.gz"
mkdir -m 700 "$WORKDIR"
gh release download "$TAG" --repo 1linhao/trojanpanelnext \
  --pattern "$ARCHIVE" --pattern release-manifest.json --pattern SHA256SUMS \
  --dir "$WORKDIR"
for file in "$ARCHIVE" release-manifest.json SHA256SUMS; do
  gh attestation verify "$WORKDIR/$file" \
    --repo 1linhao/trojanpanelnext \
    --signer-workflow 1linhao/trojanpanelnext/.github/workflows/publish-images.yml
done
mkdir "$WORKDIR/assets"
tar -xzf "$WORKDIR/$ARCHIVE" -C "$WORKDIR/assets"
(cd "$WORKDIR/assets" && sha256sum -c SHA256SUMS)
```

复制版本化模板到权限为 `0600` 的配置副本，编辑域名和其他部署值；先运行只读的 `validate`，检查输出及目标主机后，再明确执行需要 root 的安装：

```bash
cd "$WORKDIR/assets"
cp ./config-web.yaml ./web-site.yaml
chmod 600 ./web-site.yaml
# 编辑 ./web-site.yaml
./bootstrap.sh validate --mode web --config ./web-site.yaml
sudo ./bootstrap.sh install --mode web --config ./web-site.yaml
```

`bootstrap.sh` 会在执行安装器前校验版本、固定资产集、摘要、镜像引用及配置；安装器越过主机变更边界前还会再次验证安全配置快照。归档 attestation 验证发布来源，SHA256 校验归档内资产完整性，两者都应通过。不要在未核实签名者、版本或摘要失败时继续安装。

## 安全与人工门禁

以下检查是每次真实部署的人工放行点。安装器只在本机执行已审核的配置和资产，不替操作者决定这些门禁：

- **发布门禁**：确认候选标签、GitHub Release 的 `isPrerelease` 状态、attestation 签名者、`release-manifest.json` 中的镜像 digest 以及 `sha256sum -c SHA256SUMS` 结果。
- **主机门禁**：确认目标为受支持的 Debian 12、具有 root 权限、没有未授权的既有容器或监听者，并由操作者确认 DNS、80/443 及 Node 协议端口的防火墙范围。
- **入口门禁**：独立部署使用安装器管理的 Caddy；确认域名记录已生效并允许 Caddy 申请/续订 ACME 证书后才运行安装。外部入口和 `--entry-spec` 属于另一条集成流程。
- **凭据门禁**：配置文件和 Node 身份文件保持 `0600`/root-only；Node 引导包通过受信通道传输，口令不放在命令行，CA 私钥和 Web mTLS 私钥不离开 Web 主控。
- **变更门禁**：先执行 `validate` 并保存无秘密输出，安装成功后人工检查 Web、Node 和 mTLS/gRPC 健康状态；`--force`、`remove`、`--purge-data` 以及稳定版切换都要单独确认。

候选版只用于隔离环境验收。验收、日志检查、证书状态检查和生产流量切换完成前，不应把候选版当作稳定版使用。

## Web 主控安装

从已验证的 Release 归档目录复制配置模板：

```bash
cp ./config-web.yaml ./web.yaml
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

## Node Agent 安装

在 Web 主控登记 Node 后，使用同一版本 Release 中的 `node-bundle` 生成加密引导包。先把
Node 模板复制到权限为 `0600` 的工作文件，填写节点域名、Web MariaDB/Redis 地址和镜像；
身份 ID、代次及三组专用凭据会从 `node-identity register|rotate` 生成的文件中注入：

```bash
cp ./config-node.yaml ./node-sg.yaml
chmod 600 ./node-sg.yaml
./node-bundle create \
  --credential-file /tpdata/trojan-panel/config/node-identities/node-sg.g1.json \
  --node-config ./node-sg.yaml \
  --client-ca /tpdata/trojanpanelnext-pki/client-ca.crt \
  --output ./node-sg.g1.age
```

口令默认从终端交互读取并在创建时确认；自动化可设置 `TP_NODE_BUNDLE_PASSWORD`，但口令没有
命令行参数。包固定且仅含 `config-node.yaml`、`manifest.json`、`pki/client-ca.crt`；公开 CA
经过解析校验，包中不含 `client-ca.key`、Web `client.key` 或其他私钥。只需通过可信通道把
`.age` 文件传到 Node 主机。

在 Node 主机上使用与 Web 相同的 Release 安装。安装器在校验 Release 资产后只把明文解到 `/dev/shm`
的私有目录，并在成功或失败退出时清理。默认交互输入口令；非交互场景可把环境变量显式传给
`sudo`：

```bash
sudo ./bootstrap.sh validate --mode node --bundle ./node-sg.g1.age
sudo ./bootstrap.sh install --mode node --bundle ./node-sg.g1.age
# 非交互示例：sudo env TP_NODE_BUNDLE_PASSWORD="$TP_NODE_BUNDLE_PASSWORD" \
#   ./bootstrap.sh install --mode node --bundle ./node-sg.g1.age
```

安装器会先以 Node 的专用身份检查 MariaDB、Redis cache ACL 与 Redis auth ACL，然后等待
Web→Node mTLS/gRPC 验证。在 Node 安装等待期间，从 Web 主控的另一终端执行：

```bash
sudo docker exec trojan-panel /tpdata/trojan-panel/trojan-panel \
  node-identity verify --id <node-identity-id> --challenge <installer-printed-challenge>
```

challenge 由本次安装随机生成并打印，只能使用该次输出。调用使用 Web 持有的客户端证书并校验
Node 服务端证书；Node 的 `/healthz` 只在身份 ID、代次、服务器 ID 与 challenge 全部精确匹配后
就绪，四项检查全部通过安装才返回成功。Node 使用固定生产频率通过新连接复检三组数据层身份，
凭据失效后至多 10 秒退出；轮换或撤销后，旧包不能通过安装检查，已经运行的旧代 Node 也会停止。
新代次需生成新包重装。

如需日后卸载，Node 主机必须保留一份从 bundle 解出的完整配置（其中含专用凭据），而不是只保留
模板 `node-sg.yaml`。解出后立即限制权限；安装器只允许用 `--config` 执行移除，完成后可删除临时解包目录：

```bash
mkdir -m 700 ./node-bundle-extracted
./node-bundle extract --bundle ./node-sg.g1.age --directory ./node-bundle-extracted
install -m 600 ./node-bundle-extracted/config-node.yaml ./node-installed.yaml
rm -rf ./node-bundle-extracted
```

直接使用权限为 `0600` 的 `--config` 仍用于开发、移除和证书刷新，但正式 Node 首装应使用
加密 `--bundle`。

仅当外部宿主管理系统已生成 Protocol v1 EntrySpec 时，才通过 `--entry-spec` 把它交给安装器；普通独立部署不需要此参数：

```bash
sudo ./install.sh install --mode node --config ./node-sg.yaml \
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

combined 卸载后，使用原 combined 配置重跑不会自动恢复已移除的角色；必须明确指定恢复角色，
并先确认该角色的域名、端口和数据仍属于本次部署。Entry journal 必须仍有与当前目标匹配的
`committed_target.digest`（64 位十六进制摘要），否则必须先做显式迁移：

```bash
sudo ./install.sh install --mode combined --config ./combined-site.yaml --restore-role web
```

`--restore-role` 只接受当前处于 inactive 状态的 `web` 或 `node`，普通重跑和证书刷新不会隐式恢复角色。

删除服务及生成数据：

```bash
sudo ./install.sh remove --mode node --config ./node-installed.yaml --purge-data
```

combined 使用两个不同且都解析到本机公网 IP 的域名。单个共享入口拥有 80/443，Node 内核协议
端口保持直连；Node 使用 Web 主控在本机签发的独立 MariaDB/Redis 身份。安装与证书续订后的消费方
刷新命令为（Release 归档中的 `config-combined.yaml` 应先复制成权限为 `0600` 的工作文件）：

```bash
cp ./config-combined.yaml ./combined-site.yaml
chmod 600 ./combined-site.yaml
# 编辑 ./combined-site.yaml
./bootstrap.sh validate --mode combined --config ./combined-site.yaml
sudo ./bootstrap.sh install --mode combined --config ./combined-site.yaml
sudo ./bootstrap.sh refresh-cert --mode combined --config ./combined-site.yaml
```

`refresh-cert` 不接管 ACME；共享入口仍负责续订。它只在 Node 证书代次变化时重启 Core。
使用 combined 配置配合 `remove --mode web|node` 可移除单个角色：共享入口会保留并重写为剩余域名，
MariaDB/Redis 等共享资源不会被单角色卸载误删；Node 移除会先撤销其控制面身份。
即使先移除 Web，再移除 Node，安装器也会用一次性控制面 CLI 撤销该身份。combined 安装
拒绝接管已有的独立 Node 入口；成功前还会验证 Web→Node mTLS/gRPC。

## 配置文件

Release 的 `config-web.yaml` 包含域名、镜像、服务端口、mTLS 身份目录以及主控内部凭据。

Release 的 `config-node.yaml` 包含节点域名、主控数据库连接、Redis 连接、gRPC、公开 CA 目录与证书路径。

Release 的 `config-combined.yaml` 包含 Web 与 Node 双域名、本机 Node 公网 IP、独立 Node 身份
凭据路径及共享入口端口。`node_identity_credential_file` 必须位于
`/tpdata/trojan-panel/config/node-identities/` 下并保持 root-only。

外部 TLS 模式仍使用 `config-web.yaml` 或 `config-node.yaml`，并在复制出的工作文件中设置以下键：

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

## 发布资产构成与校验边界

正式发布工作流使用 `release/generate-assets.sh` 生成同一版本的 `bootstrap.sh`、
`install.sh`、`node-bundle`、用于安全打开/原子写入敏感配置的 `secure-file`、`web|node|combined` 配置模板、`release-manifest.json` 和
`SHA256SUMS`。产品镜像和运行时镜像都以 `name@sha256:<digest>` 固定；
`bootstrap.sh` 会先调用同包内的 `verify-assets.sh` 校验版本、资产摘要、镜像引用和配置；
发布包内的 `install.sh` 在被直接调用时也会执行同一预检。验证器只依赖 Debian 12
基础系统提供的 Bash、awk、grep 与 coreutils，不要求宿主预装 `jq`；它先依据固定资产集合校验
`SHA256SUMS`，拒绝 bundle 路径中的符号链接，并只接受生成器输出的 printable ASCII + LF manifest；
且不会在此之前 source 或执行其他随包程序。两条入口只有在完整校验 15 个资产（包括
`secure-file` 和 `node-bundle`）后才首次执行 helper，并在安全快照后再次验证配置契约；全部通过后才越过宿主变更边界。

发布流水线保留产品镜像的 SBOM 与最大级别 provenance，并为三张产品镜像和三个 Release
文件（归档、manifest、SHA256SUMS）创建 GitHub artifact attestation。流水线用固定的发布工作流
签名者重新验证这些 subject/digest；本地下载时也应指定相同签名者，而不是只校验仓库名：

发布配置契约使用 `deployment_mode`、`api_image`、`web_image` 和 `node_agent_image`；旧的
`purpose`、`panel_image`、`ui_image` 和 `core_image` 只供既有安装配置兼容读取，不会出现在新模板中。

`combined` 由安装器正式编排：Web 与 Node 复用本机数据服务和单一共享入口，并为两个域名维护
独立证书；Node 内核监听不会转发到统一 L4 入口。无秘密的 manifest 结构示例见
[example-release-manifest.json](release/example-release-manifest.json)。Release 使用 tar.gz 保留脚本可执行位，并同时附带 manifest 与 SHA256SUMS。按上一节先验证三个 Release 文件的 attestation，再解包并验证包内摘要；RC 与稳定版本必须按明确标签下载，不能从 `latest` 选择候选版：

```bash
archive=trojanpanelnext-installer-<version>.tar.gz
mkdir trojanpanelnext-installer
tar -xzf "${archive}" -C trojanpanelnext-installer
cd trojanpanelnext-installer
sha256sum -c SHA256SUMS
```

复制对应模板并编辑副本，不要修改发布包中受摘要保护的模板；然后验证实际配置：

```bash
cp ./config-web.yaml ./deployment.yaml
./bootstrap.sh validate --mode web --config ./deployment.yaml
```

`verify-assets.sh` 也可单独检查下载包和配置；日常安装请通过 `bootstrap.sh`，使校验先于安装器执行。

## 支持

本项目来源：[TrojanPanel 原项目](https://github.com/trojanpanel)。
