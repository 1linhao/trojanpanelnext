# @tp-ui/icons

[简体中文](README.md) | English

A semantic icon registry with a consistent 24×24 view box, stroke weight, optical alignment, and `currentColor` inheritance.

## Responsibilities

- Maintains stable semantic icon names.
- Renders inline SVG with Vue 2 `h()`.
- Preserves caller classes, styles, attributes, and event handlers.
- Gives navigation, buttons, and dialogs one icon renderer.

The package does not own button styling, layout, permissions, or business navigation.

## Usage

This private workspace package is built with the Web source and is not installed from the public npm registry. See the [development guide](../../../../../docs/development_EN.md#web-ui) for setup.

```js
import { iconNames, renderIcon } from '@tp-ui/icons'
```

## Verify

```bash
npm run check --workspace @tp-ui/icons
```
