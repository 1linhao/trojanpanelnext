# 客户端配置

## 使用订阅或单节点配置

在 Web 中登录账户，选择与目标客户端匹配的订阅或导出配置。可通过订阅地址、节点 URL、二维码或配置文件导入。

| 服务 | 客户端要求 |
| --- | --- |
| Xray | 支持所选协议、传输与 TLS / Reality 参数 |
| Hysteria2 | 支持 Hysteria2，并能访问对应 UDP 端口 |
| NaiveProxy | 支持 NaiveProxy HTTPS 代理身份与服务端证书 |

订阅支持 Xray、sing-box 和 Clash 系列模板；实际可用协议由目标客户端能力决定。证书域名、客户端对外端口、账户有效期与额度应与代理配置一致。含凭据的订阅和导出配置不要公开分享。

客户端安装与参数说明请参考对应项目的当前文档：

- [Xray](https://xtls.github.io/)
- [Hysteria2](https://v2.hysteria.network/)
- [NaiveProxy](https://github.com/klzgrad/naiveproxy)

账户及模板操作见[Web 使用指南](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/user-guide.md#accounts)。

## 对外端口与实际端口

在节点管理的原有新增 / 编辑表单中开启 **端口转发**，填写 **对外端口**（`1–65535`）和 **实际端口**（`101–29999`）。客户端订阅及连接配置使用对外端口；Node 仍按实际端口监听。关闭开关时 `externalPort: 0`，客户端使用实际端口。

这只保存映射，不自动安装或配置宿主机转发器。使用 TLS TCP 代理时，可自行设置 Nginx 根据独立域名 SNI 透传，后端证书必须覆盖客户端服务器名。Hysteria2 使用 UDP；TCP SNI 示例不能直接分流 QUIC，端口跳跃的公开集合也须完整转发到实际 UDP 端口。

变更后刷新客户端订阅，并验证真正的公网连接。配置步骤、Nginx 示例与协议依据见[端口转发指南](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.13/docs/port-forwarding.md)。
