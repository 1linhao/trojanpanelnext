/* eslint-env browser */
/* global Vue */

import { createUiRuntime } from '@tp-ui/contracts'
import { createVue2Components } from '@tp-ui/components-vue2'
import { createFrostedMaterial } from '@tp-ui/material-frosted'
import { createNativeMotion } from '@tp-ui/motion-native'
createUiRuntime({
  material: createFrostedMaterial(),
  motion: createNativeMotion(),
  initialTheme: { mode: 'light', palette: 'blue' }
})
Vue.use(createVue2Components({ include: ['UiButton', 'UiInput'] }))
new Vue({
  el: '#app',
  data: { value: 'Minimal host layout' },
  template: `<main class="minimal-host" data-ui-surface="canvas"><section class="lab-card" data-ui-surface="panel"><p class="lab-kicker">MINIMAL LAB</p><h1>无 AppShell 的第二布局消费者</h1><p>同一套组件与 Frosted 材质由普通宿主布局组合。</p><ui-input v-model="value" /><div class="lab-row"><ui-button tone="accent">{{ value }}</ui-button></div></section></main>`
})
