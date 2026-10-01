# TrojanPanel Next 脚本库

简体中文 | [English](README_EN.md)

`tp.sh` 是 v1.0.2-rc.2 的统一入口，按选定版本下载对应命令、公共依赖及配置模板。完整说明见[部署指南](../docs/deployment.md)。

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) --help
```

| 命令 | 用途 |
| --- | --- |
| `deps install` | 在支持的 Debian/Ubuntu 上安装依赖，复用已有兼容 Docker 和 yq |
| `deps remove` | 按安装记录卸载新增 Docker 软件包和未修改的 yq |
| `web` | 提示填写 Web 配置并安装，或通过 `--config` 使用已有配置 |
| `node` | 提示填写 Node 配置并安装，支持 `--certificate-mode external` |
| `config web\|node --output <file>` | 生成选定版本的配置模板 |
| `validate --config <file>` | 校验 YAML 和版本、字段 |
| `install --config <file> [--force]` | 使用配置部署或重建服务 |
| `--version <target> update --config <file>` | 更新选定版本的 Web 或 Node 产品镜像，保留配置和数据 |
| `remove --config <file> --keep-data` | 卸载并保留数据 |
| `remove --config <file> --purge-data` | 卸载并删除项目数据 |
| `--version <version>` | 选择发布版本，可放在命令前或后 |
| `--entry-version` | 显示入口默认版本 |

部署实现与模板位于 `deploy/`；下载和调用所需文件由入口完成。目录及命令之间的依赖不需要手工拼装。安装和校验使用同一发布版本的配置、脚本与产品镜像；更新命令将现有受支持配置绑定到选定的目标发布版本。

在 root Bash 中先单独安装依赖，再执行 `web`、`node` 或 `install`；部署命令只检查依赖。

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) deps install
```

自动依赖安装支持运行 systemd 的 Debian 12/13、Ubuntu 22.04/24.04（amd64/arm64）。入口自身需要 Bash、curl、CA 证书、grep、coreutils；`deps` 还需 util-linux 的 `flock`。最小引导与其他 Linux 的手动安装见[依赖指南](../docs/deployment.md#dependencies)。

## 配置文件部署

下载模板后编辑，再校验和安装；模板权限为 `0600`，已有文件不会覆盖。

Web 模板：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) config web --output ./web.yaml
```

Node 模板：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) config node --output ./node.yaml
```

用 `nano` 或 `vi` 编辑，所有字段位于 `trojanpanelnext` 映射内。Web 至少改 `hostname`、`email`；首次安装的数据库和 Redis 空密码会生成后写回配置。Node 至少改域名、Caddy 邮箱、Web 数据库和 Redis 地址/实际密码、真实服务器 ID 与 TLS 服务器名，安装前准备 Web 公开 CA。外部证书模式改 `node_certificate_mode: external` 和已存在的证书/私钥绝对路径，无需邮箱。

[Web 最少修改](../docs/deployment.md#configuration-web-minimum) · [登记服务器](../docs/deployment.md#node-registration) · [Node 最少修改](../docs/deployment.md#configuration-node-minimum) · [Node CA 准备](../docs/deployment.md#configuration-node-ca)

Web 校验：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) validate --config ./web.yaml
```

Web 安装：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) install --config ./web.yaml
```

Node 校验：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) validate --config ./node.yaml
```

Node 安装：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) install --config ./node.yaml
```

## 更新产品镜像

在对应的 Web 或 Node 主机执行，使用实际部署 YAML。必须指定目标版本；Node 使用 `./node.yaml`。更新保留凭据、数据、端口和 PKI，不升级 MariaDB、Redis 或 Caddy。配置兼容范围、备份、短暂中断和恢复限制见[镜像更新](../docs/deployment.md#updates)。

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) --version 1.0.2-rc.2 update --config ./web.yaml
```

## 卸载依赖

卸载项目和其他 Docker 服务后，按安装记录清理新增依赖：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0.2-rc.2/scripts/tp.sh) deps remove
```

已有依赖、基础工具、Docker 数据和外部证书保留。Docker 仍有容器（含停止容器）、共享 containerd 中其他 namespace 仍有容器、Node 维护服务尚未清理或运行状态无法确认时拒绝删除 Docker。依赖安装中断时先重跑 `deps install` 修复，再卸载。完整范围见[依赖卸载](../docs/deployment.md#dependency-removal)。`deps` 同样支持 `--version <版本号>`。

[依赖安装](../docs/deployment.md#dependency-install) · [一键部署](../docs/deployment.md#web) · [版本选择](../docs/deployment.md#versions) · [配置字段](../docs/deployment.md#configuration) · [镜像更新](../docs/deployment.md#updates) · [项目卸载](../docs/deployment.md#removal) · [依赖卸载](../docs/deployment.md#dependency-removal)
