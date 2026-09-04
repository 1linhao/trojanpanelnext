import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { MATERIAL_CUSTOM_PROPERTIES } from '@tp-ui/contracts'
import { createFrostedMaterial } from '../../src/index.js'

test('applies material through stable root attributes', () => {
  const attributes = {}
  const material = createFrostedMaterial({
    root: {
      setAttribute: (name, value) => {
        attributes[name] = value
      }
    },
    document: null
  })
  material.apply({ resolvedMode: 'dark', palette: 'violet' })
  assert.deepEqual(attributes, {
    'data-ui-material': 'frosted',
    'data-theme': 'dark',
    'data-palette': 'violet'
  })
  assert.deepEqual(material.getCapabilities().palettes, [
    'blue',
    'violet',
    'emerald',
    'amber'
  ])
})

test('frosted package explicitly implements every public material property', async () => {
  const css = (
    await Promise.all(
      ['material.css', 'production.css', 'overlay.css'].map((file) =>
        readFile(new URL(`../../src/${file}`, import.meta.url), 'utf8')
      )
    )
  ).join('\n')
  for (const property of MATERIAL_CUSTOM_PROPERTIES) {
    assert.match(css, new RegExp(`${property}\\s*:`), property)
  }
})

test('defines required material tokens, modes, palettes, surfaces, and fallback', async () => {
  const css = await readFile(
    new URL('../../src/material.css', import.meta.url),
    'utf8'
  )
  for (const token of [
    '--ui-canvas-bg',
    '--ui-surface-bg',
    '--ui-surface-border',
    '--ui-surface-shadow',
    '--ui-surface-backdrop',
    '--ui-ink',
    '--ui-accent',
    '--ui-control-bg'
  ]) {
    assert.match(css, new RegExp(token))
  }
  for (const value of [
    'dark',
    'violet',
    'emerald',
    'amber',
    'raised',
    'overlay',
    'navigation'
  ])
    assert.match(css, new RegExp(value))
  assert.match(css, /@supports not/)
  assert.doesNotMatch(
    css,
    /data-ui-material='frosted'[\s\S]*rgba\(129, 85, 231, 0\.18\)/,
    'the default blue canvas must not mix in the violet palette'
  )
  assert.doesNotMatch(css, /#app|\.prototype-|\.dashboard-|\.account-/)
})
test('overlay material is opt-in and cannot leak glass into flat mode', async () => {
  const css = await readFile(
    new URL('../../src/overlay.css', import.meta.url),
    'utf8'
  )
  const rules = [
    ...css.replace(/\/\*[\s\S]*?\*\//g, '').matchAll(/([^{}]+)\{([^{}]+)\}/g)
  ]
  assert.equal(rules.length, 2)
  for (const [, selector, declarations] of rules) {
    assert.match(selector.trim(), /^:root\[data-ui-material='frosted'\]/)
    for (const declaration of declarations
      .split(';')
      .map((part) => part.trim())
      .filter(Boolean)) {
      assert.match(declaration, /^--ui-/)
    }
  }
})

test('production dialogs reuse the global content surface transparency', async () => {
  const css = await readFile(
    new URL('../../src/production.css', import.meta.url),
    'utf8'
  )
  assert.match(css, /--ui-dialog-bg:\s*var\(--ui-surface-bg\)/)
  assert.doesNotMatch(css, /--ui-dialog-bg:\s*var\(--glass-popover\)/)
})
