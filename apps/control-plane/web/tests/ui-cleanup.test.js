const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const vm = require('node:vm')
const { transformSync } = require('@babel/core')
const compiler = require('vue/compiler-sfc')

const root = path.resolve(__dirname, '..')
const read = (file) => fs.readFileSync(path.join(root, file), 'utf8')
function loadModule(source, dependencies, globals = {}) {
  const { code } = transformSync(source, {
    babelrc: false, configFile: false,
    plugins: ['@babel/plugin-transform-modules-commonjs']
  })
  const exports = {}
  vm.runInNewContext(code, {
    ...globals,
    exports,
    require(name) {
      assert.ok(name in dependencies, `Unexpected dependency: ${name}`)
      return dependencies[name]
    }
  })
  return exports
}
const loadBranding = (setting) => loadModule(read('src/utils/panel-branding.js'), {
  vue: { observable: (value) => value }, '@/api/system': { setting }
})

test('mobile fields and search boxes retain the shared width cap', () => {
  const postcss = require('postcss')
  for (const [name, selector] of Object.entries({ LiquidInput: '.liquid-input', LiquidNumberInput: '.liquid-number-input', LiquidSelect: '.liquid-select', LiquidDatePicker: '.liquid-date-picker' })) {
    const css = compiler.parse({ source: read(`src/components/${name}/index.vue`) }).styles[0].content
    let bounded = false
    postcss.parse(css).walkRules((rule) => {
      if (rule.selector !== selector) return
      rule.walkDecls((decl) => {
        if (decl.prop === 'max-width') {
          assert.notEqual(decl.value, 'none', `${name} must not remove its width cap on mobile`)
          if (decl.value === 'var(--control-max-width)') bounded = true
        }
      })
    })
    assert.ok(bounded, `${name} must declare the shared maximum width`)
  }
  const css = postcss.parse(read('src/styles/prototype-runtime.scss'))
  let searchBounded = false, shrinkableInput = false
  css.walkRules((rule) => {
    if (rule.selector === '.search-box') rule.walkDecls('max-width', (decl) => { searchBounded ||= decl.value === 'min(100%, var(--control-max-width))' })
    if (rule.selector === '.search-box input') rule.walkDecls('min-width', (decl) => { shrinkableInput ||= decl.value === '0' })
  })
  assert.ok(searchBounded, 'search shell must cap its width even when mobile flex grows')
  assert.ok(shrinkableInput, 'search input must shrink within its shell')
})

test('form controls inherit stable labels and errors without replacing explicit accessible names', () => {
  const adapter = loadModule(read('src/mixins/liquid-form-control.js'), {}).default
  const item = { label: '服务器名称', labelId: 'label-1', errorId: 'error-1', error: '' }
  const control = { _uid: 2, liquidFormItem: item, $attrs: {} }
  let attrs = adapter.computed.controlAttrs.call(control)
  assert.equal(attrs.id, 'liquid-control-2')
  assert.equal(attrs['aria-labelledby'], 'label-1')
  item.error = '必填'
  control.$attrs = { id: 'explicit', 'aria-label': '自定义名称', 'aria-describedby': 'hint' }
  attrs = adapter.computed.controlAttrs.call(control)
  assert.equal(attrs.id, 'explicit')
  assert.equal(attrs['aria-label'], '自定义名称')
  assert.equal(attrs['aria-labelledby'], undefined)
  assert.equal(attrs['aria-describedby'], 'hint error-1')
  assert.equal(attrs['aria-invalid'], 'true')
  item.error = ''
  attrs = adapter.computed.controlAttrs.call(control)
  assert.equal(attrs['aria-describedby'], 'hint')
  assert.equal(attrs['aria-invalid'], undefined)
  control.liquidFormItem = null
  assert.equal(adapter.computed.controlAttrs.call(control).id, 'explicit')
})

test('form item labels target registered primary controls and follow conditional removal', () => {
  const { LiquidFormItem } = loadModule(read('src/components/LiquidStructural/index.js'), { vue: {} })
  const item = { controls: [] }
  const first = { controlAttrs: { id: 'first' } }, second = { controlAttrs: { id: 'second' } }
  LiquidFormItem.methods.addControl.call(item, first)
  LiquidFormItem.methods.addControl.call(item, first)
  LiquidFormItem.methods.addControl.call(item, second)
  assert.equal(item.controls.length, 2)
  assert.equal(LiquidFormItem.computed.labelTarget.call(item), 'first')
  LiquidFormItem.methods.removeControl.call(item, first)
  assert.equal(LiquidFormItem.computed.labelTarget.call(item), 'second')
  for (const name of ['LiquidInput', 'LiquidNumberInput', 'LiquidSelect', 'LiquidDatePicker', 'LiquidSwitch']) {
    assert.match(read(`src/components/${name}/index.vue`), /v-bind="controlAttrs"/)
  }
})

test('form item min/max validation follows rule type across number, string, and array values', async () => {
  const { LiquidFormItem } = loadModule(read('src/components/LiquidStructural/index.js'), { vue: {} })
  const validateValue = (value, rules) => {
    const item = {
      value, appliedRules: rules, error: '',
      liquidForm: { model: {} },
      $on: () => {}, $emit: () => {}
    }
    return LiquidFormItem.methods.validate.call(item)
  }

  // Numeric values with type 'number' must compare range, not digit count.
  assert.ok(await validateValue(5, [{ type: 'number', min: 5, max: 500 }]), '5 within 5-500 must pass')
  assert.ok(await validateValue(6, [{ type: 'number', min: 5, max: 500 }]), '6 within 5-500 must pass')
  assert.ok(await validateValue(500, [{ type: 'number', min: 5, max: 500 }]), '500 within 5-500 must pass')
  assert.ok(!(await validateValue(501, [{ type: 'number', min: 5, max: 500, message: '超范围' }])), '501 must fail')
  assert.ok(!(await validateValue(4, [{ type: 'number', min: 5, max: 500 }])), '4 below 5-500 must fail')
  assert.ok(await validateValue(-1, [{ type: 'number', min: -1, max: 1024000 }]), '-1 must pass quota range')
  assert.ok(await validateValue(1024000, [{ type: 'number', min: -1, max: 1024000 }]), 'quota upper bound must pass')

  // Non-finite numeric strings must not silently pass a number range.
  assert.ok(!(await validateValue('abc', [{ type: 'number', min: 5, max: 500 }])), 'non-numeric string must fail number range')

  // String values still validate character counts.
  assert.ok(await validateValue('ab', [{ min: 1, max: 5 }]), '2 chars within 1-5 must pass')
  assert.ok(!(await validateValue('abcdef', [{ min: 1, max: 5, message: '过长' }])), '6 chars must fail max 5')

  // Array values validate element counts.
  assert.ok(await validateValue(['a', 'b'], [{ min: 1, max: 3 }]), '2 items within 1-3 must pass')
  assert.ok(!(await validateValue(['a', 'b', 'c', 'd'], [{ min: 1, max: 3, message: '过多' }])), '4 items must fail max 3')
})

test('pagination emits controlled page/limit updates before notifying the query', async () => {
  const { transformSync } = require('@babel/core')
  const source = read('src/components/Pagination/index.vue')
  const parsed = compiler.parse({ source })
  const script = (parsed.descriptor || parsed).script.content
  const { code } = transformSync(script, {
    babelrc: false, configFile: false,
    plugins: ['@babel/plugin-transform-modules-commonjs']
  })
  const exports = {}
  vm.runInNewContext(code, {
    exports,
    require(name) { assert.equal(name, '@/utils/scroll-to'); return { scrollTo: () => {} } }
  })
  const component = exports.default

  const emitted = []
  const instance = {
    $emit: (...args) => emitted.push(args),
    autoScroll: true
  }
  // Size change: limit is persisted, the page is clamped into the new range,
  // then the query is notified with the resulting page/limit pair.
  const sizeInstance = { ...instance, currentPage: 9, pageSize: 10, total: 30 }
  component.methods.handleSizeChange.call(sizeInstance, 50)
  assert.deepEqual(emitted.map(([event]) => event), ['update:limit', 'update:page', 'pagination'])
  assert.deepEqual({ ...emitted[2][1] }, { page: 1, limit: 50 })
  emitted.length = 0
  // Page change: page is committed before the query notification.
  const pageInstance = { ...instance, currentPage: 1, pageSize: 20 }
  component.methods.handleCurrentChange.call(pageInstance, 2)
  assert.deepEqual(emitted.map(([event]) => event), ['update:page', 'pagination'])
  assert.deepEqual({ ...emitted[1][1] }, { page: 2, limit: 20 })
})

