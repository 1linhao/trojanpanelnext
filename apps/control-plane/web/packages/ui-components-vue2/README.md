# @tp-ui/components-vue2

Vue 2 无皮肤组件包，负责控件行为、DOM anatomy、可访问性、Overlay 生命周期和稳定几何；所有视觉值均通过 `@tp-ui/contracts` 的语义变量获取。

## 职责

- 导出 `UiButton`、`UiInput`、`UiPanel`、`UiSheet`、`UiDialog`。
- 提供选择性插件 `createVue2Components({ include })`，默认不注册任何组件。
- 管理 Dialog Portal、Overlay Stack、Escape、Tab 焦点约束、滚动锁和焦点返回。
- 提供统一按钮交互 Controller 与可替换动画 Adapter 接口。
- 暴露稳定的 `data-ui-component`、`data-ui-part`、`data-ui-surface` 和 motion 属性。

## 非职责

- 不拥有颜色、模糊、透明度、边框或阴影配方。
- 不导入图标、材质、布局、Router、Vuex 或业务模块。
- 不全量注册组件，也不提供业务表单、表格和选择器实现。
- 不直接实现页面切换动画引擎。

## 安装

当前包是 private workspace 包：

```json
{
  "dependencies": {
    "@tp-ui/components-vue2": "0.1.0",
    "@tp-ui/contracts": "0.1.0"
  },
  "peerDependencies": {
    "vue": "^2.6.11"
  }
}
```

```js
import Vue from 'vue'
import { createVue2Components } from '@tp-ui/components-vue2'
import '@tp-ui/contracts/base.css'
import '@tp-ui/components-vue2/geometry.css'
import '@tp-ui/components-vue2/button-interactions.css'

Vue.use(createVue2Components({ include: ['UiPanel', 'UiDialog'] }))
```

## 公开接口

- 命名组件导出：按需局部注册。
- `createVue2Components({ include, renderIcon, dialogLabels })`：按白名单全局注册。
- `createButtonInteractionController()`：发现当前和动态加入的交互控件。
- `createCssButtonInteractionAdapter()`：默认 `nav-lift` CSS Adapter。
- `BUTTON_INTERACTION`：公开交互属性和值。

Panel 通过 `variant="auth|content|metric"` 选择稳定结构。Panel、Sheet、Dialog 分别暴露 `data-ui-component="panel|sheet|dialog"`；调用方定制时只能使用公开属性、parts、props、slots 与 CSS variables，不能选择 `.tp-ui-*` 内部类。

## 组合示例

```js
import { createVue2Components } from '@tp-ui/components-vue2'
import { renderIcon } from '@tp-ui/icons'

Vue.use(
  createVue2Components({
    include: ['UiButton', 'UiPanel', 'UiSheet', 'UiDialog'],
    renderIcon,
    dialogLabels: { close: '关闭对话框' }
  })
)
```

```vue
<ui-panel variant="content" motion-role="panel" motion-key="settings">
  <template #header>设置</template>
  页面内容
</ui-panel>
```

## 兼容矩阵

| 环境 | 支持情况 |
| --- | --- |
| Vue 2.6 | 支持 |
| Vue 2.7 | 支持，生产使用版本 |
| Vue 3 | 不支持 |
| SSR | 未承诺；Dialog 默认需要浏览器 DOM |
| 无材质运行 | 支持，使用 contracts fallback |
| 图标包 | 可选，由组合入口注入 |

## 验证

```bash
npm run check --workspace @tp-ui/components-vue2
```

测试覆盖选择性注册、布尔 prop validator、Panel/Sheet/Dialog anatomy、Overlay Stack、真实 Chromium 焦点循环和按钮悬浮命中区。
