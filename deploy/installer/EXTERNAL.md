# 外部入口简明契约

完整设计见 [EntryController 方案](../../docs/entry-controller/方案设计.md)、
[external driver v1](../../docs/entry-controller/external-driver-v1.md) 和
[外部入口实现契约](../../docs/外部入口实现契约.md)。当前 `tls_mode: external` 是兼容入口；它尚未
自动安装 external driver；提供合规的本地 driver 后，`entryctl reconcile/remove` 已能按 protocol
v1 执行并持久化恢复 journal。

## 分层职责

| 流量 | 默认所有者 | external 需要做什么 |
| --- | --- | --- |
| Web 80/443 | 宿主入口 | ACME、TLS 终止，反代到回环 UI |
| Node 协议端口 | Xray/NaiveProxy/Hysteria2 | 不代理；内核直接监听并终止自己的 TLS |
| Node 明文回落端口 | 宿主入口 | 仅为明确要求回落的路由提供 `/tpdata/web` |
| Core API/gRPC/traffic stats | 内部服务 | 用防火墙限制到授权来源，不对公网开放 |

Node Agent 把观测状态写到：

```text
/tpdata/trojan-panel-core/external/routes.json
```

它用于检查直接协议端口、443 冲突、防火墙和明文回落，不是 nginx `stream` 动态配置源。只有
`external_fallback_listener_required: true` 的路由才需要明文 HTTP fallback；不要在 fallback
端口启用 TLS。

证书源目录必须包含有效且匹配域名的证书/私钥。安装器把它们原子复制到：

```text
/tpdata/trojan-panel-core/cert/fullchain.pem
/tpdata/trojan-panel-core/cert/privkey.pem
```

续订必须由唯一的证书所有者触发同一条 reconcile 链。Node 的最小消费者动作是：

```text
install.sh refresh-cert --mode node --config <0600-file>
```

该动作只校验并原子刷新托管副本；证书变化时重启 Core，未变化时返回 `unchanged`，不安装包、
不操作数据库。外部证书所有者仍须以受控、超时且可观测的 deploy hook 调用它；只有安装器动作而
没有触发与状态记录，不能称为自动续订闭环。

首次实机接通前必须验证：80/443 所有权、证书签发/续订、nginx 配置检查与回滚、TCP/UDP 直接
协议端口、防火墙范围、fallback 明文响应，以及 Caddy ↔ external 的失败恢复。
