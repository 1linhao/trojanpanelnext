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

API 服务器移除集成测试需要一次性 MySQL/MariaDB 实例。在 `apps/control-plane/api/` 中为当前终端设置 `TP_REMOVAL_TEST_DSN` 后执行：

```bash
go test ./dao -run '^TestNodeRemovalIntegration$' -count=1
```

DSN 用户需具备创建和删除测试数据库的权限；每个子测试使用独立数据库并在结束后清理，不修改 DSN 中指定的数据库。未设置该变量时集成测试跳过；这些 DAO 测试使用共享连接，不能并行运行。测试覆盖纯 Web 删除、远程卸载两种数据模式、失败回滚、重复删除及跨服务器共享记录的隔离。

用户备注 DAO 测试也使用 `TP_REMOVAL_TEST_DSN`，执行 `go test ./dao -run '^TestAccountRemarkMigrationAndDAO$' -count=1`，覆盖新旧表迁移、Unicode 文本和省略更新保留备注。

备注 API 权限测试使用专用的一次性 MariaDB 和 Redis 实例。设置 `TP_ACCOUNT_REMARK_TEST_DSN`（MariaDB DSN）及 `TP_ACCOUNT_REMARK_TEST_REDIS`（Redis `主机:端口`）后，执行 `go test ./api -run '^TestAccountRemarkAPIPrivacy$' -count=1`。测试会创建并在结束后删除 `trojan_panel_db`；该数据库已存在时拒绝运行。Redis 测试实例需无需认证、仅用于测试。覆盖 sysadmin / admin / 普通用户、旧会话降权、备注保存与清空，以及登录、个人资料和订阅不泄漏备注。未设置变量时跳过该集成测试。

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

### 界面规范

功能入口放在所属管理页。仪表盘新增面板需要明确的产品需求，不承担安装说明入口；Node 登记与部署操作放在服务器管理中。

复用已有 `UiDialog` / Liquid 系列控件和 UI contracts 中的语义变量。说明采用结构化表单标签、标准帮助、弹窗或文档链接，不在业务页面堆积小字说明。关键数字 ID 与地址分列，使用正常字号和足够的对比度。

<a id="browser-tests"></a>
## 浏览器测试

需要已有 Chromium 和 ChromeDriver。

`npm run test:server-delete:e2e` 启动隔离模拟 API、Web 和 ChromeDriver，使用端口 `18081`、`18082`、`18888`、`9518`。测试检查服务器移除弹窗的“取消”“删除”“卸载”“彻底卸载”、纯 Web 删除与远程卸载的 API 请求分派、取消不发请求、离线卸载失败保留记录后显式删除，以及桌面和手机布局。产物位于 `.local/server-delete-dialog/`。

该浏览器测试使用模拟 API 请求记录器，只验证界面行为和请求参数，不执行真实 Node 宿主机卸载或 SQL 清理。服务器数据清理的事务原子性、协议配置清理及其他服务器的数据隔离由 API 的 `dao/node_removal_integration_test.go` 覆盖，运行方式见[API 与 Node Agent](#go-services)。

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
bash tests/deploy/update_test.sh
bash tests/deploy/uninstall_test.sh
bash tests/deploy/persistence_test.sh
bash tests/deploy/external_certificate_test.sh
bash tests/deploy/quick_deploy_test.sh
```

测试使用 Bash、Docker 测试替身和 mikefarah/yq v4。ShellCheck 用于检查 `scripts/` 和 `tests/deploy/` 内的 Bash 脚本。发布标签、脚本声明、模板和产品镜像必须绑定到同一版本。
