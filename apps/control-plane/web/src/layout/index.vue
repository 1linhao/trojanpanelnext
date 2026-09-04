<template>
  <ui-app-shell :model="shellModel" :labels="shellLabels" :show-user="false" @navigate="go" @action="handleShellAction">
    <template #brand>
      <panel-logo />
      <span class="prototype-brand__name"><strong>{{ branding.systemName }}</strong></span>
    </template>
    <template #icon="{ name }"><app-icon :name="name" class="prototype-nav-icon" /></template>
    <template #actions>
      <button v-if="isLocalPreview" class="cap small local-role-preview" type="button"
        :title="isAdmin ? '切换到普通用户演示' : '切换到管理员演示'" @click="switchPreviewRole">
        <app-icon :name="isAdmin ? 'user' : 'setting'" aria-hidden="true" />
        <span>{{ isAdmin ? '普通用户' : '管理员' }}</span>
      </button>
      <liquid-theme-toggle />
      <div class="user-pill prototype-topbar-user">
        <button class="prototype-profile-entry" type="button" title="我的" aria-label="我的个人资料"
          :aria-current="activePath === '/modify/index' ? 'page' : null" @click="go('/modify/index')">
          <span class="avatar" aria-hidden="true">{{ initials }}</span>
          <strong>{{ username || 'Trojan Panel' }}</strong>
        </button>
        <button class="icon-btn" type="button" title="退出登录" aria-label="退出登录" @click="logout">
          <app-icon name="switch-button" aria-hidden="true" />
        </button>
      </div>
    </template>
    <app-main />
  </ui-app-shell>
</template>

<script>
import { mapGetters } from 'vuex'
import AppMain from './components/AppMain'
import PanelLogo from '@/components/PanelLogo'
import { panelBranding, loadPanelSettings } from '@/utils/panel-branding'
import LiquidThemeToggle from '@/components/LiquidThemeToggle'
import { setToken } from '@/utils/auth'
import { PAGE_TITLES, createTrojanPanelShellModel } from '@/adapters/trojan-panel-shell'

export default {
  name: 'TrojanPanelLayout',
  components: { AppMain, PanelLogo, LiquidThemeToggle },
  data: () => ({
    branding: panelBranding,
    shellLabels: { navigation: '移动端功能导航', profile: '我的个人资料', logout: '退出登录' }
  }),
  created() { loadPanelSettings().catch(() => {}) },
  computed: {
    ...mapGetters(['roles', 'username']),
    isAdmin() { return this.roles.some((role) => role === 'sysadmin' || role === 'admin') },
    isLocalPreview() {
      return import.meta.env.DEV && ['127.0.0.1', 'localhost'].includes(window.location.hostname)
    },
    activePath() { return this.$route.path },
    pageTitle() {
      if (!this.isAdmin && this.activePath === '/dashboard/index') return '我的首页'
      if (!this.isAdmin && this.activePath === '/node-manage/node-list') return '我的节点'
      return PAGE_TITLES[this.activePath] || this.$t(`route.${this.$route.meta.title}`)
    },
    initials() { return (this.username || 'TP').slice(0, 2).toUpperCase() },
    shellModel() {
      return createTrojanPanelShellModel({
        roles: this.roles, username: this.username, activePath: this.activePath,
        pageTitle: this.pageTitle, branding: this.branding
      })
    }
  },
  methods: {
    switchPreviewRole() {
      const target = this.isAdmin ? 'user' : 'admin'
      setToken(`Bearer ${target === 'user' ? 'user-token' : 'mock-token'}`)
      window.location.assign(`${window.location.origin}/?previewRole=${target}#/dashboard/index`)
    },
    handleShellAction(action) {
      if (action === 'brand') this.go('/dashboard/index')
      if (action === 'profile') this.go('/modify/index')
    },
    go(path) { if (path !== this.$route.path) this.$router.push(path).catch(() => true) },
    async logout() {
      await this.$store.dispatch('account/logout')
      await this.$router.push('/login')
    }
  }
}
</script>
