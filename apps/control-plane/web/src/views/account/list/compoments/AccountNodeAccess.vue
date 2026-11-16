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
    return { loaded: false, loading: false, saving: false, error: '', restricted: false, nodeIds: [], nodes: [], search: '' }
  },
  computed: {
    filteredNodes() {
      const search = this.search.trim().toLowerCase()
      return this.nodes.filter(node => node.name.toLowerCase().includes(search))
    }
  },
  created() {
    this.load()
  },
  methods: {
    async load() {
      this.loading = true
      this.error = ''
      try {
        const { data } = await selectAccountNodeAccess(this.account.id)
        this.restricted = data.restricted
        this.nodeIds = data.nodeIds
        this.nodes = data.nodes
        this.loaded = true
      } catch {
        this.error = this.$t('accountNodeAccess.loadFailed')
      } finally {
        this.loading = false
      }
    },
    async save() {
      if (!this.loaded || this.saving) return
      this.saving = true
      this.error = ''
      try {
        await updateAccountNodeAccess({
          id: this.account.id,
          restricted: this.restricted,
          nodeIds: this.restricted ? this.nodeIds : []
        })
        this.$message({ type: 'success', message: this.$t('accountNodeAccess.saved') })
        this.$emit('close')
      } catch (error) {
        this.error = error.message === 'node agents must be upgraded and online before restricting access'
          ? this.$t('accountNodeAccess.agentsNotReady')
          : this.$t('accountNodeAccess.saveFailed')
      } finally {
        this.saving = false
      }
    }
  }
}
</script>

<style scoped>
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
