<template>
  <button
    class="liquid-button"
    data-ui-control
    data-ui-interaction="nav-lift"
    :data-ui-size="controlSize"
    :class="[`is-${type}`, { 'is-icon-only': icon && !$slots.default }]"
    type="button"
    :disabled="disabled || loading"
    v-bind="$attrs"
    @click="$emit('click', $event)"
  >
    <app-icon v-if="loading" name="loading" aria-hidden="true" />
    <app-icon v-else-if="icon" :name="icon" aria-hidden="true" />
    <span v-if="$slots.default"><slot /></span>
  </button>
</template>

<script>
import controlSize from '@/mixins/liquid-control-size'
export default {
  name: 'LiquidButton',
  mixins: [controlSize],
  inheritAttrs: false,
  props: {
    type: { type: String, default: 'default' },
    icon: { type: String, default: '' },
    disabled: { type: Boolean, default: false },
    loading: { type: Boolean, default: false }
  }
}
</script>
