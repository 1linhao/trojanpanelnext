import test from 'node:test'
import assert from 'node:assert/strict'
import { iconNames, renderIcon } from '../../src/index.js'

test('icons use stable semantic names and currentColor', () => {
  const h = (tag, data, children) => ({ tag, data, children })
  const icon = renderIcon(h, 'home')
  assert.ok(iconNames.includes('home'))
  assert.equal(icon.data.attrs.stroke, 'currentColor')
  assert.throws(() => renderIcon(h, 'missing'), /Unknown icon/)
  assert.throws(() => renderIcon(h, 'toString'), /Unknown icon/)
})

test('every icon follows the navigation outline contract', () => {
  const h = (tag, data, children) => ({ tag, data, children })
  for (const name of iconNames) {
    const icon = renderIcon(h, name)
    assert.equal(icon.tag, 'svg', name)
    assert.equal(icon.data.attrs.viewBox, '0 0 24 24', name)
    assert.equal(icon.data.attrs.fill, 'none', name)
    assert.equal(icon.data.attrs.stroke, 'currentColor', name)
    assert.equal(icon.data.attrs['stroke-width'], '2.35', name)
    assert.equal(icon.data.attrs['stroke-linecap'], 'round', name)
    assert.equal(icon.data.attrs['stroke-linejoin'], 'round', name)
    assert.equal(icon.data.attrs['aria-hidden'], 'true', name)
    assert.ok(icon.children.length, name)
    for (const shape of icon.children) {
      assert.ok(['path', 'rect', 'circle'].includes(shape.tag), name)
      assert.equal(shape.data.attrs.fill, undefined, name)
      assert.equal(shape.data.attrs.stroke, undefined, name)
    }
  }
  assert.equal(renderIcon(h, 'dashboard').children.length, 4)
})

test('renderer preserves Vue bindings and semantic loading class', () => {
  const click = () => {}
  const h = (tag, data, children) => ({ tag, data, children })
  const icon = renderIcon(h, 'loading', { width: '24' }, {
    class: { 'is-open': true }, staticClass: 'control-icon',
    attrs: { title: 'loading' }, style: { opacity: 0.5 }, on: { click }
  })
  assert.deepEqual(icon.data.class, ['app-icon', 'app-icon--loading', { 'is-open': true }])
  assert.equal(icon.data.staticClass, 'control-icon')
  assert.equal(icon.data.attrs.title, 'loading')
  assert.equal(icon.data.attrs.width, '24')
  assert.equal(icon.data.style.opacity, 0.5)
  assert.equal(icon.data.on.click, click)
})
