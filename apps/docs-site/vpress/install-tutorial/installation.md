# 部署 TrojanPanel Next v1.0.1

## 目录

- [依赖与网络](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/deployment.md#dependencies)
- [一键安装依赖](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/deployment.md#dependency-install)
- [卸载依赖](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/deployment.md#dependency-removal)
- [选择版本](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/deployment.md#versions)
- [Web 部署](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/deployment.md#web)
- [Node 部署](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/deployment.md#node)
- [外部证书](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/deployment.md#external-certificates)
- [配置文件部署](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/deployment.md#configuration)
- [重建与卸载](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/deployment.md#removal)
- [服务器重新接入](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/deployment.md#reconnect)
- [故障排查](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/deployment.md#troubleshooting)

## 一键安装

在 root Bash 中先安装依赖。自动安装支持 Debian 12/13、Ubuntu 22.04/24.04（amd64/arm64，运行 systemd）；`deps` 还需要 util-linux 的 `flock`。入口最小工具与其他 Linux 的手动准备见[依赖说明](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/deployment.md#dependencies)。

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.1/scripts/tp.sh) deps install
```

准备域名 DNS 与所需端口后，执行对应部署命令。

Web 主控：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.1/scripts/tp.sh) web
```

Node Agent：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.1/scripts/tp.sh) node
```

Node 使用已有证书：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.1/scripts/tp.sh) node --certificate-mode external
```

部署脚本提示输入配置；Node 部署前先在 Web 登记服务器并复制当前公开客户端 CA。`web`、`node`、`install` 只检查依赖，也不会替已有 Nginx 设置端口分流。

## 卸载依赖

先卸载项目及其他 Docker 服务并等待 Node 维护服务清理，然后执行：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.1/scripts/tp.sh) deps remove
```

仅清理安装记录中的新增 Docker 软件包与未修改的 yq。已有依赖、基础工具、Docker 数据和外部证书保留。存在任何 Docker 容器（含停止容器）、共享 containerd 的其他 namespace 中仍有容器、Node 维护服务或运行状态无法确认时拒绝删除 Docker。依赖安装中断时先重跑 `deps install` 修复，再卸载；完整范围见[依赖卸载](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/deployment.md#dependency-removal)。

## 完整参考

仓库 [docs/deployment.md](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/deployment.md) 是部署说明的维护源，包含依赖安装、所有参数、YAML 字段、端口、卸载和证书维护。英文版见 [docs/deployment_EN.md](https://github.com/1linhao/trojanpanelnext/blob/v1.0.1/docs/deployment_EN.md)。选择指定版本时，入口从该版本标签加载命令、模板和镜像；文档与源码一同发布。