test('code editor treats JSON and YAML as equal language capabilities', async () => {
  const { transformSync } = require('@babel/core')
  const source = read('src/components/LiquidCodeEditor/index.vue')
  const parsed = compiler.parse({ source })
  const script = (parsed.descriptor || parsed).script.content
  const { code } = transformSync(script, {
    babelrc: false, configFile: false,
    plugins: ['@babel/plugin-transform-modules-commonjs']
  })
  const exports = {}
  vm.runInNewContext(code, {
    exports,
    require(name) {
      if (name === 'js-yaml') return require('js-yaml')
      assert.ok(name.endsWith('liquid-control-emitter') || name.endsWith('liquid-form-control'), `Unexpected dependency: ${name}`)
      return {}
    }
  })
  const component = exports.default

  const makeInstance = (format, text) => ({
    format, text, error: '', formatErrorPrefix: '',
    $emit: () => {}, processor: component.computed.processor.call({ format }),
    errorPrefix: component.computed.errorPrefix.call({ format, formatErrorPrefix: '' })
  })

  // JSON behavior is unchanged: legal parse, format idempotence, illegal error.
  const json = makeInstance('json', '{"a":1}')
  assert.ok(component.methods.validate.call(json))
  assert.equal(json.error, '')
  const jsonPretty = makeInstance('json', '{\n  "a": 1\n}')
  component.methods.formatContent.call(jsonPretty)
  assert.equal(jsonPretty.text, '{\n  "a": 1\n}')
  assert.equal(jsonPretty.error, '')
  const jsonBroken = makeInstance('json', '{a:1}')
  assert.ok(!component.methods.validate.call(jsonBroken))
  assert.match(jsonBroken.error, /JSON 格式错误/)

  // YAML reaches the same capabilities natively, not via JSON round-trip.
  const yamlHeader = 'port: 7890\n# 保持注释\nrules:\n  - A\n  - B\n'
  const yaml = makeInstance('yaml', yamlHeader)
  assert.ok(component.methods.validate.call(yaml))
  assert.equal(yaml.error, '')
  const yamlBroken = makeInstance('yaml', 'rules: [A, B')
  assert.ok(!component.methods.validate.call(yamlBroken))
  assert.match(yamlBroken.error, /YAML 格式错误/)
  // Format preserves YAML semantics: comments are dropped by dump but keys,
  // nesting and scalar values survive; empty content is a no-op.
  const yamlFormat = makeInstance('yaml', yamlHeader)
  component.methods.formatContent.call(yamlFormat)
  assert.equal(yamlFormat.error, '')
  assert.match(yamlFormat.text, /# 保持注释/)
  assert.match(yamlFormat.text, /port: 7890/)
  assert.match(yamlFormat.text, /rules:/)
  assert.match(yamlFormat.text, /- A/)
  const yamlAnchor = makeInstance('yaml', 'a: &x 1\nb: *x\n')
  component.methods.formatContent.call(yamlAnchor)
  assert.equal(yamlAnchor.error, '')
  assert.match(yamlAnchor.text, /&x/)
  assert.match(yamlAnchor.text, /\*x/)
  assert.deepEqual(require('js-yaml').safeLoad(yamlAnchor.text), { a: 1, b: 1 })
  const brokenOriginal = 'rules: [A, B'
  const invalidFormat = makeInstance('yaml', brokenOriginal)
  component.methods.formatContent.call(invalidFormat)
  assert.equal(invalidFormat.text, brokenOriginal, 'invalid YAML must not overwrite the draft')
  const described = component.computed.describedBy.call({
    controlAttrs: { 'aria-describedby': 'form-error' }, error: 'bad', localErrorId: 'editor-error'
  })
  assert.equal(described, 'form-error editor-error')
  // Unknown language keeps the editor inert (no button, no validation).
  const text = makeInstance('', 'anything:')
  assert.equal(text.processor, null)
  assert.ok(component.methods.validate.call(text))
})

test('import busy ownership awaits the caller promise and permits failure retry', async () => {
  const parsed = compiler.parse({ source: read('src/components/ImportTip/index.vue') })
  const component = loadModule(parsed.script.content, {
    '@/utils/liquid-feedback': { Message() {} }
  }).default
  let settle
  let calls = 0
  const instance = {
    uploading: false,
    fileList: [{ raw: { name: 'accounts.json' } }],
    importData: () => { calls++; return new Promise((resolve, reject) => { settle = { resolve, reject } }) }
  }
  const first = component.methods.submitImport.call(instance)
  assert.equal(instance.uploading, true)
  assert.equal(await component.methods.submitImport.call(instance), false)
  assert.equal(calls, 1)
  settle.reject(new Error('network'))
  await assert.rejects(first, /network/)
  assert.equal(instance.uploading, false)
  const retry = component.methods.submitImport.call(instance)
  assert.equal(calls, 2)
  settle.resolve()
  assert.equal(await retry, true)
})

test('latest list request owns stale responses, loading, and unmount invalidation', () => {
  const mixin = loadModule(read('src/mixins/latest-list-request.js'), {}).default
  const instance = { listRequestVersion: 0, listRequestActive: true, listLoading: false, listError: 'old' }
  Object.assign(instance, Object.fromEntries(Object.entries(mixin.methods).map(([key, method]) => [key, method.bind(instance)])))
  const first = instance.beginListRequest()
  const second = instance.beginListRequest()
  assert.equal(instance.ownsListRequest(first), false)
  instance.finishListRequest(first)
  assert.equal(instance.listLoading, true, 'stale request cannot clear a newer loading state')
  instance.finishListRequest(second)
  assert.equal(instance.listLoading, false)
  mixin.beforeDestroy.call(instance)
  assert.equal(instance.ownsListRequest(second), false)
})

test('native form item binds label, change/blur validation, and live error ARIA', async () => {
  const { LiquidFormItem } = loadModule(read('src/components/LiquidStructural/index.js'), { vue: {} })
  const attrs = new Map()
  const listeners = {}
  const control = {
    id: '',
    addEventListener: (name, fn) => { listeners[name] = fn },
    removeEventListener: (name) => { delete listeners[name] },
    hasAttribute: (name) => attrs.has(name),
    setAttribute: (name, value) => attrs.set(name, value),
    getAttribute: (name) => attrs.get(name) || '',
    removeAttribute: (name) => attrs.delete(name)
  }
  const triggers = []
  const item = {
    _uid: 7, label: '用户名', labelId: 'label-7', errorId: 'error-7', error: '', nativeControl: null,
    $el: { querySelector: () => control },
    validate: async (trigger) => { triggers.push(trigger); item.error = '必填'; return false }
  }
  for (const name of ['bindNativeControl', 'unbindNativeControl', 'syncNativeControlAttrs']) item[name] = LiquidFormItem.methods[name].bind(item)
  item.bindNativeControl()
  assert.equal(control.id, 'liquid-native-control-7')
  assert.equal(attrs.get('aria-labelledby'), 'label-7')
  await listeners.change()
  await listeners.blur()
  assert.deepEqual(triggers, ['change', 'blur'])
  assert.equal(attrs.get('aria-invalid'), 'true')
  assert.equal(attrs.get('aria-describedby'), 'error-7')
})

test('tabs expose real tab-panel ids and retain roving keyboard behavior', () => {
  const parsed = compiler.parse({ source: read('src/components/LiquidTabs/index.vue') })
  const component = loadModule(parsed.script.content, {}).default
  const focused = []
  const emitted = []
  const instance = {
    tabs: [{ value: 'one' }, { value: 'two' }], resolvedIdPrefix: 'settings',
    $refs: { tab: [{ focus: () => focused.push('one') }, { focus: () => focused.push('two') }] },
    $emit: (...args) => emitted.push(args), $nextTick: (fn) => fn(),
    idPart: component.methods.idPart
  }
  assert.equal(component.methods.tabId.call(instance, 'one'), 'settings-tab-one')
  assert.equal(component.methods.panelId.call(instance, 'one'), 'settings-panel-one')
  const event = { key: 'ArrowRight', preventDefault() {} }
  component.methods.handleKeydown.call(instance, event, 'one')
  assert.deepEqual(emitted[0], ['change', 'two'])
  assert.deepEqual(focused, ['two'])
})

test('date Enter and confirm share the same commit-and-close path', () => {
  const parsed = compiler.parse({ source: read('src/components/LiquidDatePicker/index.vue') })
  const component = loadModule(parsed.script.content, {
    '@/mixins/liquid-control-emitter': {}, '@/mixins/liquid-form-control': {}, '@/mixins/liquid-control-size': {}
  }, { document: {}, window: {} }).default
  let emitted, closed = 0
  const selected = new Date(2026, 8, 1, 9, 30)
  const instance = {
    resolveDraftSelection: () => selected, manualError: true, selectedDate: null,
    outputValue: () => 123, emitValue: (value) => { emitted = value }, closePopover: () => { closed++ },
    $nextTick() {}, $refs: {}
  }
  component.methods.confirmSelection.call(instance)
  assert.equal(emitted, 123)
  assert.equal(closed, 1)
  assert.equal(instance.manualError, false)
})

test('export client changes reset the template and stale QR result', () => {
  const parsed = compiler.parse({ source: read('src/views/node/list/components/ExportNodeDialog.vue') })
  const component = loadModule(parsed.script.content, new Proxy({}, {
    has: () => true,
    get: (_, name) => name === 'copy-to-clipboard' ? () => true : {}
  }), { window: {} }).default
  const instance = {
    activeClient: 'one', selectedTemplate: 'old', qrCodeSrc: 'data:image/png;base64,old',
    options: [
      { id: 'one', templates: [{ id: 'a' }] },
      { id: 'two', templates: [{ id: 'b' }] }
    ]
  }
  Object.defineProperty(instance, 'activeOption', {
    get() { return component.computed.activeOption.call(instance) }
  })
  instance.selectDefaultTemplate = component.methods.selectDefaultTemplate.bind(instance)
  component.methods.selectClient.call(instance, 'two')
  assert.equal(instance.activeClient, 'two')
  assert.equal(instance.selectedTemplate, 'b')
  assert.equal(instance.qrCodeSrc, '')
})

test('separate traffic computes each direction ratio before choosing the dominant one', () => {
  const parsed = compiler.parse({ source: read('src/views/node-server/list/index.vue') })
  const component = loadModule(parsed.script.content, new Proxy({}, {
    has: () => true,
    get: () => ({})
  })).default
  const percent = component.methods.trafficPercent
  assert.equal(percent({ limitMode: 'separate', uploadUsed: 80, uploadLimit: 100, downloadUsed: 100, downloadLimit: 1000 }), 80)
  assert.equal(percent({ limitMode: 'separate', uploadUsed: 20, uploadLimit: 0, downloadUsed: 50, downloadLimit: 100 }), 50)
  assert.equal(percent({ limitMode: 'combined', totalUsed: 30, totalLimit: 120 }), 25)
})

test('invalid active template blocks settings save before serialization or API work', () => {
  const parsed = compiler.parse({ source: read('src/views/system/base/components/template-config.vue') })
  let saves = 0
  const component = loadModule(parsed.script.content, {
    '@/api/system': { updateSystemById: () => { saves++; return Promise.resolve() } },
    '@/components/ClientTemplateEditor': {},
    'js-yaml': require('js-yaml')
  }).default
  const instance = {
    $refs: { templateEditor: { validate: () => false } },
    get systemConfig() { throw new Error('serialization must not run') }
  }
  component.methods.updateData.call(instance)
  assert.equal(saves, 0)
})

test('select/date tail geometry uses scalar horizontal tokens for every size', () => {
  const postcss = require('postcss')
  for (const name of ['LiquidSelect', 'LiquidDatePicker']) {
    const css = compiler.parse({ source: read(`src/components/${name}/index.vue`) }).styles[0].content
    postcss.parse(css).walkDecls((decl) => {
      if (decl.value.includes('calc(')) assert.doesNotMatch(decl.value, /ui-control-size-padding(?:,|\))/)
    })
    assert.match(css, /--ui-control-size-padding-x/)
  }
  const geometry = read('packages/ui-components-vue2/src/geometry.css')
  for (const size of ['sm', 'md', 'lg']) {
    assert.match(geometry, new RegExp(`data-ui-size='${size}'[\\s\\S]*?--ui-control-size-padding-x`))
  }
})

test('LiquidInput readonly reaches the native input and textarea VNodes', () => {
  const Vue = require('vue/dist/vue.common.js')
  const parsed = compiler.parse({ source: read('src/components/LiquidInput/index.vue') })
  const component = loadModule(parsed.script.content, {
    '@/mixins/liquid-control-emitter': {},
    '@/mixins/liquid-form-control': { computed: { controlAttrs: () => ({}) } }
  }).default
  const compiled = Vue.compile(parsed.template.content)
  const Control = Vue.extend({ ...component, render: compiled.render, staticRenderFns: compiled.staticRenderFns })
  for (const type of ['text', 'textarea']) {
    const vm = new Control({ propsData: { readonly: true, type } })
    const field = vm._render().children.find((child) => child.tag === (type === 'textarea' ? 'textarea' : 'input'))
    assert.equal(field.data.attrs.readonly, true)
    vm.$destroy()
  }
})

