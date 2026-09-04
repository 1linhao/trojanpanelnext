<template>
  <main class="auth not-found">
    <liquid-theme-toggle class="auth-theme-toggle" />
    <ui-panel variant="auth" motion-role="page" motion-key="not-found">
      <div class="auth-brand">
        <panel-logo />
        <p class="ui-supporting-text">{{ branding.systemName }}</p>
        <h1>404 · 页面不存在</h1>
      </div>
      <p class="ui-supporting-text">请检查页面地址，或返回首页继续使用。</p>
      <button type="button" class="cap primary auth-submit" @click="goHome">返回首页</button>
    </ui-panel>
  </main>
</template>

<script>
import PanelLogo from '@/components/PanelLogo'
import LiquidThemeToggle from '@/components/LiquidThemeToggle'
import { panelBranding, loadPanelSettings } from '@/utils/panel-branding'

export default {
  name: 'NotFoundPage',
  components: { LiquidThemeToggle, PanelLogo },
  data: () => ({ branding: panelBranding }),
  created() { loadPanelSettings().catch(() => {}) },
  methods: {
    goHome() { this.$router.push('/dashboard/index').catch(() => {}) }
  }
}
</script>

<style scoped>
.not-found { text-align: center; }
.not-found .auth-submit { margin-top: 20px; }
</style>
