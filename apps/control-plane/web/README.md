# Trojan Panel UI · Frosted Glass

Trojan Panel 的响应式 Web 管理界面。本分支采用统一的磨砂玻璃（Frosted Glass）设计语言，覆盖管理员与普通用户页面，并适配桌面和手机浏览器。

## 主要特性

- 亮色、暗色主题首次载入时跟随浏览器，并在浏览器主题变化时实时同步
- 海蓝、紫罗兰、翡翠、琥珀四套颜色主题；颜色选择保存在当前浏览器中
- 向支持 `theme-color` 的手机浏览器同步页面主题色
- 桌面侧栏与手机横向滑动导航使用一致的图标和状态样式
- 表格、表单、日期选择器、下拉层、弹窗和加载状态使用统一控件
- 响应式管理页面以及管理员、普通用户两种界面权限

> 浏览器顶栏和底栏是否采用网页主题色由浏览器及系统版本决定。Via、Chrome 等浏览器需开启“跟随网页颜色”或同类选项。

## 本地开发

要求 Node.js `^20.19.0 || >=22.12.0`（推荐 Node 22）和 Yarn Classic 1.22。项目基于 Vue 2.7.16、Vite 7 与官方 `@vitejs/plugin-vue2`。不需要 OpenSSL legacy 参数，也不会修改系统软件。

```bash
npx --yes yarn@1.22.22 install --frozen-lockfile
npm run serve
```

`serve` 与 `build` 会先从各资源包公开源码生成本地 `dist`，因此干净克隆不需要预先运行资源包测试，也不会依赖未纳入版本控制的旧构建产物。

默认开发地址为 `http://127.0.0.1:8888/`，接口由开发服务器代理至 `http://127.0.0.1:8081/`。仓库提供本地界面测试用的模拟接口：

```bash
MOCK_API_PORT=18081 node tests/mock-api-server.js
# 另一个终端：
MOCK_API_TARGET=http://127.0.0.1:18081 npm run serve -- --port 18888
```

## 构建

```bash
npm run build
```

构建保留生产应用代码混淆，框架与第三方依赖分包不混淆；页面使用动态导入按需加载。`src/settings.js` 使用 ESM，客户端 API 前缀通过 `.env` 中的 `VITE_BASE_API` 配置（默认 `/api`）。`MOCK_API_TARGET` 只供开发/预览服务器使用，不暴露给浏览器。

```bash
npm run lint -- --no-fix
npm run test:ui-libraries
npm run test:ui-cleanup
npm run test:vite-proxy
npm run build:ui-labs
npm run build
# 上面的 mock 与 18888 开发服务保持运行；需要本机已有 Chromium / ChromeDriver
npm run test:live-stack:e2e
```

`test:live-stack:e2e` 使用本地模拟账号与验证码测试数据，仅针对模拟后端。测试覆盖登录、系统配置、三种订阅模板编辑/格式化、移动导航及浏览器错误。生产产物可通过 `MOCK_API_TARGET=http://127.0.0.1:18081 npm run preview -- --port 18889` 预览，再用 `LIVE_WEB_URL=http://127.0.0.1:18889 npm run test:live-stack:e2e` 验证。

`npm run build` 会清空 `dist/`，如需把实验室一起放入产物，请在主构建后再执行 `npm run build:ui-labs`。Yarn lock 是依赖锁定来源，CI 和部署构建应使用 `--frozen-lockfile`；npm 可用于运行脚本。

构建产物位于 `dist/`，可通过 Nginx 或项目 Docker 镜像部署。

## 相关项目

- [Trojan Panel](https://github.com/trojanpanel/trojan-panel)
- [Trojan Panel UI 上游项目](https://github.com/trojanpanel/trojan-panel-ui)
- [trojan](https://github.com/trojan-gfw/trojan)
- [trojan-go](https://github.com/p4gefau1t/trojan-go)
- [Xray-core](https://github.com/XTLS/Xray-core)
- [hysteria](https://github.com/HyNetwork/hysteria)
- [naiveproxy](https://github.com/klzgrad/naiveproxy)
