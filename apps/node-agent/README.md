# TrojanPanel Next Node Agent

简体中文 | [English](README_EN.md)

木马面板内核

## 支持的节点类型

1. Xray
2. Hysteria2
3. NaiveProxy

Trojan-Go 与 Hysteria v1 已退役。为兼容历史数据库，其类型编号仍保留且禁止复用。

默认数据处理：

1. 读取/写入 account 表中的 username, pass, hash, quota, download, upload, ip_limit, download_speed_limit, upload_speed_limit。
   pass, hash 需要哈希处理，quota, upload, download, download_speed_limit, upload_speed_limit 单位是 byte

主要逻辑：

1. API实时更新（数据库到应用）有效账户：account.quota < 0 or account.download +
   account.upload < account.quota
2. 定期更新 account.download、account.upload
3. account.quota=0, 该用户被禁用

## 创建数据库表语句示例

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

## 防止循环依赖

router->api->middleware->app->service/dao->core

## 引导就绪与凭据失效

Node Agent 启动前会用配置中的专用身份建立新连接，检查 MariaDB 的 `node_server` 读取权限、
Redis cache 身份的 `trojan-panel-core:*` 读写权限和 auth 身份的共享 JWT 键读取权限。任一失败
都会阻止启动。运行期间使用固定生产频率重新建立连接复检，并在凭据失效后至多 10 秒退出；
轮换或撤销导致旧身份认证失败时进程退出，
由容器重启策略重试，但旧配置不会恢复健康。

`GET /healthz` 在当前 `node_identity_id`、`identity_generation`、`server_id` 与本次安装生成的
`bootstrap_challenge` 收到经过客户端证书认证的 Web→Node gRPC 状态调用前返回 `503`，之后返回
`200`。就绪标记以 `0600` 原子写入运行目录，并与全部四个值绑定；复制旧标记或重放旧 challenge
不能让当前安装就绪。安装器用这一端点作为最终门禁。

排障时可在容器内设置 `TP_VERIFY_NODE_DATA_SERVICES=mariadb|redis|all` 单独执行数据层探测；
输出只报告成功类别或通用失败，不打印凭据。

## 构建

[compile.bat](compile.bat)

## 支持

- [TrojanPanel 原项目](https://github.com/trojanpanel)
- [trojan](https://github.com/trojan-gfw/trojan)
- [Xray-core](https://github.com/XTLS/Xray-core)
- [hysteria](https://github.com/apernet/hysteria)
- [naiveproxy](https://github.com/klzgrad/naiveproxy)
