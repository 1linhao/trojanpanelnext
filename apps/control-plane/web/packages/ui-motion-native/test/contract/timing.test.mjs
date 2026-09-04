import test from 'node:test'
import assert from 'node:assert/strict'
import { afterTransition, scrollElementTo } from '../../src/index.js'
import { readFile } from 'node:fs/promises'

function fixture(mode = 'full', prefersReduced = false) {
  const frames = new Map()
  let id = 0
  const root = { getAttribute: () => mode }
  const view = {
    matchMedia: () => ({ matches: prefersReduced }),
    getComputedStyle: () => ({ getPropertyValue: () => '300ms' }),
    performance: { now: () => 0 },
    requestAnimationFrame(callback) { frames.set(++id, callback); return id },
    cancelAnimationFrame(id) { frames.delete(id) }
  }
  const element = { scrollTop: 100, ownerDocument: { defaultView: view, documentElement: root } }
  return { element, frames, view, setMode: (next) => { mode = next }, tick(time) {
    const callbacks = [...frames.values()]
    frames.clear()
    callbacks.forEach((callback) => callback(time))
  } }
}

test('scrolling consumes timing tokens, replacing pending animations', () => {
  const f = fixture()
  let completed = 0
  scrollElementTo(f.element, 0, { onComplete: () => completed++ })
  f.tick(150)
  assert.equal(f.element.scrollTop, 50)
  scrollElementTo(f.element, 20)
  assert.equal(f.frames.size, 1)
  f.tick(300)
  assert.equal(f.element.scrollTop, 20)
  assert.equal(completed, 0)
  assert.equal(f.frames.size, 0)
})

test('reduced/none scrolling is immediate, full overrides the OS preference', () => {
  for (const mode of ['none', 'reduced', null]) {
    const f = fixture(mode, true)
    let completed = 0
    scrollElementTo(f.element, 0, { duration: 800, onComplete: () => completed++ })
    assert.equal(f.element.scrollTop, 0)
    assert.equal(f.frames.size, 0)
    assert.equal(completed, 1)
  }
  const f = fixture('full', true)
  scrollElementTo(f.element, 0)
  assert.equal(f.frames.size, 1)
  f.setMode('none')
  f.tick(10)
  assert.equal(f.element.scrollTop, 0)
  assert.equal(f.frames.size, 0)
})

test('feedback removal follows computed multi-property timing and zero-motion', () => {
  let timer, delay, completed = 0
  const element = { ownerDocument: { defaultView: {
    getComputedStyle: () => ({ transitionDuration: '0.1s, 180ms', transitionDelay: '20ms' }),
    setTimeout: (callback, duration) => { timer = callback; delay = duration },
    clearTimeout() {}
  } } }
  afterTransition(element, () => completed++)
  assert.equal(delay, 200)
  timer()
  timer()
  assert.equal(completed, 1)
  element.ownerDocument.defaultView.getComputedStyle = () => ({ transitionDuration: '0s', transitionDelay: '0s' })
  afterTransition(element, () => completed++)
  assert.equal(completed, 2)
})

test('CSS only uses the OS preference as a fallback before runtime mode resolution', async () => {
  const css = await readFile(new URL('../../src/motion.css', import.meta.url), 'utf8')
  assert.match(css, /@media \(prefers-reduced-motion: reduce\)\s*{\s*:root:not\(\[data-ui-motion\]\)/)
  assert.match(css, /--ui-motion-spin:/)
  assert.match(css, /--ui-motion-ambient:/)
})
