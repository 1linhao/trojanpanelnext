# 常见问题

## Web 和 Node 有什么区别？

Web 提供面板、API、数据库与缓存。Node 运行代理实例；一台 Node 可以运行多个代理节点，Web 可以管理多台 Node。[架构说明](/start/system-structure)介绍各组件关系。

## 如何选择安装版本？

统一脚本入口的 `--version` 指定目标版本。入口下载该版本的脚本与模板，并使用绑定的产品镜像。所有参数见[版本说明](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/deployment.md#versions)。

## 安装前需要哪些软件？

Docker Engine、Bash、curl、mikefarah/yq v4、OpenSSL 和常用系统工具；Node 还需要 systemd。脚本不自动安装这些依赖。[依赖说明](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/deployment.md#dependencies)包含完整清单。

## 登录提示 timeout of 5000ms exceeded？

先检查 Web API、MariaDB 和 Redis 容器是否运行，浏览器 `/api` 请求是否正确转发，数据库和缓存地址是否可达，以及服务日志的具体错误。按实际端口放行防火墙；排查步骤见[故障处理](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/deployment.md#troubleshooting)。

## Node 能使用现有证书吗？

可以。外部模式接收 fullchain PEM 和私钥路径，跳过 Node Caddy 和证书申请，续签由 Certbot 等外部工具负责。见[外部证书部署](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/deployment.md#external-certificates)。

## 删除后能重新接入吗？

可以。保留数据删除后，在 Web 重新创建服务器，更新 Node 配置里的新服务器 ID 与当前公开 CA，重新安装后创建所需代理。见[重新接入说明](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/deployment.md#reconnect)。

## 同一端口能运行多个服务吗？

需要按实际协议配置反向代理或 TLS SNI 分流。脚本不自动更改已有 Nginx 配置；Node 可使用外部证书与已有证书管理方案协作。TCP 与 UDP 分别监听，端口规划见[网络说明](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/deployment.md#network)。

## 证书是否自动续签？

默认公网证书由 Caddy 维护。外部模式由外部证书工具维护。内部 mTLS 身份与 CA 由 Web API 自动维护；NaiveProxy 加载变更证书时可能短暂中断连接。详见[证书管理](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/certificates.md)。
