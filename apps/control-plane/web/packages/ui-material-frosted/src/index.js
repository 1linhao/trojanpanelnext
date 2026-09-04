/* eslint-env browser, es2021 */

import { PALETTES } from '@tp-ui/contracts'

const THEME_COLORS = Object.freeze({
  light: Object.freeze({
    blue: '#0a7cff',
    violet: '#8155e7',
    emerald: '#078b6c',
    amber: '#cf7100'
  }),
  dark: Object.freeze({
    blue: '#3f9bff',
    violet: '#a98bff',
    emerald: '#45d4ad',
    amber: '#ffad42'
  })
})

function updateBrowserChrome(documentTarget, state) {
  if (!documentTarget) return
  let themeColor = documentTarget.querySelector?.('meta[name="theme-color"]')
  if (!themeColor) {
    themeColor = documentTarget.createElement?.('meta')
    themeColor?.setAttribute('name', 'theme-color')
    documentTarget.head?.appendChild(themeColor)
  }
  themeColor?.setAttribute(
    'content',
    THEME_COLORS[state.resolvedMode][state.palette]
  )
  let colorScheme = documentTarget.querySelector?.('meta[name="color-scheme"]')
  if (!colorScheme) {
    colorScheme = documentTarget.createElement?.('meta')
    colorScheme?.setAttribute('name', 'color-scheme')
    documentTarget.head?.appendChild(colorScheme)
  }
  colorScheme?.setAttribute('content', state.resolvedMode)
}

export function createFrostedMaterial({ root, document: documentTarget } = {}) {
  const target = root || globalThis.document?.documentElement
  const ownerDocument = documentTarget || globalThis.document
  return Object.freeze({
    apply(state) {
      if (target) {
        target.setAttribute('data-ui-material', 'frosted')
        target.setAttribute('data-theme', state.resolvedMode)
        target.setAttribute('data-palette', state.palette)
      }
      updateBrowserChrome(ownerDocument, state)
      return state
    },
    getCapabilities() {
      const supports = globalThis.CSS?.supports?.bind(globalThis.CSS)
      return Object.freeze({
        backdropFilter: Boolean(
          supports?.('backdrop-filter', 'blur(1px)') ||
            supports?.('-webkit-backdrop-filter', 'blur(1px)')
        ),
        colorScheme: true,
        palettes: PALETTES,
        modes: Object.freeze(['light', 'dark'])
      })
    }
  })
}
