import { scrollElementTo } from '@tp-ui/motion-native'

// Keep the application API; timing and reduced-motion live in the adapter.
export function scrollTo(to, duration, callback) {
  return scrollElementTo(document.scrollingElement || document.documentElement, to, {
    duration,
    onComplete: callback
  })
}
