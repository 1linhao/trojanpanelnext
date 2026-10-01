# 开发指南

[文档首页](README.md) · [English](development_EN.md)

## 目录

- [源码结构](#source-layout)
- [API 与 Node Agent](#go-services)
- [Web 界面](#web-ui)
- [浏览器测试](#browser-tests)
- [文档站](#documentation-site)
- [脚本库](#script-library)

<a id="source-layout"></a>
## 源码结构

| 路径 | 内容 |
| --- | --- |
| `apps/control-plane/api/` | Web API、业务数据和 mTLS 身份维护 |
| `apps/control-plane/web/` | Vue 2.7 / Vite Web 界面 |
| `apps/control-plane/web/packages/` | 内部 UI 契约、组件、主题、布局、图标与动画包 |
| `apps/control-plane/web/examples/` | 可运行的 UI 组合示例与自动化验证入口 |
| `apps/node-agent/` | Node Agent、代理运行时和宿主机维护服务 |
| `apps/docs-site/` | VuePress 文档站 |
| `scripts/` | 用户部署脚本入口、命令与配置模板 |
| `tests/deploy/` | 部署脚本测试 |
| `tools/` | 仓库完整性、文档和发布检查工具 |
| `docs/` | 当前版本的部署、操作、证书、API 和开发文档 |

测试产物、临时部署配置和调查记录放在被 Git 忽略的 `.local/`；生成的 Web、UI 包与文档站产物不提交到仓库。

<a id="go-services"></a>
## API 与 Node Agent

CI 使用 Go 1.23。分别在 `apps/control-plane/api/` 和 `apps/node-agent/` 执行：

```bash
go test ./...
go build ./...
```

Node 宿主机维护服务的入口是 `apps/node-agent/cmd/host-agent/`。运行服务需要对应的数据库、Redis、配置及证书；部署参数见[部署指南](deployment.md)。

<a id="web-ui"></a>
## Web 界面

要求 Node.js `^20.19.0 || >=22.12.0`（CI 使用 Node 22）与 Yarn Classic 1.22。在 `apps/control-plane/web/` 执行：

```bash
npx --yes yarn@1.22.22 install --frozen-lockfile
npm run serve
```

默认地址是 `http://127.0.0.1:8888/`，API 默认代理到 `http://127.0.0.1:8081/`。可在一个终端启动模拟 API，再在另一个终端启动界面：

```bash
MOCK_API_PORT=18081 node tests/mock-api-server.js
```

```bash
MOCK_API_TARGET=http://127.0.0.1:18081 npm run serve -- --port 18888
```

`MOCK_API_TARGET` 仅用于开发与预览服务器。`VITE_BASE_API` 设置浏览器 API 前缀，默认 `/api`。

```bash
npm run lint -- --no-fix
npm run test:ui-libraries
npm run test:ui-cleanup
npm run test:vite-proxy
npm run build
```

`serve` 和 `build` 会先构建内部 UI 包，Web 构建输出在 `dist/`。UI 包为私有 workspace 包，使用各自 `package.json` 的版本；它们的版本与产品发布标签独立。依赖锁定来源是 `yarn.lock`。

<a id="browser-tests"></a>
## 浏览器测试

需要已有 Chromium 和 ChromeDriver。

`npm run test:server-delete:e2e` 启动隔离模拟 API、Web 和 ChromeDriver，使用端口 `18081`、`18082`、`18888`、`9518`。测试覆盖服务器删除弹窗、两种删除模式、取消、失败重试和移动布局，产物位于 `.local/server-delete-dialog/`。

`npm run test:live-stack:e2e` 使用上面启动的模拟 API 与 `18888` 界面，覆盖登录、系统设置、订阅模板和移动导航。生产构建可通过 `MOCK_API_TARGET=http://127.0.0.1:18081 npm run preview -- --port 18889` 预览，再指定 `LIVE_WEB_URL=http://127.0.0.1:18889` 执行相同测试。

`npm run build:ui-labs` 构建 `examples/` 中的 UI 组合示例。若需与主应用放在同一个 `dist/`，在主构建后执行此命令，因为主构建会清空输出目录。`npm run test:ui-labs:e2e` 检查组件、材质与布局的组合行为。

<a id="documentation-site"></a>
## 文档站

在 `apps/docs-site/` 执行：

```bash
npx --yes pnpm@8.15.9 install --frozen-lockfile
NODE_OPTIONS=--openssl-legacy-provider npx --yes pnpm@8.15.9 run docs:build
```

VuePress 1 构建需要上述 OpenSSL 参数；Web 的 Vite 构建不需要它。文档站输出到 `apps/docs-site/docs/`，完整部署操作的维护源是 `docs/deployment.md`。

<a id="script-library"></a>
## 脚本库

用户命令位于 `scripts/`，完整接口见[脚本库说明](../scripts/README.md)。在仓库根目录执行：

```bash
node tools/check-readme-languages.mjs
node tools/check-installer-release.mjs
bash tests/deploy/installer_cli_test.sh
bash tests/deploy/entrypoint_test.sh
bash tests/deploy/uninstall_test.sh
bash tests/deploy/persistence_test.sh
bash tests/deploy/external_certificate_test.sh
bash tests/deploy/quick_deploy_test.sh
```

测试使用 Bash、Docker 测试替身和 mikefarah/yq v4。ShellCheck 用于检查 `scripts/` 和 `tests/deploy/` 内的 Bash 脚本。发布标签、脚本声明、模板和产品镜像必须绑定到同一版本。
