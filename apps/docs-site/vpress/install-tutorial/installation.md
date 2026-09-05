# 安装 TrojanPanel Next

TrojanPanel Next 使用配置驱动的非交互安装器。每台服务器只选择一种用途：`web` 主控或 `node` Agent。

## 准备仓库

```bash
git clone https://github.com/1linhao/trojanpanelnext.git
cd trojanpanelnext/deploy/installer
```

## 安装 Web 主控

```bash
cp examples/web.yaml ./web.yaml
chmod 600 ./web.yaml
```

编辑 `web.yaml` 中的 `hostname` 和 `email`，确保域名已经解析到主控服务器。

```bash
./install.sh validate --mode web --config ./web.yaml
sudo ./install.sh install --mode web --config ./web.yaml
```

首次安装生成的 MariaDB 与 Redis 密码会写回 `web.yaml`。请妥善保存该文件，不要提交到 Git。

安装器同时在 `/tpdata/trojanpanelnext-pki` 生成主控 mTLS 身份。Node Agent 只需要其中的
`client-ca.crt`；CA 私钥与主控客户端证书不得离开 Web 主控。

## 安装 Node Agent

```bash
cp examples/node-agent.yaml ./node-agent.yaml
chmod 600 ./node-agent.yaml
```

编辑节点域名，并从 Web 主控的 `web.yaml` 填入 MariaDB 和 Redis 连接信息。通过可信的
文件传输或密钥管理系统，将 Web 主控的 `/tpdata/trojanpanelnext-pki/client-ca.crt`
复制到 Node 的同一路径。

```bash
./install.sh validate --mode node --config ./node-agent.yaml
sudo ./install.sh install --mode node --config ./node-agent.yaml
```

## 更新容器

更新配置中的镜像版本后，使用 `--force` 重新创建容器：

```bash
sudo ./install.sh install --mode web --config ./web.yaml --force
```

## 卸载

保留数据：

```bash
sudo ./install.sh remove --mode web --config ./web.yaml
```

同时删除生成数据：

```bash
sudo ./install.sh remove --mode node --config ./node-agent.yaml --purge-data
```

完整参数和配置字段见仓库中的[安装器说明](https://github.com/1linhao/trojanpanelnext/tree/main/deploy/installer)。
