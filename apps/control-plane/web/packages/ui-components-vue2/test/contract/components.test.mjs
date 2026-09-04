import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import {
  COMPONENTS,
  BUTTON_INTERACTION,
  UiButton,
  UiDialog,
  UiInput,
  UiPanel,
  UiSheet,
  createButtonInteractionController,
  createCssButtonInteractionAdapter,
  createVue2Components
} from '../../src/index.js'

test('exports named components and selective plugin registration', () => {
  const names = []
  createVue2Components({ include: ['UiButton'] }).install({
    component: (name) => names.push(name)
  })
  assert.deepEqual(names, ['UiButton'])
  assert.deepEqual(Object.keys(COMPONENTS), [
    'UiButton',
    'UiInput',
    'UiPanel',
    'UiSheet',
    'UiDialog'
  ])
  assert.throws(
    () => createVue2Components({ include: ['Missing'] }),
    /Unknown UI components/
  )
})

test('selective plugin defaults to an empty registration set', () => {
  const names = []
  createVue2Components().install({ component: (name) => names.push(name) })
  assert.deepEqual(names, [])
})

test('Vue prop validators return booleans for valid and invalid public values', () => {
  for (const [component, props] of [
    [UiPanel, ['tone', 'density', 'state', 'motionRole']],
    [UiDialog, ['tone', 'motionRole']],
    [UiButton, ['tone', 'surface', 'size']],
    [UiInput, ['tone', 'size']]
  ]) {
    for (const prop of props) {
      assert.equal(
        component.props[prop].validator(component.props[prop].default),
        true
      )
      assert.equal(
        component.props[prop].validator('not-a-contract-value'),
        false
      )
    }
  }
})

test('panel variants expose stable anatomy and motion identity', () => {
  const vnode = UiPanel.render(
    (tag, data, children) => ({ tag, data, children }),
    {
      props: {
        variant: 'auth',
        tag: 'section',
        tone: 'neutral',
        density: 'comfortable',
        state: 'idle',
        motionRole: 'shared',
        motionKey: 'auth-primary'
      },
      data: { attrs: {}, class: 'consumer-class' },
      slots: () => ({ default: ['form'], header: ['brand'] })
    }
  )
  assert.equal(vnode.tag, 'section')
  assert.equal(vnode.data.attrs['data-ui-panel-variant'], 'auth')
  assert.equal(vnode.data.attrs['data-ui-component'], 'panel')
  assert.equal(vnode.data.attrs['data-ui-motion-role'], 'shared')
  assert.equal(vnode.data.attrs['data-ui-motion-key'], 'auth-primary')
  assert.deepEqual(vnode.data.style[1], {
    '--ui-view-transition-name': 'ui-auth-primary'
  })
  assert.equal(vnode.data.style[1].viewTransitionName, undefined)
  assert.equal(vnode.children[0].data.attrs['data-ui-part'], 'header')
  assert.equal(vnode.children[1].data.attrs['data-ui-part'], 'body')
})

test('sheet uses the raised surface recipe', () => {
  const vnode = UiSheet.render(
    (tag, data, children) => ({ tag, data, children }),
    {
      props: {
        tag: 'aside',
        tone: 'neutral',
        density: 'comfortable',
        state: 'idle',
        motionRole: 'panel',
        motionKey: 'node-1'
      },
      data: { attrs: {} },
      slots: () => ({ default: ['detail'] })
    }
  )
  assert.equal(vnode.data.attrs['data-ui-surface'], 'raised')
  assert.equal(vnode.data.attrs['data-ui-component'], 'sheet')
  assert.deepEqual(vnode.data.style[1], {
    '--ui-view-transition-name': 'ui-node-1'
  })
  assert.equal(vnode.children[1].data.attrs['data-ui-part'], 'body')
})

test('capture is opt-in and mobile dialog geometry stays centered', async () => {
  const css = await readFile(
    new URL('../../src/geometry.css', import.meta.url),
    'utf8'
  )
  assert.match(css, /\[data-ui-view-transitions='active'\] \.tp-ui-panel/)
  assert.match(
    css,
    /view-transition-name: var\(--ui-view-transition-name, none\)/
  )
  const mobile = css.slice(css.indexOf('@media (max-width: 760px)'))
  assert.match(css, /\.tp-ui-dialog-layer\s*{[^}]*place-items: center;/)
  assert.doesNotMatch(mobile, /place-items: end|align-items: flex-end/)
  assert.match(mobile, /safe-area-inset-top/)
  assert.match(mobile, /safe-area-inset-bottom/)
})

