<template>
  <span class="liquid-tag" :class="`is-${type}`">
    <button v-if="$listeners.click" class="liquid-tag__action" type="button" @click="$emit('click', $event)"><slot /></button>
    <slot v-else />
    <button
      v-if="closable"
      class="liquid-tag__close"
      type="button"
      aria-label="移除"
      @click.stop="$emit('close', $event)"
    >
      <app-icon name="close" aria-hidden="true" />
    </button>
  </span>
</template>

<script>
export default {
  name: 'LiquidTag',
  props: {
    type: { type: String, default: 'default' },
    closable: { type: Boolean, default: false }
  }
}
</script>

<style scoped>
.liquid-tag {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-height: 28px;
  padding: 4px 9px;
  border: 1px solid var(--hairline);
  border-radius: 999px;
  color: var(--ink-2);
  background: var(--glass-soft);
  font-size: 12px;
  font-weight: 650;
}
.liquid-tag.is-success {
  color: var(--ok-fg);
  background: var(--ok-bg);
}
.liquid-tag.is-warning {
  color: var(--warn-fg);
  background: var(--warn-bg);
}
.liquid-tag.is-danger {
  color: var(--bad-fg);
  background: var(--bad-bg);
}
.liquid-tag__close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  padding: 0;
  border: 0;
  border-radius: 50%;
  color: inherit;
  background: color-mix(in srgb, currentColor 10%, transparent);
  cursor: pointer;
}
.liquid-tag__action {
  min-width: 0;
  padding: 0;
  border: 0;
  color: inherit;
  background: transparent;
  font: inherit;
  cursor: pointer;
}
</style>
