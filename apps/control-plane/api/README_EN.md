# TrojanPanel Next API

[简体中文](README.md) | English

The control-plane backend provides account, node, subscription, system configuration, and administration APIs.

## Development

```bash
go test ./...
go build ./...
```

Windows build helper: [compile.bat](compile.bat)

## Initial administrator bootstrap

The `sysadmin` seed account in a fresh database cannot authenticate by default. The production
installer points `TP_INITIAL_SYSADMIN_PASSWORD_FILE` at a regular, symlink-free path whose file
permissions are exactly `0600`; the API reads it and initializes the password before it starts listening.
Restarts and installer replays never overwrite an existing non-empty administrator credential.
The installer verifies that credential through a read-only command executed inside the API container.
The command starts no HTTP server, Redis client, limiter, or scheduled task and neither issues a session
nor changes login state.

## Node identity lifecycle CLI

The API binary inside the Web control-plane container also provides the `node-identity` CLI. Create a
root-only directory on the Web host, then register a Node. Its name, domain, and public IP bind a stable
Node identity:

```bash
sudo install -d -m 0700 /tpdata/trojan-panel/config/node-identities
sudo docker exec trojan-panel /tpdata/trojan-panel/trojan-panel node-identity register \
  --name node-sg --domain node-sg.example.com --public-ip 203.0.113.10 \
  --credential-file /tpdata/trojan-panel/config/node-identities/node-sg.g1.json
```

The CLI publishes the credential file atomically in the same directory with mode `0600`, never replaces
an existing target, and rejects symlinked paths. The control plane stores a SHA-256 commitment to the
exact contents, so pre-positioned, modified, or cross-generation replays are rejected before a data
service is touched. Terminal output and lifecycle events contain only the Node ID, generation, action,
result, and fixed error codes—never MariaDB or Redis secrets. This plaintext file is restricted staging
material on the Web control plane. A later delivery step creates the encrypted Node bootstrap bundle;
do not put this file in release assets, logs, or issues.

Registration creates a dedicated MariaDB user, two Redis ACL users, and a `node_server` registration.
The cache identity can read and write only `trojan-panel-core:*`; the auth identity can only read shared
JWT/token keys and cannot write them. A replay
must use the current generation's original credential file, so a different path cannot mint untracked
credentials. Rotation writes a new file:

```bash
sudo docker exec trojan-panel /tpdata/trojan-panel/trojan-panel node-identity rotate \
  --id <node-identity-id> \
  --credential-file /tpdata/trojan-panel/config/node-identities/node-sg.g2.json
```

Rotation atomically replaces both Redis identities before replacing the MariaDB password and advancing
the generation. After a cross-service partial failure, the old Redis passwords are already invalid and
the identity remains `rotating`; repair the dependency and rerun with the same ID and unchanged
credential file to converge without another generation. MariaDB locks serialize lifecycle commands per identity.

```bash
sudo docker exec trojan-panel /tpdata/trojan-panel/trojan-panel node-identity status --id <node-identity-id>
sudo docker exec trojan-panel /tpdata/trojan-panel/trojan-panel node-identity revoke --id <node-identity-id>
sudo docker exec trojan-panel /tpdata/trojan-panel/trojan-panel node-identity force-evict --id <node-identity-id>
```

`revoke` removes every data-layer credential while retaining the `node_server` registration and a
disabled Node identity audit tombstone. `force-evict` additionally removes the active `node_server`
registration and explicitly records incident-response intent. Neither command contacts the Node host, so both work while that host is
offline, but neither promises to remove processes, certificates, or data from the unreachable host.
Replaying the same action converges safely.

## Support

- [Original TrojanPanel project](https://github.com/trojanpanel)
- [trojan](https://github.com/trojan-gfw/trojan)
- [Xray-core](https://github.com/XTLS/Xray-core)
- [hysteria](https://github.com/apernet/hysteria)
- [naiveproxy](https://github.com/klzgrad/naiveproxy)
