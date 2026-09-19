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
安装器通过只接受本机连接、不会签发会话或修改登录限流状态的只读 API 验证该凭据；公网 UI 入口会阻断此路径。

## 支持

- [TrojanPanel 原项目](https://github.com/trojanpanel)
- [trojan](https://github.com/trojan-gfw/trojan)
- [Xray-core](https://github.com/XTLS/Xray-core)
- [hysteria](https://github.com/apernet/hysteria)
- [naiveproxy](https://github.com/klzgrad/naiveproxy)
