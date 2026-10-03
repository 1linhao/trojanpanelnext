<template>
  <div>
    <ui-panel v-if="authorized" class="section" motion-key="node-container-inventory" v-liquid-loading="loading || serversLoading">
      <div class="section-head">
        {{ $t('containerManagement.inventory') }}
        <liquid-button size="sm" :disabled="loading || serversLoading || submitting" @click="refresh">
          {{ $t('containerManagement.refresh') }}
        </liquid-button>
      </div>
      <liquid-form label-width="150px">
        <liquid-form-item :label="$t('containerManagement.selectServer')">
          <liquid-select v-model="selectedServerId" filterable :disabled="submitting" @change="selectServer">
            <option v-for="server in servers" :key="server.id" :value="server.id" :label="`#${server.id} · ${server.name}`" />
          </liquid-select>
        </liquid-form-item>
      </liquid-form>
      <div v-if="loadError || serverError" role="alert" class="container-error">{{ loadError || serverError }}</div>
      <p v-if="!serversLoading && !serverError && !servers.length" class="node-grid__empty">{{ $t('containerManagement.empty') }}</p>
      <liquid-descriptions v-if="inventory" :column="2" border>
        <liquid-descriptions-item :label="$t('containerManagement.currentVersion')">{{ inventory.currentVersion || '—' }}</liquid-descriptions-item>
        <liquid-descriptions-item :label="$t('containerManagement.targetVersion')">{{ inventory.targetVersion }}</liquid-descriptions-item>
        <liquid-descriptions-item :label="$t('containerManagement.image')" :span="2"><span class="image-name">{{ inventory.image || '—' }}</span></liquid-descriptions-item>
      </liquid-descriptions>
      <liquid-button v-if="selectedServerId" type="primary" class="submit" :disabled="!canUpdate" :loading="submitting" @click="submitUpdate">
        {{ $t('containerManagement.update') }}
      </liquid-button>
    </ui-panel>
    <ui-panel v-if="authorized && inventory" class="section" motion-key="node-container-job">
      <div class="section-head">
        {{ $t('containerManagement.recentJob') }}
        <liquid-tag v-if="job" :type="statusType(job.status)">{{ statusLabel(job.status) }}</liquid-tag>
      </div>
      <liquid-descriptions v-if="job" :column="2" border>
        <liquid-descriptions-item label="ID">{{ job.id }}</liquid-descriptions-item>
        <liquid-descriptions-item :label="$t('table.status')">{{ statusLabel(job.status) }}</liquid-descriptions-item>
        <liquid-descriptions-item :label="$t('containerManagement.fromVersion')">{{ job.fromVersion || '—' }}</liquid-descriptions-item>
        <liquid-descriptions-item :label="$t('kernel.targetVersion')">{{ job.targetVersion }}</liquid-descriptions-item>
        <liquid-descriptions-item :label="$t('containerManagement.startedAt')">{{ formatTime(job.startedAt) }}</liquid-descriptions-item>
        <liquid-descriptions-item :label="$t('containerManagement.finishedAt')">{{ formatTime(job.finishedAt) }}</liquid-descriptions-item>
        <liquid-descriptions-item :label="$t('kernel.error')" :span="2">{{ job.error || '—' }}</liquid-descriptions-item>
      </liquid-descriptions>
      <p v-else class="node-grid__empty">{{ $t('containerManagement.noJob') }}</p>
    </ui-panel>
    <ui-panel v-if="!authorized">{{ $t('containerManagement.permissionRequired') }}</ui-panel>
  </div>
</template>

<script>
import { selectNodeServerList } from '@/api/node-server'
import { containerInventory, updateNodeContainer } from '@/api/container-management'
import { MessageBox } from '@/utils/liquid-feedback'
import checkPermission from '@/utils/permission'
import { timeStampToDate } from '@/utils'

const maxPollFailures = 6
const activeStatuses = ['queued', 'running']

