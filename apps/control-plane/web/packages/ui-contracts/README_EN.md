# @tp-ui/contracts

[简体中文](README.md) | English

Framework-independent contracts for the composable TrojanPanel Next UI. Theme, material, layout, component, icon, and motion packages use these enums, models, and CSS custom properties as their shared language.

## Responsibilities

- Defines surface, tone, size, density, state, theme, palette, and motion values.
- Validates input and normalizes immutable shell models.
- Composes theme, material, and motion controllers.
- Registers every public UI custom property and safe base fallback.

The package does not render DOM, read application state, or define a concrete visual material.

## Usage

This private workspace package is built with the Web source and is not installed from the public npm registry. See the [development guide](../../../../../docs/development_EN.md#web-ui) for setup.

```js
import { createUiRuntime } from '@tp-ui/contracts'
import '@tp-ui/contracts/base.css'
```

Only import entries exposed by `package.json#exports`; internal `src/*` paths are not public.

## Verify

```bash
npm run check --workspace @tp-ui/contracts
```
