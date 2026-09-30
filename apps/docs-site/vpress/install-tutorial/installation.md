# 安装 TrojanPanel Next

每台服务器通过独立安装器获取远程 YAML 模板，编辑后执行安装。配置的 `trojanpanelnext.purpose` 决定 `web` 主控或 `node` Agent。

## 预装依赖

安装器不会安装 yq、Docker 或其他工具。先安装 Docker Engine、mikefarah/yq v4、curl、OpenSSL、tar、coreutils、findutils 和 awk，并启动 Docker 服务。不要使用 Python 的同名 yq 软件包。

Debian/Ubuntu：

```bash
sudo apt-get update
sudo apt-get install -y curl ca-certificates openssl tar coreutils findutils gawk docker.io
sudo systemctl enable --now docker
```

yq v4 的 amd64/arm64 下载、SHA-256 校验和安装命令见[安装器说明](https://github.com/1linhao/trojanpanelnext/blob/main/deploy/installer/README.md)。完成后检查：

```bash
yq --version
sudo docker info >/dev/null
```

## 获取安装器

当前安装器版本为 `0.1.0-rc.5`；在 Web 和 Node 服务器分别执行：

```bash
curl -fsSL --connect-timeout 10 --max-time 60 \
  https://raw.githubusercontent.com/1linhao/trojanpanelnext/v0.1.0-rc.5/deploy/installer/install.sh \
  -o install.sh
chmod +x install.sh
./install.sh --version
```

脚本与远程模板绑定到同一 Git 标签，产品镜像标签为不带 `v` 的 `0.1.0-rc.5`。无需准备仓库工作树。

## 安装 Web 主控

```bash
./install.sh config web
nano web.yaml
./install.sh validate --config ./web.yaml
sudo ./install.sh install --config ./web.yaml
```

编辑 `hostname` 和 `email`，确认域名解析正确，TCP 80、443 可达且未被占用。首次安装生成的 MariaDB 与 Redis 密码会写回 `web.yaml`，文件权限为 `0600`，再次获取模板会拒绝覆盖。

安装器在 `/tpdata/trojanpanelnext-pki` 生成主控 mTLS 身份。保留该目录及工作配置。面板首次登录为 `sysadmin` / `123456`，登录后修改密码；该密码与数据库、Redis 凭据分别管理。

## 安装 Node Agent

先在 Web 面板创建节点服务器，填写 Node 地址、gRPC 端口、TLS 服务器名（Node 域名），将实际服务器 ID 写入 Node YAML 的 `node_server_id`。

通过可信文件传输，把 Web 的 `/tpdata/trojanpanelnext-pki/client-ca.crt` 放到 Node 同一路径。只复制公开 CA，CA 私钥和客户端证书应留在 Web。

```bash
./install.sh config node
nano node.yaml
./install.sh validate --config ./node.yaml
sudo ./install.sh install --config ./node.yaml
```

填写节点域名、主控数据库和 Redis 地址、密码、对应端口，以及面板中实际的 `node_server_id`。确保 Node 能访问 Web 的 TCP 9507、6378，Web 能访问 Node 的 TCP 8100，Node TCP 80 支持证书签发。数据库和 Redis 端口只允许受信任节点来源。

Node Caddy 默认 HTTPS 端口为 8863；代理业务端口在面板另行配置。内部 API 和 UI 端口无需统一对公网放行。安装器不自动注册节点或传输 CA。

配置校验只检查 YAML 与字段，不验证网络、证书文件或服务健康。

## 重建与卸载

```bash
sudo ./install.sh install --config ./web.yaml --force
sudo ./install.sh install --config ./node.yaml --force
```

`--force` 重建 API、UI、Agent 和 Caddy，已有 MariaDB 与 Redis 保留。使用未发布的测试镜像时不要添加该选项。

保留数据卸载：

```bash
sudo ./install.sh remove --config ./web.yaml
sudo ./install.sh remove --config ./node.yaml
```

删除对应服务数据：

```bash
sudo ./install.sh remove --config ./web.yaml --purge-data
sudo ./install.sh remove --config ./node.yaml --purge-data
```

PKI 目录、伪装站目录及原始 YAML 仍会保留；自定义到默认目录以外的运行时路径需单独管理。完整参数和配置字段见[安装器说明](https://github.com/1linhao/trojanpanelnext/blob/main/deploy/installer/README.md)。

## 证书自动维护

- Web 与 Node 的公网域名证书由 Caddy 自动续签。保留 Caddy 数据目录，并保持域名解析及 ACME 验证端口可达。
- Node Agent 每分钟检查 NaiveProxy 的证书。新证书与私钥有效时，保存运行配置和用户信息、校验配置，再重启对应实例；现有连接会短暂中断，客户端需重新连接。失败会记录日志并在下次检查时重试。
- Web API 启动时及每 5 分钟检查内部 mTLS 身份，客户端证书剩余不足 90 天时重新签发；新证书有效期最多 825 天，且不超过 CA 到期时间。CA 私钥仅保留在 Web。
- CA 剩余不足 365 天时开始轮换：通过现有 mTLS 通道分发新旧 CA，所有已登记的 mTLS Node 确认后切换客户端身份。保留旧身份至少 24 小时，待所有节点确认移除旧 CA 后清理旧私钥。离线或不支持轮换的旧版 Node 会阻止推进，每 5 分钟重试。
- 首次引导仍需手动复制 Web 当前的 `client-ca.crt`。新节点请先登记到面板并使用当前公开 CA 文件；未登记节点不在轮换范围。升级先更新所有 Node，再更新 Web；修改现有 YAML 镜像标签为 `0.1.0-rc.5`，执行 `install --config ... --force` 以更新挂载。Node 重装会保留已有运行时 CA 文件，避免旧引导副本覆盖新信任；Agent 同时更新引导 CA 副本，供清理运行数据后的重新安装使用。
- Web API 挂载 `pki_bundle_dir` 并拥有签发权限，Node 仅能写入公开 CA 信任文件。完整备份 Web 的 PKI 目录（包含 `state.json` 和 `generations`），不要只备份根目录的符号链接。证书轮换失败可查看 API / Agent 日志。节点离线超过旧 CA 的有效期或恢复过期的备份时，需要人工重新引导信任。
