# External Entry Driver Protocol v1

`external` provider is the ownership boundary between EntryController and a
host manager such as VPS Factory. EntryController never edits or deletes the
manager's nginx, certificate scheduler, firewall, or certificate source.

## Invocation

The executable is selected by `external_driver.path` and pinned into the
recovery journal by absolute path plus SHA-256. Protocol v1 invokes exactly:

```text
<driver> provider <probe|prepare|activate|verify|rollback|remove> --spec <absolute-0600-file>
```

The spec is a regular non-symlink file, owned by the trusted uid and mode 0600.
The driver is an absolute regular non-symlink executable, owned by the trusted
uid and not writable by group/other. Neither file may contain credentials.

Each action must be idempotent. Retrying the same action after a lost response
must converge without creating a second listener, certificate scheduler, or
resource owner. `remove` must succeed when the requested resources are already
absent. `rollback` restores or confirms the last committed external state.

## Input examples

Node direct mode does not ask the host manager to proxy protocol ports:

```json
{
  "schema_version": 1,
  "revision": 7,
  "deployment_id": "trojanpanelnext-node",
  "provider": "external",
  "purpose": "node",
  "domain": "node.example.com",
  "certificate": {
    "managed_dir": "/tpdata/trojan-panel-core/cert",
    "source_dir": "/etc/vps-factory/certs/node.example.com"
  },
  "ingress": {
    "node_exposure": "direct",
    "route_manifest": "/tpdata/trojan-panel-core/external/routes.json",
    "fallbacks": [{"listen": "127.0.0.1:8443", "root": "/tpdata/web"}]
  },
  "external_driver": {
    "protocol_version": 1,
    "path": "/usr/local/libexec/trojanpanelnext-entry-provider"
  }
}
```

Web mode asks the external ingress to terminate TLS and proxy only to loopback:

```json
{
  "schema_version": 1,
  "revision": 3,
  "deployment_id": "trojanpanelnext-web",
  "provider": "external",
  "purpose": "web",
  "domain": "panel.example.com",
  "certificate": {
    "managed_dir": "/tpdata/trojan-panel-ui/cert",
    "source_dir": "/etc/vps-factory/certs/panel.example.com"
  },
  "ingress": {"web_upstream": "127.0.0.1:8888"},
  "external_driver": {
    "protocol_version": 1,
    "path": "/usr/local/libexec/trojanpanelnext-entry-provider"
  }
}
```

## Output

Every successful invocation writes one JSON object to stdout. The authoritative
schema is [external-driver-result.schema.json](schema/external-driver-result.schema.json).
The action and deployment id must exactly match the request.

Probe example:

```json
{
  "schema_version": 1,
  "driver_protocol_version": 1,
  "provider": "external",
  "action": "probe",
  "deployment_id": "trojanpanelnext-node",
  "status": "ok",
  "capabilities": [
    "certificate.material",
    "certificate.renew",
    "certificate.notify",
    "ingress.plain_fallback",
    "lifecycle.prepare",
    "lifecycle.rollback"
  ],
  "resources": [],
  "listeners": []
}
```

Verify example for Node:

```json
{
  "schema_version": 1,
  "driver_protocol_version": 1,
  "provider": "external",
  "action": "verify",
  "deployment_id": "trojanpanelnext-node",
  "status": "healthy",
  "capabilities": ["certificate.material", "certificate.renew", "certificate.notify", "ingress.plain_fallback", "lifecycle.prepare", "lifecycle.rollback"],
  "certificate": {
    "domain": "node.example.com",
    "cert_path": "/etc/vps-factory/certs/node.example.com/fullchain.pem",
    "key_path": "/etc/vps-factory/certs/node.example.com/privkey.pem",
    "fingerprint": "sha256:example",
    "generation": 4,
    "renewal_owner": "external",
    "last_hook_status": "ok"
  },
  "resources": [{"kind": "certificate", "id": "node.example.com", "owner": "external", "retention": "preserve"}],
  "listeners": [{"transport": "tcp", "address": "127.0.0.1", "port": 8443, "purpose": "plain-fallback", "owner": "external"}]
}
```

Web verification additionally reports the externally owned TCP 80 and 443
listeners and capability `ingress.web_https`; its upstream remains loopback.

Action-specific successful statuses are:

| action | allowed status |
| --- | --- |
| `probe` | `ok`, `unchanged` |
| `prepare` | `prepared`, `unchanged` |
| `activate` | `active`, `unchanged` |
| `verify` | `healthy` |
| `rollback` | `rolled_back`, `unchanged` |
| `remove` | `removed`, `unchanged` |

## Failure exit codes

| exit | meaning | controller code |
| ---: | --- | --- |
| 64 | invalid request/spec | `invalid_spec` |
| 65 | required capability unsupported | `unsupported_capability` |
| 66 | resource owned by another deployment/provider | `ownership_conflict` |
| 67 | dependency unavailable | `dependency_missing` |
| 68 | transient provider mutation failure | phase-specific failure |
| 69 | verification failed | `verification_failed` |
| 70 | rollback failed | `rollback_failed` |
| 71 | remove failed | `driver_failed` |

Other nonzero exits are `driver_failed`; malformed success JSON is also rejected.
stderr is diagnostic only and must not contain secrets.

## Journal and recovery

Before mutation, the controller atomically records a 0600 ObservedState. An
unfinished journal pins the driver path and SHA-256; a changed executable cannot
inherit it. Recovery calls `probe`, then `rollback`, before a new `prepare`.
Failure of recovery rollback is durable as `failed/unhealthy`; the next retry
again rolls back before progressing. Desired revisions older than either the
journaled desired revision or committed observed revision are rejected before
the driver is called.

Controller status reports the last journaled result. Live external observation
happens during `plan`/`reconcile`, not during `status`, because status accepts no
spec and therefore has no trusted driver identity.

`remove` always delegates external resources to the driver before deleting the
controller journal. `--purge` is exposed as `ENTRY_PURGE=1`; it is only a request.
The driver must still refuse shared or unowned certificate deletion.
