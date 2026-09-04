import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { createNativeMotion } from '../../src/index.js'

test('motion controller applies only a stable semantic mode', () => {
  const attributes = {}
  const controller = createNativeMotion({
    root: {
      setAttribute: (name, value) => {
        attributes[name] = value
      }
    }
  })
  controller.apply({ mode: 'system', resolvedMode: 'reduced' })
  assert.equal(attributes['data-ui-motion'], 'reduced')
  assert.equal(controller.getCapabilities().available, true)
})

test('motion modes publish a semantic scrolling behavior', async () => {
  const css = await readFile(
    new URL('../../src/motion.css', import.meta.url),
    'utf8'
  )
  assert.match(css, /--ui-motion-scroll-behavior: smooth/)
  assert.match(
    css,
    /data-ui-motion='reduced'[\s\S]*--ui-motion-scroll-behavior: auto/
  )
  assert.match(
    css,
    /data-ui-motion='none'[\s\S]*--ui-motion-scroll-behavior: auto/
  )
})
