# 统一部署配置：本地初始化

简体中文 | [English](README_EN.md)

在 Linux、macOS 或 Windows WSL 操作机安装 Bash、curl、tar、OpenSSH、`sha256sum` 或 `shasum`，以及自行安装 [mikefarah yq v4](https://github.com/mikefarah/yq#install)。正式入口是同一固定 GitHub Release 的归档：先从 Release 页面核对 tag、归档 SHA256 和发布证明，再下载归档并校验 SHA256，最后运行归档内的 CLI。`v0.1.0-rc.3` 尚不包含统一 CLI，请使用包含该资产的新版本。以下占位值须替换为同一 Release 的真实值。

```bash
TAG='v<version>'
SHA256='<release-archive-sha256>'
ARCHIVE="trojanpanelnext-installer-${TAG#v}.tar.gz"
curl -fL "https://github.com/1linhao/trojanpanelnext/releases/download/${TAG}/${ARCHIVE}" -o "$ARCHIVE"
printf '%s  %s\n' "$SHA256" "$ARCHIVE" | sha256sum -c -
mkdir -m 700 .tpnext-release
tar -xzf "$ARCHIVE" -C .tpnext-release
(cd .tpnext-release && sha256sum -c SHA256SUMS)
bash .tpnext-release/client/tpnext.sh init \
  --config "$PWD/deployment.local.yaml" \
  --tag "$TAG" --sha256 "$SHA256" --archive "$PWD/$ARCHIVE" \
  --work-dir "$PWD/deployment.local"
bash .tpnext-release/client/tpnext.sh plan --config "$PWD/deployment.local.yaml"
```

macOS 将两处 `sha256sum` 换成 `shasum -a 256`。`init` 再次校验固定归档 SHA、归档内每项 SHA/manifest、版本和镜像 digest，校验前不运行归档内其他程序；它只在本机创建配置和工作目录，不连接 VPS。操作者只调用 `client/tpnext.sh`；同包内的拓扑解析、校验器和模板是 CLI 的内部资产。归档还提供 `config-web.yaml`、`config-node.yaml`、`config-combined.yaml`，供后续 #38 按唯一宿主编译远端 installer 配置，不将统一 YAML 直接交给远端 installer。`check` 和 `deploy` 由后续工单提供。

默认模板表示外部电脑通过 SSH 操作独立 Web 和 Node VPS。若操作机是 Web VPS，使用 `--web-transport local`；生成的示例让一个 Node 与 Web 引用同一 `web-host`，规划为 `combined`。编辑 YAML 可把 Node 改为另一个 `ssh` host，或追加多个各自独立的 Node。远端 `combined` 也可将 Node 的 host 改成 Web host，并把 `node_caddy_https_port` 设为 443；共享入口使用 80/443。一个 Web 宿主最多引用一个 Node；Web 与 Node 域名必须不同。同一宿主由相同 host id 显式表示，不由 IP 或 SSH 别名推断。

`hosts` 只描述执行连接；`web.domain`、`web.public_ip` 与每个 Node 的 `domain`、`public_ip` 描述服务地址。SSH 可用 root key 登录，或普通用户 key 登录并在后续部署阶段使用 `sudo -n`。`node_key` 是后续 Node 状态路径的稳定键；追加或重排数组不会改变已有路径。删除或改名并不授权卸载、撤销或轮换。`plan` 仅做 YAML/拓扑校验并打印无秘密的逐宿主计划；本票不提供 `check` 或 `deploy`。

配置文件权限为 0600，验证资产工作目录为 0700。请使用 `*.local.yaml` 配置名；在 Git 工作树内，CLI 只接受已被 Git ignore 的配置和工作目录。已有配置和工作目录不会被覆盖。不要把 Web 高权限统一 YAML 拷到 Node VPS。
