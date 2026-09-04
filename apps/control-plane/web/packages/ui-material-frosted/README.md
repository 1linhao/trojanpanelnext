# @tp-ui/material-frosted

Trojan Panel 生产磨砂玻璃材质包。它实现 contracts 登记的完整材质变量，通过根主题属性和语义 Surface 改变视觉，不修改组件 DOM。

## 职责

- 实现日间/夜间模式和 blue、violet、emerald、amber 四套调色板。
- 提供 Canvas、Panel、Raised、Overlay、Control、Navigation 配方。
- 拥有颜色、透明度、边缘高光、模糊、阴影、状态色和浏览器 chrome 元数据。
- 提供无 `backdrop-filter` 时的降级。
- 为生产应用和独立 Lab 提供不同但互斥的样式入口。

## 非职责

- 不定义组件结构、业务类、路由、Vuex 或产品文案。
- 不选择 `.tp-ui-*`、`#app` 或业务页面类名。
- 不拥有动画时间线或布局几何。
- 不与 Flat Test 同时加载。

## 安装

```json
{
  "dependencies": {
    "@tp-ui/material-frosted": "0.1.0",
    "@tp-ui/contracts": "0.1.0"
  }
}
```

## 样式入口

- `material.css`：独立 Integration Lab 的精简材质。
- `production.css`：保留当前 Trojan Panel 生产视觉的完整兼容配方。
- `overlay.css`：Dialog/Overlay 语义属性上的材质变量。

## 组合示例

生产组合：

```js
import { createFrostedMaterial } from '@tp-ui/material-frosted'
import '@tp-ui/material-frosted/production.css'
import '@tp-ui/material-frosted/overlay.css'

const material = createFrostedMaterial({
  root: document.documentElement,
  document
})
```

不要同时导入 `material.css` 与 `production.css`，以免两个 profile 竞争同一变量。

## 公开接口

- `createFrostedMaterial({ root?, document? })`。
- `apply(themeState)`：设置 `data-ui-material="frosted"`、`data-theme`、`data-palette`。
- `getCapabilities()`：报告 backdrop filter、颜色模式和调色板能力。
- CSS exports：`material.css`、`production.css`、`overlay.css`。

## 兼容矩阵

| 能力 | 支持情况 |
| --- | --- |
| light / dark | 支持 |
| system 模式 | 由 runtime/environment 解析后支持 |
| 四种调色板 | 支持 |
| `backdrop-filter` | 支持；缺失时有不透明降级 |
| Vue | 无依赖 |
| Flat Test 同页并存 | 不支持，单次只能选择一个材质 |
| 生产旧变量 | `production.css` 保留兼容 |

## 验证

```bash
npm run check --workspace @tp-ui/material-frosted
```

测试会逐项比对 `MATERIAL_CUSTOM_PROPERTIES`，并检查模式、调色板、Surface、降级和 Overlay 隔离。
