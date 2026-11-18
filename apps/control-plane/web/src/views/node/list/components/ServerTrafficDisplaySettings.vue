<template>
  <ui-panel motion-key="server-traffic-display-settings">
    <div class="toolbar traffic-settings">
      <strong>{{ $t('trafficDisplay.sync') }}</strong>
      <liquid-switch
        v-model="enabled"
        :aria-label="$t('trafficDisplay.sync')"
        :active-text="$t('trafficDisplay.on')"
        :inactive-text="$t('trafficDisplay.off')"
        :disabled="!loaded || saving"
      />
      <liquid-select
        v-model="serverIds"
        multiple
        clearable
        filterable
        class="traffic-settings__servers"
        :placeholder="$t('trafficDisplay.selectServers')"
        :aria-label="$t('trafficDisplay.selectServers')"
        :disabled="!loaded || saving || !servers.length"
      >
        <option v-for="server in servers" :key="server.id" :label="server.name" :value="server.id" />
      </liquid-select>
      <liquid-button v-if="!loading && !loaded" size="small" @click="load">
        {{ $t('trafficDisplay.retry') }}
      </liquid-button>
      <liquid-button
        v-else
        size="small"
        type="primary"
        :disabled="!loaded || !dirty"
        :loading="saving"
        @click="save"
      >
        {{ $t('trafficDisplay.save') }}
      </liquid-button>
    </div>
    <p v-if="loading" role="status">{{ $t('trafficDisplay.loading') }}</p>
    <p v-else-if="error" class="bad-text" role="alert">{{ error }}</p>
    <p v-else-if="loaded && !servers.length" class="ui-supporting-text">{{ $t('trafficDisplay.noServers') }}</p>
  </ui-panel>
</template>

<script>
import { selectServerTrafficDisplay, updateServerTrafficDisplay } from '@/api/node'

export default {
  name: 'ServerTrafficDisplaySettings',
  data() {
    return {
      enabled: false,
      serverIds: [],
      servers: [],
      saved: null,
      loaded: false,
      loading: false,
      saving: false,
      error: ''
    }
  },
  computed: {
    dirty() {
      return this.loaded && (
        this.enabled !== this.saved.enabled ||
        this.serverIds.length !== this.saved.serverIds.length ||
        this.serverIds.some(id => !this.saved.serverIds.includes(id))
      )
    }
  },
  created() {
    this.load()
  },
  methods: {
    async load() {
      this.loading = true
      this.loaded = false
      this.error = ''
      try {
        const { data } = await selectServerTrafficDisplay()
        this.enabled = data.enabled
        this.serverIds = data.serverIds
        this.saved = { enabled: data.enabled, serverIds: data.serverIds }
        this.servers = data.servers
        this.loaded = true
      } catch {
        this.error = this.$t('trafficDisplay.loadFailed')
      } finally {
        this.loading = false
      }
    },
    async save() {
      if (!this.dirty || this.saving) return
      const config = { enabled: this.enabled, serverIds: this.serverIds }
      this.saving = true
      this.error = ''
      try {
        await updateServerTrafficDisplay(config)
        this.saved = config
        this.$emit('saved')
        this.$message({ type: 'success', message: this.$t('trafficDisplay.saved') })
      } catch (error) {
        if (error.message === 'a selected server does not exist or is being removed') {
          await this.load()
          if (!this.error) this.error = this.$t('trafficDisplay.selectionChanged')
        } else {
          this.error = this.$t('trafficDisplay.saveFailed')
        }
      } finally {
        this.saving = false
      }
    }
  }
}
</script>

<style scoped>
.traffic-settings__servers {
  width: min(100%, 260px);
}
.traffic-settings + p {
  margin: 10px 0 0;
}
</style>
