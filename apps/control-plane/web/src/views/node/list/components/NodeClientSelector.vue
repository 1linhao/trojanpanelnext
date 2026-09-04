<template>
  <div class="client-selector">
    <div
      class="client-selector__grid"
      role="group"
      :aria-label="$t('table.nodeClients').toString()"
    >
      <button
        v-for="client in clients"
        :key="client.value"
        class="client-choice"
        :class="{ 'is-selected': isClientSelected(client.value) }"
        type="button"
        :aria-pressed="isClientSelected(client.value).toString()"
        @click="toggleClient(client.value)"
      >
        <span class="client-choice__mark">{{ client.mark }}</span>
        <span class="client-choice__label">{{ client.label }}</span>
        <app-icon name="check" class="client-choice__check" aria-hidden="true" />
      </button>
    </div>

    <p class="client-selector__hint ui-supporting-text">
      {{ $t('table.nodeClientsTip') }}
    </p>

    <div v-if="showNaiveWarning" class="client-selector__warning" role="alert">
      <app-icon name="warning-outline" aria-hidden="true" />
      <span>{{ $t('table.naiveClientWarning') }}</span>
    </div>
  </div>
</template>

<script>
import { isNaiveProxy } from '@/utils/node'

export default {
  name: 'NodeClientSelector',
  inject: {
    nodeForm: {
      from: 'nodeFormModel'
    }
  },
  data() {
    return {
      clients: [
        { value: 'sing-box', label: 'sing-box', mark: 'S' },
        { value: 'clash-meta', label: 'Clash.Meta', mark: 'C' },
        { value: 'v2ray', label: 'V2Ray', mark: 'V' },
        { value: 'shadowrocket', label: 'Shadowrocket', mark: 'R' }
      ]
    }
  },
  computed: {
    showNaiveWarning() {
      const selected = Array.isArray(this.nodeForm.clients)
        ? this.nodeForm.clients
        : []
      return (
        isNaiveProxy(this.nodeForm) &&
        (selected.includes('clash-meta') || selected.includes('v2ray'))
      )
    }
  },
  methods: {
    isClientSelected(value) {
      return Array.isArray(this.nodeForm.clients)
        ? this.nodeForm.clients.includes(value)
        : false
    },
    toggleClient(value) {
      const selected = new Set(
        Array.isArray(this.nodeForm.clients) ? this.nodeForm.clients : []
      )
      if (selected.has(value)) selected.delete(value)
      else selected.add(value)
      this.$set(
        this.nodeForm,
        'clients',
        this.clients
          .map((client) => client.value)
          .filter((client) => selected.has(client))
      )
    }
  }
}
</script>

<style scoped>
.client-selector__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  width: 100%;
}

.client-choice {
  display: grid;
  grid-template-columns: 30px minmax(0, 1fr) 20px;
  align-items: center;
  width: 100%;
  min-width: 0;
  min-height: 50px;
  margin: 0;
  padding: 8px 12px;
  border: 1px solid var(--control-border);
  border-radius: 16px;
  color: var(--ink-2);
  background: var(--control-fill);
  box-shadow: inset 0 1px 0 var(--spec-soft);
  font-family: var(--font);
  font-size: 13px;
  font-weight: 650;
  text-align: left;
  cursor: pointer;
  appearance: none;
}

.client-choice:hover {
  color: var(--ink);
  border-color: var(--rim);
  background: var(--glass-strong);
  transform: translateY(-1px);
}

.client-choice:focus-visible {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 18%, transparent),
    inset 0 1px 0 var(--spec-soft);
}

.client-choice.is-selected {
  color: var(--accent);
  border-color: color-mix(in srgb, var(--accent) 62%, var(--rim));
  background: color-mix(in srgb, var(--accent) 12%, var(--control-fill));
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent) 13%, transparent),
    inset 0 1px 0 var(--spec-soft);
}

.client-choice__mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  color: var(--ink-2);
  font-size: 12px;
  font-weight: 700;
  background: var(--glass-soft);
  border-radius: 9px;
}

.client-choice.is-selected .client-choice__mark {
  color: var(--on-accent);
  background: var(--accent);
}

.client-choice__label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.client-choice__check {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  color: var(--accent-deep);
  opacity: 0;
  transform: scale(0.72);
  transition: opacity var(--ui-motion-fast) var(--ui-motion-easing-standard), transform var(--ui-motion-fast) var(--ui-motion-easing-standard);
}

.client-choice.is-selected .client-choice__check {
  opacity: 1;
  transform: scale(1);
}

.client-selector__hint {
  margin: 9px 0 0;
}

.client-selector__warning {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin-top: 10px;
  padding: 10px 12px;
  border: 1px solid color-mix(in srgb, var(--warn-fg) 28%, var(--rim));
  border-radius: 14px;
  color: var(--warn-fg);
  background: var(--warn-bg);
  font-size: 12px;
  line-height: 1.55;
}

@media (max-width: 520px) {
  .client-selector__grid {
    grid-template-columns: 1fr;
  }
}
</style>
