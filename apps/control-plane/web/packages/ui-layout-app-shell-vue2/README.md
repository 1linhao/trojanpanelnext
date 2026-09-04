# @tp-ui/layout-app-shell-vue2

Vue 2 应用外壳布局包。它只负责 Header、桌面导航、移动导航、内容视口和响应式几何，并通过意图事件与业务应用通信。

## 职责

- 渲染规范化 `ShellModel`。
- 在桌面和平板/手机断点间切换导航布局。
- 保持溢出移动导航的当前项可见。
- 通过 slots 注入品牌、图标、用户信息与附加动作。
- 发出 `navigate`、`logout`、`action` 等意图事件。
- 暴露 `data-ui-component="app-shell"` 和稳定的语义 Surface。

## 非职责

- 不读取 Router、Vuex、Token、角色或业务 API。
- 不依赖组件包、材质包、图标包或动画实现包。
- 不拥有颜色、阴影或模糊配方。
- 不硬编码动画模式；滚动行为消费 `--ui-motion-scroll-behavior`。

## 安装

```json
{
  "dependencies": {
    "@tp-ui/layout-app-shell-vue2": "0.1.0",
    "@tp-ui/contracts": "0.1.0"
  },
  "peerDependencies": {
    "vue": "^2.6.11"
  }
}
```

```js
import Vue from 'vue'
import { createAppShell } from '@tp-ui/layout-app-shell-vue2'
import '@tp-ui/contracts/base.css'
import '@tp-ui/layout-app-shell-vue2/layout.css'

Vue.use(createAppShell())
```

## 公开接口

- `UiAppShell`：接收 `model`、`labels`、`showUser`。
- `createAppShell()`：仅注册 `UiAppShell`。
- Slots：`brand`、`icon`、`actions`、`user`、default。
- Events：`navigate(key)`、`logout()`、`action(name)`。

## 组合示例

```vue
<ui-app-shell
  :model="shellModel"
  :labels="{ navigation: '主导航', profile: '个人资料', logout: '退出' }"
  @navigate="$router.push($event)"
  @logout="logout"
>
  <template #icon="{ name }"><app-icon :name="name" /></template>
  <router-view />
</ui-app-shell>
```

应用样式只能通过 `data-ui-component="app-shell"`、slots 和公开变量扩展，不能选择 `.tp-ui-shell__*` 内部类。

## 兼容矩阵

| 环境 | 支持情况 |
| --- | --- |
| Vue 2.6 / 2.7 | 支持 |
| Vue 3 | 不支持 |
| Router / Vuex | 无依赖，由应用 Adapter 连接 |
| 桌面导航 | `>1060px` |
| 平板/手机导航 | `<=1060px`，支持横向溢出 |
| Motion full | Token 决定平滑滚动 |
| Motion reduced / none | 自动滚动，无平滑动画 |

## 验证

```bash
npm run check --workspace @tp-ui/layout-app-shell-vue2
```

契约测试验证事件隔离、可访问名称、移动导航 anatomy、Motion Token 和无业务框架导入。
