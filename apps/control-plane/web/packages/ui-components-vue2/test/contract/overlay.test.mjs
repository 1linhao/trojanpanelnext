import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { acquireOverlay } from '../../src/overlay-stack.js'
import { UiDialog } from '../../src/index.js'

function fixture() {
  const listeners = {}
  const doc = {
    body: { style: { overflow: 'auto' } },
    addEventListener: (name, handler) => { listeners[name] = handler },
    removeEventListener: (name) => { delete listeners[name] }
  }
  const node = (children = []) => {
    const element = {
      ownerDocument: doc, children, isConnected: true, tabIndex: 0,
      contains(target) { return target === this || children.some((child) => child.contains(target)) },
      querySelectorAll: () => children,
      getClientRects: () => [{}],
      closest: () => null,
      focus() { doc.activeElement = this; listeners.focusin?.({ target: this }) }
    }
    return element
  }
  const key = (key, shiftKey = false, prevented = false) => {
    const event = { key, shiftKey, defaultPrevented: prevented, preventDefault() { this.defaultPrevented = true } }
    listeners.keydown?.(event)
    return event
  }
  return { doc, node, key, listeners }
}

test('nested overlays own Escape, trap Tab, lock scroll, and restore focus in order', () => {
  const { doc, node, key, listeners } = fixture()
  const launch = node(), first = node(), last = node(), dialog = node([first, last])
  launch.focus()
  let outerClosed = 0, innerClosed = 0
  const release = acquireOverlay(dialog, { close: () => outerClosed++ })
  assert.equal(doc.body.style.overflow, 'hidden')
  dialog.focus()
  key('Tab')
  assert.equal(doc.activeElement, first)
  key('Tab')
  assert.equal(doc.activeElement, last, 'forward Tab must advance through real dialog controls')
  key('Tab', true)
  assert.equal(doc.activeElement, first)
  key('Tab', true)
  assert.equal(doc.activeElement, last)
  key('Tab')
  assert.equal(doc.activeElement, first)
  const inner = node()
  const releaseInner = acquireOverlay(inner, { close: () => innerClosed++ })
  inner.focus()
  key('Escape')
  assert.equal(innerClosed, 1)
  assert.equal(outerClosed, 0)
  key('Escape', false, true)
  assert.equal(innerClosed, 1)
  releaseInner()
  assert.equal(doc.activeElement, first)
  assert.equal(doc.body.style.overflow, 'hidden')
  release()
  release()
  assert.equal(doc.activeElement, launch)
  assert.equal(doc.body.style.overflow, 'auto')
  assert.deepEqual(listeners, {})
})

test('destroying a lower overlay does not steal focus or lose the external return target', () => {
  const { doc, node } = fixture()
  const launch = node(), first = node(), outer = node([first]), inner = node()
  launch.focus()
  const releaseOuter = acquireOverlay(outer)
  first.focus()
  const releaseInner = acquireOverlay(inner)
  inner.focus()
  releaseOuter()
  assert.equal(doc.activeElement, inner)
  releaseInner()
  assert.equal(doc.activeElement, launch)
  assert.equal(doc.body.style.overflow, 'auto')
})

test('Escape opt-out and focus escaping the modal are respected', () => {
  const { doc, node, key } = fixture()
  const outside = node(), dialog = node()
  outside.focus()
  const release = acquireOverlay(dialog, { closeOnEscape: false, close: () => assert.fail('closed') })
  outside.focus()
  assert.equal(doc.activeElement, dialog)
  assert.equal(key('Escape').defaultPrevented, false)
  assert.equal(key('Tab').defaultPrevented, true)
  release()
})

test('dialog teardown removes a body portal even when parent is destroyed while visible', () => {
  const calls = []
  UiDialog.beforeDestroy.call({
    appendToBody: true,
    $el: { nodeType: 1, remove: () => calls.push('remove') },
    restoreFocus: () => calls.push('release')
  })
  assert.deepEqual(calls, ['release', 'remove'])
  UiDialog.beforeDestroy.call({
    appendToBody: false,
    $el: { nodeType: 1, remove: () => assert.fail('non-portal') },
    restoreFocus() {}
  })
})

test('opening then hiding before nextTick cannot register an invisible overlay', () => {
  let tick
  const instance = { visible: true, $nextTick: (callback) => { tick = callback } }
  UiDialog.methods.open.call(instance)
  instance.visible = false
  tick()
  assert.equal(instance._releaseOverlay, undefined)
})

test('real Chromium DOM advances forward from the mobile dialog close button', (t) => {
  const chromium = ['/usr/bin/chromium', '/usr/bin/chromium-browser'].find(fs.existsSync)
  if (!chromium) return t.skip('Chromium is not available')
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'tp-overlay-dom-'))
  try {
    const here = path.dirname(fileURLToPath(import.meta.url))
    fs.copyFileSync(path.resolve(here, '../../src/overlay-stack.js'), path.join(directory, 'overlay-stack.js'))
    const fields = Array.from({ length: 12 }, (_, index) => `<input id="field-${index + 1}">`).join('')
    fs.writeFileSync(path.join(directory, 'index.html'), `<!doctype html><body>
      <section id="dialog" tabindex="-1"><button id="close">关闭</button>${fields}</section>
      <script type="module">
        import { acquireOverlay } from './overlay-stack.js'
        const dialog = document.querySelector('#dialog')
        acquireOverlay(dialog)
        document.querySelector('#close').focus()
        const visited = []
        for (let index = 0; index < 12; index++) {
          document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true }))
          visited.push(document.activeElement.id)
        }
        document.body.dataset.visited = visited.join(',')
      </script></body>`)
    const result = spawnSync(chromium, [
      '--headless', '--no-sandbox', '--disable-gpu', '--allow-file-access-from-files',
      '--virtual-time-budget=1000', '--dump-dom', `file://${path.join(directory, 'index.html')}`
    ], { encoding: 'utf8' })
    assert.equal(result.status, 0, result.stderr)
    assert.match(result.stdout, /data-visited="field-1,field-2,field-3,field-4,field-5,field-6,field-7,field-8,field-9,field-10,field-11,field-12"/)
  } finally {
    fs.rmSync(directory, { recursive: true, force: true })
  }
})
