# @tp-ui/motion-native

[简体中文](README.md) | English

The default native motion package. It provides a motion controller, semantic timing tokens, and framework-independent helpers while preserving a replacement seam for future motion engines.

## Responsibilities

- Implements `createNativeMotion()` and reduced-motion environment handling.
- Publishes `data-ui-motion="full|reduced|none"` on the root element.
- Defines semantic duration, easing, displacement, and scrolling values.
- Disables displacement, animation, and smooth scrolling when motion is reduced or disabled.
- Exposes cancellable transition and scrolling helpers.

## Usage

```js
import { createNativeMotion, createMotionEnvironment } from '@tp-ui/motion-native'
import '@tp-ui/motion-native/motion.css'
```

## Verify

```bash
npm run check --workspace @tp-ui/motion-native
```