test('old password empty validation uses the dedicated required message', async () => {
  const parsed = compiler.parse({ source: read('src/views/account/modify/components/ModifyPass.vue') })
  const component = loadModule(parsed.script.content, new Proxy({}, { has: () => true, get: () => ({}) })).default
  const instance = { $t: (key) => key, $store: { getters: { username: 'demo' } } }
  const state = component.data.call(instance)
  const rule = state.updateRules.oldPass[0]
  assert.equal(rule.message, 'table.oldPassRequired')
  await new Promise((resolve) => rule.validator(rule, '', (error) => {
    assert.equal(error.message, 'table.oldPassRequired')
    resolve()
  }))
})

test('navigation interaction owns button transitions; component skins cannot override them', () => {
  const postcss = require('postcss')
  const targets = {
    'src/styles/buttons.scss': ['.liquid-button'],
    'src/styles/prototype-runtime.scss': ['.cap', '.icon-btn', '.dd-item', '.nav-item', '.prototype-mobile-nav button'],
    'src/components/LiquidNumberInput/index.vue': ['.liquid-number-input__step'],
    'src/components/LiquidSwitch/index.vue': ['.liquid-switch'],
    'src/components/LiquidCodeEditor/index.vue': ['.liquid-code-editor__toolbar button'],
    'src/components/LiquidSelect/index.vue': ['.liquid-select__trigger', '.liquid-select__option'],
    'src/components/LiquidDatePicker/index.vue': ['.liquid-date-picker__trigger'],
    'src/views/node/list/components/NodeClientSelector.vue': ['.client-choice']
  }
  for (const [file, selectors] of Object.entries(targets)) {
    const source = read(file)
    const css = file.endsWith('.vue') ? compiler.parse({ source }).styles[0].content : source
    postcss.parse(css).walkRules((rule) => {
      if (selectors.includes(rule.selector)) rule.walkDecls(/^transition/, () => assert.fail(file + ': duplicate button transition'))
    })
  }
  assert.match(read('packages/ui-components-vue2/src/button-interactions.css'), /transition-duration: var\(--ui-motion-slow, 300ms\)/)
})

test('avatar and account name form a separate keyboard-operable profile entry', () => {
  const source = read('src/layout/index.vue')
  assert.match(source, /class="prototype-profile-entry"[\s\S]*?type="button"[\s\S]*?aria-label="我的个人资料"[\s\S]*?@click="go\('\/modify\/index'\)"/)
  assert.match(source, /<strong>\{\{ username \|\| 'Trojan Panel' \}\}<\/strong>\s*<\/button>\s*<button[\s\S]*?aria-label="退出登录"/)
})

test('control size aliases resolve to the public size contract and compact controls consume it', () => {
  const sizes = loadModule(read('src/mixins/liquid-control-size.js'), {}).default
  for (const [size, expected] of Object.entries({ mini: 'sm', small: 'sm', medium: 'md', default: 'md', large: 'lg', sm: 'sm', md: 'md', lg: 'lg' })) {
    assert.equal(sizes.computed.controlSize.call({ size }), expected)
    assert.equal(sizes.props.size.validator(size), true)
  }
  for (const name of ['LiquidButton', 'LiquidSelect', 'LiquidDatePicker']) {
    assert.match(read(`src/components/${name}/index.vue`), /:data-ui-size="controlSize"/)
  }
  assert.match(read('src/styles/buttons.scss'), /min-height: var\(--ui-control-size-height, 38px\)/)
  assert.match(read('src/components/LiquidSelect/index.vue'), /min-height: var\(--ui-control-size-height, 42px\)/)
})

test('clear controls are native siblings and closed popovers do not swallow dialog Escape', () => {
  for (const name of ['LiquidSelect', 'LiquidDatePicker']) {
    const source = read(`src/components/${name}/index.vue`)
    assert.doesNotMatch(source, /role="button"/)
    assert.match(source, /<button[^>]*aria-label="清空(?:日期)?"/)
    const script = compiler.parse({ source }).script.content
    const component = loadModule(script, {
      '@/mixins/liquid-control-emitter': {},
      '@/mixins/liquid-form-control': {},
      '@/mixins/liquid-control-size': {}
    }, { document: {} }).default
    let prevented = 0, closed = 0
    const instance = { open: false, closeMenu: () => closed++, closePopover: () => closed++, $refs: { trigger: { focus() {} } } }
    const event = { preventDefault: () => prevented++, stopPropagation() {} }
    component.methods.handleEscape.call(instance, event)
    assert.equal(prevented, 0)
    assert.equal(closed, 0)
    instance.open = true
    component.methods.handleEscape.call(instance, event)
    assert.equal(prevented, 1)
    assert.equal(closed, 1)
  }
})

test('confirm and prompt render shared dialog controls, validate safely, and settle exactly once', async () => {
  let instance, removed = 0, destroyed = 0
  const dialog = {}, input = {}, button = {}
  function Vue(options) {
    instance = this
    Object.assign(this, options.data, {
      _uid: 9, $refs: { input: { focus() {} } },
      $el: { remove: () => removed++ },
      $nextTick: (callback) => callback(),
      $destroy: () => destroyed++,
      $mount() {}
    })
    for (const [name, method] of Object.entries(options.methods)) this[name] = method.bind(this)
    this.render = () => options.render.call(this, (tag, data, children) => ({ tag, data, children }))
  }
  const { MessageBox } = loadModule(read('src/utils/liquid-feedback.js'), {
    vue: Vue, '@tp-ui/components-vue2': { UiDialog: dialog }, '@tp-ui/icons': { renderIcon() {} },
    '@tp-ui/motion-native': { afterTransition() {} },
    '@/components/LiquidInput': input, '@/components/LiquidButton': button
  }, { document: { body: { appendChild() {} } } })
  const promise = MessageBox.prompt('名称', '编辑', { inputPattern: /^[a-z]+$/g, inputValue: '123', inputErrorMessage: '请使用字母' })
  const vnode = instance.render()
  assert.equal(vnode.tag, dialog)
  assert.equal(vnode.data.props.role, 'alertdialog')
  assert.equal(vnode.children[1].children[0].tag, input)
  assert.equal(vnode.children[2].children[0].tag, button)
  instance.confirm()
  assert.equal(instance.error, '请使用字母')
  assert.equal(destroyed, 0)
  instance.value = 'valid'
  instance.confirm()
  instance.confirm()
  const result = await promise
  assert.equal(result.value, 'valid')
  assert.equal(result.action, 'confirm')
  assert.equal(destroyed, 1)
  assert.equal(removed, 1)
  const cancel = MessageBox.confirm('删除？', '确认')
  instance.render().data.on.close()
  await assert.rejects(cancel, (error) => error === 'cancel')
})

