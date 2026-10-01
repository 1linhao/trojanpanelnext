# 证书管理

[文档首页](README.md) · [部署指南](deployment.md#certificate-maintenance)

## 目录

- [公网证书](#public-certificates)
- [外部证书](#external-certificates)
- [证书加载](#certificate-loading)
- [Web 与 Node 的 mTLS](#mtls)
- [备份与故障处理](#recovery)

<a id="public-certificates"></a>
## 公网证书

Web HTTPS 由 Web Caddy 管理。Node 默认由独立 Node Caddy 申请与续签域名证书，代理和管理接口读取该证书。续签需要域名 DNS 正确、ACME 验证端口可达，并保留 Caddy 数据目录以复用签发状态。

<a id="external-certificates"></a>
## 外部证书

Node 的 `node_certificate_mode: external` 跳过 Node Caddy 和 ACME，读取配置指定的 fullchain PEM 与未加密私钥。证书必须在有效期内、匹配 Node 域名并与私钥配对；代理使用的其他域名也须被证书覆盖。

Certbot、acme.sh 或其他外部工具负责申请与续签。脚本只读挂载证书和私钥所在目录，以及符号链接最终指向的目录，支持 Certbot `live/` 到 `archive/` 的更新。保持路径和目录稳定；改变证书路径或目标目录后，使用当前配置重新部署 Node。

外部证书目录须独立于项目数据和可写 mTLS 信任目录。项目卸载保留外部证书、Nginx、Certbot 和相关配置。详细参数见[外部证书部署](deployment.md#external-certificates)。

<a id="certificate-loading"></a>
## 证书加载

| 服务 | 证书变化后的行为 |
| --- | --- |
| Web Caddy | 管理并更新 Web 公网 HTTPS 身份 |
| Node gRPC / 宿主机维护 HTTPS | 新 TLS 握手读取当前证书与私钥 |
| Hysteria2 | 新握手读取当前证书 |
| NaiveProxy | 每分钟检查实际证书；保存运行配置及用户，重启变更实例加载新证书，期间可能短暂中断 |
| Xray | 按其证书文件加载机制更新，默认约一小时；面板重启对应代理可立即加载 |

NaiveProxy 会拒绝无效的替换证书，维护失败会重试。续签工具写入证书与私钥时应按其标准机制更新文件，避免留下不匹配的文件对。

<a id="mtls"></a>
## Web 与 Node 的 mTLS

Web API 维护内部 CA 与客户端身份。CA 私钥和 Web 客户端私钥只保留在 Web，Node 首次部署仅接收公开 `client-ca.crt`。CA 与客户端身份以完整 generation 保存，`state.json` 记录当前状态，新握手读取同一 generation 的证书和私钥。

Web 每 5 分钟检查证书。客户端身份剩余有效期不足 90 天时续签，最多有效 825 天且不超过 CA。CA 剩余不足 365 天时准备新 CA，通过已认证的 mTLS 通道向全部登记的 mTLS Node 分发双 CA；全部确认后才切换客户端身份。

切换后保留旧身份至少 24 小时。清理前再次分发双 CA，再分发单独的新 CA，全部节点确认后删除退休私钥。离线节点会阻止轮换推进并触发重试。Node 在新握手时读取更新后的信任文件。

<a id="recovery"></a>
## 备份与故障处理

完整备份 Web PKI 目录，包括 `state.json`、`generations/` 和符号链接；不要只备份导出的单个 PEM。新节点使用 Web 当前公开 CA 引导信任。

长期离线导致原 CA 已过期时，需要从 Web 获取当前公开 CA 并重新引导节点。公网证书申请失败时查看对应 Caddy 或外部签发工具日志，检查 DNS、验证端口、证书链、有效期与文件权限。

使用 Nginx webroot 验证时，检查 Certbot pre/post hook 是否会停止 Nginx，并执行 `certbot renew --cert-name <域名> --dry-run --no-random-sleep-on-renew` 验证续签。外部证书续签与 Nginx 的证书加载由外部管理工具配置。
