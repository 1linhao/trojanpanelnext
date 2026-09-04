import Vue from 'vue'
import { UiDialog } from '@tp-ui/components-vue2'
import { renderIcon } from '@tp-ui/icons'
import { afterTransition } from '@tp-ui/motion-native'
import LiquidButton from '@/components/LiquidButton'
import LiquidInput from '@/components/LiquidInput'

const ensureHost = (className) => {
  let host = document.querySelector(`.${className}`)
  if (!host) {
    host = document.createElement('div')
    host.className = className
    document.body.appendChild(host)
  }
  return host
}

export function Message(options) {
  const normalized = typeof options === 'string' ? { message: options } : options || {}
  const item = document.createElement('div')
  item.className = `liquid-message is-${normalized.type || 'info'}`
  item.textContent = normalized.message == null ? '' : String(normalized.message)
  item.setAttribute('role', normalized.type === 'error' ? 'alert' : 'status')
  ensureHost('liquid-message-host').appendChild(item)
  let closed = false
  let timer
  const frame = requestAnimationFrame(() => { if (!closed) item.classList.add('is-visible') })
  const close = () => {
    if (closed) return
    closed = true
    cancelAnimationFrame(frame)
    window.clearTimeout(timer)
    item.classList.remove('is-visible')
    afterTransition(item, () => item.remove())
  }
  const duration = normalized.duration == null ? 3000 : normalized.duration
  if (duration > 0) timer = window.setTimeout(close, duration)
  return { close }
}

const feedbackTypes = ['success', 'warning', 'info', 'error']
feedbackTypes.forEach((type) => {
  Message[type] = (message) => Message({ message, type })
})

const openBox = (message, title, options = {}, prompt = false) => new Promise((resolve, reject) => {
  let settled = false
  const vm = new Vue({
    data: { visible: true, value: options.inputValue == null ? '' : String(options.inputValue), error: '' },
    methods: {
      finish(confirmed) {
        if (settled) return
        settled = true
        this.visible = false
        if (confirmed) resolve(prompt ? { value: this.value, action: 'confirm' } : 'confirm')
        else reject('cancel')
        this.$nextTick(() => {
          const element = this.$el
          this.$destroy()
          element?.remove()
        })
      },
      confirm() {
        const pattern = options.inputPattern
        if (prompt && pattern) {
          pattern.lastIndex = 0
          if (!pattern.test(this.value)) {
            this.error = options.inputErrorMessage || '格式不正确'
            this.$nextTick(() => this.$refs.input?.focus())
            return
          }
        }
        this.finish(true)
      }
    },
    render(h) {
      const description = `ui-feedback-description-${this._uid}`
      const errorId = `ui-feedback-error-${this._uid}`
      return h(UiDialog, {
        props: {
          visible: this.visible, title: title || '提示', width: '440px',
          role: 'alertdialog', describedBy: description, renderIcon,
          customClass: 'ui-feedback-dialog',
          closeOnClickModal: options.closeOnClickModal !== false,
          closeOnEscape: options.closeOnPressEscape !== false
        },
        on: {
          close: () => this.finish(false),
          open: () => {
            if (prompt) this.$refs.input?.focus()
          }
        }
      }, [
        h('p', { attrs: { id: description }, class: 'ui-feedback-description' }, [String(message == null ? '' : message)]),
        prompt ? h('form', { on: { submit: (event) => { event.preventDefault(); this.confirm() } } }, [
          h(LiquidInput, {
            ref: 'input', props: { value: this.value },
            attrs: { 'aria-label': title || '请输入', 'aria-describedby': `${description} ${errorId}`, 'aria-invalid': String(Boolean(this.error)) },
            on: { input: (value) => { this.value = value; this.error = '' } }
          }),
          h('div', { class: 'ui-feedback-error', attrs: { id: errorId, role: 'status' } }, [this.error])
        ]) : null,
        h('div', { slot: 'footer', class: 'dialog-footer' }, [
          h(LiquidButton, { on: { click: () => this.finish(false) } }, [options.cancelButtonText || '取消']),
          h(LiquidButton, { props: { type: 'primary' }, on: { click: this.confirm } }, [options.confirmButtonText || '确定'])
        ])
      ])
    }
  })
  vm.$mount()
  document.body.appendChild(vm.$el)
})

export const MessageBox = {
  confirm(message, title, options) { return openBox(message, title, options) },
  prompt(message, title, options) { return openBox(message, title, options, true) }
}

export const Notification = (options) => Message({ ...options, duration: options && options.duration != null ? options.duration : 4500 })
feedbackTypes.forEach((type) => {
  Notification[type] = (options) => Notification(typeof options === 'string' ? { message: options, type } : { ...options, type })
})
