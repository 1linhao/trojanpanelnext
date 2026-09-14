# testsg 实机验收记录

日期：2026-09-13 至 2026-09-15
主机：`159.223.79.236`（Debian 13，约 464 MiB RAM）

## 拓扑

- Web：`testweb.lhsite.site:443` → VPS Factory 管理的宿主 nginx → `127.0.0.1:8888`
- Node：`testsg.lhsite.site`，内核协议采用 direct exposure；Hysteria2 使用 `24444/udp`
- 控制面、MariaDB、Redis 与 Node Agent 部署在同一台测试机
- 80、nginx、certbot/certd、证书导出与续订 timer 由 VPS Factory 单写
- Web 与 Node 使用不同域名和证书；Node Core 使用 Web PKI 签发的客户端 CA 验证 gRPC 客户端

## 已通过项目

1. 两个 A 记录均解析到测试机。
2. VPS Factory takeover 完成，SSH 仅允许受管密钥；`nginx` 与 `certd-renew.timer` active。
3. Let's Encrypt 正式证书签发成功，导出到：
   - `/etc/vps-factory/certs/testweb.lhsite.site`
   - `/etc/vps-factory/certs/testsg.lhsite.site`
4. 当前源码构建的 API、Web、Node Agent 镜像可启动；MariaDB、Redis、API、UI、Core 均为 running。
5. `https://testweb.lhsite.site/` 返回 200，证书主机名与系统信任链验证通过。
6. 默认管理账户可登录，控制面成功创建 `testsg` Node Server。
7. 控制面通过 gRPC mTLS 获取 Node CPU、内存和磁盘状态。
8. 无客户端证书的 gRPC TLS 握手被拒绝；携带 Web 客户端证书时 TLS 1.3 握手与主机名验证通过。
9. 公网侧已阻断 `6378/8081/8082/8100/8888/9507`，本机控制面仍可访问 Node gRPC；namespaced systemd oneshot 会在开机后恢复规则。
10. 小内存主机已增加 1 GiB swap；Node 二进制改为在构建机编译后上传，避免生产主机编译争用。
11. certd consumer hook 已由 VPS Factory 下发；Node hook 在证书未变化时返回 `unchanged` 且不重启 Core。
12. VPS Factory 将 Web 的公开客户端 CA 声明式下发到 Node；安装器以 CA 摘要标记 Core，旧容器
    首次迁移时自动重建，第二次应用摘要未变且 Core 启动时间保持不变。
13. 控制面创建 Hysteria2 节点成功，节点状态为正常，`24444/udp` 由真实 `hysteria2` 进程监听；
    `routes.json` 包含 UDP、TLS、SNI 与内核自终止 TLS 的路由事实。
14. changed hook 使用同域短期测试证书完成可恢复演练：托管副本变化后 Core 重启；恢复正式证书后
    再次重启，正式证书摘要完全恢复，Hysteria2 listener 与路由清单自动恢复。
15. 公网 `https://testweb.lhsite.site` 仍返回 200 且系统信任链校验通过；内部管理端口继续阻断。

## 验收中发现并处理的问题

- 私有 GHCR 镜像无法匿名拉取：改用当前源码构建的本地测试镜像完成验证。
- 首次安装中断后重新上传空密码配置，造成 Redis 容器与 API 配置的密码代次不一致：保留数据目录并重建 Redis 容器，API 恢复正常。
- 远端直接编译 Node Agent 令 464 MiB 主机发生严重内存争用：终止构建容器、增加 swap，并改为上传构建产物。
- 宿主初始没有入站防火墙，内部端口可公网连接：已用 `trojanpanelnext-firewall.service` 持久化最小阻断规则；后续应由 VPS Factory 服务生命周期正式 adoption。
- Factory 首次只替换 Node CA 文件，运行中的 Core 仍信任旧 CA：TrojanPanelNext installer 现将 CA
  SHA-256 写入容器环境，旧容器或 CA 变化时重建，未变化时保持幂等。
- 控制面旧实现会在 gRPC AddNode 失败后继续写数据库，留下状态异常的孤儿节点；本次删除后重建
  完成验收，该错误传播问题需在上游控制面服务中另行修正。

## 尚未关闭

- Xray/NaiveProxy 的 TCP direct listener 尚未逐一创建拨测；Hysteria2 UDP 路径已通过。
- changed hook 已通过替换/恢复同域有效证书验证，不需要为此强制调用正式 ACME 续订。
- VPS Factory 基础更新报告新内核待重启；本次未主动重启测试机。
