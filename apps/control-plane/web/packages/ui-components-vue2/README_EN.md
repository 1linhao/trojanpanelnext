# @tp-ui/components-vue2

[简体中文](README.md) | English

Skinless Vue 2 components that own behavior, DOM anatomy, accessibility, overlay lifecycles, and stable geometry. Visual values come from the semantic properties defined by `@tp-ui/contracts`.

## Responsibilities

- Exports `UiButton`, `UiInput`, `UiPanel`, `UiSheet`, and `UiDialog`.
- Provides selective registration through `createVue2Components({ include })`.
- Manages dialog portals, overlay stacking, Escape handling, focus trapping, scroll locking, and focus return.
- Exposes stable `data-ui-*` attributes and replaceable interaction adapters.

The package does not own colors, materials, icons, routing, Vuex, or business forms.

## Usage

```js
import Vue from 'vue'
import { createVue2Components } from '@tp-ui/components-vue2'
import '@tp-ui/contracts/base.css'
import '@tp-ui/components-vue2/geometry.css'

Vue.use(createVue2Components({ include: ['UiButton', 'UiDialog'] }))
```

## Verify

```bash
npm run check --workspace @tp-ui/components-vue2
```
