import { createButtonInteractionController } from '@tp-ui/components-vue2'

let releaseRuntime = null

/**
 * Production composition Adapter. Product components know only the stable
 * data-ui-interaction Interface; animation engines are selected here.
 */
export function installUiInteractions({ root = document } = {}) {
  releaseRuntime?.()
  const buttons = createButtonInteractionController({ root }).mount()

  releaseRuntime = () => {
    buttons.destroy()
    releaseRuntime = null
  }
  return releaseRuntime
}
