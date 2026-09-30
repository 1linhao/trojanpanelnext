# 安装 TrojanPanel Next

下载命令入口 `tp.sh`，获取配置模板，编辑 YAML 后执行安装。入口按命令下载 `common.sh` 和对应子脚本，安装时还会下载卸载脚本，以便部署 Node 宿主机维护服务。`trojanpanelnext.purpose` 是部署类型的唯一来源：`web` 部署主控，`node` 部署节点 Agent。所有命令均无交互。

## 系统与软件依赖

支持 Ubuntu 20.04+、Debian 11+ 或同类 Linux，CPU 为 `linux/amd64` 或 `linux/arm64`，建议至少 1 GiB 内存。安装和卸载需要 root。Node 主机必须运行 systemd。

**安装器不会自动安装 yq、Docker 或其他软件。请先安装依赖，再执行安装命令。**

| 命令 | 预装依赖 |
| --- | --- |
| 所有入口命令 | Bash、curl、CA 证书、grep、coreutils（包含 mktemp、chmod、rm） |
| `config` | 上述依赖，coreutils 的 ln、dirname，目标目录可写 |
| `validate` | 上述依赖及 [mikefarah/yq v4](https://github.com/mikefarah/yq)，与 Python 的同名 yq 不兼容 |
| `install` | 上述依赖、Docker Engine、mikefarah/yq v4、OpenSSL、tar、coreutils、findutils、awk；Node 还需要 systemd |
| `remove` | 上述依赖、Docker Engine、mikefarah/yq v4、coreutils（含 realpath、rmdir）；Node 维护服务清理需要 systemctl |

Debian/Ubuntu 可先执行：

```bash
sudo apt-get update
sudo apt-get install -y bash grep curl ca-certificates openssl tar coreutils findutils gawk docker.io
sudo systemctl enable --now docker
```

按[安装器说明](https://github.com/1linhao/trojanpanelnext/blob/main/deploy/installer/README.md#系统与软件依赖)中的 amd64/arm64 下载、SHA-256 校验步骤手工安装 mikefarah/yq v4.53.6。完成后检查：

```bash
yq --version
sudo docker info >/dev/null
```

以 root 登录时可省略 `sudo`。其他发行版使用对应包管理器安装依赖。Docker 服务必须已启动。

## 版本绑定与获取入口

当前版本为 `0.1.0-rc.9`。每台 Web 或 Node 服务器都执行：

```bash
curl -fsSL --connect-timeout 10 --max-time 60 \
  https://raw.githubusercontent.com/1linhao/trojanpanelnext/v0.1.0-rc.9/deploy/installer/tp.sh \
  -o tp.sh
chmod +x tp.sh
./tp.sh --version
```

入口默认从 GitHub Raw 的 `v0.1.0-rc.9` 下载所选命令及 `common.sh`，安装时额外下载 `uninstall.sh`。下载文件经过版本匹配检查和 Bash 语法检查，在临时目录执行，结束后清理。配置模板也默认使用同一标签；API、Web 和 Node Agent 镜像使用 `:0.1.0-rc.9`。Git 标签带 `v`，镜像标签不带 `v`。无需本地仓库。

`schema_version: 1` 是 YAML 配置结构版本，与产品版本分别维护。已有配置应使用 `trojanpanelnext.purpose: web` 或 `node`；旧部署类型参数不再受支持。

## 命令入口与子脚本

```text
./tp.sh config web|node [--output <file>]
./tp.sh validate --config <file>
./tp.sh install --config <file> [--force]
./tp.sh remove --config <file> [--keep-data | --purge-data]
```

`--purge` 是 `--purge-data` 的别名，不能与 `--keep-data` 合用。`--force` 只用于安装；数据选项只用于卸载。`./tp.sh <command> --help` 查看命令帮助。

| 入口命令 | 对应子脚本及直接调用方式 |
| --- | --- |
| `config` | `bash config.sh web\|node [--output <file>]` |
| `validate` | `bash validate.sh --config <file>` |
| `install` | `bash install.sh --config <file> [--force]` |
| `remove` | `bash uninstall.sh --config <file> [--keep-data \| --purge-data]` |

直接运行子脚本时需要同目录、同版本的 `common.sh`；Node 安装还需要同目录的 `uninstall.sh`。子脚本不再接受 `config`、`validate`、`install` 或 `remove` 前缀。只下载单独的 `install.sh` 无法使用；推荐通过入口下载匹配的文件。

## 网络准备

域名的 A/AAAA 记录应解析到各自服务器，所用端口不能被其他服务占用。

| 方向 | 默认端口及用途 |
| --- | --- |
| 浏览器、证书签发服务 → Web | TCP 80、443：Caddy 与 HTTPS 面板 |
| Node → Web | TCP 9507：MariaDB；TCP 6378：Redis，只允许受信任 Node 来源 |
| Node 宿主机维护服务 → Web | TCP 443：HTTPS 卸载结果确认回调 |
| 证书签发服务 → Node | TCP 80：默认 HTTP 域名验证 |
| Web → Node | TCP 8100：mTLS gRPC 控制通道，只允许 Web 来源 |
| Web → Node 宿主机维护服务 | TCP 8101：mTLS HTTPS 卸载通道，固定为 `grpc_port + 1`，只允许 Web 来源 |
| 访问节点伪装站 → Node | TCP 8863：默认 Caddy HTTPS 端口 |
| 代理客户端 → Node | 面板中实际配置的代理 TCP/UDP 端口 |

API 8081、UI 8888、Node API 8082 使用主机网络监听，不应统一对公网放行。更改端口时同步调整防火墙和 YAML；维护端口不能单独配置，`grpc_port` 最大为 `65534`。AAAA 记录存在时也必须保证 IPv6 可达。

## 安装 Web 主控

```bash
./tp.sh config web
nano web.yaml
./tp.sh validate --config ./web.yaml
sudo ./tp.sh install --config ./web.yaml
```

编辑 `hostname` 和 `email`。`config` 默认生成 `web.yaml`，权限为 `0600`；已有目标文件或符号链接会直接拒绝，下载中断不会留下半截目标配置。也可使用 `--output ./production-web.yaml`。

首次安装会生成 MariaDB 与 Redis 密码，写回 `web.yaml`，并在 `pki_bundle_dir` 生成主控 mTLS 身份。请保留配置和 PKI，实际配置含敏感凭据，不要提交到 Git。

安装后访问 `https://<hostname>`；首次登录用户名 `sysadmin`、默认密码 `123456`，登录后及时修改。数据库与 Redis 密码不是面板登录密码。

## 安装 Node Agent

先安装 Web，在面板中创建节点服务器：填写 Node 地址、gRPC 端口和 TLS 服务器名（Node 域名），取得实际服务器 ID 后填入 Node YAML 的 `node_server_id`。ID 必须至少为 `1`；不要照抄模板示例 ID。

在 Node 的 `pki_bundle_dir`（默认 `/tpdata/trojanpanelnext-pki`）准备 Web 生成的 `client-ca.crt`。通过可信文件传输复制公开 CA，**不要复制 `client-ca.key`、`client.key` 或 `client.crt`**。

```bash
./tp.sh config node
nano node.yaml
./tp.sh validate --config ./node.yaml
sudo ./tp.sh install --config ./node.yaml
```

填写 Node 的 `hostname`、`email`，Web 的 `mariadb_host`、`redis_host` 和连接密码；核对端口以及 `node_server_id` 与面板中实际服务器 ID 一致。模板中的 ID 和密码都是示例值。Node 需要连接 Web 的数据库和 Redis，宿主机维护服务还需要访问 Web HTTPS（TCP 443）完成卸载结果确认；安装器不自动注册服务器或获取主控 CA。

Node 安装在宿主机部署 `trojanpanelnext-host.service`，提供 mTLS HTTPS 维护接口，端口固定为 `grpc_port + 1`（默认 `8101`）。使用防火墙仅允许 Web 来源；它使用本地保存的配置及卸载脚本执行卸载。Agent 容器不挂载 `docker.sock`。安装后可执行 `sudo systemctl status trojanpanelnext-host.service` 检查服务。

`validate` 只校验 YAML 和配置字段，不检查 DNS、证书文件、网络连通性、systemd 状态或服务健康。通过校验不等于已经具备完整部署条件。

### 使用已有证书，跳过 Node Caddy

先由宿主机的 Nginx＋Certbot 或其他工具签发证书，在完整 Node YAML 中设置：

```yaml
trojanpanelnext:
  # 保留其他数据库、Redis、mTLS 和服务器 ID 配置。
  hostname: node.example.com
  node_certificate_mode: external
  node_certificate_path: /etc/letsencrypt/live/node.example.com/fullchain.pem
  node_private_key_path: /etc/letsencrypt/live/node.example.com/privkey.pem
```

```bash
./tp.sh validate --config ./node.yaml
sudo ./tp.sh install --config ./node.yaml
# 已安装 Node 切换模式或路径时：
sudo ./tp.sh install --config ./node.yaml --force
```

| 配置 | 含义 |
| --- | --- |
| `node_certificate_mode: caddy` | 默认值，启动 Node Caddy 并申请、续签证书。 |
| `node_certificate_mode: external` | 使用现有 PEM；跳过 Node Caddy、申请和等待；Node Caddy 端口配置不使用。 |
| `node_certificate_path` | 宿主机完整证书链的绝对路径，例如 Certbot `fullchain.pem`。 |
| `node_private_key_path` | 与证书配对、未加密的 PEM 私钥绝对路径。 |

安装检查文件可读、有效期、证书覆盖 `hostname`、私钥配对。Agent、代理和宿主机维护服务使用同一对路径；代理使用其他域名时，也须由这张证书覆盖，可使用多域名 SAN 或通配符证书。此模式无需占用签证端口 80/8863；Nginx、Certbot 及代理业务监听由宿主机和面板分别配置。安装器不自动设置 SNI 分流。

证书目录及符号链接目标目录均只读挂载，支持 Certbot 更新 `live/<domain>` 的文件链接和 `archive/<domain>` 的版本文件。保持路径和目录稳定；移动文件或把链接指向其他目录后，加 `--force` 重新安装以更新挂载。请使用独立证书目录：不接受直接挂载 `/`、`/etc`、`/root` 等宽泛目录，证书也不能位于项目数据、PKI、伪装站或维护目录中。

续签后，gRPC、宿主机维护服务及 Hysteria2 在新 TLS 握手时读取文件；NaiveProxy 每分钟检测，证书变化后短暂重启受影响的实例；Xray 使用文件热加载，默认周期为一小时，可在面板重启该代理立即加载。Nginx 使用同一证书时，由 Certbot deploy hook 检查并 reload Nginx。

切换时迁移原有代理配置中与旧证书、私钥路径相同的引用，保留账户和其他参数；自定义的其他证书引用保留。`--force` 成功切换后移除原项目 Node Caddy 容器，旧证书数据保留到项目数据清理时。外部证书在本地两种卸载模式及 Web 远程卸载中均保留，Nginx、Certbot 及其配置也保留；与项目清理路径重叠时会拒绝操作。

## 重建与升级

修改配置中的应用镜像后，使用 `--force` 重新创建 API、UI、Agent 或 Caddy 容器。升级到 `0.1.0-rc.9` 时，先把所有 Node YAML 的 `core_image` 更新到 `:0.1.0-rc.9`，再更新 Web YAML 的 `panel_image` 和 `ui_image`；在各主机下载此版本入口，按以下顺序执行：

```bash
# 先在每台 Node 主机执行，安装宿主机维护服务
sudo ./tp.sh install --config ./node.yaml --force
# 所有 Node 更新成功后，在 Web 主机执行
sudo ./tp.sh install --config ./web.yaml --force
```

升级前在已有 Node YAML 中填写真实的 `node_server_id`，并放行仅限 Web 来源的维护端口。旧版 Node 未部署维护服务时不能通过 Web 完成宿主机卸载。

`--force` 不重建已有 MariaDB、Redis 容器。安装器通常复用本地镜像，重建应用容器时会拉取配置指定的镜像。使用未发布测试镜像时请勿执行 `--force`。

## 卸载

在对应服务器执行。保留数据卸载：

```bash
sudo ./tp.sh remove --config ./web.yaml --keep-data
sudo ./tp.sh remove --config ./node.yaml --keep-data
```

彻底卸载：

```bash
sudo ./tp.sh remove --config ./web.yaml --purge-data
sudo ./tp.sh remove --config ./node.yaml --purge-data
```

| 行为 | `--keep-data` | `--purge-data` / `--purge` |
| --- | --- | --- |
| 对应用途的容器及匿名 volume | 删除 | 删除 |
| 容器与配置所用镜像仓库的全部本地 tag | 删除未被其他容器使用的镜像 | 删除未被其他容器使用的镜像 |
| 业务数据、PKI、伪装站、自定义内核运行目录 | 保留 | 删除所有项目服务目录及这些路径 |
| 原始部署 YAML | 保留 | 删除 |
| 安装的宿主机维护服务及其工作副本 | 删除 | 删除 |
| 手工下载的 `tp.sh` 与 Docker Engine 本身 | 保留 | 保留 |

Web 对应 MariaDB、Redis、API、UI、Web Caddy 容器；Node 对应 Agent、Node Caddy 容器。卸载包括配置中的镜像仓库及容器实际使用的镜像仓库，也包括对应用途默认 GHCR 仓库的全部本地历史 tag；仍被其他容器引用的共享镜像保留并提示。不执行 `docker system prune` 或全局镜像清理。

彻底卸载会清理项目的所有服务数据目录（含已停用用途遗留目录）、`pki_bundle_dir`、伪装站 `WEB_PATH`、自定义 `kernel_runtime_path`、配置的身份文件和原始 YAML；Node 的维护文件位于 `/etc/trojanpanelnext-host`、`/usr/local/lib/trojanpanelnext-host` 和 `/etc/systemd/system/trojanpanelnext-host.service`。当同一宿主机仍存在另一用途的项目容器时，会在删除前拒绝彻底卸载，避免破坏共享数据。

不加数据选项时使用 YAML 的 `purge_data`，默认模板为 `0`。`--keep-data` 明确覆盖 YAML 的 `purge_data: 1`；`--purge-data` 明确启用彻底卸载。通常保持 YAML 的 `force`、`purge_data` 为 `0`，在命令行选择操作。

### 从 Web 删除节点服务器

点击节点服务器页面的“删除”按钮后，弹窗提供“取消”“删除”“彻底删除”。选择“删除”会先调用 Node 的宿主机维护服务，以保留数据模式卸载容器和镜像；成功回报后，Web 在事务中删除服务器及关联代理配置，保留该服务器的流量和内核任务历史。“彻底删除”使用彻底卸载模式，同时清理该服务器对应的 Web 流量、内核任务记录。涉及其他服务器的共享任务记录不会一并删除。

节点离线、旧版 Node 没有维护服务、mTLS/网络失败或本地卸载失败时，Web 报错并保留服务器登记与关联代理配置；连接失败不会当作卸载成功。单个代理节点的删除仍只删除该代理，不卸载宿主机。

远程卸载期间，维护服务暂存 TLS 身份与卸载回执，待 Web 成功提交删除事务、双方完成结果确认后，再清理维护服务及其文件。结果确认和辅助服务清理失败时会依据持久化待办在后台重试，并支持服务或主机重启后恢复。界面显示“清理待完成”（`cleanupPending`）时，机器的主要服务已卸载，Web 记录已删除，暂存维护文件还在等待清理。保持 Web 到 Node 维护端口、Node 到 Web HTTPS（TCP 443）连通，直到清理完成。

本地 `tp.sh remove` 只操作本机，不同步清理 Web 登记。如需同时卸载 Node 和清理登记，请直接在 Web 的节点服务器页面发起删除。

### 删除后重新接入 Node

“删除”保留 Node 的数据、PKI、HTTPS 证书和原始部署 YAML，可以重新安装接入。它已删除 Web 中原有服务器及关联代理配置，因此不会自动恢复原来的代理节点。

1. 等待宿主机维护服务完成清理，在 Web 重新添加服务器（可以使用相同名称和地址），取得新服务器 ID。
2. 修改保留的 `node.yaml`，将 `trojanpanelnext.node_server_id` 改为新的 ID；核对数据库、Redis 和 TLS 配置。不能继续使用旧 ID。
3. 在 Node 执行 `sudo ./tp.sh install --config ./node.yaml`。安装器会重新下载镜像，复用保留的数据和有效证书，重新部署维护服务。
4. 在 Web 检查服务器在线后，重新创建需要的代理节点。原流量和内核任务历史保留在旧服务器 ID 下，不会迁移到新 ID。

“彻底删除”会清除上述本地配置及数据，再次接入需按首次安装步骤准备配置和 Web 公共 CA。


## 脚本与模板来源

`TP_SCRIPT_REF` 默认是 `v0.1.0-rc.9`，可用 GitHub 上实际存在的分支或完整提交 SHA 同时覆盖子脚本和模板来源；入口仍要求所有脚本版本匹配。测试时也应从相同 ref 获取 `tp.sh`。`TP_CONFIG_REF` 优先级更高，可单独覆盖模板来源；已有 YAML 的镜像标签不会随这些变量改变。需要 root 的命令通过 `sudo env TP_SCRIPT_REF=<ref> ./tp.sh ...` 显式传递变量。

完整参数与开发发布校验见[安装器说明](https://github.com/1linhao/trojanpanelnext/blob/main/deploy/installer/README.md)。

## 证书自动维护

- Web 与 Node 的公网域名证书由 Caddy 自动续签。保留 Caddy 数据目录，并保持域名解析及 ACME 验证端口可达。
- Node Agent 每分钟检查 NaiveProxy 的证书。新证书与私钥有效时，保存运行配置和用户信息、校验配置，再重启对应实例；现有连接会短暂中断，客户端需重新连接。失败会记录日志并在下次检查时重试。
- Web API 启动时及每 5 分钟检查内部 mTLS 身份，客户端证书剩余不足 90 天时重新签发；新证书有效期最多 825 天，且不超过 CA 到期时间。CA 私钥仅保留在 Web。
- CA 剩余不足 365 天时开始轮换：通过现有 mTLS 通道分发新旧 CA，所有已登记的 mTLS Node 确认后切换客户端身份。保留旧身份至少 24 小时，待所有节点确认移除旧 CA 后清理旧私钥。离线或不支持轮换的旧版 Node 会阻止推进，每 5 分钟重试。
- 首次引导仍需手动复制 Web 当前的 `client-ca.crt`。新节点请先登记到面板并使用当前公开 CA 文件；未登记节点不在轮换范围。升级先更新所有 Node，再更新 Web；修改现有 YAML 镜像标签为 `0.1.0-rc.9`，执行 `./tp.sh install --config ... --force` 以更新挂载并部署 Node 宿主机维护服务。Node 重装会保留已有运行时 CA 文件，避免旧引导副本覆盖新信任；Agent 同时更新引导 CA 副本，供清理运行数据后的重新安装使用。
- Web API 挂载 `pki_bundle_dir` 并拥有签发权限，Node 仅能写入公开 CA 信任文件。完整备份 Web 的 PKI 目录（包含 `state.json` 和 `generations`），不要只备份根目录的符号链接。证书轮换失败可查看 API / Agent 日志。节点离线超过旧 CA 的有效期或恢复过期的备份时，需要人工重新引导信任。
