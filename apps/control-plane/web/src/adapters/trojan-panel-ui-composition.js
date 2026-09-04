import { PALETTES, createUiRuntime } from '@tp-ui/contracts'
import { createVue2Components } from '@tp-ui/components-vue2'
import { createAppShell } from '@tp-ui/layout-app-shell-vue2'
import { createFrostedMaterial } from '@tp-ui/material-frosted'
import {
  createNativeMotion,
  createMotionEnvironment
} from '@tp-ui/motion-native'
import { renderIcon } from '@tp-ui/icons'
import { bindUiRuntime } from '@/utils/theme'
import AppIcon from '@/components/AppIcon'
import LiquidInput from '@/components/LiquidInput'
import LiquidButton from '@/components/LiquidButton'
import LiquidLoading from '@/directives/liquid-loading'
import { structuralComponents } from '@/components/LiquidStructural'
import { Message, MessageBox, Notification } from '@/utils/liquid-feedback'
import { installUiInteractions } from '@/adapters/ui-interactions'

const lazyComponents = Object.freeze({
  LiquidNumberInput: () => import('@/components/LiquidNumberInput'),
  LiquidSelect: () => import('@/components/LiquidSelect'),
  LiquidSwitch: () => import('@/components/LiquidSwitch'),
  LiquidTag: () => import('@/components/LiquidTag'),
  LiquidDatePicker: () => import('@/components/LiquidDatePicker'),
  LiquidTabs: () => import('@/components/LiquidTabs')
})

const PALETTE_KEY = 'trojan-panel-color-palette'
const LEGACY_THEME_KEY = 'trojan-panel-color-scheme'
let productionRuntime

function createThemeEnvironment(scope) {
  const media = scope.matchMedia?.('(prefers-color-scheme: dark)')
  return {
    getSystemMode: () => (media?.matches ? 'dark' : 'light'),
    subscribeSystemMode(listener) {
      if (!media) return () => {}
      const handler = () => listener(media.matches ? 'dark' : 'light')
      media.addEventListener?.('change', handler)
      return () => media.removeEventListener?.('change', handler)
    },
    ...createMotionEnvironment(scope)
  }
}

export function installProductionUi(Vue, scope = window) {
  if (productionRuntime) return productionRuntime
  scope.localStorage.removeItem(LEGACY_THEME_KEY)
  const savedPalette = scope.localStorage.getItem(PALETTE_KEY)
  productionRuntime = createUiRuntime({
    material: createFrostedMaterial({
      root: scope.document.documentElement,
      document: scope.document
    }),
    motion: createNativeMotion({ root: scope.document.documentElement }),
    initialTheme: {
      mode: 'system',
      palette: PALETTES.includes(savedPalette) ? savedPalette : 'blue'
    },
    initialMotion: { mode: 'system' },
    environment: createThemeEnvironment(scope)
  })
  bindUiRuntime(productionRuntime)
  Vue.use(
    createVue2Components({
      include: ['UiPanel', 'UiSheet', 'UiDialog'],
      renderIcon,
      dialogLabels: { close: '关闭对话框' }
    })
  )
  Vue.use(createAppShell())
  Object.entries(structuralComponents).forEach(([name, component]) =>
    Vue.component(name, component)
  )
  Vue.component('AppIcon', AppIcon)
  Vue.component('LiquidInput', LiquidInput)
  Vue.component('LiquidButton', LiquidButton)
  Object.entries(lazyComponents).forEach(([name, component]) =>
    Vue.component(name, component)
  )
  Vue.directive('liquid-loading', LiquidLoading)
  Vue.prototype.$message = Message
  Vue.prototype.$confirm = MessageBox.confirm
  Vue.prototype.$prompt = MessageBox.prompt
  Vue.prototype.$notify = Notification
  installUiInteractions({ root: scope.document })
  return productionRuntime
}

export function getProductionUiRuntime() {
  return productionRuntime
}
