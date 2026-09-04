let activeBindings = 0
let overlay = null

function createOverlay() {
  const element = document.createElement('div')
  element.className = 'liquid-global-loading'
  element.setAttribute('aria-label', '加载中')
  element.setAttribute('role', 'status')
  element.setAttribute('popover', 'manual')
  element.innerHTML = `
    <span class="liquid-global-loading__surface">
      <svg viewBox="0 0 40 40" aria-hidden="true">
        <circle cx="20" cy="20" r="15"></circle>
      </svg>
    </span>`
  document.body.appendChild(element)
  if (element.showPopover) element.showPopover()
  return element
}

function show() {
  activeBindings += 1
  if (!overlay) overlay = createOverlay()
}

function hide() {
  activeBindings = Math.max(0, activeBindings - 1)
  if (activeBindings || !overlay) return
  if (overlay.hidePopover) {
    try {
      overlay.hidePopover()
    } catch (error) {
      // The browser may already have dismissed the popover.
    }
  }
  overlay.remove()
  overlay = null
}

function updateBinding(el, value) {
  const next = Boolean(value)
  if (next === Boolean(el.__liquidLoadingActive)) return
  el.__liquidLoadingActive = next
  if (next) show()
  else hide()
}

export default {
  bind(el, binding) {
    el.__liquidLoadingActive = false
    updateBinding(el, binding.value)
  },
  update(el, binding) {
    updateBinding(el, binding.value)
  },
  unbind(el) {
    if (el.__liquidLoadingActive) hide()
    delete el.__liquidLoadingActive
  }
}
