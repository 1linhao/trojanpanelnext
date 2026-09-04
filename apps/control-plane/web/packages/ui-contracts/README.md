# @tp-ui/contracts

Trojan Panel 可组合 UI 的框架无关契约包。主题、材质、布局、组件、图标和动画包都以这里的枚举、模型及 CSS Custom Property 清单为共同语言。

## 职责

- 定义 Surface、Tone、Size、Density、State、Theme、Palette 与 Motion 枚举。
- 提供抛错式运行时校验器，以及供 Vue 等框架使用的布尔判定器。
- 归一化并冻结 `ShellModel`。
- 提供 Theme、Material、Motion Controller 的组合运行时。
- 维护 `UI_CUSTOM_PROPERTIES` 全量登记表和 `MATERIAL_CUSTOM_PROPERTIES` 材质必实现子集。
- 通过 `base.css` 提供无材质时仍安全可用的回退值。

## 非职责

- 不渲染 Vue 组件或 DOM。
- 不读取 Router、Vuex、用户角色、Token 或业务 API。
- 不包含具体颜色、玻璃模糊或产品布局配方。
- 不替代具体材质与动画 Controller。

## 安装

当前包是仓库内的 private workspace 包。根目录执行 `npm install` 后即可通过固定版本依赖使用：

```json
{
  "dependencies": {
    "@tp-ui/contracts": "0.1.0"
  }
}
```

禁止导入 `@tp-ui/contracts/src/*`；只使用 `package.json#exports` 暴露的入口。

## 公开接口

```js
import {
  MATERIAL_CUSTOM_PROPERTIES,
  PALETTES,
  UI_CUSTOM_PROPERTIES,
  createShellModel,
  createUiRuntime,
  isSize,
  validateSize
} from '@tp-ui/contracts'
import '@tp-ui/contracts/base.css'
```

- `validate*()`：非法值抛出 `TypeError`，用于运行时边界。
- `is*()`：始终返回布尔值，适用于 Vue prop validator。
- `createShellModel()`：返回不可变的应用外壳模型。
- `createUiRuntime()`：组合注入的材质、动画与环境 Controller。
- `MATERIAL_CUSTOM_PROPERTIES`：每个材质包必须逐项实现，不能只依赖其他包的回退。

## 组合示例

```js
import { createUiRuntime } from '@tp-ui/contracts'
import { createFrostedMaterial } from '@tp-ui/material-frosted'
import { createNativeMotion } from '@tp-ui/motion-native'

const runtime = createUiRuntime({
  material: createFrostedMaterial(),
  motion: createNativeMotion(),
  initialTheme: { mode: 'system', palette: 'blue' },
  initialMotion: { mode: 'system' }
})

runtime.theme.setPalette('emerald')
runtime.motion.setMode('reduced')
```

样式顺序应从 `base.css` 开始，再加载组件几何、布局几何、选定材质和动画实现。

## 兼容矩阵

| 环境 | 支持情况 |
| --- | --- |
| Node.js | `^20.19.0` 或 `>=22.12.0`，用于构建和测试 |
| 浏览器 | 现代 Chromium、Firefox、Safari；不要求框架 |
| Vue | 无依赖 |
| CommonJS | 不支持；包使用 ESM |
| 深层导入 | 不支持 |

## 验证

```bash
npm run check --workspace @tp-ui/contracts
```

该命令执行 lint、契约测试、构建、exports 检查和 pack smoke test。
