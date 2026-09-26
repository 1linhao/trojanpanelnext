# TrojanPanel Next 安装器

简体中文 | [English](README_EN.md)

## 非 combined：分别部署 Web 和 Node

两台 VPS 均使用 Debian 12 x86_64，至少 1 GiB 内存。以下命令以 root SSH 登录为例；把 WEB_IP、NODE_IP、panel.example.com 和 node.example.com 换成实际值，域名提前解析到相应 VPS。若只能以普通用户登录，传输后运行 sudo -i，并把 VPS 命令中的 cd ~/tpnext-upload 改为该用户上传目录的绝对路径。

### 本地电脑：下载安装包，填写 Web 配置

本地需要 curl、tar、ssh/scp 和 SHA-256 校验工具；macOS 可使用 shasum。下载 [v0.1.0-rc.2](https://github.com/1linhao/trojanpanelnext/releases/tag/v0.1.0-rc.2) 的安装包并校验：

~~~bash
set -e
umask 077
WORK="$HOME/trojanpanelnext-rc2"
ARCHIVE=trojanpanelnext-installer-0.1.0-rc.2.tar.gz
EXPECTED_SHA256=84366904c9884fdb7bb9d6767c7d3d6958b83e530fccb73e156b4d3d90b59471
mkdir -p "$WORK/assets" "$WORK/config"
chmod 700 "$WORK" "$WORK/config"
curl -fL --retry 3 "https://github.com/1linhao/trojanpanelnext/releases/download/v0.1.0-rc.2/$ARCHIVE" -o "$WORK/$ARCHIVE"
if command -v sha256sum >/dev/null 2>&1; then
  printf '%s  %s\n' "$EXPECTED_SHA256" "$WORK/$ARCHIVE" | sha256sum -c -
else
  printf '%s  %s\n' "$EXPECTED_SHA256" "$WORK/$ARCHIVE" | shasum -a 256 -c -
fi
tar -xzf "$WORK/$ARCHIVE" -C "$WORK/assets"
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$WORK/assets" && sha256sum -c SHA256SUMS)
else
  (cd "$WORK/assets" && shasum -a 256 -c SHA256SUMS)
fi
cp "$WORK/assets/config-web.yaml" "$WORK/config/web.yaml"
chmod 600 "$WORK/config/web.yaml"
~~~

编辑 $WORK/config/web.yaml：hostname 填 Web 域名，email 填联系邮箱；密码首装可留空。保留 asset_version 和镜像摘要，不修改 assets 目录中的原始模板。任一校验失败就停止。后续本地命令在同一个终端执行，以保留 WORK、ARCHIVE 和 SSH 变量。

~~~bash
WEB_SSH=root@WEB_IP
ssh "$WEB_SSH" 'mkdir -p ~/tpnext-upload && chmod 700 ~/tpnext-upload'
scp "$WORK/$ARCHIVE" "$WORK/config/web.yaml" "$WEB_SSH:tpnext-upload/"
~~~

### Web VPS：安装并登记 Node

登录 Web VPS，在 root shell 执行。新系统可能没有 curl，先安装它；VPS 无需安装 gh 或 Git。

~~~bash
set -e
apt-get update
apt-get install -y ca-certificates curl
cd ~/tpnext-upload
ARCHIVE=trojanpanelnext-installer-0.1.0-rc.2.tar.gz
printf '%s  %s\n' '84366904c9884fdb7bb9d6767c7d3d6958b83e530fccb73e156b4d3d90b59471' "$ARCHIVE" | sha256sum -c -
mkdir -p assets
tar -xzf "$ARCHIVE" -C assets
(cd assets && sha256sum -c SHA256SUMS)
chmod 600 web.yaml
./assets/bootstrap.sh validate --mode web --config "$PWD/web.yaml"
./assets/bootstrap.sh install --mode web --config "$PWD/web.yaml"
~~~

安装器会安装其他缺失依赖；生成的密码会写回 Web VPS 的 web.yaml，不要再用本地空密码配置覆盖。确认面板 HTTPS 可访问后，修改下面的 Node 域名和公网 IP，登记身份并记下输出的身份 ID：

~~~bash
install -d -m 700 /tpdata/trojan-panel/config/node-identities
docker exec trojan-panel /tpdata/trojan-panel/trojan-panel node-identity register --name node-1 --domain node.example.com --public-ip NODE_IP --credential-file /tpdata/trojan-panel/config/node-identities/node-1.g1.json
~~~

### 本地电脑：填写 Node 配置

~~~bash
cp "$WORK/assets/config-node.yaml" "$WORK/config/node.yaml"
chmod 600 "$WORK/config/node.yaml"
~~~

编辑 $WORK/config/node.yaml：hostname 和 grpc_tls_server_name 填登记的 Node 域名；mariadb_host、redis_host 填 Web VPS 可达地址；control_plane_public_ip 填 Web 公网 IP；email 填联系邮箱。Web 改过数据库或 Redis 端口时，同步修改 Node 对应端口。身份 ID、代次和数据库/Redis 凭据由 Web 打包时注入，不手填模板占位值；保留 asset_version 与镜像摘要。

必须先编辑 Node 配置，再制作加密包；加密后修改本地 YAML 不会改变包内配置。编辑完成后发送到 Web VPS：

~~~bash
scp "$WORK/config/node.yaml" "$WEB_SSH:tpnext-upload/node.yaml"
~~~

### Web VPS：生成加密引导包

再次登录 Web VPS，在 root shell 执行。命令会交互式询问至少 12 字符的口令；Node 安装时需要同一口令。不要把口令写进命令行或配置文件。

~~~bash
cd ~/tpnext-upload
chmod 600 node.yaml
./assets/node-bundle create --credential-file /tpdata/trojan-panel/config/node-identities/node-1.g1.json --node-config "$PWD/node.yaml" --client-ca /tpdata/trojanpanelnext-pki/client-ca.crt --output "$PWD/node-1.g1.age"
~~~

只传输加密后的 node-1.g1.age；Web 上的明文身份文件和 CA 私钥不离开 Web VPS。

### 本地电脑：发送安装包和加密包到 Node

~~~bash
scp "$WEB_SSH:tpnext-upload/node-1.g1.age" "$WORK/node-1.g1.age"
chmod 600 "$WORK/node-1.g1.age"
NODE_SSH=root@NODE_IP
ssh "$NODE_SSH" 'mkdir -p ~/tpnext-upload && chmod 700 ~/tpnext-upload'
scp "$WORK/$ARCHIVE" "$WORK/node-1.g1.age" "$NODE_SSH:tpnext-upload/"
~~~

### Node VPS：安装；Web VPS：验证身份

登录 Node VPS，在 root shell 执行。Node VPS 不需要 gh、Git 或 Web 的明文身份文件。

~~~bash
set -e
apt-get update
apt-get install -y ca-certificates curl
cd ~/tpnext-upload
ARCHIVE=trojanpanelnext-installer-0.1.0-rc.2.tar.gz
printf '%s  %s\n' '84366904c9884fdb7bb9d6767c7d3d6958b83e530fccb73e156b4d3d90b59471' "$ARCHIVE" | sha256sum -c -
mkdir -p assets
tar -xzf "$ARCHIVE" -C assets
(cd assets && sha256sum -c SHA256SUMS)
chmod 600 node-1.g1.age
./assets/bootstrap.sh validate --mode node --bundle "$PWD/node-1.g1.age"
./assets/bootstrap.sh install --mode node --bundle "$PWD/node-1.g1.age"
~~~

保持 Node 安装终端运行。它输出 challenge 后，另开终端登录 Web VPS，用登记输出的身份 ID 和本次 challenge 验证：

~~~bash
docker exec trojan-panel /tpdata/trojan-panel/trojan-panel node-identity verify --id '填入登记输出的身份 ID' --challenge '填入 Node 安装输出的 challenge'
~~~

验证成功后 Node 安装完成。按 /tpdata/trojanpanelnext-network/allowlist.md 配置防火墙和云安全组；Web 公网入口通常为 80/443，Node 默认为 80/8863，Node gRPC 8100 只允许 Web 访问。

### 配置字段：Web / Node

字段都位于 YAML 的 trojanpanelnext 下。未要求修改的路径、端口和镜像摘要先保留模板值。

| 字段 | 作用 |
| --- | --- |
| schema_version、asset_version、deployment_mode | 格式、安装包版本和部署模式；保留模板值。 |
| hostname、email | 当前 VPS 的域名和 ACME 联系邮箱；Node 域名须与登记时相同。 |
| caddy_image、mariadb_image、redis_image、api_image、web_image、node_agent_image | 固定摘要的镜像；不要改为 latest。 |
| mariadb_port、redis_port | Web 的数据库/Redis 端口；Node 上须与 Web 一致。 |
| mariadb_password、redis_password、sysadmin_password | Web 首装可留空，生成后写回 Web VPS 配置。 |
| panel_port、ui_port | Web 面板 API/UI 的本机端口，由 Caddy 反代。 |
| mariadb_host、redis_host | Node 连接 Web 数据服务的地址。 |
| mariadb_user、mariadb_password、redis_username、redis_password、redis_auth_username、redis_auth_password | Node 凭据由加密包注入，不手填占位值。 |
| database、account_table | Node 使用的数据库与账号表；通常保留默认值。 |
| node_server_id、node_identity_id、node_identity_generation | Node 的服务器 ID、身份 UUID 和代次；由加密包注入。 |
| control_plane_public_ip | Web 公网 IP，用于限定 Node 数据入口的来源。 |
| grpc_tls_mode、grpc_tls_server_name、grpc_port、core_port | Node mTLS 模式、证书名称、gRPC 与内核端口。 |
| node_caddy_http_port、node_caddy_https_port | Node Caddy 的 HTTP/HTTPS 端口。 |
| pki_bundle_dir、grpc_client_ca_path、grpc_client_cert_path、grpc_client_key_path、grpc_server_ca_path | mTLS 证书及密钥路径；首次安装保留默认值。 |
| kernel_runtime_path | Node 内核运行目录；保留默认值。 |
| force、purge_data | 强制重建及卸载时清除数据；首装保持 0。 |

## combined：在一台 VPS 部署 Web 和 Node

VPS 使用 Debian 12 x86_64、至少 1 GiB 内存。准备两个不同的域名，均解析到这台 VPS。命令以 root SSH 登录为例；若使用普通用户，传输后运行 sudo -i，并将 cd ~/tpnext-upload 改为该用户上传目录的绝对路径。

### 本地电脑：下载安装包，填写 combined 配置

本地需要 curl、tar、ssh/scp 和 SHA-256 校验工具；macOS 可使用 shasum。此流程可独立执行：

~~~bash
set -e
umask 077
WORK="$HOME/trojanpanelnext-rc2"
ARCHIVE=trojanpanelnext-installer-0.1.0-rc.2.tar.gz
EXPECTED_SHA256=84366904c9884fdb7bb9d6767c7d3d6958b83e530fccb73e156b4d3d90b59471
mkdir -p "$WORK/assets" "$WORK/config"
chmod 700 "$WORK" "$WORK/config"
curl -fL --retry 3 "https://github.com/1linhao/trojanpanelnext/releases/download/v0.1.0-rc.2/$ARCHIVE" -o "$WORK/$ARCHIVE"
if command -v sha256sum >/dev/null 2>&1; then
  printf '%s  %s\n' "$EXPECTED_SHA256" "$WORK/$ARCHIVE" | sha256sum -c -
else
  printf '%s  %s\n' "$EXPECTED_SHA256" "$WORK/$ARCHIVE" | shasum -a 256 -c -
fi
tar -xzf "$WORK/$ARCHIVE" -C "$WORK/assets"
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$WORK/assets" && sha256sum -c SHA256SUMS)
else
  (cd "$WORK/assets" && shasum -a 256 -c SHA256SUMS)
fi
cp "$WORK/assets/config-combined.yaml" "$WORK/config/combined.yaml"
chmod 600 "$WORK/config/combined.yaml"
~~~

编辑 $WORK/config/combined.yaml：web_hostname 填面板域名，node_hostname 填 Node 域名，node_public_ip 填 VPS 公网 IP，node_name 填 Node 名称，email 填联系邮箱。保留镜像摘要、asset_version 和身份文件路径。只编辑 config 目录中的副本，校验失败则停止。

在同一个本地终端发送安装包和配置：

~~~bash
COMBINED_SSH=root@COMBINED_IP
ssh "$COMBINED_SSH" 'mkdir -p ~/tpnext-upload && chmod 700 ~/tpnext-upload'
scp "$WORK/$ARCHIVE" "$WORK/config/combined.yaml" "$COMBINED_SSH:tpnext-upload/"
~~~

### combined VPS：安装

登录 combined VPS，在 root shell 执行；无需安装 gh 或 Git。

~~~bash
set -e
apt-get update
apt-get install -y ca-certificates curl
cd ~/tpnext-upload
ARCHIVE=trojanpanelnext-installer-0.1.0-rc.2.tar.gz
printf '%s  %s\n' '84366904c9884fdb7bb9d6767c7d3d6958b83e530fccb73e156b4d3d90b59471' "$ARCHIVE" | sha256sum -c -
mkdir -p assets
tar -xzf "$ARCHIVE" -C assets
(cd assets && sha256sum -c SHA256SUMS)
chmod 600 combined.yaml
./assets/bootstrap.sh validate --mode combined --config "$PWD/combined.yaml"
./assets/bootstrap.sh install --mode combined --config "$PWD/combined.yaml"
~~~

确认面板 HTTPS、Node gRPC/mTLS 健康，并按 /tpdata/trojanpanelnext-network/allowlist.md 配置防火墙和云安全组。combined 的 Caddy 入口占用 80/443；安装器不会自行修改防火墙。

### 配置字段：combined

字段都位于 YAML 的 trojanpanelnext 下。未要求修改的端口、路径及镜像摘要先保留模板值。

| 字段 | 作用 |
| --- | --- |
| schema_version、asset_version、deployment_mode | 格式、安装包版本和部署模式；保留模板值。 |
| web_hostname、node_hostname | 面板与 Node 的两个不同域名。 |
| node_name、node_public_ip | 本机 Node 名称与 VPS 公网 IP。 |
| node_identity_credential_file | 本机 Node 身份文件路径；保留默认值。 |
| email | ACME 联系邮箱。 |
| caddy_image、mariadb_image、redis_image、api_image、web_image、node_agent_image | 固定摘要的镜像；不要改为 latest。 |
| mariadb_port、redis_port | 本机数据库和 Redis 端口。 |
| panel_port、ui_port | 面板 API/UI 本机端口，由共享 Caddy 反代。 |
| core_port、grpc_port、grpc_tls_mode | Node 内核、gRPC 端口与 mTLS 模式。 |
| node_caddy_http_port、node_caddy_https_port | 共享入口的 HTTP/HTTPS 端口，默认 80/443。 |
| pki_bundle_dir | mTLS 材料存放目录；保留默认值。 |
| force、purge_data | 强制重建及卸载时清除数据；首装保持 0。 |

配置文件、生成的密码和加密包口令属于敏感信息；仅存于受限目录，不上传到公开仓库。
