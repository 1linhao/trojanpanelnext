import Vue from 'vue'

const px = (value) => {
  if (value === undefined || value === null || value === '') return undefined
  return typeof value === 'number' || /^\d+$/.test(String(value))
    ? `${value}px`
    : value
}

const getValue = (object, path) => {
  if (!path) return object
  return String(path)
    .replace(/\[(\w+)\]/g, '.$1')
    .split('.')
    .filter(Boolean)
    .reduce((value, key) => (value == null ? value : value[key]), object)
}

const isEmptyValue = (value) =>
  value === '' || value === null || value === undefined ||
  (Array.isArray(value) && !value.length)

// WEB-022: min/max must compare against the rule type, not String(value).length.
// Numbers with type 'number' validate range; strings validate character count;
// arrays validate element count. Values that cannot be parsed as a number fail
// a 'number' rule instead of silently passing.
const validateRange = (rule, value) => {
  if (rule.type === 'number' || typeof value === 'number') {
    const numeric = typeof value === 'number' ? value : Number(value)
    if (!Number.isFinite(numeric)) return rule.message || '请输入数字'
    if (rule.min != null && numeric < rule.min) return rule.message || `不能小于 ${rule.min}`
    if (rule.max != null && numeric > rule.max) return rule.message || `不能大于 ${rule.max}`
    return ''
  }
  if (Array.isArray(value)) {
    if (rule.min != null && value.length < rule.min) return rule.message || `至少选择 ${rule.min} 项`
    if (rule.max != null && value.length > rule.max) return rule.message || `最多选择 ${rule.max} 项`
    return ''
  }
  const length = String(value).length
  if (rule.min != null && length < rule.min) return rule.message || `至少输入 ${rule.min} 个字符`
  if (rule.max != null && length > rule.max) return rule.message || `最多输入 ${rule.max} 个字符`
  return ''
}

export const LiquidTableColumn = {
  name: 'LiquidTableColumn',
  props: {
    prop: String,
    label: [String, Number],
    width: [String, Number],
    minWidth: [String, Number],
    align: String,
    type: String
  },
  created() {
    let table = this.$parent
    while (table && table.$options.name !== 'LiquidTable') table = table.$parent
    this.liquidTable = table
    if (table) table.addColumn(this)
  },
  beforeDestroy() { if (this.liquidTable) this.liquidTable.removeColumn(this) },
  render(h) { return h('span', { class: 'liquid-table-column-definition' }) }
}

export const LiquidTable = {
  name: 'LiquidTable',
  props: {
    data: { type: Array, default: () => [] },
    border: Boolean
  },
  data: () => ({ columns: [] }),
  methods: {
    addColumn(column) {
      if (!this.columns.includes(column)) this.columns.push(column)
    },
    removeColumn(column) { this.columns = this.columns.filter((item) => item !== column) }
  },
  render(h) {
    const columns = this.columns.map((column) => ({
      prop: column.prop,
      label: column.label,
      width: column.width,
      minWidth: column.minWidth,
      align: column.align,
      type: column.type,
      slot: column.$scopedSlots.default
    }))
    const colgroup = h('colgroup', columns.map((column) => h('col', {
      style: { width: px(column.width), minWidth: px(column.minWidth) }
    })))
    const head = h('thead', [h('tr', columns.map((column) => h('th', {
      style: { textAlign: column.align || 'left', width: px(column.width), minWidth: px(column.minWidth) }
    }, [String(column.label == null ? '' : column.label)])))])
    const rows = this.data && this.data.length
      ? this.data.map((row, index) => h('tr', { key: row.id == null ? index : row.id }, columns.map((column) => {
        let content
        if (column.slot) content = column.slot({ row, $index: index, column })
        else if (column.type === 'index') content = String(index + 1)
        else {
          const value = getValue(row, column.prop)
          content = value == null ? '' : String(value)
        }
        return h('td', { style: { textAlign: column.align || 'left' } }, Array.isArray(content) ? content : [content])
      })))
      : [h('tr', { class: 'liquid-table__empty-row' }, [h('td', { attrs: { colspan: Math.max(columns.length, 1) } }, ['暂无数据'])])]
    return h('div', { class: ['liquid-table', { 'is-bordered': this.border }] }, [
      h('div', { class: 'liquid-table__definitions', attrs: { 'aria-hidden': 'true' } }, this.$slots.default),
      h('div', { class: 'liquid-table__scroll' }, [h('table', [colgroup, head, h('tbody', rows)])])
    ])
  }
}

