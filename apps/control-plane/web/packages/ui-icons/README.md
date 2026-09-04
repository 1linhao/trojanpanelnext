# @tp-ui/icons

框架轻耦合的语义图标注册表。所有图标使用统一 viewBox、线宽和光学对齐，默认继承 `currentColor`。

## 职责

- 维护稳定的语义图标名称。
- 以 Vue 2 `h()` renderer 输出内联 SVG。
- 统一 24×24 viewBox、圆角端点/连接和描边宽度。
- 保留调用方传入的 class、style、attrs 与事件绑定。
- 让导航、按钮和 Dialog 通过同一 renderer 使用图标。

## 非职责

- 不决定按钮背景、尺寸、阴影或交互动画。
- 不包含位图、Sprite、遮罩图标或独立矩形底色。
- 不依赖组件、布局、材质、Router 或业务状态。
- 不把图标名称映射成业务权限。

## 安装

```json
{
  "dependencies": {
    "@tp-ui/icons": "0.1.0"
  }
}
```

## 公开接口

```js
import { iconNames, renderIcon } from '@tp-ui/icons'
```

- `iconNames`：所有受支持的稳定语义名称。
- `renderIcon(h, name, attrs?, vnodeData?)`：返回内联 SVG VNode。

## 组合示例

```js
Vue.use(
  createVue2Components({
    include: ['UiDialog'],
    renderIcon
  })
)
```

```js
export default {
  functional: true,
  props: { name: String },
  render(h, context) {
    return renderIcon(h, context.props.name, {}, context.data)
  }
}
```

未知名称应在开发阶段通过注册表或测试发现，不应由业务页面临时嵌入另一套 SVG。

## 兼容矩阵

| 环境 | 支持情况 |
| --- | --- |
| Vue 2 render function | 支持 |
| Vue 3 VNode API | 未提供适配器 |
| 浏览器颜色主题 | 支持，继承 `currentColor` |
| SSR | 仅依赖传入的 `h()`，可由宿主验证 |
| 独立 CSS | 不需要 |
| 旧 Sprite / mask | 不支持 |

## 验证

```bash
npm run check --workspace @tp-ui/icons
```

测试检查语义名称唯一性、SVG anatomy、`currentColor`、统一描边和 Vue binding 透传。
