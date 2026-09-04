import { BUTTON_INTERACTION } from './button-interactions.js'
import { acquireOverlay } from './overlay-stack.js'
import {
  isDensity,
  isMotionRole,
  isSize,
  isState,
  isSurface,
  isTone
} from '@tp-ui/contracts'

export {
  BUTTON_INTERACTION,
  createButtonInteractionController,
  createCssButtonInteractionAdapter
} from './button-interactions.js'

function semanticData(component, overrides = {}) {
  const state = component.disabled
    ? 'disabled'
    : component.loading
    ? 'loading'
    : component.invalid
    ? 'invalid'
    : 'idle'
  return {
    'data-ui-surface': overrides.surface || component.surface || 'control',
    'data-ui-tone': component.tone || 'neutral',
    'data-ui-state': state,
    'data-ui-size': component.size || 'md'
  }
}

export const PANEL_VARIANTS = Object.freeze(['auth', 'content', 'metric'])

const sharedSurfaceProps = {
  tone: { type: String, default: 'neutral', validator: isTone },
  density: { type: String, default: 'comfortable', validator: isDensity },
  state: { type: String, default: 'idle', validator: isState },
  motionRole: { type: String, default: 'panel', validator: isMotionRole },
  motionKey: { type: String, default: '' }
}

function motionName(key) {
  if (!key) return undefined
  const safeKey = String(key)
    .trim()
    .replace(/[^a-zA-Z0-9_-]+/g, '-')
    .replace(/^-+/, '')
  return safeKey ? `ui-${safeKey}` : undefined
}

function surfaceData(context, { surface, classes }) {
  const props = context.props
  const attrs = {
    ...(context.data.attrs || {}),
    'data-ui-surface': surface,
    'data-ui-tone': props.tone,
    'data-ui-density': props.density,
    'data-ui-state': props.state,
    'data-ui-motion-role': props.motionRole,
    'data-ui-motion-key': props.motionKey || null
  }
  const name = motionName(props.motionKey)
  return {
    ...context.data,
    attrs,
    class: [classes, context.data.class],
    // Keep animation identity inert until the motion adapter opts into capture.
    // A permanent view-transition-name creates a backdrop root, isolating glass.
    style: [
      context.data.style,
      name ? { '--ui-view-transition-name': name } : null
    ]
  }
}

function surfaceChildren(h, context, prefix) {
  const slots = context.slots()
  return [
    slots.header
      ? h(
          'header',
          {
            class: `${prefix}__header`,
            attrs: { 'data-ui-part': 'header' }
          },
          slots.header
        )
      : null,
    h(
      'div',
      {
        class: `${prefix}__body`,
        attrs: { 'data-ui-part': 'body' }
      },
      slots.default
    ),
    slots.footer
      ? h(
          'footer',
          {
            class: `${prefix}__footer`,
            attrs: { 'data-ui-part': 'footer' }
          },
          slots.footer
        )
      : null
  ]
}

export const UiPanel = {
  name: 'UiPanel',
  functional: true,
  props: {
    ...sharedSurfaceProps,
    variant: {
      type: String,
      default: 'content',
      validator: (value) => PANEL_VARIANTS.includes(value)
    },
    tag: { type: String, default: 'section' }
  },
  render(h, context) {
    const { variant, tag } = context.props
    const data = surfaceData(context, {
      surface: variant === 'auth' ? 'raised' : 'panel',
      classes: ['tp-ui-panel', `tp-ui-panel--${variant}`]
    })
    data.attrs['data-ui-component'] = 'panel'
    data.attrs['data-ui-panel-variant'] = variant
    return h(tag, data, surfaceChildren(h, context, 'tp-ui-panel'))
  }
}

export const UiSheet = {
  name: 'UiSheet',
  functional: true,
  props: {
    ...sharedSurfaceProps,
    tag: { type: String, default: 'aside' }
  },
  render(h, context) {
    const data = surfaceData(context, {
      surface: 'raised',
      classes: ['tp-ui-sheet']
    })
    data.attrs['data-ui-component'] = 'sheet'
    return h(
      context.props.tag,
      data,
      surfaceChildren(h, context, 'tp-ui-sheet')
    )
  }
}

