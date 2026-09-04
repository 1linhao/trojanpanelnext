<template>
  <div
    class="liquid-select"
    data-ui-control
    :data-ui-size="controlSize"
    :class="{ 'is-focused': open, 'is-disabled': disabled }"
  >
    <select
      ref="native"
      class="liquid-select__native"
      :value="multiple ? undefined : value"
      :disabled="disabled"
      :multiple="multiple"
      hidden
      tabindex="-1"
      aria-hidden="true"
    >
      <slot />
    </select>

    <button
      ref="trigger"
      class="liquid-select__trigger"
      type="button"
      v-bind="controlAttrs"
      :disabled="disabled"
      :aria-controls="`liquid-select-menu-${_uid}`"
      :aria-expanded="String(open)"
      aria-haspopup="listbox"
      @click="toggleMenu"
      @keydown.down.prevent="openMenu"
      @keydown.enter.prevent="toggleMenu"
      @keydown.space.prevent="toggleMenu"
      @keydown.esc="handleEscape"
    >
      <span :class="{ 'is-placeholder': !hasSelection }">
        {{ selectedLabel || placeholder || '请选择' }}
      </span>
      <app-icon
        v-if="!clearable || !hasSelection"
        name="arrow-down" class="liquid-select__arrow"
        :class="{ 'is-open': open }"
        aria-hidden="true" />
    </button>
    <button v-if="clearable && hasSelection" type="button" class="liquid-select__clear" aria-label="清空" :disabled="disabled" @click.stop="clearSelection"><app-icon name="close" aria-hidden="true" /></button>

    <div
      ref="menu"
      :id="`liquid-select-menu-${_uid}`"
      @keydown="handleMenuKeydown"
      class="liquid-select__menu"
      popover="manual"
      :style="menuStyle"
      role="listbox"
      :aria-multiselectable="multiple ? 'true' : undefined"
      @click.stop
    >
      <div v-if="filterable" class="liquid-select__search-wrap">
        <app-icon name="search" aria-hidden="true" />
        <input
          ref="search"
          v-model.trim="query"
          class="liquid-select__search"
          type="search"
          aria-label="搜索选项"
          placeholder="搜索"
        />
      </div>
      <div class="liquid-select__options">
        <button
          v-for="(option, index) in filteredOptions"
          :key="`${index}-${option.label}`"
          class="liquid-select__option"
          :class="{ 'is-selected': isSelected(option.value) }"
          type="button"
          role="option"
          :aria-selected="String(isSelected(option.value))"
          :disabled="option.disabled"
          @click="choose(option.value)"
        >
          <span>{{ option.label }}</span>
          <app-icon
            v-if="isSelected(option.value)"
            name="check"
            aria-hidden="true" />
        </button>
        <div v-if="!filteredOptions.length" class="liquid-select__empty">
          暂无匹配项
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import emitter from '@/mixins/liquid-control-emitter'
import formControl from '@/mixins/liquid-form-control'
import controlSize from '@/mixins/liquid-control-size'

