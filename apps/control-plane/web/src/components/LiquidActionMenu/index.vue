<template>
  <div class="row-actions">
    <liquid-button
      ref="trigger"
      class="liquid-action-menu__trigger"
      icon="arrow-down"
      :title="$t('table.actions')"
      :aria-label="label"
      :aria-controls="menuId"
      :aria-expanded="String(open)"
      aria-haspopup="menu"
      @click="toggleMenu"
      @keydown.native.down.prevent="openMenu(false)"
      @keydown.native.up.prevent="openMenu(true)"
    />
    <div
      :id="menuId"
      ref="menu"
      class="liquid-action-menu__popup liquid-menu-surface"
      popover="auto"
      role="menu"
      :aria-label="label"
      @toggle="handleNativeToggle"
      @click.capture="handleSelection"
      @keydown="handleMenuKeydown"
    >
      <slot />
    </div>
  </div>
</template>

<script>
export default {
  name: 'LiquidActionMenu',
  props: {
    label: { type: String, required: true }
  },
  data: () => ({ open: false }),
  computed: {
    menuId() { return `liquid-action-menu-${this._uid}` }
  },
  beforeDestroy() {
    this.closeMenu()
  },
  methods: {
    menuItems() {
      return Array.from(this.$refs.menu.querySelectorAll('[role="menuitem"]:not(:disabled)'))
    },
    toggleMenu() {
      if (this.open) this.closeMenu(true)
      else this.openMenu(false)
    },
    openMenu(last) {
      if (this.open) return
      const { menu, trigger } = this.$refs
      this.open = true
      menu.showPopover({ source: trigger.$el })
      this.updatePosition()
      if (!this.open) return
      window.addEventListener('scroll', this.updatePosition, true)
      window.addEventListener('resize', this.updatePosition)
      const items = this.menuItems()
      items[last ? items.length - 1 : 0]?.focus({ preventScroll: true })
    },
    closeMenu(restoreFocus = false) {
      if (!this.open) return
      this.$refs.menu.hidePopover()
      this.stopTracking()
      if (restoreFocus) this.$refs.trigger.$el.focus({ preventScroll: true })
    },
    stopTracking() {
      this.open = false
      window.removeEventListener('scroll', this.updatePosition, true)
      window.removeEventListener('resize', this.updatePosition)
    },
    handleNativeToggle() {
      if (!this.$refs.menu.matches(':popover-open')) this.stopTracking()
    },
    updatePosition() {
      const { menu } = this.$refs
      const trigger = this.$refs.trigger.$el
      const anchor = trigger.getBoundingClientRect()
      const scrollArea = trigger.closest('.tbl-wrap')?.getBoundingClientRect()
      const { clientWidth, clientHeight } = document.documentElement
      if (anchor.bottom <= Math.max(0, scrollArea?.top ?? 0) ||
          anchor.top >= Math.min(clientHeight, scrollArea?.bottom ?? clientHeight)) {
        this.closeMenu()
        return
      }
      const margin = 12
      const gap = 6
      const below = clientHeight - anchor.bottom - gap - margin
      const above = anchor.top - gap - margin
      const openAbove = below < menu.scrollHeight && above > below
      const width = menu.offsetWidth
      menu.style.left = `${Math.max(margin, Math.min(anchor.right - width, clientWidth - width - margin))}px`
      menu.style.top = openAbove ? 'auto' : `${anchor.bottom + gap}px`
      menu.style.bottom = openAbove ? `${clientHeight - anchor.top + gap}px` : 'auto'
      menu.style.maxHeight = `${openAbove ? above : below}px`
    },
    handleSelection(event) {
      if (event.target.closest('[role="menuitem"]:not(:disabled)')) this.closeMenu(true)
    },
    handleMenuKeydown(event) {
      if (event.key === 'Escape' || event.key === 'Tab') {
        if (event.key === 'Escape') {
          event.preventDefault()
          event.stopPropagation()
        }
        this.closeMenu(true)
        return
      }

      if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
      const items = this.menuItems()
      if (!items.length) return
      event.preventDefault()
      const index = items.indexOf(event.target)
      const step = event.key === 'ArrowDown' ? 1 : -1
      const next = { Home: 0, End: items.length - 1 }[event.key] ??
        (index + step + items.length) % items.length
      items[next].focus()
    }
  }
}
</script>

<style scoped>
.liquid-action-menu__trigger ::v-deep .app-icon {
  transition: transform var(--ui-motion-fast) var(--ui-motion-easing-standard);
}

.liquid-action-menu__trigger[aria-expanded='true'] {
  color: var(--accent);
  background: var(--glass-strong);
}

.liquid-action-menu__trigger[aria-expanded='true'] ::v-deep .app-icon {
  transform: rotate(180deg);
}

.liquid-action-menu__popup {
  position: fixed;
  inset: auto;
  min-width: 160px;
  max-width: calc(100vw - 24px);
  overflow: auto;
  overscroll-behavior: contain;
}

.liquid-action-menu__popup ::v-deep [role='menuitem'] {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-height: 38px;
  padding: 8px 11px;
  text-align: left;
}

.liquid-action-menu__popup ::v-deep [role='menuitem'].danger {
  color: var(--bad-fg);
}
</style>
