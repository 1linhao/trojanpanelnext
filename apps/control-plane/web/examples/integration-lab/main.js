/* eslint-env browser */
/* global Vue */

import { createShellModel, createUiRuntime } from '@tp-ui/contracts'
import { createVue2Components } from '@tp-ui/components-vue2'
import { createAppShell } from '@tp-ui/layout-app-shell-vue2'
import { createFrostedMaterial } from '@tp-ui/material-frosted'
import { createFlatTestMaterial } from '@tp-ui/material-flat-test'
import {
  createMotionEnvironment,
  createNativeMotion
} from '@tp-ui/motion-native'
import { renderIcon } from '@tp-ui/icons'

const materials = {
  frosted: createFrostedMaterial(),
  flat: createFlatTestMaterial()
}
let selectedMaterial = 'frosted'
const materialSelector = {
  apply: (state) => materials[selectedMaterial].apply(state),
  getCapabilities: () => materials[selectedMaterial].getCapabilities()
}
const media = window.matchMedia('(prefers-color-scheme: dark)')
const runtime = createUiRuntime({
  material: materialSelector,
  motion: createNativeMotion(),
  initialTheme: { mode: 'system', palette: 'blue' },
  initialMotion: { mode: 'system' },
  environment: {
    getSystemMode: () => (media.matches ? 'dark' : 'light'),
    subscribeSystemMode(listener) {
      const handler = () => listener(media.matches ? 'dark' : 'light')
      media.addEventListener('change', handler)
      return () => media.removeEventListener('change', handler)
    },
    ...createMotionEnvironment(window)
  }
})

Vue.use(createVue2Components({ include: ['UiButton', 'UiInput'] }))
Vue.use(createAppShell())
Vue.component('LabIcon', {
  functional: true,
  props: { name: String },
  render: (h, context) =>
    renderIcon(h, context.props.name, { width: 20, height: 20 })
})

new Vue({
  el: '#app',
  data: {
    material: selectedMaterial,
    motion: runtime.motion.getState().resolvedMode,
    mode: runtime.theme.getState().resolvedMode,
    name: 'Composable UI',
    activeKey: 'dashboard'
  },
  computed: {
    model() {
      return createShellModel({
        brand: { mark: 'T', name: 'Trojan Panel', subtitle: 'COMPOSITION LAB' },
        activeKey: this.activeKey,
        pageTitle: this.activeKey === 'dashboard' ? '组合实验室' : '资源库契约',
        user: { name: 'lab', label: '退出意图' },
        groups: [
          {
            key: 'main',
            label: '验证',
            items: [
              {
                key: 'dashboard',
                label: '组合矩阵',
                mobileLabel: '矩阵',
                icon: 'home'
              },
              {
                key: 'contracts',
                label: '公开契约',
                mobileLabel: '契约',
                icon: 'nodes'
              }
            ]
          }
        ]
      })
    }
  },
  methods: {
    selectMaterial(name) {
      selectedMaterial = name
      this.material = name
      materialSelector.apply(runtime.theme.getState())
    },
    toggleMode() {
      const next =
        runtime.theme.getState().resolvedMode === 'dark' ? 'light' : 'dark'
      this.mode = runtime.theme.setMode(next).resolvedMode
    },
    toggleMotion() {
      const next =
        runtime.motion.getState().resolvedMode === 'none' ? 'full' : 'none'
      this.motion = runtime.motion.setMode(next).resolvedMode
    }
  },
  template: `<ui-app-shell :model="model" @navigate="activeKey = $event"><template slot="icon" slot-scope="slot"><lab-icon v-if="slot.name" :name="slot.name" /></template><template slot="actions"><ui-button size="sm" @click="selectMaterial(material === 'frosted' ? 'flat' : 'frosted')">材质：{{ material }}</ui-button><ui-button size="sm" @click="toggleMode">模式：{{ mode }}</ui-button><ui-button size="sm" @click="toggleMotion">动画：{{ motion }}</ui-button></template><section class="lab-card" data-ui-surface="panel" data-ui-tone="neutral"><p class="lab-kicker">INTEGRATION LAB A / B</p><h2>组件、布局、材质和动画通过契约组合</h2><p>切换按钮只修改 composition root；UiInput、UiButton 和 UiAppShell 的源码保持不变。</p><label>示例输入<ui-input v-model="name" aria-label="示例输入" /></label><div class="lab-row"><ui-button tone="accent">{{ name || '确认' }}</ui-button><ui-button :disabled="true">禁用状态</ui-button></div></section></ui-app-shell>`
})
