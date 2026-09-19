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
The installer verifies that credential through a read-only loopback API that neither issues a session
nor changes login-limit state; the public UI entry blocks this path.

## Support

- [Original TrojanPanel project](https://github.com/trojanpanel)
- [trojan](https://github.com/trojan-gfw/trojan)
- [Xray-core](https://github.com/XTLS/Xray-core)
- [hysteria](https://github.com/apernet/hysteria)
- [naiveproxy](https://github.com/klzgrad/naiveproxy)
