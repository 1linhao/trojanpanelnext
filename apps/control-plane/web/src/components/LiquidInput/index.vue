<template>
  <div
    class="liquid-input"
    :class="{ 'is-focused': focused, 'is-disabled': disabled, 'is-compact': compact }"
  >
    <textarea
      v-if="type === 'textarea'"
      ref="field"
      v-bind="controlAttrs"
      class="liquid-input__field liquid-input__textarea"
      :value="value"
      :disabled="disabled"
      :readonly="readonly"
      @input="handleInput"
      @change="handleChange"
      @focus="handleFocus"
      @blur="handleBlur"
    />
    <input
      v-else
      ref="field"
      v-bind="controlAttrs"
      class="liquid-input__field"
      :type="type"
      :value="value"
      :disabled="disabled"
      :readonly="readonly"
      @input="handleInput"
      @change="handleChange"
      @focus="handleFocus"
      @blur="handleBlur"
    />
    <button
      v-if="clearable && !clearSuppressed && value !== '' && value !== undefined && value !== null"
      class="liquid-input__clear"
      type="button"
      aria-label="清空"
      title="清空"
      :disabled="disabled"
      @click="clear"
    >
      <app-icon name="close" aria-hidden="true" />
    </button>
  </div>
</template>

<script>
import emitter from '@/mixins/liquid-control-emitter'
import formControl from '@/mixins/liquid-form-control'

export default {
  name: 'LiquidInput',
  mixins: [formControl, emitter],
  inheritAttrs: false,
  props: {
    value: { type: [String, Number], default: '' },
    type: { type: String, default: 'text' },
    clearable: { type: Boolean, default: false },
    disabled: { type: Boolean, default: false },
    readonly: { type: Boolean, default: false },
    compact: { type: Boolean, default: false }
  },
  data() {
    return { focused: false }
  },
  computed: {
    clearSuppressed() {
      return this.disabled || this.readonly
    }
  },
  methods: {
    notify(value) {
      this.$emit('input', value)
      this.dispatch('LiquidFormItem', 'liquid.form.change', [value])
    },
    handleInput(event) {
      this.notify(event.target.value)
    },
    handleChange(event) {
      this.$emit('change', event.target.value)
    },
    handleFocus(event) {
      this.focused = true
      this.$emit('focus', event)
    },
    handleBlur(event) {
      this.focused = false
      this.$emit('blur', event)
      this.dispatch('LiquidFormItem', 'liquid.form.blur', [event.target.value])
    },
    clear() {
      if (this.disabled || this.readonly) return
      this.notify('')
      this.$emit('clear')
      this.$nextTick(() => this.$refs.field?.focus())
    },
    focus() {
      this.$refs.field.focus()
    }
  }
}
</script>

<style scoped>
.liquid-input {
  display: flex;
  align-items: center;
  width: min(100%, var(--control-max-width));
  max-width: var(--control-max-width);
  min-width: 0;
  min-height: 42px;
  padding: 0 10px;
  border: 1px solid var(--control-border);
  border-radius: 14px;
  color: var(--ink);
  background: var(--control-fill);
  box-shadow: inset 0 1px 0 var(--spec-soft);
  transition: border-color var(--ui-motion-fast) var(--ui-motion-easing-standard), box-shadow var(--ui-motion-fast) var(--ui-motion-easing-standard);
}
.liquid-input.is-focused {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 18%, transparent),
    inset 0 1px 0 var(--spec-soft);
}
.liquid-input__field {
  flex: 1;
  width: 100%;
  min-width: 0;
  min-height: 40px;
  padding: 0 4px;
  border: 0;
  outline: 0;
  color: var(--ink);
  background: transparent;
  font: inherit;
}
.liquid-input.is-compact {
  min-height: 34px;
  border-radius: 11px;
}
.liquid-input.is-compact .liquid-input__field {
  min-height: 32px;
}
.liquid-input__textarea {
  min-height: var(--ui-editor-body-min-height, 92px);
  padding: 10px 4px;
  resize: vertical;
}
.liquid-input__clear {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  padding: 0;
  border: 0;
  border-radius: 50%;
  color: var(--ink-3);
  background: var(--glass-soft);
  cursor: pointer;
}
.liquid-input.is-disabled {
  cursor: not-allowed;
  opacity: 0.58;
}
@media (max-width: 760px) {
  .liquid-input__textarea {
    min-height: var(
      --ui-editor-mobile-body-min-height,
      var(--ui-editor-body-min-height, 92px)
    );
  }
}
</style>