export default {
  name: 'NodeContainerManagement',
  props: { initialServerId: { type: Number, default: 0 } },
  data() {
    return {
      servers: [], selectedServerId: 0, inventory: null,
      serversLoading: false, loading: false, submitting: false,
      loadError: '', serverError: '', timer: null, requestSequence: 0, submissionSequence: 0, pollFailures: 0, inactive: false, disposed: false
    }
  },
  computed: {
    authorized() { return checkPermission(['sysadmin']) },
    currentServer() { return this.servers.find((server) => server.id === this.selectedServerId) },
    job() { return this.inventory && this.inventory.job },
    canUpdate() {
      return Boolean(!this.disposed && !this.inactive && this.authorized && this.currentServer && this.inventory &&
        this.inventory.updateSupported === true && this.inventory.targetVersion &&
        !this.loading && !this.submitting && !this.loadError &&
        !(this.job && activeStatuses.includes(this.job.status)))
    }
  },
  watch: {
    initialServerId(id) { if (this.servers.length) this.selectServer(id) },
    authorized(value) {
      this.clearState()
      if (value) this.loadServers()
    }
  },
  created() { this.loadServers() },
  activated() {
    if (!this.inactive) return
    this.inactive = false
    this.refresh()
  },
  deactivated() {
    this.inactive = true
    this.requestSequence++
    this.submissionSequence++
    this.submitting = false
    this.loading = false
    this.serversLoading = false
    this.stopPolling()
  },
  beforeDestroy() {
    this.disposed = true
    this.requestSequence++
    this.stopPolling()
  },
  methods: {
    statusLabel(status) { return this.$t(`containerManagement.statuses.${status}`) },
    statusType(status) { return { queued: 'info', running: 'warning', succeeded: 'success', failed: 'danger' }[status] || 'info' },
    formatTime(value) {
      const timestamp = value ? Date.parse(value) : NaN
      return Number.isFinite(timestamp) ? timeStampToDate(timestamp, true) : '—'
    },
    stopPolling() {
      clearTimeout(this.timer)
      this.timer = null
    },
    clearState() {
      this.requestSequence++
      this.submissionSequence++
      this.stopPolling()
      this.servers = []
      this.selectedServerId = 0
      this.inventory = null
      this.loading = false
      this.serversLoading = false
      this.submitting = false
      this.loadError = ''
      this.serverError = ''
      this.pollFailures = 0
    },
    currentRequest(sequence, id) {
      return !this.disposed && !this.inactive && this.authorized && sequence === this.requestSequence && id === this.selectedServerId
    },
    async loadServers() {
      if (this.disposed || this.inactive || !this.authorized) return
      const sequence = ++this.requestSequence
      this.serversLoading = true
      this.serverError = ''
      try {
        const response = await selectNodeServerList()
        if (this.disposed || !this.authorized || sequence !== this.requestSequence) return
        this.servers = (response.data || []).filter((server) => Number.isSafeInteger(server.id) && server.id > 0)
        const id = this.initialServerId || (this.servers[0] && this.servers[0].id) || 0
        this.serversLoading = false
        if (id) return this.selectServer(id)
      } catch (error) {
        if (!this.disposed && this.authorized && sequence === this.requestSequence) this.serverError = this.$t('containerManagement.serverLoadFailed', { error: error.message })
      } finally {
        if (!this.disposed && sequence === this.requestSequence) this.serversLoading = false
      }
    },
    selectServer(value) {
      this.requestSequence++
      this.submissionSequence++
      this.submitting = false
      this.pollFailures = 0
      this.stopPolling()
      this.inventory = null
      this.loadError = ''
      this.loading = false
      const id = Number(value)
      this.selectedServerId = this.servers.some((server) => server.id === id) ? id : 0
      if (!this.selectedServerId) {
        if (id) this.loadError = this.$t('containerManagement.invalidInventory')
        return
      }
      return this.loadInventory()
    },
    refresh() {
      if (this.loading || this.serversLoading || this.submitting) return
      this.pollFailures = 0
      return this.servers.length ? this.loadInventory() : this.loadServers()
    },
    schedulePoll(sequence, id, delay = 2000) {
      if (!this.currentRequest(sequence, id) || !this.job || !activeStatuses.includes(this.job.status)) return
      this.stopPolling()
      this.timer = setTimeout(() => {
        this.timer = null
        if (this.currentRequest(sequence, id)) this.loadInventory()
      }, delay)
    },
    async loadInventory() {
      const id = this.selectedServerId
      if (this.disposed || this.inactive || !this.authorized || !this.currentServer) return
      this.stopPolling()
      const sequence = ++this.requestSequence
      this.loading = true
      this.loadError = ''
      const previousJob = this.job
      try {
        const response = await containerInventory(id)
        if (!this.currentRequest(sequence, id)) return
        const inventory = response.data
        if (!inventory || inventory.nodeId !== id || !inventory.targetVersion ||
          (inventory.job && !['queued', 'running', 'succeeded', 'failed'].includes(inventory.job.status))) {
          throw new Error(this.$t('containerManagement.invalidInventory'))
        }
        this.inventory = inventory
        this.pollFailures = 0
        if (inventory.updateSupported !== true) {
          this.loadError = this.$t('containerManagement.unsupported')
          return
        }
        if (previousJob && this.job && previousJob.id === this.job.id && activeStatuses.includes(previousJob.status) && this.job.status === 'succeeded') {
          this.$notify({ title: this.$t('containerManagement.updateSuccess'), message: this.$t('containerManagement.updateSuccess'), type: 'success', duration: 3000 })
        }
        this.schedulePoll(sequence, id)
      } catch (error) {
        if (this.currentRequest(sequence, id)) {
          if (this.job && activeStatuses.includes(this.job.status)) {
            this.pollFailures++
            const retrying = this.pollFailures <= maxPollFailures
            this.loadError = this.$t(retrying ? 'containerManagement.pollInterrupted' : 'containerManagement.pollStopped', { error: error.message })
            if (retrying) this.schedulePoll(sequence, id, Math.min(2000 * Math.pow(2, this.pollFailures), 15000))
          } else {
            this.loadError = this.$t('containerManagement.unavailable', { error: error.message })
          }
        }
      } finally {
        if (this.currentRequest(sequence, id)) this.loading = false
      }
    },
    async submitUpdate() {
      if (!this.canUpdate) return
      const id = this.selectedServerId
      const sequence = this.requestSequence
      const targetVersion = this.inventory.targetVersion
      const submission = ++this.submissionSequence
      this.submitting = true
      try {
        await MessageBox.confirm(
          this.$t('containerManagement.updateConfirm', { name: this.currentServer.name, version: targetVersion }),
          this.$t('confirm.warn'),
          { type: 'warning', confirmButtonText: this.$t('confirm.yes'), cancelButtonText: this.$t('confirm.cancel') }
        )
        if (!this.currentRequest(sequence, id)) return
        const response = await updateNodeContainer({ nodeServerId: id })
        if (!this.currentRequest(sequence, id)) return
        const job = response.data
        if (!job || !job.id || !['queued', 'running', 'succeeded', 'failed'].includes(job.status)) throw new Error(this.$t('containerManagement.invalidInventory'))
        this.inventory = { ...this.inventory, job }
        this.schedulePoll(sequence, id)
        if (!activeStatuses.includes(job.status)) await this.loadInventory()
      } catch (error) {
        if (error === 'cancel' || error === 'close') return
        if (this.currentRequest(sequence, id)) this.loadError = this.$t('containerManagement.updateFailed', { error: error.message || String(error) })
      } finally {
        if (!this.disposed && this.submissionSequence === submission) this.submitting = false
      }
    }
  }
}
</script>

<style scoped>
.section { margin-bottom: 20px; }
.section-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 16px; padding-bottom: 14px; border-bottom: 1px solid var(--hairline); color: var(--ink); font-weight: 700; }
.container-error { margin-bottom: 20px; color: var(--bad-fg); }
.image-name { overflow-wrap: anywhere; }
.submit { margin-top: 20px; }
</style>
