<template>
  <button
    class="liquid-switch"
    v-bind="controlAttrs"
    :class="{ 'is-active': checked }"
    type="button"
    role="switch"
    :aria-checked="checked.toString()"
    :disabled="disabled"
    @click="toggle"
  >
    <span
      class="liquid-switch__option"
      :class="{ 'is-current': !checked }"
      aria-hidden="true"
    >
      {{ inactiveText }}
    </span>
    <span
      class="liquid-switch__option"
      :class="{ 'is-current': checked, 'is-enabled': checked }"
      aria-hidden="true"
    >
      {{ activeText }}
    </span>
  </button>
</template>

<script>
import emitter from '@/mixins/liquid-control-emitter'
import formControl from '@/mixins/liquid-form-control'

export default {
  name: 'LiquidSwitch',
  inheritAttrs: false,
  mixins: [formControl, emitter],
  props: {
    value: { type: [Boolean, Number, String], default: false },
    activeValue: { type: [Boolean, Number, String], default: true },
    inactiveValue: { type: [Boolean, Number, String], default: false },
    activeText: { type: String, default: '开启' },
    inactiveText: { type: String, default: '禁用' },
    disabled: { type: Boolean, default: false }
  },
  computed: {
    checked() {
      return this.value === this.activeValue
    }
  },
  methods: {
    toggle() {
      const value = this.checked ? this.inactiveValue : this.activeValue
      this.$emit('input', value)
      this.$emit('change', value)
      this.dispatch('LiquidFormItem', 'liquid.form.change', [value])
    }
  }
}
</script>

<style scoped>
.liquid-switch {
  display: inline-grid;
  grid-template-columns: repeat(2, minmax(52px, 1fr));
  align-items: center;
  gap: 3px;
  min-height: 38px;
  padding: 4px;
  border: 1px solid var(--control-border);
  border-radius: 999px;
  color: var(--ink-2);
  background: var(--control-fill);
  box-shadow: inset 0 1px 0 var(--spec-soft), var(--shadow-soft);
  font: inherit;
  cursor: pointer;
  -webkit-tap-highlight-color: transparent;
}
.liquid-switch__option {
  display: grid;
  place-items: center;
  min-height: 30px;
  padding: 0 12px;
  border-radius: 999px;
  color: var(--ink-3);
  font-size: 12.5px;
  font-weight: 650;
  line-height: 1;
  white-space: nowrap;
  transition: color var(--ui-motion-fast) var(--ui-motion-easing-standard), background var(--ui-motion-fast) var(--ui-motion-easing-standard),
    box-shadow var(--ui-motion-fast) var(--ui-motion-easing-standard), transform var(--ui-motion-fast) var(--ui-motion-easing-standard);
}
.liquid-switch__option.is-current {
  color: var(--ink);
  background: var(--glass-strong);
  box-shadow: inset 0 1px 0 var(--spec),
    0 4px 12px -6px var(--ui-control-strong-shadow-ink);
}
.liquid-switch__option.is-current.is-enabled {
  color: var(--accent-deep);
}
.liquid-switch:active:not(:disabled) .liquid-switch__option.is-current {
  transform: scale(0.97);
}
.liquid-switch:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--accent) 58%, transparent);
  outline-offset: 2px;
}
.liquid-switch:disabled {
  cursor: not-allowed;
  opacity: 0.58;
}
</style>
