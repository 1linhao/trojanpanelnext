import test from 'node:test'
import assert from 'node:assert/strict'
import {
  MATERIAL_CUSTOM_PROPERTIES,
  UI_CUSTOM_PROPERTIES,
  createShellModel,
  createUiRuntime,
  isMotionRole,
  isSurface,
  validateMotionRole,
  validateSurface
} from '../../src/index.js'

test('validates semantic enum values', () => {
  assert.equal(validateSurface('panel'), 'panel')
  assert.throws(
    () => validateSurface('business-card'),
    /surface must be one of/
  )
  assert.equal(validateMotionRole('shared'), 'shared')
  assert.throws(
    () => validateMotionRole('bounce'),
    /motion role must be one of/
  )
  assert.equal(isSurface('panel'), true)
  assert.equal(isSurface('business-card'), false)
  assert.equal(isMotionRole('shared'), true)
  assert.equal(isMotionRole('bounce'), false)
})

test('normalizes, freezes, and validates shell models', () => {
  const model = createShellModel({
    activeKey: '/home',
    groups: [
      { key: 'main', label: 'Main', items: [{ key: '/home', label: 'Home' }] }
    ]
  })
  assert.ok(Object.isFrozen(model))
  assert.ok(Object.isFrozen(model.groups[0].items[0]))
  assert.throws(
    () =>
      createShellModel({
        groups: [
          { key: 'a', items: [{ key: 'same' }] },
          { key: 'b', items: [{ key: 'same' }] }
        ]
      }),
    /duplicate shell item key/
  )
})

test('runtime composes theme and material without owning DOM', () => {
  const applications = []
  let systemListener
  let motionListener
  const runtime = createUiRuntime({
    material: {
      apply: (theme) => applications.push(theme),
      getCapabilities: () => ({ backdropFilter: true })
    },
    initialTheme: { mode: 'system', palette: 'violet' },
    initialMotion: { mode: 'system' },
    motion: {
      apply: (motion) => applications.push(motion),
      getCapabilities: () => ({ available: true, reducedMotion: true })
    },
    environment: {
      getSystemMode: () => 'dark',
      getReducedMotion: () => true,
      subscribeSystemMode: (listener) => {
        systemListener = listener
      },
      subscribeReducedMotion: (listener) => {
        motionListener = listener
      }
    }
  })
  assert.equal(runtime.theme.getState().resolvedMode, 'dark')
  assert.equal(runtime.theme.setPalette('amber').palette, 'amber')
  assert.equal(applications.length, 3)
  assert.equal(runtime.material.getCapabilities().backdropFilter, true)
  assert.equal(runtime.motion.getState().resolvedMode, 'reduced')
  systemListener('light')
  assert.equal(runtime.theme.getState().resolvedMode, 'light')
  motionListener(false)
  assert.equal(runtime.motion.getState().resolvedMode, 'full')
  assert.equal(runtime.motion.setMode('none').resolvedMode, 'none')
  assert.equal(runtime.motion.getCapabilities().available, true)
  assert.ok(UI_CUSTOM_PROPERTIES.includes('--ui-surface-bg'))
})

test('custom-property contract has unique, well-formed names for every public seam', () => {
  assert.equal(new Set(UI_CUSTOM_PROPERTIES).size, UI_CUSTOM_PROPERTIES.length)
  UI_CUSTOM_PROPERTIES.forEach((property) =>
    assert.match(property, /^--ui-[a-z0-9-]+$/)
  )
  for (const property of [
    '--ui-dialog-bg',
    '--ui-overlay-backdrop-bg',
    '--ui-button-interaction-shadow',
    '--ui-control-size-padding-x',
    '--ui-navigation-mobile-selected-bg',
    '--ui-navigation-mobile-selected-shadow',
    '--ui-view-transition-name',
    '--ui-motion-easing-emphasized',
    '--ui-motion-scroll-behavior'
  ])
    assert.ok(UI_CUSTOM_PROPERTIES.includes(property), property)
  assert.equal(
    new Set(MATERIAL_CUSTOM_PROPERTIES).size,
    MATERIAL_CUSTOM_PROPERTIES.length
  )
  MATERIAL_CUSTOM_PROPERTIES.forEach((property) =>
    assert.ok(UI_CUSTOM_PROPERTIES.includes(property), property)
  )
})
