# 上游提案：trojanpanelnext 安装器增加「外部 TLS / 反向代理」模式

> 用途：这是一份可以直接贴到 <https://github.com/1linhao/trojanpanelnext/issues> 的提案稿。
> 标题建议：**建议为 deploy/installer 增加「外部证书与反向代理」安装模式（external TLS）**。
> 依据：`docs/调研-trojanpanelnext-installer.md`（main 分支 `deploy/installer` 的逐字段调研）。
>
> **落地状态（本仓库 `feat/external` 分支）**：已按**变体 2** 实现，键名与提案草稿略有差异，
> 外部实现方所需的完整接口规范已单独成文：`docs/外部入口实现契约.md`。实现要点见文末
> 「§6 已实现内容」。

## 1. 我们是谁、想做什么

我们维护一个声明式的 VPS 管理系统：把「一台机器该长成什么样」写成模板，接管 SSH 之后自动
装好并长期保持。trojanpanelnext 是我们希望托管的一类服务，用 `install.sh install --mode web|node
--config F` 非交互安装这一点非常适合自动化——先谢谢你们把这个安装器做成了无交互、单入口、
一份 YAML 的形态。

我们这套系统在同一台机器上还有别的服务，因此有两条**全局约定**（对一台机器而不是对某个服务）：

1. **80 端口由宿主 nginx 长期持有**：它只把 `/.well-known/acme-challenge/` 指向我们的挑战目录，
   其余请求 444。证书由我们的 certd（certbot webroot）签发与自动续订，导出到
   `/etc/vps-factory/certs/<域名>/{fullchain.pem,privkey.pem}`（目录 0700、私钥 0600），
   续订成功后我们 reload nginx。
2. **443（以及各服务的对外端口）由宿主 nginx 做反向代理**：每个服务一个配置文件
   `/etc/nginx/conf.d/<服务名>.conf`，证书直接用上面那个导出目录。

也就是说：**证书与「谁监听 80/443」这两件事，我们希望由外部统一接管。**

## 2. 现状与冲突（基于 main 分支 `deploy/installer` 的实际内容）

| 上游行为 | 出处 | 与我们冲突的点 |
| --- | --- | --- |
| 反代用 caddy 容器：`--network=host --restart always` | `install.sh` | 与宿主 nginx 抢端口 |
| web 的 Caddyfile 是 `${TP_WEB_DOMAIN} { reverse_proxy 127.0.0.1:${UI_PORT} }`，**没有** `http_port`/`https_port` | `install.sh` 生成的 `/tpdata/custom/web-caddy/Caddyfile` | Caddy 默认占宿主 **80 与 443** |
| node 的 Caddyfile 显式 `http_port 80` / `https_port 8863` | 同上 | 占宿主 **80**（8863 可配，80 不可） |
| Caddy 自己走 ACME（Let's Encrypt）签发与续订 | Caddyfile 未指定 `tls` | 与我们的 certd 重复签发；且需要 80/443 |
| node 安装会 `wait_for_cert()` 最多 60×3 秒，超时 `exit 1` | `install.sh` | 我们的 nginx 占着 80 时，Caddy 签不出证书，node 安装直接失败 |
| node 的 core 从 **Caddy 的存储目录**读证书：`${TP_DATA}/custom/node-caddy/data/caddy/certificates/acme-v02.api.letsencrypt.org-directory/${domain}/${domain}.crt|.key` | `install.sh` 写入 core 的 `config.ini` | 证书的**来源与存放位置被绑定在 Caddy 上**，外部提供的证书没有入口 |
| web 侧不等证书，签不下来也算安装成功 | `install.sh` | 冲突在 web 上是**静默**的：日志里什么都看不出来 |

结论：只要宿主上有别的东西占着 80/443，这个安装器要么失败（node），要么悄悄装出一个用不了
TLS 的 web（Caddy 起不来或证书拿不到）。而反过来，如果让上游 Caddy 占着 80/443，我们这台机器
上的其它服务就没法用统一的入口与证书。

## 3. 诉求

希望 `deploy/installer` 增加一个**「证书与反向代理不由安装器负责」的模式**，让外部入口可以接管。
下面两个变体任选其一即可解决我们的问题，**变体 1 的改动量最小，我们推荐它**。

### 变体 1（推荐，最小改动）：保留 Caddy，但让它用外部证书、不占 80/443

配置新增（默认值保持现状，老用户零影响）：

```yaml
trojanpanelnext:
  schema_version: 1
  purpose: node                 # 或 web
  hostname: node1.example.com
  # 新增段
  tls:
    mode: external              # acme（默认，= 现在的行为）| external
    cert_path: /etc/vps-factory/certs/node1.example.com/fullchain.pem
    key_path: /etc/vps-factory/certs/node1.example.com/privkey.pem
    # 可选：证书更新后要执行的命令（我们会传 `systemctl reload nginx` 之类；留空则不执行）
    reload_command: ""
  # web 也补上这两个键（node 已有）
  web_caddy_http_port: 8080     # 名字随你们；web 现在完全没有这个旋钮
  web_caddy_https_port: 8443
```

`mode: external` 时安装器的行为：

