# @tp-ui/layout-app-shell-vue2

[简体中文](README.md) | English

A Vue 2 application shell responsible for the header, desktop navigation, mobile navigation, content viewport, and responsive geometry.

## Responsibilities

- Renders a normalized `ShellModel`.
- Switches between desktop and mobile navigation.
- Keeps the active mobile navigation item visible.
- Exposes brand, icon, profile, and action slots.
- Emits `navigate`, `logout`, and `action` intents.

The shell does not read Router, Vuex, tokens, roles, or business APIs and does not define material colors.

## Usage

```js
import Vue from 'vue'
import { createAppShell } from '@tp-ui/layout-app-shell-vue2'
import '@tp-ui/contracts/base.css'
import '@tp-ui/layout-app-shell-vue2/layout.css'

Vue.use(createAppShell())
```

## Verify

```bash
npm run check --workspace @tp-ui/layout-app-shell-vue2
```