export const LiquidForm = {
  name: 'LiquidForm',
  componentName: 'LiquidForm',
  props: {
    model: Object,
    rules: Object,
    labelWidth: String,
    labelPosition: { type: String, default: 'right' }
  },
  provide() { return { liquidForm: this } },
  data: () => ({ fields: [] }),
  methods: {
    addField(field) { if (!this.fields.includes(field)) this.fields.push(field) },
    removeField(field) { this.fields = this.fields.filter((item) => item !== field) },
    clearValidate(props) {
      const selected = props ? (Array.isArray(props) ? props : [props]) : null
      this.fields.forEach((field) => { if (!selected || selected.includes(field.prop)) field.clearValidate() })
    },
    resetFields() { this.fields.forEach((field) => field.resetField()) },
    validate(callback) {
      return Promise.all(this.fields.map((field) => field.validate())).then((results) => {
        const valid = results.every(Boolean)
        if (callback) callback(valid)
        return valid
      }).catch(() => {
        if (callback) callback(false)
        return false
      })
    }
  },
  render(h) {
    return h('form', {
      class: ['liquid-form', `is-label-${this.labelPosition}`],
      on: { submit: (event) => event.preventDefault() }
    }, this.$slots.default)
  }
}

export const LiquidFormItem = {
  name: 'LiquidFormItem',
  componentName: 'LiquidFormItem',
  inject: { liquidForm: { default: null } },
  props: { label: [String, Number], prop: String, rules: [Object, Array], labelWidth: String },
  provide() { return { liquidFormItem: this } },
  data: () => ({ error: '', initialValue: undefined, controls: [], nativeControl: null }),
  computed: {
    labelId() { return `liquid-form-label-${this._uid}` },
    errorId() { return `liquid-form-error-${this._uid}` },
    labelTarget() { return this.controls[0]?.controlAttrs?.id || this.nativeControl?.id },
    value() { return this.liquidForm && this.prop ? getValue(this.liquidForm.model, this.prop) : undefined },
    appliedRules() {
      const formRules = this.liquidForm && this.liquidForm.rules && this.liquidForm.rules[this.prop]
      const rules = this.rules || formRules || []
      return Array.isArray(rules) ? rules : [rules]
    }
  },
  mounted() {
    this.initialValue = this.value
    if (this.liquidForm && this.prop) this.liquidForm.addField(this)
    this.$on('liquid.form.change', () => this.validate('change'))
    this.$on('liquid.form.blur', () => this.validate('blur'))
    this.bindNativeControl()
  },
  updated() { this.bindNativeControl(); this.syncNativeControlAttrs() },
  beforeDestroy() {
    this.unbindNativeControl()
    if (this.liquidForm) this.liquidForm.removeField(this)
  },
  methods: {
    addControl(control) { if (!this.controls.includes(control)) this.controls.push(control) },
    removeControl(control) { this.controls = this.controls.filter((item) => item !== control) },
    bindNativeControl() {
      const control = this.$el?.querySelector('input:not([type="hidden"]), textarea, select')
      if (control === this.nativeControl) return
      this.unbindNativeControl()
      if (!control) return
      this.nativeControl = control
      if (!control.id) control.id = `liquid-native-control-${this._uid}`
      this._nativeChange = () => this.validate('change').then(() => this.syncNativeControlAttrs())
      this._nativeBlur = () => this.validate('blur').then(() => this.syncNativeControlAttrs())
      control.addEventListener('change', this._nativeChange)
      control.addEventListener('blur', this._nativeBlur)
      this.syncNativeControlAttrs()
    },
    unbindNativeControl() {
      if (!this.nativeControl) return
      this.nativeControl.removeEventListener('change', this._nativeChange)
      this.nativeControl.removeEventListener('blur', this._nativeBlur)
      this.nativeControl = null
    },
    syncNativeControlAttrs() {
      const control = this.nativeControl
      if (!control) return
      if (this.label != null && !control.hasAttribute('aria-label')) control.setAttribute('aria-labelledby', this.labelId)
      if (this.error) {
        control.setAttribute('aria-invalid', 'true')
        const ids = new Set((control.getAttribute('aria-describedby') || '').split(/\s+/).filter(Boolean))
        ids.add(this.errorId)
        control.setAttribute('aria-describedby', Array.from(ids).join(' '))
      } else {
        control.removeAttribute('aria-invalid')
        const ids = (control.getAttribute('aria-describedby') || '').split(/\s+/).filter((id) => id && id !== this.errorId)
        if (ids.length) control.setAttribute('aria-describedby', ids.join(' '))
        else control.removeAttribute('aria-describedby')
      }
    },
    clearValidate() { this.error = '' },
    resetField() {
      if (!this.liquidForm || !this.prop) return
      const segments = this.prop.replace(/\[(\w+)\]/g, '.$1').split('.')
      const key = segments.pop()
      const target = segments.reduce((value, segment) => value && value[segment], this.liquidForm.model)
      if (target) Vue.set(target, key, this.initialValue)
      this.clearValidate()
    },
    async validate(trigger) {
      const rules = this.appliedRules.filter((rule) => !trigger || !rule.trigger || rule.trigger === trigger || (Array.isArray(rule.trigger) && rule.trigger.includes(trigger)))
      for (const rule of rules) {
        const empty = isEmptyValue(this.value)
        let error = ''
        if (rule.required && empty) error = rule.message || '此项为必填项'
        else if (!empty) error = validateRange(rule, this.value)
        if (!error && rule.pattern && !empty && !rule.pattern.test(String(this.value))) error = rule.message || '格式不正确'
        else if (!error && rule.validator) {
          error = await new Promise((resolve) => {
            let settled = false
            const done = (reason) => { if (!settled) { settled = true; resolve(reason ? (reason.message || String(reason)) : '') } }
            try {
              const result = rule.validator(rule, this.value, done)
              if (result && typeof result.then === 'function') result.then(() => done()).catch(done)
            } catch (reason) { done(reason) }
          })
        }
        if (error) { this.error = error; return false }
      }
      this.error = ''
      return true
    }
  },
  render(h) {
    return h('div', { class: ['liquid-form-item', { 'is-error': this.error }] }, [
      this.label != null ? h('label', { class: 'liquid-form-item__label', attrs: { id: this.labelId, for: this.labelTarget } }, [String(this.label)]) : null,
      h('div', { class: 'liquid-form-item__content' }, [
        ...(this.$slots.default || []),
        this.error ? h('div', { class: 'liquid-form-item__error', attrs: { id: this.errorId, role: 'status' } }, [this.error]) : null
      ])
    ])
  }
}

export const LiquidDescriptionsItem = { name: 'LiquidDescriptionsItem', functional: true, props: { label: [String, Number] }, render: () => null }
export const LiquidDescriptions = {
  name: 'LiquidDescriptions',
  props: { column: { type: Number, default: 3 }, border: Boolean },
  render(h) {
    const items = (this.$slots.default || []).filter((vnode) => vnode && vnode.componentOptions).map((vnode) => ({
      label: vnode.componentOptions.propsData && vnode.componentOptions.propsData.label,
      content: vnode.componentOptions.children || []
    }))
    return h('dl', { class: ['liquid-descriptions', { 'is-bordered': this.border }], style: { gridTemplateColumns: `repeat(${this.column}, minmax(0, 1fr))` } }, items.map((item) => h('div', { class: 'liquid-descriptions__item' }, [h('dt', [String(item.label || '')]), h('dd', item.content)])))
  }
}

export const structuralComponents = {
  LiquidDescriptions, LiquidDescriptionsItem, LiquidForm, LiquidFormItem,
  LiquidTable, LiquidTableColumn
}
