<template>
  <div class="liquid-palette-picker">
    <button
      v-if="!inline"
      ref="trigger"
      class="liquid-palette-picker__trigger"
      type="button"
      aria-label="切换颜色主题"
      title="颜色主题"
      :aria-expanded="String(open)"
      @click="toggle"
      @keydown.esc.prevent="close"
    >
      <span class="liquid-palette-picker__swatch" :data-palette-swatch="palette" />
    </button>
    <div
      ref="menu"
      :class="['liquid-palette-picker__menu', { 'is-inline': inline }]"
      :popover="inline ? null : 'manual'"
      :style="menuStyle"
      role="radiogroup"
      aria-label="颜色主题"
      @click.stop
    >
      <span class="ui-field-label">颜色主题</span>
      <span v-if="inline" class="ui-supporting-text">
        选择会保存在当前浏览器中
      </span>
      <button
        v-for="(option, index) in options"
        :key="option.value"
        ref="radio"
        type="button"
        :class="{ 'is-selected': palette === option.value }"
        role="radio"
        :aria-checked="String(palette === option.value)"
        :tabindex="palette === option.value || (index === 0 && !options.some((item) => item.value === palette)) ? 0 : -1"
        @click="choose(option.value)"
        @keydown="handleArrowKeydown($event, index)"
      >
        <span :data-palette-swatch="option.value" />
        {{ option.label }}
        <app-icon v-if="palette === option.value" name="check" />
      </button>
    </div>
  </div>
</template>

<script>
import { applyPalette, getThemeState } from '@/utils/theme'

export default {
  name: 'LiquidPalettePicker',
  props: {
    inline: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      open: false,
      palette: getThemeState().palette,
      menuStyle: {},
      options: [
        { value: 'blue', label: '海蓝' },
        { value: 'violet', label: '紫罗兰' },
        { value: 'emerald', label: '翡翠' },
        { value: 'amber', label: '琥珀' }
      ]
    }
  },
  mounted() {
    if (this.inline) return
    document.addEventListener('click', this.handleOutside)
    window.addEventListener('resize', this.updatePosition)
  },
  beforeDestroy() {
    if (this.inline) return
    this.close()
    document.removeEventListener('click', this.handleOutside)
    window.removeEventListener('resize', this.updatePosition)
  },
  methods: {
    toggle() {
      if (this.open) this.close()
      else this.show()
    },
    show() {
      this.open = true
      this.$nextTick(() => {
        this.updatePosition()
        if (this.$refs.menu.showPopover) this.$refs.menu.showPopover()
      })
    },
    close() {
      if (!this.open) return
      this.open = false
      if (this.$refs.menu && this.$refs.menu.hidePopover) {
        try {
          this.$refs.menu.hidePopover()
        } catch (error) {
          // The browser may already have dismissed the popover.
        }
      }
    },
    choose(palette) {
      this.palette = applyPalette(palette).palette
      if (!this.inline) this.close()
    },
    // WEB-016: roving focus inside the radiogroup. Arrows/Home/End both move
    // focus and select, matching native radio behavior.
    handleArrowKeydown(event, index) {
      if (!['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
      event.preventDefault()
      const last = this.options.length - 1
      let next
      if (event.key === 'Home') next = 0
      else if (event.key === 'End') next = last
      else if (event.key === 'ArrowUp' || event.key === 'ArrowLeft') next = (index - 1 + this.options.length) % this.options.length
      else next = (index + 1) % this.options.length
      this.choose(this.options[next].value)
      this.$nextTick(() => this.$refs.radio && this.$refs.radio[next] && this.$refs.radio[next].focus())
    },
    updatePosition() {
      if (!this.open || !this.$refs.trigger) return
      const rect = this.$refs.trigger.getBoundingClientRect()
      this.menuStyle = {
        top: `${rect.bottom + 8}px`,
        right: `${Math.max(12, window.innerWidth - rect.right)}px`
      }
    },
    handleOutside(event) {
      if (!this.open) return
      if (this.$el.contains(event.target) || this.$refs.menu.contains(event.target)) return
      this.close()
    }
  }
}
</script>

<style scoped>
.liquid-palette-picker__trigger {
  display: grid;
  place-items: center;
  width: 38px;
  height: 38px;
  padding: 0;
  border: 1px solid var(--rim);
  border-radius: 50%;
  background: var(--glass-input);
  box-shadow: inset 0 1px 0 var(--spec), var(--shadow-soft);
  backdrop-filter: var(--ui-backdrop-control);
}
.liquid-palette-picker__swatch,
.liquid-palette-picker__menu button > span {
  width: 18px;
  height: 18px;
  border: 2px solid var(--spec);
  border-radius: 50%;
  box-shadow: 0 2px 7px color-mix(in srgb, var(--accent) 28%, transparent);
}
[data-palette-swatch='blue'] { background: var(--ui-palette-blue); }
[data-palette-swatch='violet'] { background: var(--ui-palette-violet); }
[data-palette-swatch='emerald'] { background: var(--ui-palette-emerald); }
[data-palette-swatch='amber'] { background: var(--ui-palette-amber); }
.liquid-palette-picker__menu {
  position: fixed;
  z-index: 5000;
  width: 190px;
  margin: 0;
  padding: 8px;
  border: 1px solid var(--rim);
  border-radius: var(--r-md);
  color: var(--ink);
  background: linear-gradient(150deg, var(--spec-soft), transparent 46%),
    var(--glass-popover);
  box-shadow: var(--shadow), inset 0 1px 0 var(--spec);
  backdrop-filter: var(--ui-backdrop-surface);
}
.liquid-palette-picker__menu > .ui-field-label {
  display: block;
  padding: 7px 10px;
}
.liquid-palette-picker__menu > .ui-supporting-text {
  display: block;
  padding: 0 10px 9px;
}
.liquid-palette-picker__menu button {
  display: grid;
  grid-template-columns: 24px 1fr 18px;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-height: 40px;
  padding: 6px 10px;
  text-align: left;
}
.liquid-palette-picker__menu.is-inline {
  position: static;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px;
  width: 100%;
  padding: 0;
  border: 0;
  background: transparent;
  box-shadow: none;
  backdrop-filter: none;
}
.liquid-palette-picker__menu.is-inline > .ui-field-label,
.liquid-palette-picker__menu.is-inline > .ui-supporting-text {
  grid-column: 1 / -1;
}
@media (max-width: 520px) {
  .liquid-palette-picker__menu.is-inline {
    grid-template-columns: 1fr;
  }
}
</style>
