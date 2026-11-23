<template>
  <ui-dialog
    :title="$t('accountNodeAccess.title', { username: account.username })"
    :visible="true"
    :close-on-click-modal="false"
    :close-on-escape="!saving"
    :show-close="!saving"
    width="480px"
    @update:visible="$emit('close')"
  >
    <p v-if="loading" role="status">{{ $t('accountNodeAccess.loading') }}</p>
    <p v-if="error" class="bad-text" role="alert">{{ error }}</p>
    <liquid-button v-if="!loading && !loaded" @click="load">
      {{ $t('accountNodeAccess.retry') }}
    </liquid-button>
    <div v-if="hasTrackedAccess" role="status" class="ui-supporting-text account-node-sync">
      <p v-if="superseded">{{ $t('accountNodeAccess.superseded') }}</p>
      <p>{{ $t('accountNodeAccess.subscriptionSaved') }} {{ subscriptionScope }}</p>
      <p>{{ syncMessage }}</p>
      <p v-if="sync && sync.legacy">{{ $t('accountNodeAccess.legacy', { count: sync.legacy }) }}</p>
      <p v-if="sync && sync.unreported">{{ $t('accountNodeAccess.unreported', { count: sync.unreported }) }}</p>
    </div>
    <template v-if="loaded">
      <fieldset class="account-node-mode" :disabled="saving">
        <legend>{{ $t('accountNodeAccess.action') }}</legend>
        <label>
          <input v-model="restricted" :value="false" type="radio" name="account-node-mode" />
          {{ $t('accountNodeAccess.all') }}
        </label>
        <label>
          <input v-model="restricted" :value="true" type="radio" name="account-node-mode" />
          {{ $t('accountNodeAccess.selected') }}
        </label>
      </fieldset>
      <div v-if="restricted" class="account-node-selection">
        <liquid-input
          v-if="nodes.length"
          v-model="search"
          :placeholder="$t('accountNodeAccess.search')"
          :aria-label="$t('accountNodeAccess.search')"
          :disabled="saving"
          clearable
        />
        <p class="ui-supporting-text">
          {{ nodeIds.length ? $t('accountNodeAccess.count', { count: nodeIds.length }) : $t('accountNodeAccess.none') }}
        </p>
        <div v-if="filteredNodes.length" class="account-node-options">
          <label v-for="node in filteredNodes" :key="node.id">
            <input v-model="nodeIds" :value="node.id" :disabled="saving" type="checkbox" />
            <span>{{ node.name }}</span>
          </label>
        </div>
        <p v-else class="ui-supporting-text">{{ $t('accountNodeAccess.noResults') }}</p>
      </div>
    </template>
    <div slot="footer" class="dialog-footer">
      <liquid-button :disabled="saving" @click="$emit('close')">{{ $t('confirm.cancel') }}</liquid-button>
      <liquid-button type="primary" :disabled="!loaded || saving" @click="save">
        {{ $t('accountNodeAccess.save') }}
      </liquid-button>
    </div>
  </ui-dialog>
</template>

<script>
import { selectAccountNodeAccess, updateAccountNodeAccess } from '@/api/account'

export default {
  name: 'AccountNodeAccess',
  props: {
    account: { type: Object, required: true }
  },
  data() {
    return {
      loaded: false, loading: false, saving: false, error: '', restricted: false, nodeIds: [], nodes: [], search: '',
      savedRevision: '', sync: null, syncCheck: 0, pollError: false
    }
  },
  computed: {
    hasTrackedAccess() {
      return !!(this.savedRevision || (this.sync && this.sync.revision !== '0'))
    },
    superseded() {
      return this.savedRevision && this.sync && this.savedRevision !== this.sync.revision
    },
    subscriptionScope() {
      if (!this.sync) return ''
      if (!this.sync.restricted) return this.$t('accountNodeAccess.subscriptionAll')
      if (!this.sync.granted) return this.$t('accountNodeAccess.subscriptionNone')
      return this.$t('accountNodeAccess.subscriptionSelected', { count: this.sync.granted })
    },
    syncMessage() {
      if (this.pollError) return this.$t('accountNodeAccess.checkFailed')
      if (!this.sync) return this.$t('accountNodeAccess.checkPending')
      if (!this.sync.servers) return this.$t('accountNodeAccess.noServers')
      return this.$t(this.sync.confirmed === this.sync.servers ? 'accountNodeAccess.confirmed' : 'accountNodeAccess.syncing', this.sync)
    },
    filteredNodes() {
      const search = this.search.trim().toLowerCase()
      return this.nodes.filter(node => node.name.toLowerCase().includes(search))
    }
  },
  created() {
    this.load()
  },
  beforeDestroy() {
    this.stopChecking()
  },
  methods: {
    stopChecking() {
      this.syncCheck++
      clearTimeout(this.syncTimer)
    },
    scheduleCheck(check) {
      if (check !== this.syncCheck || !this.hasTrackedAccess) return
      const settled = !this.pollError && this.sync && this.sync.confirmed === this.sync.servers
      this.syncTimer = setTimeout(() => this.refreshSync(check), settled ? 15000 : 5000)
    },
    async refreshSync(check) {
      try {
        const { data } = await selectAccountNodeAccess(this.account.id, true)
        if (check !== this.syncCheck) return
        this.sync = data
        this.pollError = false
      } catch {
        if (check !== this.syncCheck) return
        this.pollError = true
      }
      this.scheduleCheck(check)
    },
    async load() {
      const check = this.syncCheck
      this.loading = true
      this.error = ''
      try {
        const { data } = await selectAccountNodeAccess(this.account.id)
        if (check !== this.syncCheck) return
        this.restricted = data.restricted
        this.nodeIds = data.nodeIds
        this.nodes = data.nodes
        this.loaded = true
        this.sync = data.sync
        this.scheduleCheck(check)
      } catch {
        if (check !== this.syncCheck) return
        this.error = this.$t('accountNodeAccess.loadFailed')
      } finally {
        this.loading = false
      }
    },
    async save() {
      if (!this.loaded || this.saving) return
      this.stopChecking()
      const check = this.syncCheck
      this.saving = true
      this.error = ''
      try {
        const { data } = await updateAccountNodeAccess({
          id: this.account.id,
          restricted: this.restricted,
          nodeIds: this.restricted ? this.nodeIds : []
        })
        if (check !== this.syncCheck) return
        this.savedRevision = data.revision
        this.sync = null
        this.pollError = false
        this.refreshSync(check)
      } catch (error) {
        if (check !== this.syncCheck) return
        this.error = error.message === 'node agents must be upgraded and online before restricting access'
          ? this.$t('accountNodeAccess.agentsNotReady')
          : this.$t('accountNodeAccess.saveFailed')
        this.scheduleCheck(check)
      } finally {
        this.saving = false
      }
    }
  }
}
</script>

<style scoped>
.account-node-sync p {
  margin-block: 6px;
}

.account-node-mode {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 24px;
  min-inline-size: 0;
  padding: 0;
  border: 0;
}
.account-node-mode legend {
  margin-bottom: 10px;
  font-weight: 600;
}
.account-node-mode label,
.account-node-options label {
  display: flex;
  align-items: baseline;
  gap: 8px;
}
.account-node-options label {
  min-height: 32px;
  padding: 4px 3px;
  overflow-wrap: anywhere;
}
.account-node-selection {
  margin-top: 16px;
}
.account-node-options {
  max-height: min(35vh, 250px);
  overflow: auto;
}
</style>