1. 生成的 Caddyfile 用 `tls <cert_path> <key_path>`（**不启用 ACME**），并用配置里的
   http/https 端口（web 也能挪开 80/443）；
2. 不因证书不存在而失败：node 的 `wait_for_cert()` 改为等待 `cert_path`/`key_path` 出现
   （两者都在、且能被读取即可），或者直接跳过这一步；
3. node 写入 core 的证书路径改用 `tls.cert_path` / `tls.key_path`，不再硬编码
   `custom/node-caddy/data/caddy/certificates/...`；
4. `reload_command` 若非空，在证书被替换后由你们决定是否执行（也可以只在 `validate` 里提示
   外部自己负责续订后的重载）。

### 变体 2（彻底）：不跑 Caddy，全部交给外部入口

`mode: external` 时安装器不创建、不启动 `*-caddy` 容器，也不写 Caddyfile；对外只暴露
「面板 UI 端口 / 核心需要的端口」，TLS 终止、伪装站与协议回落全部由外部入口负责。
这一条对你们改动更大（要回答「没有 Caddy 时伪装站与回落由谁承担」），我们只在你们认为
架构上更干净时才建议走这条路。

## 4. 我们愿意承担的部分

- 新键的默认值保持现状（`tls.mode: acme` = 现在的行为），**对现有用户零影响**；
- 我们可以提 PR，也可以按你们的风格补 `deploy/installer/tests/` 里的契约测试用例
  （例如 `--mode node` + `tls.mode: external` 时不出现 `--restart always` 的 caddy 容器、
  Caddyfile 里不出现 ACME 相关配置）；
- 我们这边会把 installer 固定到某个 commit + sha256 再调用（供应链上不能只认 main）。
- 我们只需要一个**能被自动化调用的稳定契约**；具体键名叫什么、放在 `tls` 还是顶层，完全听你们的。

## 5. 为什么不能改成「让 Caddy 继续管 80/443」

我们不是不愿意少写点代码，而是这台机器上 80/443 必须由统一入口持有：同一台机器上还要跑
其它服务（同步、密码库等），它们的域名、证书、HTTP→HTTPS 跳转、以及 80 端口上的 ACME
挑战都走同一个入口。一个服务独占 80/443，等于这台机器上不能再放别的对外服务。

## 附：另一件建议单独开 issue 的事（安全）

`apps/control-plane/api/dao/mysql.go` 的 `sqlInitStr` 在数据库为空时会 seed 一条**仓库内硬编码**
的管理员记录：

```sql
INSERT INTO account VALUES (1,'sysadmin','<pass>','<hash>',-1,…)
```

我们本地核对过：`SHA-224(pass) == hash`（56 位十六进制，与源码里的 `util.SHA224String` 一致）。
也就是说**每一次全新部署都会带一个部署间完全相同、且公开可查的 sysadmin 凭据**，
而 `install.sh` 只会打印 `Default username: sysadmin`。

建议（我们这边也会做，但根因在上游）：首次启动改为生成随机口令并打印一次，或要求
`install` 时通过配置/环境变量提供初始口令；至少在 README 里明确提示「装完立刻改密」。

## 6. 已实现内容与后续修正（本仓库）

采纳**变体 2**：`tls_mode: external` 时安装器不创建任何反代容器，外向暴露收窄为
「面板入口 + 内核协议端口」。进一步审查内核真实监听与 TLS 模型后，入口采用分层职责：Web TLS
由外部入口终止；Node 内核端口直接监听并自行终止 TLS；外部入口只承载 HTTP-01 和明确要求的
明文伪装站 fallback，不默认代理全部内核流量。

配置键改为**扁平键**（不采用本提案草稿里的嵌套 `tls:` 段）：`cfg_apply` 按顶层键读取配置，
嵌套对象会引入第二套读取路径；扁平键同时满足「默认值保持现状、老用户零影响」。

| 提案草稿 | 实际实现 | 说明 |
| --- | --- | --- |
| `tls.mode` | `tls_mode` | `acme`（默认）/ `external` |
| `tls.cert_path` + `tls.key_path` | `tls_cert_dir`（+ 可选 `tls_cert_file`/`tls_key_file`） | 安装器在目录内自动配对，复制到 `${TP_DATA}/trojan-panel-core/cert/` 并以只读方式挂给内核 |
| `tls.reload_command` | **未实现** | 安装器只在被调用时运行一次，安装时执行续订钩子会产生误导；续订→重跑安装器→重启内核实例的流程写入契约文档 |
| `web_caddy_http_port` 等 | 未新增 | 变体 2 下没有 Caddy，端口旋钮失去意义；`caddy_image`/`node_caddy_*` 在该模式下被读取但不使用 |
| — | `bind_address` | 只控制面板 UI 的监听地址（建议 `127.0.0.1`）；不控制 Core、gRPC 或内核监听 |

安装器负责伪装站内容（`/tpdata/web`）与托管证书。节点 agent 把当前服务的每个内核 listener
写成机器可读的 `routes.json`；它用于端口/防火墙检查、443 冲突检查和必要的明文 fallback，
不是 nginx `stream` 动态配置源。完整规范见 `docs/外部入口实现契约.md`，后续 Module 设计见
`docs/entry-controller/方案设计.md`。
