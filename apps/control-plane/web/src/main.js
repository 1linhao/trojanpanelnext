import Vue from 'vue'
import 'normalize.css/normalize.css'
import '@tp-ui/contracts/base.css'
import '@tp-ui/motion-native/motion.css'
import '@tp-ui/components-vue2/geometry.css'
import '@tp-ui/layout-app-shell-vue2/layout.css'
import '@tp-ui/components-vue2/button-interactions.css'
import '@tp-ui/material-frosted/production.css'
import '@tp-ui/material-frosted/overlay.css'
import '@/styles/index.scss'
import App from './App'
import store from '@/store'
import router from '@/router'
import '@/permission'
import i18n from '@/lang'
import { installProductionUi } from '@/adapters/trojan-panel-ui-composition'

installProductionUi(Vue)

Vue.config.productionTip = false

new Vue({
  el: '#app',
  router,
  store,
  i18n,
  render: (h) => h(App)
})
