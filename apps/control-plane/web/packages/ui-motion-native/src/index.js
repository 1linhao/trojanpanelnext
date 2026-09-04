/* eslint-env browser, es2021 */

const milliseconds = (value) => {
  const text = value.trim()
  return (parseFloat(text) || 0) * (text.endsWith('ms') ? 1 : 1000)
}
const scrolling = new WeakMap()

export function scrollElementTo(element, to, { duration, onComplete } = {}) {
  scrolling.get(element)?.()
  const doc = element.ownerDocument
  const view = doc.defaultView
  const root = doc.documentElement
  const reduced = () => {
    const mode = root.getAttribute('data-ui-motion')
    return mode === 'none' || mode === 'reduced' ||
      (!mode && Boolean(view.matchMedia?.('(prefers-reduced-motion: reduce)').matches))
  }
  const timing = duration == null
    ? milliseconds(view.getComputedStyle(root).getPropertyValue('--ui-motion-slow'))
    : Math.max(0, Number(duration) || 0)
  const from = element.scrollTop
  const started = view.performance.now()
  let frame
  const cancel = () => {
    view.cancelAnimationFrame(frame)
    if (scrolling.get(element) === cancel) scrolling.delete(element)
  }
  const tick = (now) => {
    const progress = reduced() || timing === 0 ? 1 : Math.min(1, Math.max(0, (now - started) / timing))
    const eased = progress < 0.5 ? 2 * progress * progress : 1 - Math.pow(-2 * progress + 2, 2) / 2
    element.scrollTop = from + (to - from) * eased
    if (progress < 1) frame = view.requestAnimationFrame(tick)
    else { scrolling.delete(element); onComplete?.() }
  }
  scrolling.set(element, cancel)
  tick(started)
  return cancel
}

// Read actual CSS timing; reduced/replaced adapters must not leave feedback
// waiting on a second, independently hard-coded animation duration.
export function afterTransition(element, callback) {
  const view = element.ownerDocument.defaultView
  const style = view.getComputedStyle(element)
  const parse = (list) => list.split(',').map(milliseconds)
  const durations = parse(style.transitionDuration)
  const delays = parse(style.transitionDelay)
  const duration = Math.max(0, ...durations.map((value, index) => value + delays[index % delays.length]))
  let timer
  let done = false
  const finish = () => {
    if (done) return
    done = true
    view.clearTimeout(timer)
    callback()
  }
  // Also works when a detached element never emits transitionend.
  if (duration === 0) finish()
  else timer = view.setTimeout(finish, duration)
  return () => { done = true; view.clearTimeout(timer) }
}

export function createNativeMotion({ root } = {}) {
  const target = root || globalThis.document?.documentElement
  return Object.freeze({
    apply(state) {
      if (target) target.setAttribute('data-ui-motion', state.resolvedMode)
      return state
    },
    getCapabilities() {
      return Object.freeze({
        available: Boolean(target),
        reducedMotion: Boolean(
          globalThis.matchMedia?.('(prefers-reduced-motion: reduce)').matches
        ),
        viewTransitions: Boolean(globalThis.document?.startViewTransition)
      })
    }
  })
}

export function createMotionEnvironment(scope = globalThis) {
  const media = scope.matchMedia?.('(prefers-reduced-motion: reduce)')
  return Object.freeze({
    getReducedMotion: () => Boolean(media?.matches),
    subscribeReducedMotion(listener) {
      if (!media) return () => {}
      const handler = () => listener(Boolean(media.matches))
      media.addEventListener?.('change', handler)
      return () => media.removeEventListener?.('change', handler)
    }
  })
}