export const UiDialog = {
  name: 'UiDialog',
  // The composition root supplies icons; standalone consumers keep a text close action.
  props: {
    visible: Boolean,
    title: [String, Number],
    width: { type: [String, Number], default: '50%' },
    customClass: String,
    closeOnClickModal: { type: Boolean, default: true },
    renderIcon: { type: Function, default: null },
    labels: {
      type: Object,
      default: () => ({ close: 'Close dialog' })
    },
    closeOnEscape: { type: Boolean, default: true },
    showClose: { type: Boolean, default: true },
    appendToBody: { type: Boolean, default: true },
    role: { type: String, default: 'dialog' },
    describedBy: String,
    tone: { type: String, default: 'neutral', validator: isTone },
    motionRole: { type: String, default: 'overlay', validator: isMotionRole },
    motionKey: { type: String, default: '' }
  },
  watch: {
    visible(value) {
      if (value) this.open()
      else this.restoreFocus()
    }
  },
  mounted() {
    if (this.visible) this.open()
  },
  beforeDestroy() {
    this.restoreFocus()
    if (this.appendToBody) this.$el?.remove()
  },
  methods: {
    open() {
      this.$nextTick(() => {
        if (!this.visible || this._releaseOverlay) return
        const dialog = this.$refs.dialog
        if (!dialog) return
        if (this.appendToBody) dialog.ownerDocument.body.appendChild(this.$el)
        this._releaseOverlay = acquireOverlay(dialog, {
          close: this.close,
          closeOnEscape: this.closeOnEscape
        })
        dialog.focus()
        this.$emit('open')
      })
    },
    close() {
      this.$emit('update:visible', false)
      this.$emit('close')
    },
    modalClick(event) {
      if (this.closeOnClickModal && event.target === event.currentTarget)
        this.close()
    },
    restoreFocus() {
      this._releaseOverlay?.()
      this._releaseOverlay = null
    }
  },
  render(h) {
    if (!this.visible) return null
    const width =
      typeof this.width === 'number' || /^\d+$/.test(String(this.width))
        ? `${this.width}px`
        : this.width
    const name = motionName(this.motionKey)
    return h(
      'div',
      {
        class: 'tp-ui-dialog-layer',
        attrs: {
          'data-ui-overlay': 'modal',
          'data-ui-motion-role': this.motionRole,
          'data-ui-motion-key': this.motionKey || null
        },
        on: { mousedown: this.modalClick }
      },
      [
        h(
          'section',
          {
            ref: 'dialog',
            class: ['tp-ui-dialog', this.customClass],
            style: {
              '--ui-dialog-width': width,
              '--ui-view-transition-name': name
            },
            attrs: {
              role: this.role,
              tabindex: '-1',
              'aria-modal': 'true',
              'aria-labelledby': `ui-dialog-title-${this._uid}`,
              'aria-describedby': this.describedBy || null,
              'data-ui-surface': 'overlay',
              'data-ui-component': 'dialog',
              'data-ui-tone': this.tone,
              'data-ui-part': 'surface'
            }
          },
          [
            h(
              'header',
              {
                class: 'tp-ui-dialog__header',
                attrs: { 'data-ui-part': 'header' }
              },
              [
                h(
                  'span',
                  {
                    class: 'tp-ui-dialog__title',
                    attrs: { id: `ui-dialog-title-${this._uid}` }
                  },
                  [String(this.title == null ? '' : this.title)]
                ),
                this.showClose
                  ? h(
                      'button',
                      {
                        class: 'tp-ui-dialog__close',
                        attrs: {
                          type: 'button',
                          'aria-label': this.labels.close,
                          'data-ui-part': 'close-action'
                        },
                        on: { click: this.close }
                      },
                      [
                        this.renderIcon
                          ? this.renderIcon(h, 'close')
                          : this.labels.close
                      ]
                    )
                  : null
              ]
            ),
            h(
              'div',
              {
                class: 'tp-ui-dialog__body',
                attrs: { 'data-ui-part': 'body' }
              },
              this.$slots.default
            ),
            this.$slots.footer
              ? h(
                  'footer',
                  {
                    class: 'tp-ui-dialog__footer',
                    attrs: { 'data-ui-part': 'footer' }
                  },
                  this.$slots.footer
                )
              : null
          ]
        )
      ]
    )
  }
}

export const UiButton = {
  name: 'UiButton',
  inheritAttrs: false,
  props: {
    tone: { type: String, default: 'neutral', validator: isTone },
    surface: { type: String, default: 'control', validator: isSurface },
    size: { type: String, default: 'md', validator: isSize },
    disabled: Boolean,
    loading: Boolean,
    type: { type: String, default: 'button' }
  },
  render(h) {
    const label =
      this.loading && this.$scopedSlots.loading
        ? this.$scopedSlots.loading()
        : this.$slots.default
    return h(
      'button',
      {
        class: 'tp-ui-button',
        attrs: {
          ...this.$attrs,
          ...semanticData(this),
          [BUTTON_INTERACTION.attribute]: BUTTON_INTERACTION.variant,
          type: this.type,
          disabled: this.disabled || this.loading,
          'aria-busy': this.loading ? 'true' : null
        },
        on: {
          ...this.$listeners,
          click: (event) => {
            if (!this.disabled && !this.loading) this.$emit('click', event)
          }
        }
      },
      label
    )
  }
}

export const UiInput = {
  name: 'UiInput',
  inheritAttrs: false,
  props: {
    value: { type: [String, Number], default: '' },
    tone: { type: String, default: 'neutral', validator: isTone },
    size: { type: String, default: 'md', validator: isSize },
    disabled: Boolean,
    invalid: Boolean
  },
  render(h) {
    return h('input', {
      class: 'tp-ui-input',
      attrs: {
        ...this.$attrs,
        ...semanticData(this, { surface: 'control' }),
        disabled: this.disabled,
        'aria-invalid': this.invalid ? 'true' : null
      },
      domProps: { value: this.value },
      on: {
        ...this.$listeners,
        input: (event) => this.$emit('input', event.target.value)
      }
    })
  }
}

export const COMPONENTS = Object.freeze({
  UiButton,
  UiInput,
  UiPanel,
  UiSheet,
  UiDialog
})

export function createVue2Components({
  include = [],
  renderIcon = null,
  dialogLabels = null
} = {}) {
  const unknown = include.filter((name) => !COMPONENTS[name])
  if (unknown.length)
    throw new TypeError(`Unknown UI components: ${unknown.join(', ')}`)
  return Object.freeze({
    install(Vue) {
      include.forEach((name) => {
        const component = COMPONENTS[name]
        Vue.component(
          name,
          name === 'UiDialog' && (renderIcon || dialogLabels)
            ? {
                ...component,
                props: {
                  ...component.props,
                  ...(renderIcon
                    ? { renderIcon: { type: Function, default: renderIcon } }
                    : {}),
                  ...(dialogLabels
                    ? { labels: { type: Object, default: () => dialogLabels } }
                    : {})
                }
              }
            : component
        )
      })
    }
  })
}
