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
