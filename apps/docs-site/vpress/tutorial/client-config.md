# 客户端配置

## 使用订阅或单节点配置

在 Web 中登录账户，选择与目标客户端匹配的订阅或导出配置。可通过订阅地址、节点 URL、二维码或配置文件导入。

| 服务 | 客户端要求 |
| --- | --- |
| Xray | 支持所选协议、传输与 TLS / Reality 参数 |
| Hysteria2 | 支持 Hysteria2，并能访问对应 UDP 端口 |
| NaiveProxy | 支持 NaiveProxy HTTPS 代理身份与服务端证书 |

订阅支持 Xray、sing-box 和 Clash 系列模板；实际可用协议由目标客户端能力决定。证书域名、端口、账户有效期与额度应与服务器配置一致。含凭据的订阅和导出配置不要公开分享。

客户端安装与参数说明请参考对应项目的当前文档：

- [Xray](https://xtls.github.io/)
- [Hysteria2](https://v2.hysteria.network/)
- [NaiveProxy](https://github.com/klzgrad/naiveproxy)

账户及模板操作见[Web 使用指南](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.8/docs/user-guide.md#accounts)。
