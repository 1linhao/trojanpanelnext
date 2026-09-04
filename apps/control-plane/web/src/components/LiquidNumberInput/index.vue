<template>
  <div
    class="liquid-number-input"
    :class="{ 'is-disabled': disabled, 'is-focused': focused }"
  >
    <input
      ref="input"
      v-bind="controlAttrs"
      class="liquid-number-input__field"
      type="number"
      inputmode="decimal"
      :value="displayValue"
      :min="finiteMin"
      :max="finiteMax"
      :step="step"
      :disabled="disabled"
      @input="handleInput"
      @change="handleChange"
      @focus="handleFocus"
      @blur="handleBlur"
    />
    <button
      class="liquid-number-input__step"
      type="button"
      aria-label="减少"
      title="减少"
      :disabled="disabled || decreaseDisabled"
      @click="stepValue(-1)"
    >
      <app-icon name="minus" aria-hidden="true" />
    </button>
    <button
      class="liquid-number-input__step"
      type="button"
      aria-label="增加"
      title="增加"
      :disabled="disabled || increaseDisabled"
      @click="stepValue(1)"
    >
      <app-icon name="plus" aria-hidden="true" />
    </button>
  </div>
</template>

<script>
import emitter from '@/mixins/liquid-control-emitter'
import formControl from '@/mixins/liquid-form-control'

export default {
  name: 'LiquidNumberInput',
  mixins: [formControl, emitter],
  inheritAttrs: false,
  props: {
    value: {
      type: Number,
      default: undefined
    },
    min: {
      type: Number,
      default: -Infinity
    },
    max: {
      type: Number,
      default: Infinity
    },
    step: {
      type: Number,
      default: 1
    },
    precision: {
      type: Number,
      default: undefined
    },
    disabled: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return { focused: false }
  },
  computed: {
    displayValue() {
      return this.value === undefined || this.value === null ? '' : this.value
    },
    finiteMin() {
      return Number.isFinite(this.min) ? this.min : undefined
    },
    finiteMax() {
      return Number.isFinite(this.max) ? this.max : undefined
    },
    decreaseDisabled() {
      return Number.isFinite(this.min) && Number(this.value) <= this.min
    },
    increaseDisabled() {
      return Number.isFinite(this.max) && Number(this.value) >= this.max
    }
  },
  methods: {
    decimalPlaces(value) {
      const valueText = String(value)
      return valueText.includes('.') ? valueText.length - valueText.indexOf('.') - 1 : 0
    },
    normalize(value, clamp = false) {
      if (value === '' || value === undefined || value === null) return undefined
      const number = Number(value)
      if (!Number.isFinite(number)) return this.value
      const digits =
        this.precision === undefined
          ? Math.max(this.decimalPlaces(this.step), this.decimalPlaces(number))
          : this.precision
      const rounded = Number(number.toFixed(digits))
      if (!clamp) return rounded
      return Math.min(this.max, Math.max(this.min, rounded))
    },
    emitValue(value) {
      this.$emit('input', value)
      this.$emit('change', value)
      this.dispatch('LiquidFormItem', 'liquid.form.change', [value])
    },
    handleInput(event) {
      const value = this.normalize(event.target.value)
      this.$emit('input', value)
      this.dispatch('LiquidFormItem', 'liquid.form.change', [value])
    },
    handleChange(event) {
      const value = this.normalize(event.target.value, true)
      this.emitValue(value)
    },
    handleFocus(event) {
      this.focused = true
      this.$emit('focus', event)
    },
    handleBlur(event) {
      this.focused = false
      const value = this.normalize(event.target.value, true)
      if (value !== this.value) this.emitValue(value)
      this.$emit('blur', event)
      this.dispatch('LiquidFormItem', 'liquid.form.blur', [value])
    },
    stepValue(direction) {
      const current = Number.isFinite(Number(this.value)) ? Number(this.value) : 0
      const value = this.normalize(current + direction * this.step, true)
      this.emitValue(value)
      this.$nextTick(() => this.$refs.input.focus())
    }
  }
}
</script>

<style scoped>
.liquid-number-input {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 32px 32px;
  align-items: center;
  gap: 5px;
  width: min(100%, var(--control-max-width));
  max-width: var(--control-max-width);
  min-width: 0;
  min-height: 42px;
  padding: 4px;
  border: 1px solid var(--control-border);
  border-radius: 14px;
  color: var(--ink);
  background: var(--control-fill);
  box-shadow: inset 0 1px 0 var(--spec-soft);
  transition: border-color var(--ui-motion-fast) var(--ui-motion-easing-standard), box-shadow var(--ui-motion-fast) var(--ui-motion-easing-standard);
}

.liquid-number-input.is-focused {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 18%, transparent),
    inset 0 1px 0 var(--spec-soft);
}

.liquid-number-input__field {
  width: 100%;
  min-width: 0;
  height: 32px;
  padding: 0 8px;
  border: 0;
  outline: 0;
  color: var(--ink);
  background: transparent;
  font: inherit;
  font-variant-numeric: tabular-nums;
  appearance: textfield;
}

.liquid-number-input__field::-webkit-inner-spin-button,
.liquid-number-input__field::-webkit-outer-spin-button {
  margin: 0;
  appearance: none;
}

.liquid-number-input__step {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  padding: 0;
  border: 1px solid var(--rim);
  border-radius: 11px;
  color: var(--ink-2);
  background: var(--glass-soft);
  box-shadow: inset 0 1px 0 var(--spec-soft);
  cursor: pointer;
}

.liquid-number-input__step:hover:not(:disabled) {
  color: var(--accent);
  border-color: color-mix(in srgb, var(--accent) 48%, var(--rim));
  background: var(--glass-strong);
}

.liquid-number-input__step:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--accent) 58%, transparent);
  outline-offset: 1px;
}

.liquid-number-input__step:disabled,
.liquid-number-input.is-disabled {
  cursor: not-allowed;
  opacity: 0.58;
}
</style>
