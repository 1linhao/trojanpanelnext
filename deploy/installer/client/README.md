# 统一部署配置：本地初始化

[English](README_EN.md)

在 Linux、macOS 或 Windows WSL 操作机安装 Bash、curl、tar、OpenSSH、`sha256sum` 或 `shasum`，以及自行安装 [mikefarah yq v4](https://github.com/mikefarah/yq#install)。`init` 不安装依赖，也不连接 VPS。先从对应 GitHub Release 核对固定 tag 和安装包 SHA256；下例 SHA 只属于 rc.3。

```bash
bash deploy/installer/client/tpnext.sh init \
  --config "$PWD/deployment.local.yaml" \
  --tag v0.1.0-rc.3 \
  --sha256 fe4e2b297756bf3a58db31f69636dd1ca8034196ef14e1184db14c4f8362d668
bash deploy/installer/client/tpnext.sh plan --config "$PWD/deployment.local.yaml"
```

默认模板表示外部电脑通过 SSH 操作独立 Web 和 Node VPS。若操作机是 Web VPS，使用 `--web-transport local`；生成的示例让一个 Node 与 Web 引用同一 `web-host`，规划为 `combined`。编辑 YAML 可把 Node 改为另一个 `ssh` host，或追加多个各自独立的 Node。远端 `combined` 也可将 Node 的 host 改成 Web host，并把 `node_caddy_https_port` 设为 443；共享入口使用 80/443。一个 Web 宿主最多引用一个 Node；Web 与 Node 域名必须不同。同一宿主由相同 host id 显式表示，不由 IP 或 SSH 别名推断。

`hosts` 只描述执行连接；`web.domain`、`web.public_ip` 与每个 Node 的 `domain`、`public_ip` 描述服务地址。SSH 可用 root key 登录，或普通用户 key 登录并在后续部署阶段使用 `sudo -n`。`node_key` 是后续 Node 状态路径的稳定键；追加或重排数组不会改变已有路径。删除或改名并不授权卸载、撤销或轮换。`plan` 仅做 YAML/拓扑校验并打印无秘密的逐宿主计划；本票不提供 `check` 或 `deploy`。

配置文件权限为 0600，验证资产工作目录为 0700。请使用 `*.local.yaml` 配置名；在 Git 工作树内，CLI 只接受已被 Git ignore 的配置和工作目录。已有配置和工作目录不会被覆盖。不要把 Web 高权限统一 YAML 拷到 Node VPS。
