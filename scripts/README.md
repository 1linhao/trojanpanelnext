# TrojanPanel Next 脚本库

简体中文 | [English](README_EN.md)

`tp.sh` 是 v1.0 的统一入口，按选定版本下载对应命令、公共依赖及配置模板。完整说明见[部署指南](../docs/deployment.md)。

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/1linhao/trojanpanelnext/v1.0/scripts/tp.sh) --help
```

| 命令 | 用途 |
| --- | --- |
| `web` | 提示填写 Web 配置并安装，或通过 `--config` 使用已有配置 |
| `node` | 提示填写 Node 配置并安装，支持 `--certificate-mode external` |
| `config web\|node --output <file>` | 生成选定版本的配置模板 |
| `validate --config <file>` | 校验 YAML 和版本、字段 |
| `install --config <file> [--force]` | 使用配置部署或重建服务 |
| `remove --config <file> --keep-data` | 卸载并保留数据 |
| `remove --config <file> --purge-data` | 卸载并删除项目数据 |
| `--version <version>` | 选择发布版本，可放在命令前或后 |
| `--entry-version` | 显示入口默认版本 |

部署实现与模板位于 `deploy/`；下载和调用所需文件由入口完成。目录及命令之间的依赖不需要手工拼装。配置、脚本与产品镜像必须属于同一发布版本。

[一键部署](../docs/deployment.md#web) · [版本选择](../docs/deployment.md#versions) · [配置字段](../docs/deployment.md#configuration) · [卸载](../docs/deployment.md#removal)