export default {
  name: 'LiquidSelect',
  mixins: [formControl, emitter, controlSize],
  inheritAttrs: false,
  props: {
    value: { type: [String, Number, Boolean, Array], default: '' },
    disabled: { type: Boolean, default: false },
    multiple: { type: Boolean, default: false },
    clearable: { type: Boolean, default: false },
    filterable: { type: Boolean, default: false },
    placeholder: { type: String, default: '' }
  },
  data() {
    return {
      open: false,
      query: '',
      options: [],
      optionsSignature: '',
      menuStyle: {}
    }
  },
  computed: {
    hasSelection() {
      return this.multiple
        ? Array.isArray(this.value) && this.value.length > 0
        : this.value !== '' && this.value !== undefined && this.value !== null
    },
    selectedLabel() {
      const selected = this.options.filter((option) =>
        this.isSelected(option.value)
      )
      return selected.map((option) => option.label).join('、')
    },
    filteredOptions() {
      if (!this.query) return this.options
      const query = this.query.toLocaleLowerCase()
      return this.options.filter((option) =>
        option.label.toLocaleLowerCase().includes(query)
      )
    }
  },
  mounted() {
    this.refreshOptions()
    document.addEventListener('click', this.handleOutside)
    window.addEventListener('resize', this.updatePosition)
    window.addEventListener('scroll', this.updatePosition, true)
  },
  updated() {
    this.refreshOptions()
  },
  beforeDestroy() {
    this.closeMenu()
    document.removeEventListener('click', this.handleOutside)
    window.removeEventListener('resize', this.updatePosition)
    window.removeEventListener('scroll', this.updatePosition, true)
  },
  methods: {
    handleEscape(event) {
      if (!this.open) return
      event.preventDefault()
      event.stopPropagation()
      this.closeMenu()
      this.$refs.trigger?.focus()
    },
    refreshOptions() {
      if (!this.$refs.native) return
      const options = Array.from(this.$refs.native.options).map((option) => ({
        label: option.label || option.textContent.trim(),
        value: Object.prototype.hasOwnProperty.call(option, '_value')
          ? option._value
          : option.value,
        disabled: option.disabled
      }))
      const signature = options
        .map((option) => `${String(option.value)}:${option.label}:${option.disabled}`)
        .join('|')
      if (signature !== this.optionsSignature) {
        this.optionsSignature = signature
        this.options = options
      }
    },
    isSelected(optionValue) {
      if (this.multiple) {
        return Array.isArray(this.value) &&
          this.value.some((value) => value === optionValue)
      }
      return this.value === optionValue
    },
    emitValue(value) {
      this.$emit('input', value)
      this.$emit('change', value)
      this.dispatch('LiquidFormItem', 'liquid.form.change', [value])
    },
    choose(optionValue) {
      if (this.multiple) {
        const values = Array.isArray(this.value) ? [...this.value] : []
        const index = values.findIndex((value) => value === optionValue)
        if (index >= 0) values.splice(index, 1)
        else values.push(optionValue)
        this.emitValue(values)
        return
      }
      this.emitValue(optionValue)
      this.closeMenu()
      this.$nextTick(() => this.$refs.trigger && this.$refs.trigger.focus())
    },
    clearSelection() {
      if (this.disabled) return
      this.emitValue(this.multiple ? [] : '')
      this.$nextTick(() => this.$refs.trigger?.focus())
    },
    toggleMenu() {
      if (this.open) this.closeMenu()
      else this.openMenu()
    },
    openMenu() {
      if (this.disabled || this.open) return
      this.refreshOptions()
      this.open = true
      this.query = ''
      this.$nextTick(() => {
        this.updatePosition()
        if (this.$refs.menu.showPopover) this.$refs.menu.showPopover()
        if (this.filterable && this.$refs.search) this.$refs.search.focus()
        else this.$refs.menu.querySelector('.liquid-select__option:not(:disabled)')?.focus()
      })
    },
    handleMenuKeydown(event) {
      if (event.key === 'Escape') {
        event.preventDefault()
        event.stopPropagation()
        this.closeMenu()
        this.$refs.trigger.focus()
        return
      }
      if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
      if (event.target === this.$refs.search && !['ArrowDown', 'ArrowUp'].includes(event.key)) return
      const items = Array.from(this.$refs.menu.querySelectorAll('.liquid-select__option:not(:disabled)'))
      if (!items.length) return
      event.preventDefault()
      const index = items.indexOf(event.target)
      const next = event.key === 'Home' ? 0 : event.key === 'End' ? items.length - 1 :
        (index + (event.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length
      items[next].focus()
    },
    closeMenu() {
      if (!this.open) return
      this.open = false
      if (this.$refs.menu && this.$refs.menu.hidePopover) {
        try {
          this.$refs.menu.hidePopover()
        } catch (error) {
          // The popover may already have been dismissed by the browser.
        }
      }
      this.$emit('blur')
      this.dispatch('LiquidFormItem', 'liquid.form.blur', [this.value])
    },
    updatePosition() {
      if (!this.open || !this.$refs.trigger) return
      const rect = this.$refs.trigger.getBoundingClientRect()
      const width = Math.min(rect.width, window.innerWidth - 24)
      const maxHeight = Math.min(320, window.innerHeight - 24)
      const roomBelow = window.innerHeight - rect.bottom - 12
      const openAbove = roomBelow < 180 && rect.top > roomBelow
      this.menuStyle = {
        left: `${Math.min(Math.max(12, rect.left), window.innerWidth - width - 12)}px`,
        top: openAbove ? 'auto' : `${rect.bottom + 6}px`,
        bottom: openAbove ? `${window.innerHeight - rect.top + 6}px` : 'auto',
        width: `${width}px`,
        maxHeight: `${maxHeight}px`
      }
    },
    handleOutside(event) {
      if (!this.open) return
      const inTrigger = this.$el && this.$el.contains(event.target)
      const inMenu = this.$refs.menu && this.$refs.menu.contains(event.target)
      if (!inTrigger && !inMenu) this.closeMenu()
    }
  }
}
</script>

<style scoped>
.liquid-select {
  position: relative;
  width: min(100%, var(--control-max-width));
  max-width: var(--control-max-width);
  min-width: 0;
}
.liquid-select__native {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  clip-path: inset(50%);
}
.liquid-select__trigger {
  --ui-button-hover-lift: 0px;
  --ui-select-tail-gap: 8px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  min-width: 0;
  min-height: var(--ui-control-size-height, 42px);
  padding: var(--ui-control-size-padding-y, 0) var(--ui-control-size-padding-x, 14px);
  border: 1px solid var(--control-border);
  border-radius: 14px;
  color: var(--ink);
  background: var(--control-fill);
  box-shadow: inset 0 1px 0 var(--spec-soft);
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.liquid-select__trigger > span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.liquid-select__trigger .is-placeholder {
  color: var(--ink-3);
}
.liquid-select__empty {
  color: var(--supporting-text-ink);
}
.liquid-select.is-focused .liquid-select__trigger {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 18%, transparent),
    inset 0 1px 0 var(--spec-soft);
}
.liquid-select__arrow,
.liquid-select__clear {
  flex: 0 0 auto;
  margin-left: var(--ui-select-tail-gap, 8px);
  color: var(--ink-3);
}
.liquid-select__arrow {
  transition: transform var(--ui-motion-fast) var(--ui-motion-easing-standard);
}
.liquid-select__arrow.is-open {
  transform: rotate(180deg);
}
.liquid-select__clear {
  position: absolute;
  right: var(--ui-control-size-padding-x, 14px);
  top: calc(50% - 12px);
  display: grid;
  place-items: center;
  width: 24px;
  height: 24px;
  padding: 0;
  border: 0;
  border-radius: 50%;
  background: var(--neutral-bg);
}
.liquid-select__menu {
  --liquid-select-counterstroke: color-mix(
    in srgb,
    var(--bg-base) 88%,
    transparent
  );
  position: fixed;
  z-index: 5000;
  box-sizing: border-box;
  margin: 0;
  padding: 7px;
  border: 1px solid var(--rim);
  border-radius: var(--r-md);
  color: var(--ink);
  background: linear-gradient(
      150deg,
      var(--spec-soft),
      transparent 46%
    ),
    var(--glass);
  box-shadow: var(--shadow), inset 0 1px 0 var(--spec),
    inset 0 -1px 0 var(--spec-soft);
  -webkit-backdrop-filter: var(--ui-backdrop-surface);
  backdrop-filter: var(--ui-backdrop-surface);
  -webkit-text-stroke: 0.25px var(--liquid-select-counterstroke);
  paint-order: stroke fill;
  text-shadow: 0 0 0.45px var(--liquid-select-counterstroke);
  overflow: hidden;
}
.liquid-select__search-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
  padding: 0 10px;
  border: 1px solid var(--control-border);
  border-radius: 11px;
  background: var(--control-fill);
}
.liquid-select__search-wrap > .app-icon {
  color: var(--ink-3);
}
.liquid-select__search {
  width: 100%;
  min-height: 34px;
  border: 0;
  outline: 0;
  color: var(--ink);
  background: transparent;
  font: inherit;
}
.liquid-select__search::placeholder {
  -webkit-text-stroke: 0.25px var(--liquid-select-counterstroke);
  text-shadow: 0 0 0.45px var(--liquid-select-counterstroke);
}
.liquid-select__options {
  max-height: 258px;
  overflow-y: auto;
  overscroll-behavior: contain;
}
.liquid-select__option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
  min-height: 38px;
  padding: 8px 11px;
  text-align: left;
  cursor: pointer;
}
.liquid-select__option:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}
.liquid-select__empty {
  padding: 14px 10px;
  text-align: center;
}
.liquid-select.is-disabled {
  opacity: 0.58;
}
</style>
