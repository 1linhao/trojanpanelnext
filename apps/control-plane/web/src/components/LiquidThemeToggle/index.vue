<template>
  <button
    type="button"
    class="liquid-theme-toggle"
    :aria-label="theme === 'dark' ? '切换浅色模式' : '切换深色模式'"
    :title="theme === 'dark' ? '切换浅色模式' : '切换深色模式'"
    @click="handleToggle"
  >
    <app-icon :name="theme === 'light' ? 'sun' : 'moon'" />
  </button>
</template>

<script>
import { getInitialTheme, toggleTheme } from '@/utils/theme'

export default {
  name: 'LiquidThemeToggle',
  data() {
    return {
      theme: getInitialTheme()
    }
  },
  mounted() {
    window.addEventListener('trojan-theme-change', this.handleThemeChange)
  },
  beforeDestroy() {
    window.removeEventListener('trojan-theme-change', this.handleThemeChange)
  },
  methods: {
    handleToggle() {
      this.theme = toggleTheme().mode
    },
    handleThemeChange(event) {
      this.theme = event.detail.mode
    }
  }
}
</script>
