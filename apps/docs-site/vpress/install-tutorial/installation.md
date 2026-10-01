# 部署 TrojanPanel Next v1.0

## 目录

- [依赖与网络](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/deployment.md#dependencies)
- [选择版本](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/deployment.md#versions)
- [Web 部署](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/deployment.md#web)
- [Node 部署](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/deployment.md#node)
- [外部证书](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/deployment.md#external-certificates)
- [配置文件部署](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/deployment.md#configuration)
- [重建与卸载](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/deployment.md#removal)
- [服务器重新接入](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/deployment.md#reconnect)
- [故障排查](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/deployment.md#troubleshooting)

## 一键安装

准备依赖、域名 DNS 与所需端口后，在 root Bash 中执行对应的一行命令。

Web 主控：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0/scripts/tp.sh) web
```

Node Agent：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0/scripts/tp.sh) node
```

Node 使用已有证书：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0/scripts/tp.sh) node --certificate-mode external
```

脚本提示输入配置；Node 部署前先在 Web 登记服务器并复制当前公开客户端 CA。脚本不会自动安装软件依赖，也不会替已有 Nginx 设置端口分流。

## 完整参考

仓库 [docs/deployment.md](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/deployment.md) 是部署说明的维护源，包含依赖安装、所有参数、YAML 字段、端口、卸载和证书维护。英文版见 [docs/deployment_EN.md](https://github.com/1linhao/trojanpanelnext/blob/v1.0/docs/deployment_EN.md)。选择指定版本时，入口从该版本标签加载命令、模板和镜像；文档与源码一同发布。
