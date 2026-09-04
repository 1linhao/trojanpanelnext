import { PALETTES } from '@tp-ui/contracts'

const PALETTE_KEY = 'trojan-panel-color-palette'
export { PALETTES as COLOR_PALETTES }
let runtime

function normalizePalette(palette) {
  return PALETTES.includes(palette) ? palette : 'blue'
}

function notifyThemeChange(theme) {
  if (typeof window.CustomEvent !== 'function') return
  window.dispatchEvent(
    new CustomEvent('trojan-theme-change', { detail: theme })
  )
}

export function getThemeState() {
  if (runtime) {
    const state = runtime.theme.getState()
    return { mode: state.resolvedMode, palette: state.palette }
  }
  return {
    mode:
      document.documentElement.getAttribute('data-theme') === 'dark'
        ? 'dark'
        : 'light',
    palette: normalizePalette(
      document.documentElement.getAttribute('data-palette')
    )
  }
}

export function applyTheme({ mode, palette } = {}) {
  if (runtime) {
    if (mode) runtime.theme.setMode(mode)
    if (palette) runtime.theme.setPalette(normalizePalette(palette))
    return getThemeState()
  }
  const current = getThemeState()
  const next = {
    mode: mode === 'dark' || mode === 'light' ? mode : current.mode,
    palette: normalizePalette(palette || current.palette)
  }
  document.documentElement.setAttribute('data-theme', next.mode)
  document.documentElement.setAttribute('data-palette', next.palette)
  notifyThemeChange(next)
  return next
}

export function getInitialTheme() {
  const runtimeTheme = document.documentElement.getAttribute('data-theme')
  if (runtimeTheme === 'dark' || runtimeTheme === 'light') return runtimeTheme
  return window.matchMedia &&
    window.matchMedia('(prefers-color-scheme: dark)').matches
    ? 'dark'
    : 'light'
}

export function bindUiRuntime(uiRuntime) {
  runtime = uiRuntime
  const publish = (state) =>
    notifyThemeChange({ mode: state.resolvedMode, palette: state.palette })
  runtime.theme.subscribe(publish)
  publish(runtime.theme.getState())
}

export function toggleTheme() {
  const current = getThemeState()
  return applyTheme({ mode: current.mode === 'dark' ? 'light' : 'dark' })
}

export function applyPalette(palette) {
  const next = applyTheme({ palette })
  localStorage.setItem(PALETTE_KEY, next.palette)
  return next
}