test('production animation timings and overlay material have a single owner', () => {
  for (const file of sourceFiles('src').filter((file) => /\.(vue|scss)$/.test(file))) {
    assert.doesNotMatch(read(file), /(?:transition|animation)[\w-]*:\s*[^;{}]*\b\d+(?:\.\d+)?m?s\b/, file)
    assert.doesNotMatch(read(file), /#[a-fA-F0-9]{3,8}\b|rgba?\(|blur\(\d/, file)
  }
  const material = read('packages/ui-material-frosted/src/overlay.css')
  assert.doesNotMatch(material, /\.tp-ui-|(?:^|[;{])\s*(?:background|border|color|backdrop-filter):/)
  for (const file of ['src/styles/frosted-surfaces.scss', 'src/styles/liquid-structural.scss', 'src/styles/prototype-runtime.scss']) {
    assert.doesNotMatch(read(file), /\.tp-ui-dialog(?:__header|__body|__footer|__close|-layer)?\s*\{/)
  }
  assert.doesNotMatch(read('src/utils/liquid-feedback.js'), /liquid-feedback-layer|liquid-message-box|setTimeout\([^\n]*180/)
  assert.doesNotMatch(read('src/utils/scroll-to.js'), /Math\.ease|requestAnimFrame/)
  const styleEntry = read('src/styles/index.scss')
  assert.doesNotMatch(styleEntry, /@import\b/)
  for (const module of ['frosted-surfaces', 'buttons', 'icons', 'liquid-structural', 'prototype-runtime']) {
    assert.match(styleEntry, new RegExp(`@use ['"]\\./${module}['"]`))
  }
  const main = read('src/main.js')
  const styleLayers = [
    '@tp-ui/contracts/base.css',
    '@tp-ui/motion-native/motion.css',
    '@tp-ui/components-vue2/geometry.css',
    '@tp-ui/layout-app-shell-vue2/layout.css',
    '@tp-ui/components-vue2/button-interactions.css',
    '@tp-ui/material-frosted/production.css',
    '@tp-ui/material-frosted/overlay.css',
    '@/styles/index.scss'
  ]
  styleLayers.reduce((previous, layer) => {
    const position = main.indexOf(`import '${layer}'`)
    assert.ok(position > previous, `${layer} must keep the public style order`)
    return position
  }, -1)
  const composition = read('src/adapters/trojan-panel-ui-composition.js')
  assert.match(composition, /createUiRuntime\(/)
  assert.match(composition, /createFrostedMaterial\(/)
  assert.match(main, /installProductionUi\(Vue\)/)
  assert.doesNotMatch(main, /Vue\.component\(['"]Liquid|structuralComponents/)
  assert.doesNotMatch(read('src/styles/icons.scss'), /prefers-reduced-motion/)
  assert.match(read('src/components/LiquidTag/index.vue'), /<button v-if="\$listeners.click"/)
  assert.match(read('src/components/LiquidTag/index.vue'), /@click.stop="\$emit\('close'/)
})

function sourceFiles(directory) {
  return fs.readdirSync(path.join(root, directory), { withFileTypes: true }).flatMap((entry) => {
    const file = path.join(directory, entry.name)
    return entry.isDirectory() ? sourceFiles(file) : [file]
  })
}

test('all production controls use known semantic icons and no legacy renderer', () => {
  const { iconNames } = loadModule(read('packages/ui-icons/src/index.js'), {})
  for (const file of sourceFiles('src').filter((name) => /\.(vue|js|scss)$/.test(name))) {
    const source = read(file)
    assert.doesNotMatch(source, /liquid-icon--|liquid-icon-svg|svg-icon|LiquidNavIcon/, file)
    if (!file.endsWith('.vue')) continue
    const template = compiler.parse({ source }).template
    if (!template) continue
    assert.deepEqual(compiler.compileTemplate({ source: template.content, filename: file }).errors, [], file)
    for (const match of template.content.matchAll(/<app-icon\b[^>]*?\sname="([^"]+)"/g)) {
      assert.ok(iconNames.includes(match[1]), `${file}: ${match[1]}`)
    }
    for (const match of template.content.matchAll(/\sicon="([^"]+)"/g)) {
      assert.ok(iconNames.includes(match[1]), `${file}: ${match[1]}`)
    }
  }
  assert.match(read('src/adapters/trojan-panel-ui-composition.js'), /createVue2Components\([\s\S]*?renderIcon/)
  assert.match(read('src/components/AppIcon/index.js'), /renderIcon\(h, props.name, \{\}, data\)/)
  assert.match(read('src/styles/icons.scss'), /app-icon--loading[\s\S]*animation:/)
  assert.doesNotMatch(read('src/styles/icons.scss'), /background:|box-shadow:|padding:|border-radius:/)
})

test('production composition uses package exports and a business-only shell adapter', () => {
  assert.doesNotMatch(read('vite.config.mjs'), /@tp-ui[^\n]+packages\//)
  assert.match(read('src/layout/index.vue'), /<ui-app-shell/)
  const adapter = read('src/adapters/trojan-panel-shell.js')
  assert.match(adapter, /createShellModel\(/)
  assert.doesNotMatch(read('packages/ui-layout-app-shell-vue2/src/index.js'), /vuex|vue-router|@\/|sysadmin/)
  assert.doesNotMatch(
    read('packages/ui-components-vue2/src/index.js'),
    /classes:\s*\[[^\]]*['"](?:glass|card|sheet)['"]/
  )
  for (const file of sourceFiles('src').filter((name) => /\.(?:vue|scss|css)$/.test(name))) {
    assert.doesNotMatch(read(file), /\.tp-ui-[a-z0-9_-]+/, file)
  }
})

test('shell adapter preserves role filtering, mobile labels, branding, and profile navigation', () => {
  const { createTrojanPanelShellModel } = loadModule(
    read('src/adapters/trojan-panel-shell.js'),
    { '@tp-ui/contracts': { createShellModel: (value) => value } }
  )
  const branding = { systemName: 'Trojan Panel' }
  const sysadmin = createTrojanPanelShellModel({
    roles: ['sysadmin'], username: 'root', activePath: '/dashboard/index', pageTitle: '仪表板', branding
  })
  const user = createTrojanPanelShellModel({
    roles: ['user'], username: 'guest', activePath: '/dashboard/index', pageTitle: '我的首页', branding
  })
  assert.ok(sysadmin.groups.flatMap((group) => group.items).some((item) => item.key === '/system/base-config'))
  assert.deepEqual(
    Array.from(user.groups.flatMap((group) => group.items), (item) => item.key),
    ['/dashboard/index', '/node-manage/node-list', '/modify/index']
  )
  assert.equal(user.groups[0].items[0].mobileLabel, '首页')
  assert.equal(user.brand.name, 'Trojan Panel')
  assert.equal(user.user.label, 'guest')
})

test('central ambient color is theme-aware and limited to phone/tablet layouts', () => {
  const css = read('src/styles/prototype-runtime.scss')
  assert.equal((read('src/App.vue').match(/class="ambient__center"/g) || []).length, 1)
  assert.match(css, /\.ambient \.ambient__center\s*\{\s*display: none;/)
  assert.match(css, /@media \(max-width: 1060px\)\s*\{\s*\.ambient \.ambient__center\s*\{[^}]*display: block;[^}]*background: var\(--blob-e\)/)
  assert.match(css, /\.ambient\s*\{[^}]*pointer-events: none;/)
})

test('branding has a safe fallback and ignores unrelated settings', () => {
  const { panelBranding, updatePanelBranding } = loadBranding()
  assert.equal(panelBranding.systemName, 'Trojan Panel')
  updatePanelBranding({ systemName: '  海蓝面板  ' })
  assert.equal(panelBranding.systemName, '海蓝面板')
  updatePanelBranding({ registerEnable: 1 })
  assert.equal(panelBranding.systemName, '海蓝面板')
  updatePanelBranding({ systemName: '  ' })
  assert.equal(panelBranding.systemName, 'Trojan Panel')
})

test('concurrent settings requests share a request; late responses cannot undo a save', async () => {
  let complete, calls = 0
  const brand = loadBranding(() => { calls++; return new Promise((resolve) => { complete = resolve }) })
  const request = brand.loadPanelSettings()
  assert.equal(brand.loadPanelSettings(), request)
  assert.equal(calls, 1)
  brand.updatePanelBranding({ systemName: 'Saved name' })
  complete({ data: { systemName: 'Stale name', captchaEnable: 1 } })
  assert.equal((await request).data.captchaEnable, 1)
  assert.equal(brand.panelBranding.systemName, 'Saved name')
})

test('failed public settings can be retried', async () => {
  let calls = 0
  const brand = loadBranding(() => ++calls === 1
    ? Promise.reject(new Error('offline'))
    : Promise.resolve({ data: { systemName: 'Restored' } }))
  await assert.rejects(brand.loadPanelSettings(), /offline/)
  await brand.loadPanelSettings()
  assert.equal(brand.panelBranding.systemName, 'Restored')
})

test('logo refresh busts caches and lets failed images retry', () => {
  const brand = loadBranding()
  const oldUrl = brand.panelBranding.logoUrl
  brand.refreshPanelLogo()
  assert.notEqual(brand.panelBranding.logoUrl, oldUrl)
  const refreshed = brand.panelBranding.logoUrl
  brand.refreshPanelLogo()
  assert.notEqual(brand.panelBranding.logoUrl, refreshed)
  const descriptor = compiler.parse({ source: read('src/components/PanelLogo/index.vue') })
  const component = loadModule(descriptor.script.content, { '@/utils/panel-branding': brand }).default
  const context = { branding: brand.panelBranding, failed: true }
  component.watch['branding.logoUrl'].call(context)
  assert.equal(context.failed, false)
  brand.updatePanelBranding({ systemName: '海蓝' })
  assert.equal(component.computed.initial.call(context), '海')
})

test('production build keeps async component skins in one stylesheet', () => {
  assert.match(read('vite.config.mjs'), /cssCodeSplit:\s*false/)
  for (const component of ['LiquidSelect', 'LiquidDatePicker']) {
    const source = read(`src/components/${component}/index.vue`)
    assert.doesNotMatch(source, /--ui-select-tail-width/)
    assert.doesNotMatch(source, /padding-right:\s*calc\(/)
  }
})

test('auth and 404 share panels and branding; 404 has a real router action', () => {
  for (const file of ['src/views/login/index.vue', 'src/views/register/index.vue', 'src/views/404.vue']) {
    const source = read(file)
    const descriptor = compiler.parse({ source })
    assert.deepEqual(compiler.compileTemplate({ source: descriptor.template.content, filename: file }).errors, [])
    assert.match(source, /<ui-panel/)
    assert.match(source, /<panel-logo/)
    assert.doesNotMatch(source, /login-container|wscn-http404|FROSTED GLASS/)
  }
  assert.match(read('src/views/404.vue'), /\$router\.push\('\/dashboard\/index'\)/)
  assert.doesNotMatch(read('src/views/404.vue'), /href=""|1200px|@keyframes/)
})

test('logo upload refreshes shared branding only after success; failures keep the previous logo', async () => {
  let fail = false, refreshes = 0, notifications = 0
  const descriptor = compiler.parse({ source: read('src/components/UploadLogo/index.vue') })
  const component = loadModule(descriptor.script.content, {
    '@/utils/liquid-feedback': { Message() {} },
    '@/api/system': { uploadLogo: async () => { if (fail) throw new Error('upload failed') } },
    '@/components/PanelLogo': {},
    '@/utils/panel-branding': { refreshPanelLogo: () => { refreshes++ } }
  }, { FormData: class { append() {} } }).default
  const context = {
    uploading: false, beforeUpload: () => true,
    $t: (key) => key, $notify: () => { notifications++ }
  }
  const event = () => ({ target: { files: [{ type: 'image/png', size: 10 }], value: 'logo.png' } })
  await component.methods.handleNativeFile.call(context, event())
  assert.equal(refreshes, 1)
  assert.equal(notifications, 1)
  assert.equal(context.uploading, false)
  fail = true
  await component.methods.handleNativeFile.call(context, event())
  assert.equal(refreshes, 1)
  assert.equal(notifications, 1)
  assert.equal(context.uploading, false)
})

test('removed CSS cannot override current labels and controls', () => {
  const styles = fs.readdirSync(path.join(root, 'src/styles'))
    .filter((file) => file.endsWith('.scss')).map((file) => read(`src/styles/${file}`)).join('\n')
  const stale = styles.match(/login-container|sidebar-container|tags-view-container|client-selector__hint|liquid-input__inner|liquid-select-dropdown|liquid-radio-button|dialog-fade/)
  assert.equal(stale && stale[0], null, 'Legacy selector was reintroduced')
  assert.match(styles, /\.fld > span:first-child\s*{\s*color: var\(--form-label-ink\)/)
  assert.match(styles, /\.ui-supporting-text\s*{\s*color: var\(--supporting-text-ink\)/)
  assert.match(styles, /\.liquid-table th\s*{\s*color: var\(--table-header-ink\)/)
  assert.match(read('src/layout/components/AppMain.vue'), /state.tagsView.cachedViews/)
  assert.equal(fs.existsSync(path.join(root, 'src/layout/components/Sidebar/index.vue')), false)
})

test('tables render an empty state while async callers supply null, then render loaded rows', () => {
  const Vue = require('vue')
  const { LiquidTable } = loadModule(read('src/components/LiquidStructural/index.js'), { vue: Vue })
  const table = new Vue({ ...LiquidTable, propsData: { data: null } })
  const text = (node) => node.text || (node.children || []).map(text).join('')
  const render = () => LiquidTable.render.call(table, table.$createElement)
  assert.match(text(render()), /暂无数据/)
  table.columns = [{ prop: 'name', label: 'Name', $scopedSlots: {} }]
  table.data = [{ id: 1, name: 'Loaded row' }]
  assert.match(text(render()), /Loaded row/)
  table.$destroy()
})

test('dashboard retains its existing panels without server onboarding or deployment instructions', () => {
  const descriptor = compiler.parse({ source: read('src/views/dashboard/admin/index.vue') })
  const keys = [...descriptor.template.content.matchAll(/motion-key="([^"]+)"/g)].map((match) => match[1])
  assert.deepEqual(keys, ['account-count', 'node-count', 'server-resources', 'traffic-rank', 'server-traffic'])
  assert.doesNotMatch(descriptor.template.content, /serverRegistration|node-deployment|node-registration/)
  assert.doesNotMatch(descriptor.script.content, /registerNodeServer/)
})

test('Node server registration opens once, preserves unrelated query values and guards permissions', () => {
  let allowed = true
  const component = loadModule(compiler.parse({ source: read('src/views/node-server/list/index.vue') }).script.content, new Proxy({}, {
    has: () => true,
    get: (_, name) => name === '@/utils/permission' ? () => allowed : {}
  })).default
  let cleared = 0, resets = 0, navigations = 0
  const instance = {
    $route: { path: '/server-manage/server-list', query: { action: 'create', filter: 'online' } },
    $router: { replace(route) { navigations++; instance.$route = route } },
    $refs: { nodeServerForm: { clearValidate() { cleared++ } } },
    resetTemp() { resets++ }, dialogFormVisible: false, dialogStatus: ''
  }
  for (const name of ['handleCreate', 'openRegistrationFromRoute']) instance[name] = component.methods[name].bind(instance)
  component.mounted.call(instance)
  assert.equal(instance.dialogFormVisible, true)
  assert.equal(instance.dialogStatus, 'create')
  assert.equal(instance.$route.query.filter, 'online')
  assert.equal(instance.$route.query.action, undefined)
  component.watch['$route.query.action'].call(instance)
  assert.equal(navigations, 1)
  assert.equal(cleared, 1)
  assert.equal(resets, 1)
  allowed = false
  instance.dialogFormVisible = false
  instance.$route.query.action = 'create'
  component.mounted.call(instance)
  instance.handleCreate()
  assert.equal(instance.dialogFormVisible, false)
  assert.equal(cleared, 1, 'permission failure cannot reset or open the form')
})

test('server actions route Web-only delete and both uninstall modes to separate APIs', async () => {
  const calls = [], notices = []
  let allowed = true, failUninstall = false, refreshes = 0
  const component = loadModule(compiler.parse({ source: read('src/views/node-server/list/index.vue') }).script.content, new Proxy({}, {
    has: () => true,
    get: (_, name) => {
      if (name === '@/utils/permission') return () => allowed
      if (name === '@/api/node-server') return {
        async deleteNodeServerById(data) { calls.push({ action: 'delete', data }); return { data: null } },
        async uninstallNodeServerById(data) {
          calls.push({ action: 'uninstall', data })
          if (failUninstall) throw new Error('Node is offline')
          return { data: { cleanupPending: true } }
        }
      }
      return {}
    }
  })).default
  const row = { id: 11, name: 'offline Node', status: 0 }
  const instance = {
    deleteServer: row, deletingServerId: 0, list: [row],
    async getList() { refreshes++ },
    $notify(notice) { notices.push(notice) }
  }
  const run = (action) => { instance.deleteServer = row; return component.methods.confirmDelete.call(instance, action) }
  await run('delete')
  assert.equal(calls[0].action, 'delete')
  assert.deepEqual(JSON.parse(JSON.stringify(calls[0].data)), { id: 11 })
  assert.match(notices[0].message, /目标机器未被卸载/)
  await run('uninstall')
  assert.equal(calls[1].action, 'uninstall')
  assert.deepEqual(JSON.parse(JSON.stringify(calls[1].data)), { id: 11, purge: false })
  await run('purge')
  assert.deepEqual(JSON.parse(JSON.stringify(calls[2].data)), { id: 11, purge: true })
  assert.equal(refreshes, 3)
  failUninstall = true
  await run('uninstall')
  assert.equal(calls.length, 4, 'failed uninstall must never request a Web-only deletion')
  assert.equal(refreshes, 3, 'failed uninstall must retain the current server row')
  assert.equal(instance.list[0], row)
  assert.equal(instance.deletingServerId, 0, 'retry is available after failure')
  assert.match(notices[3].message, /Web 记录仍保留/)
  await run('invalid')
  allowed = false
  await run('delete')
  assert.equal(calls.length, 4, 'invalid actions and other roles cannot mutate servers')
})

test('server API keeps Web deletion short and gives remote uninstall its own timeout', async () => {
  const requests = []
  const api = loadModule(read('src/api/node-server.js'), { '@/utils/request': (request) => { requests.push(request); return Promise.resolve() } })
  await api.deleteNodeServerById({ id: 9 })
  await api.uninstallNodeServerById({ id: 9, purge: true })
  assert.equal(requests[0].url, '/nodeServer/deleteNodeServerById')
  assert.equal(requests[0].timeout, 30000)
  assert.equal(requests[1].url, '/nodeServer/uninstallNodeServerById')
  assert.equal(requests[1].timeout, 240000)
})

test('server ID has a separate readable column and table empty states span every column', () => {
  const descriptor = compiler.parse({ source: read('src/views/node-server/list/index.vue') })
  const head = descriptor.template.content.match(/<thead>([\s\S]+?)<\/thead>/)[1]
  const columns = [...head.matchAll(/<th(?:\s|>)/g)].length
  assert.equal(columns, 9)
  for (const match of descriptor.template.content.matchAll(/colspan="(\d+)"/g)) assert.equal(Number(match[1]), columns)
  const idCell = descriptor.template.content.match(/<td class="server-id-cell">([\s\S]+?)<\/td>/)[1]
  assert.match(idCell, /\{\{ row\.id \}\}/)
  assert.doesNotMatch(idCell, /row\.ip|<small|muted/)
  assert.match(descriptor.styles[0].content, /\.server-id-value\s*{[^}]*color: var\(--ink\);[^}]*font-size: 14px;/)
  assert.doesNotMatch(descriptor.template.content, /serverRegistration\.description/)
  assert.doesNotMatch(read('src/views/node-server/list/compoments/NodeServerForm.vue'), /serverRegistration\.formHint/)
})

test('server creation opens deployment with the returned numeric ID, never a guessed name match', async () => {
  let result = { id: 42, name: 'Duplicate name', ip: 'new.example.com', grpcPort: 8100, grpcTlsServerName: 'tls.example.com' }
  const events = [], notices = []
  const form = loadModule(compiler.parse({ source: read('src/views/node-server/list/compoments/NodeServerForm.vue') }).script.content, {
    '@/api/node-server': { createNodeServer: async () => ({ data: result }) },
    '@/utils/account': { getFlow: (value) => String(value) },
    '@/utils/permission': () => true
  }).default
  let refreshes = 0
  const context = { creating: false, $refs: { dataForm: { validate: async () => true } }, payload: () => ({ name: 'Duplicate name' }), getList: () => { refreshes++ }, $t: (key) => key, $emit: (...args) => events.push(args), $notify: (notice) => notices.push(notice) }
  await form.methods.createData.call(context)
  assert.equal(events.find(([event]) => event === 'created')[1].id, 42)
  assert.equal(context.creating, false)
  assert.equal(refreshes, 1)
  for (result of [null, { id: '42' }, { id: 0 }]) {
    events.length = 0
    await form.methods.createData.call(context)
    assert.equal(events.some(([event]) => event === 'created'), false)
    assert.equal(notices.at(-1).type, 'warning')
  }
  result = { id: 44 }
  const beforeRefresh = refreshes
  await Promise.all([form.methods.createData.call(context), form.methods.createData.call(context)])
  assert.equal(refreshes, beforeRefresh + 1, 'double submit while validation is pending cannot create duplicate servers')
  let allowed = true
  const list = loadModule(compiler.parse({ source: read('src/views/node-server/list/index.vue') }).script.content, new Proxy({}, {
    has: () => true, get: (_, name) => name === '@/utils/permission' ? () => allowed : {}
  })).default
  const view = { deploymentServer: null, list: [{ id: 99, name: 'Duplicate name' }] }
  list.methods.handleDeployment.call(view, { id: 42, name: 'Duplicate name' })
  assert.equal(view.deploymentServer.id, 42)
  allowed = false
  view.deploymentServer = null
  list.methods.handleDeployment.call(view, { id: 42 })
  assert.equal(view.deploymentServer, null)
})

test('deployment download rejects JSON errors, including Blob errors on HTTP failure, before accepting gzip', async () => {
  const requests = []
  let response = { data: new Blob([JSON.stringify({ code: 50008, message: 'Session expired' })], { type: 'application/json' }) }
  let rejectHTTP = false
  const api = loadModule(read('src/api/node-server.js'), {
    '@/utils/request': async (request) => { requests.push(request); if (rejectHTTP) throw { response }; return response }
  }, { Blob })
  await assert.rejects(api.downloadNodeDeployment({ id: 42 }), (error) => error.message === 'Session expired' && error.code === 50008)
  rejectHTTP = true
  await assert.rejects(api.downloadNodeDeployment({ id: 42 }), (error) => error.message === 'Session expired' && error.code === 50008)
  rejectHTTP = false
  response = { data: new Blob(['<html>Gateway error</html>'], { type: 'text/html' }) }
  await assert.rejects(api.downloadNodeDeployment({ id: 42 }), /Invalid deployment archive/)
  response = { data: new Blob([new Uint8Array([0x1f, 0x8b, 0x08])], { type: 'application/gzip' }) }
  assert.equal(await api.downloadNodeDeployment({ id: 42 }), response)
  assert.equal(requests.at(-1).responseType, 'blob')
  assert.equal(requests.at(-1).url, '/nodeServer/downloadDeployment')
  assert.equal(requests.at(-1).data.id, 42)
})

test('deployment form validates mode-specific inputs and preserves separate database hosts', async () => {
  const deployment = loadModule(compiler.parse({ source: read('src/views/node-server/list/compoments/NodeServerDeployment.vue') }).script.content, {
    'copy-to-clipboard': () => true, '@/api/node-server': {}
  }).default
  const { LiquidFormItem } = loadModule(read('src/components/LiquidStructural/index.js'), { vue: {} })
  const context = { $t: (key) => key, form: { certificateMode: 'caddy', webHost: 'panel.example.com' }, metadata: { webHost: 'panel.example.com', mariadbHost: 'db.example.com', mariadbUsesWebHost: false, mariadbPort: 3307, redisHost: 'panel.example.com', redisUsesWebHost: true, redisPort: 6378 } }
  const valid = async (rules, value) => LiquidFormItem.methods.validate.call({ appliedRules: rules, value, error: '' })
  let rules = deployment.computed.rules.call(context)
  assert.equal(await valid(rules.email, ''), false)
  assert.equal(await valid(rules.email, 'not-an-email'), false)
  assert.equal(await valid(rules.email, 'admin@example.com'), true)
  assert.equal(await valid(rules.webHost, 'https://panel.example.com'), false)
  assert.equal(await valid(rules.webHost, 'panel.example.com'), true)
  context.form.certificateMode = 'external'
  context.form.webHost = 'reachable.example.com'
  rules = deployment.computed.rules.call(context)
  assert.equal(await valid(rules.email, ''), true)
  assert.equal(await valid(rules.certificatePath, ''), false)
  assert.equal(await valid(rules.certificatePath, 'relative/fullchain.pem'), false)
  assert.equal(await valid(rules.privateKeyPath, '/etc/certs/private,key.pem'), false)
  assert.equal(await valid(rules.certificatePath, '/etc/certs/fullchain.pem'), true)
  assert.equal(await valid(rules.privateKeyPath, '/etc/certs/privkey.pem'), true)
  assert.equal(deployment.computed.databaseAddress.call(context), 'db.example.com:3307')
  assert.equal(deployment.computed.redisAddress.call(context), 'reachable.example.com:6378')
  context.metadata.mariadbHost = context.metadata.webHost
  context.metadata.redisUsesWebHost = false
  assert.equal(deployment.computed.databaseAddress.call(context), 'panel.example.com:3307', 'matching hostnames must not imply a database fallback')
  assert.equal(deployment.computed.redisAddress.call(context), 'panel.example.com:6378', 'matching hostnames must not imply a Redis fallback')
  context.metadata.mariadbUsesWebHost = true
  context.metadata.redisUsesWebHost = true
  assert.equal(deployment.computed.databaseAddress.call(context), 'reachable.example.com:3307')
  assert.equal(deployment.computed.redisAddress.call(context), 'reachable.example.com:6378')
})

test('deployment download cleans the temporary link and ignores stale form parameters', async () => {
  const notices = [], requests = [], links = [], timers = new Map()
  let resolveDownload, createdUrls = 0, revokedUrls = 0
  const component = loadModule(compiler.parse({ source: read('src/views/node-server/list/compoments/NodeServerDeployment.vue') }).script.content, {
    'copy-to-clipboard': () => true,
    '@/api/node-server': { downloadNodeDeployment: (data) => { requests.push(data); return new Promise((resolve) => { resolveDownload = resolve }) } }
  }, {
    window: { setTimeout: (callback, delay) => { assert.equal(delay, 30000); const id = timers.size + 1; timers.set(id, callback); return id }, clearTimeout: (id) => timers.delete(id), URL: { createObjectURL: () => { createdUrls++; return 'blob:fixture' }, revokeObjectURL: () => { revokedUrls++ } } },
    document: { body: { appendChild: () => {} }, createElement: () => { const link = { clicks: 0, removed: false, click() { this.clicks++ }, remove() { this.removed = true } }; links.push(link); return link } }
  }).default
  const context = { serverId: 42, archiveName: 'tpnext-node-42.tar.gz', form: { webHost: 'panel.example.com', email: 'admin@example.com', certificateMode: 'caddy', certificatePath: '/previous/certificate.pem', privateKeyPath: '/previous/key.pem' }, requestSequence: 1, downloading: false, downloaded: false, pendingDownloadUrls: [], $refs: { deploymentForm: { validate: async () => true } }, $t: (key) => key, $notify: (notice) => notices.push(notice) }
  const promise = component.methods.downloadArchive.call(context)
  await new Promise((resolve) => setImmediate(resolve))
  assert.equal(requests[0].id, 42)
  assert.equal(requests[0].certificatePath, '', 'Caddy mode must not submit hidden external paths')
  assert.equal(requests[0].privateKeyPath, '')
  context.form.webHost = 'changed.example.com'
  resolveDownload({ data: new Blob([new Uint8Array([0x1f, 0x8b])]) })
  await promise
  assert.equal(requests[0].webHost, 'panel.example.com', 'download request must own a submitted snapshot')
  assert.equal(context.downloaded, false, 'old package cannot make modified parameters look downloaded')
  assert.equal(context.downloading, false)
  assert.equal(links[0].download, 'tpnext-node-42.tar.gz')
  assert.equal(links[0].clicks, 1)
  assert.equal(links[0].removed, true)
  assert.equal(createdUrls, 1)
  assert.equal(revokedUrls, 0, 'Blob URL must stay alive while the browser asynchronously starts the download')
  const [firstTimerId, firstTimer] = timers.entries().next().value
  timers.delete(firstTimerId)
  firstTimer()
  assert.equal(createdUrls, revokedUrls, 'timer releases the consumed Blob URL')
  assert.equal(context.pendingDownloadUrls.length, 0)
  assert.equal(notices.at(-1).type, 'warning')
  context.form.certificateMode = 'external'
  context.form.webHost = 'panel.example.com'
  const externalPromise = component.methods.downloadArchive.call(context)
  await new Promise((resolve) => setImmediate(resolve))
  assert.equal(requests[1].email, '', 'external mode must not submit hidden Caddy email')
  assert.equal(requests[1].certificatePath, '/previous/certificate.pem')
  assert.equal(requests[1].privateKeyPath, '/previous/key.pem')
  resolveDownload({ data: new Blob([new Uint8Array([0x1f, 0x8b])]) })
  await externalPromise
  assert.equal(context.downloaded, true)
  assert.equal(createdUrls, 2)
  assert.equal(revokedUrls, 1)
  component.beforeDestroy.call(context)
  assert.equal(createdUrls, revokedUrls, 'dialog destruction releases outstanding URLs')
  assert.equal(timers.size, 0, 'dialog destruction cancels outstanding timers')
})

test('deployment metadata must match the requested ID and trusted release format before commands appear', async () => {
  let result = { id: 42, version: read('public/version').trim().replace(/^v/, ''), webHost: 'panel.example.com' }
  const component = loadModule(compiler.parse({ source: read('src/views/node-server/list/compoments/NodeServerDeployment.vue') }).script.content, {
    'copy-to-clipboard': () => true, '@/api/node-server': { nodeServerDeployment: async () => ({ data: result }) }
  }).default
  const context = { ...component.data(), serverId: 42, $t: (key) => key }
  await component.methods.loadDeployment.call(context)
  assert.equal(context.metadata.id, 42)
  assert.equal(context.form.webHost, 'panel.example.com')
  const commands = component.computed.installCommands.call({ metadata: result, archiveName: 'tpnext-node-42.tar.gz', $t: (key) => key })
  assert.ok(commands[0].value.includes(`/v${result.version}/scripts/tp.sh) --version ${result.version} deps install`))
  assert.equal(commands[1].value, 'tar -xzf tpnext-node-42.tar.gz')
  assert.equal(commands[2].value, 'bash ./tpnext/install-node.sh')
  for (result of [{ id: 99, version: '1.0.2' }, { id: 42, version: 'main; printf secret' }]) {
    const invalid = { ...component.data(), serverId: 42, $t: (key) => key }
    await component.methods.loadDeployment.call(invalid)
    assert.equal(invalid.metadata, null)
    assert.equal(invalid.loadError, 'nodeDeployment.invalidMetadata')
  }
})


const vnodeTree = (node) => [node, ...(node.children || []).flatMap(vnodeTree)]
const vnodeText = (node) => node.text || (node.children || []).map(vnodeText).join('')
const compileRender = (template) => {
  const result = compiler.compileTemplate({ source: template, filename: 'server-traffic-fixture.vue' })
  assert.deepEqual(result.errors, [])
  return vm.runInNewContext(`${result.code}; ({ render, staticRenderFns })`)
}

test('server list renders used traffic without reset controls and an accessible deployment icon', () => {
  const Vue = require('vue/dist/vue.common.js')
  const descriptor = compiler.parse({ source: read('src/views/node-server/list/index.vue') })
  const component = loadModule(descriptor.script.content, new Proxy({}, {
    has: () => true,
    get: (_, name) => name === '@/utils/permission' ? () => true : name === '@/utils/account' ? loadModule(read('src/utils/account.js'), {}) : {}
  })).default
  const compiled = compileRender(descriptor.template.content)
  const View = Vue.extend({ ...component, components: {}, mixins: [], created: undefined, mounted: undefined, watch: {},
    beforeCreate() { this.$t = (key) => key }, render: compiled.render, staticRenderFns: compiled.staticRenderFns })
  const view = new View()
  view.listLoading = false
  view.list = [{ id: 42, name: 'Fixture Node', ip: 'fixture.example.com', grpcPort: 8100, grpcTlsMode: 'mtls',
    trafficStatus: { period: 'none', totalUsed: 2 * 1024 * 1024 * 1024 }, status: 1 }]
  const rendered = view._render()
  const row = vnodeTree(rendered).find((node) => node.tag === 'tr' && (node.children || []).filter((child) => child.tag === 'td').length === 9)
  const cells = row.children.filter((node) => node.tag === 'td')
  assert.equal(vnodeText(cells[7]).trim(), '2.0GB')
  assert.equal(vnodeTree(cells[7]).some((node) => node.tag === 'button' || node.tag === 'liquid-button'), false)
  assert.equal(vnodeTree(rendered).some((node) => String(node.data && node.data.attrs && node.data.attrs.title).includes('重置')), false)
  const deploy = vnodeTree(cells[8]).find((node) => node.data && node.data.attrs && node.data.attrs.title === 'nodeDeployment.title')
  assert.equal(deploy.tag, 'button')
  assert.equal(deploy.data.staticClass, 'icon-btn')
  assert.equal(deploy.data.attrs['aria-label'], 'nodeDeployment.title')
  assert.equal(vnodeText(deploy).trim(), '')
  assert.ok(vnodeTree(deploy).some((node) => node.tag === 'app-icon' && node.data.attrs.name === 'download'))
  view.$destroy()
})

test('traffic reset renders in the edit dialog and uses the server identity rather than unsaved form data', () => {
  const Vue = require('vue/dist/vue.common.js')
  let allowed = true
  const descriptor = compiler.parse({ source: read('src/views/node-server/list/compoments/NodeServerForm.vue') })
  const component = loadModule(descriptor.script.content, {
    '@/api/node-server': {}, '@/utils/account': loadModule(read('src/utils/account.js'), {}), '@/utils/permission': () => allowed
  }).default
  const compiled = compileRender(descriptor.template.content)
  const Form = Vue.extend({ ...component, beforeCreate() { this.$t = (key) => key }, render: compiled.render, staticRenderFns: compiled.staticRenderFns })
  const server = { id: 42, name: 'Fixture Node', ip: 'fixture.example.com', grpcPort: 8100, grpcTlsServerName: 'fixture.example.com', trafficPeriod: 'none' }
  const props = { nodeServer: server, dialogStatus: 'update', dialogVisible: true, getList: () => {}, trafficStatus: { totalUsed: 1024 * 1024 }, resettingTraffic: false }
  const form = new Form({ propsData: props })
  const events = []
  form.$on('reset-traffic', (value) => events.push(value))
  const item = vnodeTree(form._render()).find((node) => node.tag === 'liquid-form-item' && node.data.attrs.label === 'dashboard.trafficUsed')
  assert.match(vnodeText(item), /1MB/)
  const button = vnodeTree(item).find((node) => node.tag === 'liquid-button')
  assert.equal(vnodeText(button).trim(), 'traffic.resetServer')
  form.form.id = 99
  button.data.on.click()
  assert.equal(events.length, 1)
  assert.equal(events[0].id, 42)
  form.trafficStatus = { totalUsed: 0 }
  assert.match(vnodeText(form._render()), /0KB/)
  assert.equal(form.form.id, 99, 'a statistics refresh preserves unsaved form fields')
  form.resettingTraffic = true
  form.resetTraffic()
  assert.equal(events.length, 1)
  form.resettingTraffic = false
  allowed = false
  const denied = new Form({ propsData: props })
  assert.equal(vnodeTree(denied._render()).some((node) => node.tag === 'liquid-button' && vnodeText(node).trim() === 'traffic.resetServer'), false)
  form.dialogStatus = 'create'
  form.resetTraffic()
  assert.equal(events.length, 1)
  assert.equal(vnodeTree(form._render()).some((node) => node.tag === 'liquid-form-item' && node.data.attrs.label === 'dashboard.trafficUsed'), false)
  denied.$destroy()
  form.$destroy()
})

test('traffic reset confirms once, preserves its target ID, and refreshes list and dialog statistics', async () => {
  const calls = [], notices = []
  let decision, failReset = false, allowed = true, confirmations = 0
  const component = loadModule(compiler.parse({ source: read('src/views/node-server/list/index.vue') }).script.content, new Proxy({}, {
    has: () => true,
    get: (_, name) => {
      if (name === '@/utils/permission') return () => allowed
      if (name === '@/utils/liquid-feedback') return { MessageBox: { confirm() { confirmations++; return new Promise((resolve, reject) => { decision = { resolve, reject } }) } } }
      if (name === '@/api/node-server') return { resetNodeServerTraffic: async (request) => { calls.push(request); if (failReset) throw new Error('offline') } }
      return {}
    }
  })).default
  const row = { id: 42, name: 'Fixture Node', trafficStatus: { totalUsed: 2048 } }
  const context = { resettingServerId: 0, dialogFormVisible: true, dialogStatus: 'update', temp: { ...row }, list: [row],
    $t: (key) => key, $notify: (notice) => notices.push(notice),
    async getList() { this.list = [{ id: 42, trafficStatus: { totalUsed: 0 } }] } }
  const reset = component.methods.handleResetServerTraffic
  let pending = reset.call(context, row)
  assert.equal(calls.length, 0)
  await reset.call(context, row)
  assert.equal(confirmations, 1, 'pending confirmation prevents duplicate submissions')
  decision.resolve()
  row.id = 99
  await pending
  assert.equal(calls[0].id, 42)
  assert.equal(component.computed.editingTrafficStatus.call(context).totalUsed, 0)
  assert.equal(context.resettingServerId, 0)
  assert.equal(notices.length, 1)
  row.id = 42
  pending = reset.call(context, row)
  decision.reject('cancel')
  await pending
  assert.equal(calls.length, 1, 'cancellation sends no API request')
  failReset = true
  pending = reset.call(context, row)
  decision.resolve()
  await pending
  assert.equal(notices.length, 1, 'failure cannot report success')
  assert.equal(context.resettingServerId, 0)
  for (const invalid of [{ id: 7 }, { id: 0 }, { id: '42' }]) await reset.call(context, invalid)
  context.dialogStatus = 'create'
  await reset.call(context, row)
  context.dialogStatus = 'update'
  allowed = false
  await reset.call(context, row)
  assert.equal(confirmations, 3, 'creation, wrong IDs, and denied permission never reset data')
})

function nodeFormFixture(api = {}) {
  const Vue = require('vue/dist/vue.common.js')
  const descriptor = compiler.parse({ source: read('src/views/node/list/components/NodeForm.vue') })
  const utils = loadModule(read('src/utils/node.js'), {})
  const component = loadModule(descriptor.script.content, new Proxy({}, {
    has: () => true,
    get: (_, name) => name === '@/api/node' ? api : name === '@/utils/node' ? utils : {}
  })).default
  const compiled = compileRender(descriptor.template.content)
  const Form = Vue.extend({ ...component, components: {},
    beforeCreate() { this.$t = (key) => key; this.$notify = () => {} },
    render: compiled.render, staticRenderFns: compiled.staticRenderFns })
  const list = loadModule(compiler.parse({ source: read('src/views/node/list/index.vue') }).script.content,
    new Proxy({}, { has: () => true, get: () => ({}) })).default
  const node = { ...list.data.call({ $t: (key) => key }).temp, id: 42, name: 'Mapped Node', domain: 'mapped.example.com', port: 8443, externalPort: 443 }
  return new Form({ propsData: { nodeProps: node, dialogStatusProps: 'update', dialogFormVisibleProps: true,
    nodeServersProps: [], nodeTypesProps: [], getListProps: () => {} } })
}

test('the existing node form restores forwarding, preserves its provided model, and validates both port ranges', async () => {
  const form = nodeFormFixture()
  const original = form.formModel
  assert.equal(form.portForwardingEnabled, true)
  const items = vnodeTree(form._render()).filter((node) => node.tag === 'liquid-form-item')
  const external = items.find((node) => node.data.attrs.prop === 'externalPort')
  assert.equal(external.data.attrs.label, 'table.nodeExternalPort')
  const actual = items.find((node) => node.data.attrs.prop === 'port')
  assert.equal(actual.data.attrs.label, 'table.nodeActualPort')
  assert.equal(vnodeTree(external).find((node) => node.tag === 'liquid-number-input').data.attrs.max, 65535)
  assert.equal(vnodeTree(actual).find((node) => node.tag === 'liquid-number-input').data.attrs.min, 101)
  const validate = (rule, value) => new Promise((resolve) => rule.validator(rule, value, resolve))
  for (const rules of [form.createRules, form.updateRules]) {
    for (const value of [101, 29999]) assert.equal(await validate(rules.port[1], value), undefined)
    for (const value of [100, 30000, 8443.5, '443', undefined]) assert.ok(await validate(rules.port[1], value))
    for (const value of [1, 65535]) assert.equal(await validate(rules.externalPort[0], value), undefined)
    for (const value of [0, 65536, 443.5, '443', undefined]) assert.ok(await validate(rules.externalPort[0], value))
  }
  form.nodeProps = { ...form.nodeProps, id: 43, externalPort: 0, port: 2443 }
  form.resetFormModel()
  assert.equal(form.formModel, original, 'protocol children retain the same provided model object')
  assert.equal(form.portForwardingEnabled, false)
  assert.equal(form.formModel.port, 2443)
  assert.equal(form.formModel.externalPort, 0)
  assert.equal(vnodeTree(form._render()).some((node) => node.data && node.data.attrs && node.data.attrs.prop === 'externalPort'), false)
  assert.equal(vnodeTree(form._render()).find((node) => node.data && node.data.attrs && node.data.attrs.prop === 'port').data.attrs.label, 'table.nodePort')
  assert.equal(await validate(form.updateRules.externalPort[0], undefined), undefined, 'disabled forwarding does not require a hidden port')
  form.portForwardingEnabled = true
  form.handlePortForwardingChange(true)
  assert.equal(form.formModel.externalPort, undefined, 'new forwarding requires the user to enter its external port')
  form.$destroy()
})

test('node creation and editing submit actual and external ports separately and block invalid ports', async () => {
  const Vue = require('vue/dist/vue.common.js')
  const requests = []
  const form = nodeFormFixture({
    createNode: async (payload) => { requests.push({ action: 'create', ...payload }) },
    updateNodeById: async (payload) => { requests.push({ action: 'update', ...payload }) }
  })
  const item = loadModule(read('src/components/LiquidStructural/index.js'), { vue: Vue }).LiquidFormItem
  form.$refs.dataForm = {
    async validate(callback) {
      const rules = form.dialogStatusProps === 'create' ? form.createRules : form.updateRules
      const fields = form.portForwardingEnabled ? ['port', 'externalPort'] : ['port']
      const results = await Promise.all(fields.map((field) => item.methods.validate.call({
        appliedRules: rules[field], value: form.formModel[field], error: ''
      })))
      callback(results.every(Boolean))
    }
  }
  const submit = async (action) => { form[action](); await new Promise((resolve) => setImmediate(resolve)) }
  form.dialogStatusProps = 'create'
  await submit('createData')
  assert.equal(requests[0].action, 'create')
  assert.equal(requests[0].port, 8443)
  assert.equal(requests[0].externalPort, 443)
  assert.equal('portForwardingEnabled' in requests[0], false, 'the switch is local UI state')
  form.dialogStatusProps = 'update'
  form.portForwardingEnabled = false
  await submit('updateData')
  assert.equal(requests[1].id, 42)
  assert.equal(requests[1].port, 8443)
  assert.equal(requests[1].externalPort, 0)
  form.portForwardingEnabled = true
  form.formModel.externalPort = 65535
  form.formModel.port = 29999
  await submit('updateData')
  assert.equal(requests[2].externalPort, 65535)
  assert.equal(requests[2].port, 29999)
  form.formModel.externalPort = 65536
  await submit('updateData')
  form.formModel.externalPort = 443
  form.formModel.port = 100
  await submit('createData')
  assert.equal(requests.length, 3, 'invalid actual or external ports cannot reach either API')
  form.$destroy()
})

test('node cards and details show the public endpoint and its actual listener without changing plain nodes', () => {
  const Vue = require('vue/dist/vue.common.js')
  const utils = loadModule(read('src/utils/node.js'), {})
  const descriptor = compiler.parse({ source: read('src/views/node/list/index.vue') })
  const component = loadModule(descriptor.script.content, new Proxy({}, {
    has: () => true,
    get: (_, name) => {
      if (name === '@/utils/node') return utils
      if (name === '@/utils/permission') return () => true
      if (name === '@/utils/account') return loadModule(read('src/utils/account.js'), {})
      if (name === '@/utils') return { timeStampToDate: (value) => value }
      return {}
    }
  })).default
  const compiled = compileRender(descriptor.template.content)
  const translations = loadModule(read('src/lang/zh.js'), {}).default.table
  const View = Vue.extend({ ...component, components: {}, mixins: [], created: undefined,
    beforeCreate() {
      this.$t = (key, params = {}) => String(translations[key.replace(/^table\./, '')] || key)
        .replace('{external}', params.external).replace('{actual}', params.actual)
    },
    render: compiled.render, staticRenderFns: compiled.staticRenderFns })
  const view = new View()
  view.nodeTypes = [{ id: 1, name: 'Xray' }]
  view.nodeServers = [{ id: 1, name: 'Fixture server' }]
  view.listLoading = false
  view.list = [
    { id: 1, nodeServerId: 1, nodeTypeId: 1, name: 'Mapped', domain: 'mapped.example.com', port: 8443, externalPort: 443 },
    { id: 2, nodeServerId: 1, nodeTypeId: 1, name: 'Plain', domain: 'plain.example.com', port: 2443, externalPort: 0 }
  ]
  view.nodeDetail = { ...view.list[0] }
  view.detailId = 1
  const rendered = view._render()
  assert.match(vnodeText(rendered), /mapped\.example\.com:443/)
  assert.match(vnodeText(rendered), /plain\.example\.com:2443/)
  assert.match(vnodeText(rendered), /端口 443 → 8443/)
  const details = vnodeTree(rendered).filter((node) => node.data && node.data.staticClass === 'kv')
  assert.ok(details.some((node) => vnodeText(node).trim() === '对外端口443'))
  assert.ok(details.some((node) => vnodeText(node).trim() === '实际端口8443'))
  const node = { nodeTypeId: 0, port: 8443, externalPort: 443 }
  utils.handleNodeDetail(node, { externalPort: 0 })
  assert.equal(utils.nodeConnectionPort(node), 8443)
  utils.handleNodeUpdate(node, { externalPort: 443 })
  assert.equal(utils.nodeConnectionPort(node), 443)
  view.$destroy()
})

function accountRemarkFixture(api = {}, initialRoles = ['sysadmin']) {
  const Vue = require('vue/dist/vue.common.js')
  const roles = Vue.observable({ value: initialRoles })
  const descriptor = compiler.parse({ source: read('src/views/account/list/index.vue') })
  const component = loadModule(descriptor.script.content, new Proxy({}, {
    has: () => true,
    get: (_, name) => {
      if (name === '@/api/account') return { selectAccountPage: async () => ({ data: { accounts: [], total: 0 } }), ...api }
      if (name === '@/utils/permission') return (wanted) => wanted.some((role) => roles.value.includes(role))
      if (name === '@/utils/account') return loadModule(read('src/utils/account.js'), {})
      if (name === '@/utils') return { timeStampToDate: () => '2026-10-02 12:00' }
      if (name === '@/mixins/latest-list-request') return loadModule(read('src/mixins/latest-list-request.js'), {}).default
      if (name === 'copy-to-clipboard') return () => true
      return {}
    }
  })).default
  const compiled = compileRender(descriptor.template.content)
  const locale = loadModule(read('src/lang/zh.js'), {}).default
  const View = Vue.extend({ ...component, components: {}, created: undefined,
    beforeCreate() {
      this.$t = (key) => key.split('.').reduce((result, part) => result && result[part], locale) || key
      this.$notify = () => {}
    },
    render: compiled.render, staticRenderFns: compiled.staticRenderFns })
  const view = new View()
  view.listLoading = false
  view.list = []
  view.roleList = [{ id: 3, desc: 'User' }]
  return { view, roles, Vue }
}

test('account remarks are sysadmin-only escaped text and empty states match the visible columns', async () => {
  const { view, roles, Vue } = accountRemarkFixture()
  const privateText = '<img src=x onerror=alert(1)>\n<script>window.privateLeak=1</script>'
  view.list = [{ id: 42, username: 'sampleuser', roleId: 3, deleted: 0, lastLoginTime: 0, quota: 1048576, remark: privateText }]
  view.temp = { ...view.list[0] }
  view.dialogStatus = 'update'
  view.dialogFormVisible = true
  let rendered = view._render()
  const cell = vnodeTree(rendered).find((node) => node.data && node.data.staticClass === 'account-remark-cell')
  assert.equal(vnodeText(cell).trim(), privateText)
  assert.equal(vnodeTree(cell).some((node) => node.tag === 'img' || node.tag === 'script'), false, 'markup stays in text VNodes')
  assert.equal(vnodeTree(cell).some((node) => node.data && node.data.domProps && node.data.domProps.innerHTML), false)
  const field = vnodeTree(rendered).find((node) => node.data && node.data.attrs && node.data.attrs.prop === 'remark')
  assert.equal(field.data.attrs.label, '备注')
  assert.equal(vnodeTree(field).find((node) => node.tag === 'liquid-input').data.attrs.type, 'textarea')
  assert.equal(vnodeTree(field).some((node) => node.data && node.data.attrs && node.data.attrs.maxlength), false, 'Unicode length uses the existing validator')
  assert.equal(vnodeTree(rendered).filter((node) => node.tag === 'th').length, 8)
  view.dialogStatus = 'create'
  assert.equal(vnodeTree(view._render()).some((node) => node.data && node.data.attrs && node.data.attrs.prop === 'remark'), false)
  view.dialogStatus = 'update'
  roles.value = ['admin', 'user']
  await Vue.nextTick()
  assert.equal('remark' in view.temp, false)
  assert.equal('remark' in view.list[0], false)
  assert.equal(view.dialogFormVisible, false, 'losing privilege closes the private editor')
  rendered = view._render()
  assert.equal(vnodeText(rendered).includes(privateText), false)
  assert.equal(vnodeTree(rendered).filter((node) => node.tag === 'th').length, 7)
  assert.equal(vnodeTree(rendered).some((node) => node.data && node.data.attrs && node.data.attrs.prop === 'remark'), false)
  for (const role of ['sysadmin', 'admin', 'user']) {
    roles.value = [role]
    view.list = []
    view.listError = role === 'admin' ? 'request failed' : ''
    await Vue.nextTick()
    const tree = vnodeTree(view._render())
    const count = tree.filter((node) => node.tag === 'th').length
    assert.equal(count, role === 'sysadmin' ? 8 : 7)
    assert.equal(tree.find((node) => node.data && node.data.attrs && node.data.attrs.colspan).data.attrs.colspan, count)
  }
  view.$destroy()
})

test('account fetches and editors discard private fields for non-sysadmin, including a response after role loss', async () => {
  let complete
  const { view, roles, Vue } = accountRemarkFixture({ selectAccountPage: () => new Promise((resolve) => { complete = resolve }) })
  const row = { id: 42, username: 'sampleuser', quota: 1048576, remark: 'private account note' }
  view.$refs.dataForm = { clearValidate() {} }
  view.handleUpdate(row)
  assert.equal(view.temp.remark, row.remark)
  assert.equal(view.temp.quota, 1)
  const pending = view.getList()
  roles.value = ['admin']
  await Vue.nextTick()
  complete({ data: { accounts: [row], total: 1 } })
  await pending
  assert.equal('remark' in view.list[0], false)
  view.handleUpdate(row)
  assert.equal('remark' in view.temp, false, 'opening a stale privileged row cannot expose its note')
  view.resetTemp()
  assert.equal(view.temp.remark, undefined)
  assert.equal(row.remark, 'private account note', 'sanitization does not mutate the source response')
  view.$destroy()
})

test('account editing saves and clears remarks while omitted values preserve notes and other accounts', async () => {
  const requests = []
  const { view, roles, Vue } = accountRemarkFixture({
    updateAccountById: async (payload) => { requests.push({ action: 'update', ...payload }) },
    createAccount: async (payload) => { requests.push({ action: 'create', ...payload }) }
  })
  view.list = [
    { id: 42, username: 'sampleuser', quota: 1048576, remark: 'existing note' },
    { id: 43, username: 'otheruser', quota: 1048576, remark: 'other private note' }
  ]
  view.$refs.dataForm = { clearValidate() {}, validate(callback) { callback(true) } }
  const submit = async (action = 'updateData') => { view[action](); await new Promise((resolve) => setImmediate(resolve)) }
  view.handleUpdate(view.list[0])
  view.temp.remark = 'new note\nwith detail'
  await submit()
  assert.equal(requests[0].id, 42)
  assert.equal(requests[0].remark, 'new note\nwith detail')
  assert.equal(requests[0].quota, 1)
  assert.equal(view.list[0].remark, requests[0].remark)
  assert.equal(view.list[0].quota, 1048576)
  for (const preserved of [undefined, null]) {
    view.handleUpdate(view.list[0])
    view.temp.remark = preserved
    await submit()
    assert.equal('remark' in requests.at(-1), false)
    assert.equal(view.list[0].remark, 'new note\nwith detail')
  }
  view.handleUpdate(view.list[0])
  view.temp.remark = ''
  await submit()
  assert.equal(requests.at(-1).remark, '')
  assert.equal(view.list[0].remark, '')
  assert.equal(view.list[1].remark, 'other private note')
  roles.value = ['admin']
  await Vue.nextTick()
  view.temp.remark = 'stale private note'
  await submit()
  assert.equal('remark' in requests.at(-1), false, 'a stale value cannot be sent without sysadmin privilege')
  assert.equal('remark' in view.list[0], false)
  roles.value = ['sysadmin']
  await Vue.nextTick()
  view.getList = () => {}
  view.resetTemp()
  view.temp.remark = 'stale value from previous editing'
  await submit('createData')
  assert.equal(requests.at(-1).action, 'create')
  assert.equal('remark' in requests.at(-1), false, 'creation never exposes the private editing field')
  view.$destroy()
})

test('account remark validation counts Unicode characters and blocks overlong saves through the Liquid form rule', async () => {
  const requests = []
  const { view, Vue } = accountRemarkFixture({ updateAccountById: async (payload) => { requests.push(payload) } })
  const field = loadModule(read('src/components/LiquidStructural/index.js'), { vue: Vue }).LiquidFormItem
  view.list = [{ id: 42, username: 'sampleuser', quota: 1048576, remark: '' }]
  view.temp = { ...view.list[0], quota: 1 }
  view.$refs.dataForm = {
    async validate(callback) {
      callback(await field.methods.validate.call({ appliedRules: view.updateRules.remark, value: view.temp.remark, error: '' }))
    }
  }
  const submit = async () => { view.updateData(); await new Promise((resolve) => setImmediate(resolve)) }
  for (const value of ['', '字'.repeat(500), '😀'.repeat(500)]) {
    view.temp.remark = value
    await submit()
    assert.equal(requests.at(-1).remark, value)
  }
  assert.equal(requests.length, 3, '500 emoji characters are allowed despite 1000 UTF-16 code units')
  for (const value of ['字'.repeat(501), '😀'.repeat(501), 7]) {
    view.temp.remark = value
    await submit()
  }
  assert.equal(requests.length, 3, 'invalid remarks cannot reach the API')
  view.$destroy()
})
