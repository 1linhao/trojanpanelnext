<template>
  <div class="app-container version-management">
    <template v-if="authorized">
      <liquid-tabs
        id-prefix="version-management"
        :tabs="tabs"
        :active-value="activeTab"
        :label="$t('route.kernelUpgrade')"
        @change="selectTab"
      />
      <div v-if="activeTab === 'kernel'" id="version-management-panel-kernel" role="tabpanel" aria-labelledby="version-management-tab-kernel">
        <kernel-version-management />
      </div>
      <div v-else id="version-management-panel-container" role="tabpanel" aria-labelledby="version-management-tab-container">
        <node-container-management :initial-server-id="serverId" />
      </div>
    </template>
    <ui-panel v-else>{{ $t('containerManagement.permissionRequired') }}</ui-panel>
  </div>
</template>

<script>
import checkPermission from '@/utils/permission'
import KernelVersionManagement from './KernelVersionManagement.vue'
import NodeContainerManagement from './NodeContainerManagement.vue'

export default {
  name: 'VersionManagement',
  components: { KernelVersionManagement, NodeContainerManagement },
  computed: {
    authorized() { return checkPermission(['sysadmin']) },
    activeTab() { return this.$route.query.tab === 'container' ? 'container' : 'kernel' },
    serverId() {
      const id = Number(this.$route.query.serverId || 0)
      return Number.isSafeInteger(id) && id > 0 ? id : 0
    },
    tabs() {
      return [
        { value: 'kernel', label: this.$t('containerManagement.kernelTab') },
        { value: 'container', label: this.$t('containerManagement.containerTab') }
      ]
    }
  },
  methods: {
    selectTab(tab) {
      if (!this.authorized || !['kernel', 'container'].includes(tab)) return
      this.$router.replace({ path: this.$route.path, query: { ...this.$route.query, tab } }).catch(() => {})
    }
  }
}
</script>

<style scoped>
.version-management > .liquid-tabs { margin-bottom: 20px; }
.version-management ::v-deep #version-management-panel-kernel > .app-container { padding: 0; }
</style>
