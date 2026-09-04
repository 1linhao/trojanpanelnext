# @tp-ui/motion-native

默认的原生动画资源包。它实现 Motion Controller、语义时序 Token 和少量框架无关工具，并为后续 WAAPI、View Transitions 或第三方动画模块保留同级替换接口。

## 职责

- 实现 `createNativeMotion()` 和系统 reduced-motion 环境适配器。
- 在根节点发布 `data-ui-motion="full|reduced|none"`。
- 提供时长、easing、位移和滚动行为语义变量。
- 在 reduced/none 下关闭位移、动画和平滑滚动。
- 提供 `afterTransition()` 和可取消的 `scrollElementTo()`。

## 非职责

- 不拥有颜色、背景、边框、阴影或材质状态。
- 不导入组件、布局、Vue、Router 或业务模块。
- 不永久启用 View Transition capture。
- 不要求组件或布局直接导入本包实现；它们只消费契约 Token。

## 安装

```json
{
  "dependencies": {
    "@tp-ui/motion-native": "0.1.0",
    "@tp-ui/contracts": "0.1.0"
  }
}
```

```js
import { createNativeMotion, createMotionEnvironment } from '@tp-ui/motion-native'
import '@tp-ui/motion-native/motion.css'
```

## 公开接口

- `createNativeMotion({ root? })`：默认 Motion Controller。
- `createMotionEnvironment(scope?)`：读取并订阅 `prefers-reduced-motion`。
- `afterTransition(element, callback)`：按实际计算样式等待，返回取消函数。
- `scrollElementTo(element, top, options?)`：消费 Motion Token，可取消并在 reduced/none 下立即完成。
- `motion.css`：定义语义时序和 `--ui-motion-scroll-behavior`。

## 组合示例

```js
const runtime = createUiRuntime({
  material: createFrostedMaterial(),
  motion: createNativeMotion({ root: document.documentElement }),
  environment: {
    ...createMotionEnvironment(window),
    getSystemMode: () => 'light'
  },
  initialMotion: { mode: 'system' }
})

runtime.motion.setMode('none')
```

布局和组件不得硬编码 `smooth` 或具体毫秒值；应消费此包写入的公开变量。

## 兼容矩阵

| 能力 | 支持情况 |
| --- | --- |
| system | 支持，跟随系统 reduced-motion |
| full | 支持，显式覆盖系统偏好 |
| reduced | 支持，1ms/无位移/无平滑滚动 |
| none | 支持，立即完成并关闭过渡 |
| Web Animations API | 当前实现不依赖，可由后续 Controller 替换 |
| View Transitions | 仅报告能力，不永久启用 capture |
| Vue | 无依赖 |

## 验证

```bash
npm run check --workspace @tp-ui/motion-native
```

测试覆盖模式发布、系统偏好回退、滚动取消、reduced/none 即时完成和计算样式时序。
