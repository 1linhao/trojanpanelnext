import test from 'node:test'
import assert from 'node:assert/strict'
import { UiAppShell, createAppShell } from '../../src/index.js'

test('shell plugin registers only the public shell', () => {
  const names = []
  createAppShell().install({ component: (name) => names.push(name) })
  assert.deepEqual(names, ['UiAppShell'])
})

test('shell emits navigation intent without router knowledge', () => {
  const emitted = []
  const component = {
    model: {
      brand: { mark: 'T', name: 'Test', subtitle: 'Lab' },
      activeKey: '/home',
      pageTitle: 'Home',
      user: null,
      groups: [
        {
          key: 'main',
          label: 'Main',
          items: [
            { key: '/home', label: 'Home', mobileLabel: 'Home', icon: '' }
          ]
        }
      ]
    },
    $slots: {},
    $scopedSlots: {},
    labels: {
      navigation: 'Primary navigation',
      profile: 'Profile',
      logout: 'Log out'
    },
    showUser: true,
    $emit: (...args) => emitted.push(args)
  }
  const h = (tag, data, children) => ({
    tag,
    data: data || {},
    children: children || []
  })
  const vnode = UiAppShell.render.call(component, h)
  const aside = vnode.children[0]
  const nav = aside.children.find((child) =>
    child?.data?.class?.includes?.('tp-ui-shell__nav-item')
  )
  nav.data.on.click()
  assert.equal(emitted[0][0], 'navigate')
  assert.equal(emitted[0][1], '/home')
})

test('shell accessibility labels and business content are injectable', () => {
  const component = {
    model: { brand: {}, activeKey: '', pageTitle: '', user: null, groups: [] },
    labels: { navigation: '主导航', profile: '个人资料', logout: '退出' },
    showUser: true,
    $slots: {},
    $scopedSlots: { brand: () => ['brand'], user: () => ['user'] },
    $emit() {}
  }
  const h = (tag, data, children) => ({
    tag,
    data: data || {},
    children: children || []
  })
  const vnode = UiAppShell.render.call(component, h)
  assert.equal(vnode.children[2].data.attrs['aria-label'], '主导航')
  assert.deepEqual(vnode.children[0].children[0].children, ['brand'])
})

test('shell marks overflowing mobile navigation and exposes a stable icon part', () => {
  const items = Array.from({ length: 6 }, (_, index) => ({
    key: `/item-${index}`,
    label: `Item ${index}`,
    mobileLabel: `I${index}`,
    icon: `icon-${index}`
  }))
  const component = {
    model: {
      brand: {},
      activeKey: '/item-0',
      pageTitle: '',
      user: null,
      groups: [{ key: 'main', label: 'Main', items }]
    },
    labels: { navigation: '主导航', profile: '个人资料', logout: '退出' },
    showUser: false,
    $slots: {},
    $scopedSlots: { icon: ({ name }) => [name] },
    $emit() {}
  }
  const h = (tag, data, children) => ({
    tag,
    data: data || {},
    children: children || []
  })
  const vnode = UiAppShell.render.call(component, h)
  assert.equal(vnode.data.attrs['data-ui-component'], 'app-shell')
  const mobileNav = vnode.children[2]
  assert.equal(mobileNav.data.class[1]['is-scrollable'], true)
  assert.equal(
    mobileNav.children[0].children[0].data.class,
    'tp-ui-shell__nav-icon'
  )
})

test('shell delegates smooth scrolling to the public motion token', async () => {
  let options
  const nav = {
    scrollWidth: 500,
    clientWidth: 200,
    querySelector: () => ({ offsetLeft: 300, offsetWidth: 60 }),
    scrollTo: (value) => {
      options = value
    }
  }
  UiAppShell.methods.revealActiveMobileItem.call({ $refs: { mobileNav: nav } })
  assert.deepEqual(options, { left: 230 })
  assert.equal('behavior' in options, false)

  const [source, css] = await Promise.all([
    import('node:fs/promises').then(({ readFile }) =>
      readFile(new URL('../../src/index.js', import.meta.url), 'utf8')
    ),
    import('node:fs/promises').then(({ readFile }) =>
      readFile(new URL('../../src/layout.css', import.meta.url), 'utf8')
    )
  ])
  assert.doesNotMatch(source, /behavior:\s*['"]smooth['"]/)
  assert.match(css, /scroll-behavior: var\(--ui-motion-scroll-behavior, auto\)/)
  assert.match(css, /box-shadow: var\(--ui-navigation-mobile-selected-shadow\)/)
})

test('shell source has no business framework imports', async () => {
  const source = await import('node:fs/promises').then(({ readFile }) =>
    readFile(new URL('../../src/index.js', import.meta.url), 'utf8')
  )
  assert.doesNotMatch(source, /vuex|vue-router|router|store|token|roles/i)
})