test('dialog exposes overlay anatomy and close events', () => {
  const emitted = []
  const vnode = UiDialog.render.call(
    {
      visible: true,
      title: 'Edit',
      width: 480,
      customClass: '',
      showClose: true,
      labels: { close: '关闭对话框' },
      renderIcon: (h, name) => h('svg', { attrs: { 'data-icon': name } }),
      tone: 'neutral',
      motionRole: 'overlay',
      motionKey: 'edit-node',
      $slots: { default: ['form'], footer: ['save'] },
      close: () => emitted.push('close'),
      modalClick() {},
      $refs: {}
    },
    (tag, data, children) => ({ tag, data, children })
  )
  const surface = vnode.children[0]
  assert.equal(surface.data.attrs['data-ui-surface'], 'overlay')
  assert.equal(surface.data.attrs['data-ui-component'], 'dialog')
  assert.equal(surface.data.style['--ui-view-transition-name'], 'ui-edit-node')
  assert.equal(surface.data.style.viewTransitionName, undefined)
  assert.equal(surface.children[1].data.attrs['data-ui-part'], 'body')
  assert.equal(
    surface.children[0].children[1].children[0].data.attrs['data-icon'],
    'close'
  )
  assert.equal(
    surface.children[0].children[1].data.attrs['aria-label'],
    '关闭对话框'
  )
  surface.children[0].children[1].data.on.click()
  assert.deepEqual(emitted, ['close'])
})

test('composition root can supply one icon renderer without a package dependency', () => {
  const renderIcon = () => {}
  const registered = {}
  createVue2Components({
    include: ['UiDialog', 'UiPanel'],
    renderIcon
  }).install({
    component: (name, component) => {
      registered[name] = component
    }
  })
  assert.equal(registered.UiDialog.props.renderIcon.default, renderIcon)
  assert.equal(registered.UiPanel, UiPanel)
  assert.equal(UiDialog.props.renderIcon.default, null)
})

test('composition root can inject localized dialog accessibility labels', () => {
  const registered = {}
  createVue2Components({
    include: ['UiDialog'],
    dialogLabels: { close: '关闭对话框' }
  }).install({
    component: (name, component) => {
      registered[name] = component
    }
  })
  assert.deepEqual(registered.UiDialog.props.labels.default(), {
    close: '关闭对话框'
  })
  assert.deepEqual(UiDialog.props.labels.default(), { close: 'Close dialog' })
})

test('button anatomy exposes semantic attributes and loading behavior', () => {
  const emitted = []
  const vnode = UiButton.render.call(
    {
      tone: 'accent',
      surface: 'control',
      size: 'md',
      disabled: false,
      loading: true,
      type: 'button',
      $attrs: {},
      $listeners: {},
      $slots: { default: ['Save'] },
      $scopedSlots: {},
      $emit: (...args) => emitted.push(args)
    },
    (tag, data, children) => ({ tag, data, children })
  )
  assert.equal(vnode.tag, 'button')
  assert.equal(vnode.data.attrs['data-ui-state'], 'loading')
  assert.equal(vnode.data.attrs['aria-busy'], 'true')
  assert.equal(
    vnode.data.attrs[BUTTON_INTERACTION.attribute],
    BUTTON_INTERACTION.variant
  )
  vnode.data.on.click({})
  assert.equal(emitted.length, 0)
})

test('button controller connects current and dynamically added controls', () => {
  const createElement = (matches = true, descendants = []) => {
    const attributes = new Map()
    return {
      matches: () => matches,
      querySelectorAll: () => descendants,
      hasAttribute: (name) => attributes.has(name),
      setAttribute: (name, value) => attributes.set(name, value),
      removeAttribute: (name) => attributes.delete(name),
      getAttribute: (name) => attributes.get(name)
    }
  }
  const existing = createElement()
  const root = createElement(false, [existing])
  let mutationCallback
  let disconnected = false
  const controller = createButtonInteractionController({
    root,
    adapter: createCssButtonInteractionAdapter(),
    observerFactory: (callback) => {
      mutationCallback = callback
      return {
        observe() {},
        disconnect: () => {
          disconnected = true
        }
      }
    }
  })

  controller.mount()
  assert.equal(existing.getAttribute('data-ui-interaction'), 'nav-lift')

  const added = createElement()
  mutationCallback([{ addedNodes: [added], removedNodes: [] }])
  assert.equal(added.getAttribute('data-ui-interaction'), 'nav-lift')

  controller.destroy()
  assert.equal(existing.getAttribute('data-ui-interaction'), undefined)
  assert.equal(disconnected, true)
})

