import Vue from 'vue'
import { setting } from '@/api/system'

const DEFAULT_NAME = 'Trojan Panel'
export const panelBranding = Vue.observable({
  systemName: DEFAULT_NAME,
  logoUrl: '/api/image/logo'
})

let pendingSettings = null
let nameRevision = 0
let logoRevision = 0

export function updatePanelBranding({ systemName } = {}) {
  if (typeof systemName !== 'string') return
  nameRevision += 1
  panelBranding.systemName = systemName.trim() || DEFAULT_NAME
}

// Share in-flight public settings requests; do not let a late response undo a save.
export function loadPanelSettings() {
  if (pendingSettings) return pendingSettings
  const revision = nameRevision
  pendingSettings = setting().then((response) => {
    if (revision === nameRevision) updatePanelBranding(response.data)
    return response
  }).finally(() => { pendingSettings = null })
  return pendingSettings
}

export function refreshPanelLogo() {
  logoRevision += 1
  panelBranding.logoUrl = `/api/image/logo?v=${Date.now()}-${logoRevision}`
}
