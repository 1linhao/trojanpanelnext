# TrojanPanel Next API

简体中文 | [English](README_EN.md)

控制面后端服务，提供账号、节点、订阅、系统设置和管理接口。

## 开发

```bash
go test ./...
go build ./...
```

Windows 构建辅助脚本：[compile.bat](compile.bat)

## 首次管理员初始化

全新数据库的 `sysadmin` 种子账号默认不可登录。正式安装器通过
`TP_INITIAL_SYSADMIN_PASSWORD_FILE` 指向一个权限精确为 `0600`、路径不含符号链接的普通文件；API 在开始监听前
读取该文件并完成一次性密码初始化。已有非空管理员凭据不会在服务重启或安装器重跑时被覆盖。
安装器通过容器内只读认证命令验证该凭据；该命令不启动 HTTP、Redis、限流或定时任务，也不会签发会话或修改登录状态。

## Node 身份生命周期 CLI

Web 主控容器中的同一 API 二进制提供 `node-identity` CLI。先在 Web 主控宿主创建仅 root
可访问的目录，再登记 Node；名称、域名和公网 IP 共同绑定一个稳定 Node 身份：

```bash
sudo install -d -m 0700 /tpdata/trojan-panel/config/node-identities
sudo docker exec trojan-panel /tpdata/trojan-panel/trojan-panel node-identity register \
  --name node-sg --domain node-sg.example.com --public-ip 203.0.113.10 \
  --credential-file /tpdata/trojan-panel/config/node-identities/node-sg.g1.json
```

凭据文件以 `0600`、同目录原子发布、不覆盖既有目标且拒绝符号链接路径的方式创建；控制面保存
文件内容的 SHA-256 承诺，预置、修改或换代次重放都会在触碰数据服务前被拒绝。终端和生命周期事件只记录
Node ID、代次、动作、结果与固定错误码，不输出 MariaDB/Redis 密钥。该明文文件是 Web 主控上的
受限中间材料；加密 Node 引导包由后续交付流程生成，不应把它直接放进发布资产、日志或工单。

登记会创建独立的 MariaDB 用户、两个 Redis ACL 用户和 `node_server` 登记：cache 身份只能读写
`trojan-panel-core:*`，auth 身份只能读取共享 JWT/token 键，不能写入。重复登记只能使用当前代次
原凭据文件收敛，不能通过新路径生成未记录的凭据。轮换必须写入一个新的文件：

```bash
sudo docker exec trojan-panel /tpdata/trojan-panel/trojan-panel node-identity rotate \
  --id <node-identity-id> \
  --credential-file /tpdata/trojan-panel/config/node-identities/node-sg.g2.json
```

轮换先原子替换两个 Redis 身份，再替换 MariaDB 密码并提升代次，因此进入跨服务部分失败后旧 Redis
密码已不可用，状态保持 `rotating`；修复依赖后以同一 ID 和内容未改变的凭据文件重跑即可收敛，
不会再次提升代次。每个身份的生命周期命令由 MariaDB 锁串行化。

```bash
sudo docker exec trojan-panel /tpdata/trojan-panel/trojan-panel node-identity status --id <node-identity-id>
sudo docker exec trojan-panel /tpdata/trojan-panel/trojan-panel node-identity revoke --id <node-identity-id>
sudo docker exec trojan-panel /tpdata/trojan-panel/trojan-panel node-identity force-evict --id <node-identity-id>
```

`revoke` 回收全部数据层凭据，但保留 `node_server` 与不可用的 Node 身份审计墓碑；`force-evict`
还会删除活动 `node_server` 登记并明确表达故障处置意图。两者都不连接 Node 宿主，因此 Node 离线时
仍可完成，但不承诺删除失联宿主上的进程、证书或数据。重复执行同一动作会安全收敛。

## 支持

- [TrojanPanel 原项目](https://github.com/trojanpanel)
- [trojan](https://github.com/trojan-gfw/trojan)
- [Xray-core](https://github.com/XTLS/Xray-core)
- [hysteria](https://github.com/apernet/hysteria)
- [naiveproxy](https://github.com/klzgrad/naiveproxy)