test('button controller preserves an explicit animation Adapter choice', () => {
  const attributes = new Map([['data-ui-interaction', 'none']])
  const control = {
    matches: () => true,
    querySelectorAll: () => [],
    hasAttribute: (name) => attributes.has(name),
    setAttribute: (name, value) => attributes.set(name, value),
    removeAttribute: (name) => attributes.delete(name)
  }
  createButtonInteractionController({
    root: control,
    observerFactory: null
  }).mount()
  assert.equal(attributes.get('data-ui-interaction'), 'none')
})

test('button controller keeps hover across the lift gap until resting bounds are left', () => {
  const attributes = new Map()
  const handlers = new Map()
  const background = { closest: () => null }
  const control = {
    matches: (selector) => selector !== ':disabled',
    closest: () => control,
    querySelectorAll: () => [],
    hasAttribute: (name) => attributes.has(name),
    setAttribute: (name, value) => attributes.set(name, value),
    removeAttribute: (name) => attributes.delete(name),
    getAttribute: (name) => attributes.get(name),
    getBoundingClientRect: () => ({
      left: 0,
      right: 100,
      top: 0,
      bottom: 40
    })
  }
  const root = {
    matches: () => false,
    querySelectorAll: () => [control],
    addEventListener: (name, handler) => handlers.set(name, handler),
    removeEventListener: (name) => handlers.delete(name)
  }
  const controller = createButtonInteractionController({
    root,
    observerFactory: null
  }).mount()

  handlers.get('pointerover')({ target: control, clientX: 50, clientY: 39 })
  assert.equal(attributes.get('data-ui-hovered'), 'true')

  handlers.get('pointermove')({ target: background, clientX: 50, clientY: 39 })
  assert.equal(
    attributes.get('data-ui-hovered'),
    'true',
    'the resting bottom edge remains hovered after the visual moves upward'
  )

  handlers.get('pointermove')({ target: background, clientX: 50, clientY: 41 })
  assert.equal(attributes.get('data-ui-hovered'), undefined)
  controller.destroy()
})

test('shared button CSS matches the navigation hover interaction contract', async () => {
  const css = await readFile(
    new URL('../../src/button-interactions.css', import.meta.url),
    'utf8'
  )
  assert.match(css, /--ui-button-hover-lift:\s*-3px/)
  assert.match(css, /@media \(hover: hover\) and \(pointer: fine\)/)
  assert.match(css, /\[data-ui-interaction='nav-lift'\].*data-ui-hovered/s)
  assert.match(css, /:not\(:disabled\).*\[aria-disabled='true'\]/s)
  assert.match(css, /transform:\s*translateY\(var\(--ui-button-hover-lift\)\)/)
  assert.match(css, /--ui-button-interaction-shadow/)
  assert.match(css, /--ui-surface-shadow, none/)
  assert.match(css, /data-ui-motion='reduced'/)
  const movesHitTargetUp =
    /--ui-button-hover-lift:\s*-\d/.test(css) &&
    /transform:\s*translateY\(var\(--ui-button-hover-lift\)\)/.test(css)
  const preservesRestingHitArea = /data-ui-hovered/.test(css)
  assert.equal(
    movesHitTargetUp && !preservesRestingHitArea,
    false,
    'hover animation must not move the bottom hit boundary above its resting position'
  )
})

test('input emits Vue 2 v-model input events', () => {
  const emitted = []
  const vnode = UiInput.render.call(
    {
      value: 'old',
      tone: 'neutral',
      size: 'md',
      disabled: false,
      invalid: false,
      $attrs: {},
      $listeners: {},
      $emit: (...args) => emitted.push(args)
    },
    (tag, data) => ({ tag, data })
  )
  vnode.data.on.input({ target: { value: 'new' } })
  assert.deepEqual(emitted, [['input', 'new']])
})
