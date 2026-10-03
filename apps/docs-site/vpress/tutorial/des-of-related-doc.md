# 数据与配置

## 持久化数据

服务默认数据位于 `/tpdata/`。Web 包括数据库、Redis、API、UI 和 Caddy 数据；Node 包括代理运行配置、日志、Caddy 证书或配置指定的外部证书，以及公开 mTLS 信任文件。

具体路径由部署配置决定。Node 外部证书与项目可清理数据目录须分开保存。Web PKI 应完整备份，包含 `state.json`、`generations/` 和符号链接；CA 私钥与 Web 客户端私钥不复制到 Node。

## 维护

- [配置文件字段](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.12/docs/deployment.md#configuration)
- [产品镜像更新](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.12/docs/deployment.md#updates)
- [保留数据和彻底卸载](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.12/docs/deployment.md#removal)
- [证书与 mTLS 维护](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.12/docs/certificates.md)
- [源码结构与开发](https://github.com/1linhao/trojanpanelnext/blob/v1.0.2-rc.12/docs/development.md#source-layout)

直接修改代理运行配置可能被面板后续操作覆盖，代理参数优先在 Web 中维护。
