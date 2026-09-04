import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { MATERIAL_CUSTOM_PROPERTIES } from '@tp-ui/contracts'
import { createFlatTestMaterial } from '../../src/index.js'

test('flat material satisfies the same controller contract', () => {
  const attributes = {}
  const material = createFlatTestMaterial({
    root: {
      setAttribute: (name, value) => {
        attributes[name] = value
      }
    }
  })
  material.apply({ resolvedMode: 'light', palette: 'amber' })
  assert.equal(attributes['data-ui-material'], 'flat-test')
  assert.equal(attributes['data-palette'], 'amber')
  assert.equal(material.getCapabilities().backdropFilter, false)
})

test('flat material explicitly implements every public material property', async () => {
  const css = await readFile(
    new URL('../../src/material.css', import.meta.url),
    'utf8'
  )
  for (const property of MATERIAL_CUSTOM_PROPERTIES) {
    assert.match(css, new RegExp(`${property}\\s*:`), property)
  }
})
