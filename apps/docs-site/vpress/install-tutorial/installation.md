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

安装器同时在 `/tpdata/trojanpanelnext-pki` 生成主控 mTLS 身份。Node 引导包只携带其中的
公开 `client-ca.crt`；CA 私钥与主控客户端证书不得离开 Web 主控。

## 安装 Node Agent

```bash
cp ./config-node.yaml ./node-sg.yaml
chmod 600 ./node-sg.yaml
./node-bundle create \
  --credential-file /tpdata/trojan-panel/config/node-identities/node-sg.g1.json \
  --node-config ./node-sg.yaml \
  --client-ca /tpdata/trojanpanelnext-pki/client-ca.crt \
  --output ./node-sg.g1.age
```

先用 Web 主控的 `node-identity register` 登记 Node。编辑 `node-sg.yaml` 中的节点域名、
MariaDB/Redis 地址和镜像；`node-bundle` 从权限为 `0600` 的登记凭据文件注入三组专用身份，
并交互读取加密口令。把生成的 `.age` 文件传到 Node，不传明文凭据或任何私钥。

```bash
sudo ./install.sh validate --mode node --bundle ./node-sg.g1.age
sudo ./install.sh install --mode node --bundle ./node-sg.g1.age
```

安装等待期间，在 Web 主控另一终端执行
`node-identity verify --id <node-identity-id>`。MariaDB、Redis、Node API 和 Web→Node
mTLS/gRPC 四项检查全部通过后安装才成功。

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
