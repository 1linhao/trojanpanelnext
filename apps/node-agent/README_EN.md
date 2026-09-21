# TrojanPanel Next Node Agent

[简体中文](README.md) | English

Trojan Panel Core

## Supported node types

1. Xray
2. Hysteria2
3. NaiveProxy

Trojan-Go and Hysteria v1 are retired. Their historical numeric type IDs remain
reserved for database compatibility and must not be reused.

Default data processing：

1. Read/write username, pass, hash, quota, download, upload, ip_limit, download_speed_limit, upload_speed_limit in
   account. pass, hash needs to be hashed, quota, upload, download, download_speed_limit, upload_speed_limit unit is
   byte

Main logic：

1. API real-time update (database to application) valid account: account.quota < 0 or account.download +
   account.upload < account.quota
2. Regularly update account.download, account.upload
3. account.quota=0, the user is disabled

## Create database table statement example

```sql
create table trojan_panel_db.account
(
    id                   bigint(10) unsigned auto_increment comment 'auto increment primary key'
        primary key,
    username             varchar(64) default '' not null comment 'login username',
    pass                 varchar(64) default '' not null comment 'login password',
    hash                 varchar(64) default '' not null comment 'hash of pass',
    quota                bigint      default 0  not null comment 'quota unit/byte',
    download             bigint unsigned default 0 not null comment 'download unit/byte',
    upload               bigint unsigned default 0 not null comment 'upload unit/byte',
    ip_limit             tinyint(2) unsigned default 3 not null comment 'limit the number of IP devices',
    download_speed_limit bigint unsigned default 0 not null comment 'download speed limit unit/byte',
    upload_speed_limit   bigint unsigned default 0 not null comment 'upload speed limit unit/byte',
);
```

## Prevent circular dependencies

router->api->middleware->app->service/dao->core

## Bootstrap readiness and credential invalidation

Before startup, the Node Agent opens fresh connections with its dedicated identities and verifies
MariaDB `node_server` read access, Redis cache read/write access within `trojan-panel-core:*`, and
Redis auth read access to the shared JWT key. Any failure prevents startup. While running, it repeats
the checks over fresh connections at a fixed production cadence and exits within 10 seconds after a
credential becomes invalid. Rotation or revocation makes the old authentication fail and stops the
process; the container restart policy may retry, but stale configuration cannot become healthy again.

`GET /healthz` returns `503` until the current `node_identity_id`, `identity_generation`, `server_id`,
and the fresh per-installation `bootstrap_challenge` receive a Web-to-Node gRPC state call authenticated
by a client certificate, then returns `200`. The readiness marker is written atomically with mode
`0600` and bound to all four values, so a copied marker or replayed challenge cannot make the current
installation ready. The installer uses this endpoint as its final gate.

For diagnostics, set `TP_VERIFY_NODE_DATA_SERVICES=mariadb|redis|all` inside the container to run an
individual data-layer probe. Output reports only the successful category or a generic failure and
never prints credentials.

## Build

[compile.bat](compile.bat)

## Support

- [Original TrojanPanel project](https://github.com/trojanpanel)
- [trojan](https://github.com/trojan-gfw/trojan)
- [Xray-core](https://github.com/XTLS/Xray-core)
- [hysteria](https://github.com/apernet/hysteria)
- [naiveproxy](https://github.com/klzgrad/naiveproxy)
