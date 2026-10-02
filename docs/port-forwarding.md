# 代理节点端口转发

[文档首页](README.md) · [Web 使用指南](user-guide.md#proxies) · [English](port-forwarding_EN.md)

## 目录

- [对外端口与实际端口](#port-fields)
- [在 Web 中配置](#web-configuration)
- [Nginx 按 TLS 域名透传 TCP 443](#tls-sni)
- [Hysteria2 UDP 与端口跳跃](#udp-and-hopping)
- [检查与维护](#validation)

<a id="port-fields"></a>
## 对外端口与实际端口

代理前方存在 Nginx、NAT 或其他转发服务时，可以让客户端连接对外端口，Node 仍在另一个实际端口运行。

| 字段 | 范围 | 作用 |
| --- | --- | --- |
| 实际端口 `port` | `101–29999` | Node 创建和管理代理时实际监听的端口 |
| 对外端口 `externalPort` | 启用时为 `1–65535` | 自动生成的订阅、连接链接与客户端配置使用的入口端口 |
| `externalPort: 0` | 关闭端口转发配置 | 客户端配置使用实际端口 |

例如，实际端口为 `11443`、对外端口为 `443` 时，客户端连接公开地址的 `443`，外部转发服务须把流量送到 Node 的 `11443`。改变对外端口不会改变 Node 的监听端口；实际端口仍需在对应主机上可用。

**本功能保存端口映射并生成客户端入口，不部署转发服务，也不自动设置 Nginx、NAT、防火墙或证书。** 管理员负责实际转发链路。此配置用于代理节点，不更改节点服务器的 gRPC 和宿主机维护端口。

<a id="web-configuration"></a>
## 在 Web 中配置

1. 打开 **节点管理**，新增或编辑代理节点，选择所属节点服务器和协议。
2. 开启 **端口转发**，填写 **对外端口** 与 **实际端口**；例如 `443` → `11443`。
3. 按实际协议配置域名、TLS / SNI 和账户参数，保存节点。
4. 在宿主机或外部网关配置相同协议的转发并检查防火墙，再更新客户端订阅或重新导入节点。

关闭开关后，客户端重新使用实际端口。已经导入的客户端配置不会自行修改；关闭前应确认客户端能够直接到达实际端口。删除或调整节点不会自动修改宿主机上的转发规则。

<a id="tls-sni"></a>
## Nginx 按 TLS 域名透传 TCP 443

本例使用 **一个 TPNext 的 Xray Trojan/TLS TCP 代理**和**一个独立 TLS 服务**。两个域名解析到同一台公网主机，Nginx 根据客户端 SNI 转发原始 TLS 流量，证书由后端提供。`ssl_preread` 可在不终止 TLS 的情况下读取 ClientHello 的 SNI；需要可用的 stream 和 ssl_preread 模块。[Nginx 官方说明](https://nginx.org/en/docs/stream/ngx_stream_ssl_preread_module.html)

| 服务 | 客户端域名 | 公网入口 | 后端监听 |
| --- | --- | --- | --- |
| TPNext Trojan/TLS | `proxy.example.com` | TCP `443` | 同机 Node TCP `11443` |
| 独立 TLS 服务 | `site.example.com` | TCP `443` | 同机服务 TCP `8443` |

在 TPNext 中开启端口转发，设置对外端口 `443`、实际端口 `11443`，使用 TCP 传输并启用 TLS；客户端 SNI 使用 `proxy.example.com`。本例按独立域名区分服务，不使用同一 SNI 区分两个后端。

### 证书与监听准备

- Node 实际提供的证书必须覆盖 `proxy.example.com`，客户端 TLS 服务器名与分流域名一致；不要用跳过证书校验代替正确证书。
- Node 使用部署时配置的证书路径。端口映射不会为每个代理节点建立独立证书管理；同一 Node 上多个 TLS 代理域名使用默认同一证书时，证书须覆盖这些域名。
- 独立服务在 `8443` 提供自己的 TLS 和 `site.example.com` 证书。它与 TPNext 的证书管理分别维护。
- 公开 TCP `443` 由 Nginx stream 占用；独立服务原本监听 `443` 时，需要调整到后端端口。后端端口只允许转发入口或可信管理来源访问。
- 若域名签发端口由宿主机服务管理，先准备证书，再使用 [Node 外部证书模式](deployment.md#external-certificates)。映射不解决 ACME 验证端口占用。

Trojan 代理的 TLS 设置参考 [Xray 官方 Trojan 文档](https://xtls.github.io/en/config/inbounds/trojan.html)。

### 宿主机配置示例

这是 Nginx 主配置中的 `stream` 部分，与 `http` 同级；不要放进 `http` 的网站配置。已存在 `stream` 时合并其中的 `map` 和 `server`，模块加载按发行版配置完成。

```nginx
stream {
    map $ssl_preread_server_name $tls_destination {
        proxy.example.com 127.0.0.1:11443;
        site.example.com  127.0.0.1:8443;
        default           127.0.0.1:8443;
    }

    server {
        listen 443;
        # 有 IPv6 公网入口时另加：listen [::]:443;
        ssl_preread on;
        proxy_connect_timeout 5s;
        proxy_timeout 1h;
        proxy_pass $tls_destination;
    }
}
```

未知或缺少 SNI 的连接在此例交给独立服务；按实际需求选择默认后端。配置中的 `listen` 不启用 `ssl`，Nginx 不读取代理证书，也不解密或重建客户端 TLS。若后端在其他机器上，用该机器的可信地址替换 `127.0.0.1`。[stream 配置位置](https://nginx.org/en/docs/stream/ngx_stream_core_module.html#stream)

本例不发送 PROXY protocol。只有确认后端明确支持并正确启用该协议后，才设置 `proxy_protocol`；普通 TLS 后端不能直接把 PROXY 头当作 TLS ClientHello。[Nginx stream 代理指令](https://nginx.org/en/docs/stream/ngx_stream_proxy_module.html#proxy_protocol)

在宿主机检查配置后 reload：

```bash
nginx -t
systemctl reload nginx
```

根据两个域名分别检查公开入口，确认返回对应后端证书：

```bash
openssl s_client -connect proxy.example.com:443 -servername proxy.example.com -verify_hostname proxy.example.com -verify_return_error </dev/null
openssl s_client -connect site.example.com:443 -servername site.example.com -verify_hostname site.example.com -verify_return_error </dev/null
```

TLS 握手成功仅证明分流和证书链路；还需用真实代理客户端验证认证和转发。

<a id="udp-and-hopping"></a>
## Hysteria2 UDP 与端口跳跃

Hysteria2 的客户端到服务端链路使用 QUIC / UDP，即使代理的目标流量是 TCP，也仍需转发 UDP。它不能直接使用上面的 TCP `ssl_preread` 配置按域名分流。[Hysteria2 协议说明](https://v2.hysteria.network/docs/developers/Protocol/)

TCP `443` 与 UDP `443` 是不同的监听资源。同一主机可分别把 TCP `443` 用于上述 TLS 分流，把 UDP `443` 用于一个 Hysteria2 后端，但不能仅凭不同域名让多个 Hysteria2 服务共享这一个 UDP 入口。检查 HTTP/3 或其他 QUIC 服务是否已占用 UDP `443`；冲突的 UDP 服务需要不同的公开入口端口。

例如，Hysteria2 设置实际端口 `11445`、对外端口 `443` 时，需要在外部网关将 **UDP `443` → UDP `11445`**。可选用宿主机 NAT 或 Nginx stream 的 UDP 转发。Nginx 配置须显式使用 `listen ... udp`，不加 `ssl_preread`；TCP 转发不会同时转发 UDP。[Nginx UDP 转发说明](https://nginx.org/en/docs/stream/ngx_stream_proxy_module.html)

### 公开端口跳跃范围

Hysteria2 的 **端口跳跃**字段是客户端可连接的公开端口集合或范围，例如 `30000-30100`。它不会改变 Node 的单一实际监听端口，也不会为这些端口自动建立转发。

使用实际端口 `11445` 和公开跳跃范围 `30000-30100` 时，应将该范围内的 **所有 UDP 端口**转发到 UDP `11445`，并放行对应范围。若仍导出对外单端口 `443` 作为入口，也应保证 UDP `443` 的转发可用。只转发 `443` 不足以支持该跳跃范围；不同客户端对跳跃扩展的支持可能不同，应检查实际导出配置。

客户端会在公开集合中选择和切换端口，因此整个集合都必须可达。外部防火墙、NAT 和转发器的规则由管理员维护；当前 TPNext 只将跳跃参数写入支持它的客户端配置。[Hysteria2 官方端口跳跃文档](https://v2.hysteria.network/docs/advanced/Port-Hopping/)

<a id="validation"></a>
## 检查与维护

| 检查项 | 要求 |
| --- | --- |
| Node 监听 | 实际端口与面板记录一致，无冲突 |
| 转发协议 | TCP 代理转发 TCP，Hysteria2 转发 UDP；需双协议的服务分别配置 |
| 客户端入口 | 重新生成的订阅和节点配置使用对外端口；关闭时使用实际端口 |
| TLS 域名 | 分流 SNI、客户端服务器名和后端证书覆盖的域名一致 |
| 公网访问 | DNS、IPv4 / IPv6、防火墙和整个转发链路可达 |
| 端口跳跃 | 每个公开跳跃端口均能到达实际 UDP 后端 |
| 管理连接 | gRPC 和宿主机维护端口按原规则运行，不经本代理映射改变 |

面板状态在线不代表公网转发已配置正确。修改对外端口后，检查实际规则并刷新客户端订阅；修改实际端口时同时更新转发目标。卸载项目不替管理员清理宿主机 Nginx、NAT 或其他独立转发服务。
