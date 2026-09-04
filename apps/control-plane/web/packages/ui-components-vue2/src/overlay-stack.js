// Shared by every dialog, including imperative confirm/prompt consumers.
// Only the top overlay owns keyboard focus; releasing a lower one cannot steal it.
const documents = new WeakMap()
const focusableSelector = 'button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), a[href], [tabindex]:not([tabindex="-1"])'

export function acquireOverlay(element, { close, closeOnEscape = true } = {}) {
  const doc = element.ownerDocument
  let state = documents.get(doc)
  if (!state) {
    state = { stack: [], overflow: doc.body.style.overflow }
    state.keydown = (event) => {
      const top = state.stack[state.stack.length - 1]
      if (!top || event.defaultPrevented) return
      if (event.key === 'Escape' && top.closeOnEscape) {
        event.preventDefault()
        top.close()
      } else if (event.key === 'Tab') {
        const targets = Array.from(top.element.querySelectorAll(focusableSelector))
          .filter((node) => node.tabIndex >= 0 && node.getClientRects().length && !node.closest('[inert]'))
        const first = targets[0] || top.element
        const last = targets[targets.length - 1] || top.element
        const current = targets.indexOf(doc.activeElement)
        const next = current < 0
          ? (event.shiftKey ? last : first)
          : targets[(current + (event.shiftKey ? -1 : 1) + targets.length) % targets.length]
        event.preventDefault()
        next.focus()
      }
    }
    state.focusin = (event) => {
      const top = state.stack[state.stack.length - 1]
      if (top && !top.element.contains(event.target)) top.element.focus()
    }
    doc.addEventListener('keydown', state.keydown)
    doc.addEventListener('focusin', state.focusin)
    doc.body.style.overflow = 'hidden'
    documents.set(doc, state)
  }
  const entry = { element, close, closeOnEscape, returnFocus: doc.activeElement }
  state.stack.push(entry)
  let released = false
  return () => {
    if (released) return
    released = true
    const index = state.stack.indexOf(entry)
    const wasTop = index === state.stack.length - 1
    // If a parent disappears first, preserve its external focus return target.
    state.stack.forEach((other) => {
      if (element.contains(other.returnFocus)) other.returnFocus = entry.returnFocus
    })
    state.stack.splice(index, 1)
    const top = state.stack[state.stack.length - 1]
    if (!top) {
      doc.removeEventListener('keydown', state.keydown)
      doc.removeEventListener('focusin', state.focusin)
      doc.body.style.overflow = state.overflow
      documents.delete(doc)
    }
    if (wasTop) {
      const target = entry.returnFocus
      if (target?.isConnected && (!top || top.element.contains(target))) target.focus()
      else top?.element.focus()
    }
  }
}
