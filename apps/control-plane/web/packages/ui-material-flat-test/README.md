# @tp-ui/material-flat-test

用于证明材质可替换性的高对比纯色材质。它不是生产主题，但必须与 Frosted 一样完整实现公开材质契约。

## 职责

- 实现 `MATERIAL_CUSTOM_PROPERTIES` 的每一个变量。
- 支持 light、dark 和四种调色板。
- 为组件、Dialog、Overlay 与 AppShell 提供无模糊的高对比配方。
- 在 Integration Lab 中证明切换材质不需要修改组件或布局源码。

## 非职责

- 不追求生产磨砂玻璃视觉。
- 不定义业务页面样式、组件 anatomy 或布局。
- 不依赖 Vue，也不包含动画实现。
- 不允许用 contracts 或 Frosted 的定义掩盖本包缺失变量。

## 安装

```json
{
  "dependencies": {
    "@tp-ui/material-flat-test": "0.1.0",
    "@tp-ui/contracts": "0.1.0"
  }
}
```

```js
import { createFlatTestMaterial } from '@tp-ui/material-flat-test'
import '@tp-ui/material-flat-test/material.css'

const material = createFlatTestMaterial({ root: document.documentElement })
```

## 公开接口

- `createFlatTestMaterial({ root? })`。
- `apply(themeState)`：设置材质、模式和调色板根属性。
- `getCapabilities()`：报告无 backdrop filter、支持颜色模式和四套调色板。
- `material.css`：完整纯色材质配方。

## 组合示例

```js
const runtime = createUiRuntime({
  material: createFlatTestMaterial(),
  motion: createNativeMotion(),
  initialTheme: { mode: 'light', palette: 'violet' }
})
```

组件与 AppShell 的引入方式和 Frosted 组合完全相同；替换时只改变材质 Controller 与 CSS 入口。

## 兼容矩阵

| 能力 | 支持情况 |
| --- | --- |
| light / dark | 支持 |
| blue / violet / emerald / amber | 支持 |
| Dialog / Overlay | 支持，无模糊 |
| Navigation | 支持，高对比纯色 |
| `backdrop-filter` | 明确不使用 |
| Vue | 无依赖 |
| 生产使用 | 不推荐，仅用于契约和组合验证 |

## 验证

```bash
npm run check --workspace @tp-ui/material-flat-test
npm run test:ui-labs:e2e
```

包契约测试和全仓架构门禁都会逐项检查本包是否独立定义完整材质变量。
