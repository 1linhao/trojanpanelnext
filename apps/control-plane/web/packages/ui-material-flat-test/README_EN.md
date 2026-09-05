# @tp-ui/material-flat-test

[简体中文](README.md) | English

A high-contrast flat material used to prove that the public material contract is replaceable. It is a test material, not a production theme.

## Responsibilities

- Implements every `MATERIAL_CUSTOM_PROPERTIES` variable.
- Supports light, dark, and all four palettes.
- Provides non-blurred surfaces for components, dialogs, overlays, and AppShell.
- Demonstrates material replacement without changing component or layout source.

## Usage

```js
import { createFlatTestMaterial } from '@tp-ui/material-flat-test'
import '@tp-ui/material-flat-test/material.css'

const material = createFlatTestMaterial({ root: document.documentElement })
```

## Verify

```bash
npm run check --workspace @tp-ui/material-flat-test
npm run test:ui-labs:e2e
```
