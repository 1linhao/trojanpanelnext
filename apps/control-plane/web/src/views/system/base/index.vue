<template>
  <div class="prototype-page">
    <ui-panel class="prototype-config-card" motion-key="system-config">
      <div class="card-head">
        <div>
          <span class="kicker">System Preferences</span>
          <h2>系统配置</h2>
        </div>
      </div>
      <liquid-tabs
        id-prefix="system-settings"
        :tabs="tabs"
        v-model="activeName"
        label="系统配置"
      />
      <div :id="`system-settings-panel-${activeName}`" role="tabpanel" :aria-labelledby="`system-settings-tab-${activeName}`">
        <account v-if="activeName === 'account-config'" />
        <email v-else-if="activeName === 'config-email'" />
        <web-file v-else-if="activeName === 'config-web-file'" />
        <panel-config v-else-if="activeName === 'config-panel'" />
        <client-routing v-else-if="activeName === 'config-routing'" />
        <template-config v-else-if="loaded" />
      </div>
    </ui-panel>
  </div>
</template>

<script>
import Account from './components/account'
import WebFile from './components/web-file'
import Email from './components/email'
import PanelConfig from './components/panel-config'
import ClientRouting from './components/client-routing'
import TemplateConfig from './components/template-config'
import { selectSystemByName } from '@/api/system'

export default {
  name: 'SystemSettingsPage',
  components: { Account, WebFile, Email, PanelConfig, TemplateConfig, ClientRouting },
  provide() {
    return {
      systemConfigModel: this.systemConfig
    }
  },
  data() {
    return {
      activeName: 'account-config',
      loaded: false,
      tabs: [
        { value: 'account-config', label: '账号' },
        { value: 'config-email', label: '邮件' },
        { value: 'config-web-file', label: 'Web 文件' },
        { value: 'config-panel', label: '面板设置' },
        { value: 'config-template-config', label: '订阅模板' },
        { value: 'config-routing', label: '节点组分流' }
      ],
      systemConfig: {
        emailEnable: 0,
        emailHost: undefined,
        emailPassword: undefined,
        emailPort: 0,
        emailUsername: undefined,
        expireWarnDay: 0,
        expireWarnEnable: 0,
        id: 1,
        registerEnable: 1,
        registerExpireDays: 0,
        registerQuota: 0,
        resetDownloadAndUploadMonth: 0,
        trafficRankEnable: 1,
        captchaEnable: 0,
        systemName: '',
        clashTemplateName: 'Default',
        clashRule: '',
        singBoxTunTemplateName: 'TUN',
        singBoxTun: '',
        singBoxTunEntity: {},
        singBoxOutboundTemplateName: 'Outbound only',
        singBoxOutbound: '',
        singBoxOutboundEntity: {},
        xrayTemplateName: 'Default',
        xrayTemplate: '',
        xrayTemplateEntity: {},
        templateLibrary: null
      }
    }
  },
  created() {
    this.selectDate()
  },
  methods: {
    selectDate() {
      selectSystemByName().then((response) => {
        Object.assign(this.systemConfig, response.data)
        this.systemConfig.singBoxTunEntity = JSON.parse(
          this.systemConfig.singBoxTun
        )
        this.systemConfig.singBoxOutboundEntity = JSON.parse(
          this.systemConfig.singBoxOutbound
        )
        this.systemConfig.xrayTemplateEntity = JSON.parse(
          this.systemConfig.xrayTemplate
        )
        this.loaded = true
      })
    }
  }
}
</script>

<style scoped></style>
