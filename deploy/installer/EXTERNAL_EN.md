# External Entry Contract (Short Form)

See the [EntryController design](../../docs/entry-controller/方案设计.md), the
[external driver v1 protocol](../../docs/entry-controller/external-driver-v1.md), and the
[full external-entry contract](../../docs/外部入口实现契约.md). The current
`tls_mode: external` remains a compatibility input. The installer does not install a driver;
when a compliant local driver is provided, `entryctl reconcile/remove` execute protocol v1 and
persist a recovery journal.

## Layered ownership

| Traffic | Default owner | External responsibility |
| --- | --- | --- |
| Web 80/443 | Host ingress | ACME, TLS termination, and proxying to the loopback UI |
| Node protocol ports | Xray/NaiveProxy/Hysteria2 | No proxy by default; kernels listen directly and terminate their own TLS |
| Node plain fallback ports | Host ingress | Serve `/tpdata/web` only for routes that explicitly require fallback |
| Core API/gRPC/traffic stats | Internal services | Restrict to authorised callers with the firewall; never expose publicly |

The Node Agent writes observed state to:

```text
/tpdata/trojan-panel-core/external/routes.json
```

Use it to audit direct protocol ports, 443 conflicts, firewall rules, and required plain-HTTP
fallback listeners. It is not a dynamic nginx `stream` configuration source. Only a route with
`external_fallback_listener_required: true` needs a fallback listener, and that listener must not
enable TLS.

The certificate source directory must contain a valid certificate and matching private key for the
node domain. The installer atomically copies them to:

```text
/tpdata/trojan-panel-core/cert/fullchain.pem
/tpdata/trojan-panel-core/cert/privkey.pem
```

One certificate owner must drive the complete renewal reconcile. The Node consumer action is:

```text
install.sh refresh-cert --mode node --config <0600-file>
```

It only validates and atomically refreshes the managed copy, restarts Core when the pair changed,
and returns `unchanged` otherwise; it installs no packages and touches no database. The external
certificate owner must still invoke it from a controlled, timed and observable deploy hook. The
installer action without that trigger and status record is not a complete automatic-renewal loop.

Before enabling a real adapter, verify 80/443 ownership, issuance and renewal, nginx validation and
rollback, direct TCP/UDP protocol ports, firewall scope, plain fallback responses, and recovery of a
Caddy ↔ external switch.
