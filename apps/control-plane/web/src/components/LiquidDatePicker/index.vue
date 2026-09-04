<template>
  <div class="liquid-date-picker" data-ui-control :data-ui-size="controlSize" :class="{ 'is-focused': open, 'is-disabled': disabled }">
    <button ref="trigger" v-bind="controlAttrs" :aria-controls="`liquid-date-popover-${_uid}`" class="liquid-date-picker__trigger" type="button" :disabled="disabled" :aria-expanded="String(open)" aria-haspopup="dialog" @click="togglePopover" @keydown.esc="handleEscape">
      <app-icon name="date" aria-hidden="true" />
      <span :class="{ 'is-placeholder': !displayValue }">{{ displayValue || placeholder || defaultPlaceholder }}</span>
      <app-icon v-if="!clearable || !displayValue" name="arrow-down" class="liquid-date-picker__arrow" />
    </button>
      <button type="button" :disabled="disabled" v-if="clearable && displayValue" class="liquid-date-picker__clear" aria-label="清空日期" @click.stop="clearValue"><app-icon name="close" aria-hidden="true" /></button>

    <div ref="popover" :id="`liquid-date-popover-${_uid}`" @keydown.esc.prevent.stop="closePopover" class="liquid-date-picker__popover" popover="manual" role="dialog" aria-label="日期选择器" :style="popoverStyle" @click.stop>
      <label class="liquid-date-picker__manual" :class="{ 'has-error': manualError }">
        <span>日期输入</span>
        <input ref="manualInput" v-model.trim="manualText" type="text" :placeholder="manualPlaceholder" :aria-invalid="String(manualError)" @input="manualError = false" @keydown.enter.prevent="confirmSelection" />
        <small v-if="manualError">请按 {{ manualPlaceholder }} 格式输入有效日期</small>
      </label>
      <div class="liquid-date-picker__calendar" :class="{ 'is-month': type === 'month' }">
        <header>
          <button type="button" aria-label="上一年" @click="moveYear(-1)"><app-icon name="d-arrow-left" /></button>
          <button v-if="type !== 'month'" type="button" aria-label="上个月" @click="moveMonth(-1)"><app-icon name="arrow-left" /></button>
          <strong>{{ calendarTitle }}</strong>
          <button v-if="type !== 'month'" type="button" aria-label="下个月" @click="moveMonth(1)"><app-icon name="arrow-right" /></button>
          <button type="button" aria-label="下一年" @click="moveYear(1)"><app-icon name="d-arrow-right" /></button>
        </header>
        <div v-if="type === 'month'" class="liquid-date-picker__months">
          <button v-for="month in 12" :key="month" type="button" :class="{ 'is-selected': isSelectedMonth(month - 1) }" @click="pickMonth(month - 1)">{{ month }}月</button>
        </div>
        <template v-else>
          <div class="liquid-date-picker__weekdays" aria-hidden="true"><span v-for="weekday in weekdays" :key="weekday">{{ weekday }}</span></div>
          <div class="liquid-date-picker__days">
            <span v-for="blank in firstWeekday" :key="`blank-${blank}`" />
            <button v-for="day in daysInViewMonth" :key="day" type="button" :class="{ 'is-selected': isSelectedDay(day), 'is-today': isToday(day) }" @click="pickDay(day)">{{ day }}</button>
          </div>
        </template>
        <label v-if="type === 'datetime'" class="liquid-date-picker__time"><span>时间</span><input v-model="timeText" type="time" step="60" /></label>
      </div>
      <footer>
        <button v-if="clearable" type="button" @click="clearValue">清空</button>
        <button type="button" @click="pickToday">今天</button><span />
        <button type="button" @click="closePopover">取消</button>
        <button class="is-primary" type="button" @click="confirmSelection">确定</button>
      </footer>
    </div>
  </div>
</template>

<script>
import emitter from '@/mixins/liquid-control-emitter'
import formControl from '@/mixins/liquid-form-control'
import controlSize from '@/mixins/liquid-control-size'

function pad(value) { return String(value).padStart(2, '0') }
function localDate(year, month, day, hour = 0, minute = 0) {
  const date = new Date(year, month, day, hour, minute, 0, 0)
  if (date.getFullYear() !== year || date.getMonth() !== month || date.getDate() !== day) return null
  return date
}

export default {
  name: 'LiquidDatePicker',
  mixins: [formControl, emitter, controlSize],
  inheritAttrs: false,
  props: {
    value: { type: [String, Number, Date], default: '' },
    type: { type: String, default: 'date' },
    valueFormat: { type: String, default: '' },
    placeholder: { type: String, default: '' },
    clearable: { type: Boolean, default: true },
    disabled: { type: Boolean, default: false }
  },
  data() {
    const now = new Date()
    return { open: false, selectedDate: now, viewYear: now.getFullYear(), viewMonth: now.getMonth(), manualText: '', manualError: false, timeText: `${pad(now.getHours())}:${pad(now.getMinutes())}`, popoverStyle: {}, weekdays: ['日', '一', '二', '三', '四', '五', '六'] }
  },
  computed: {
    defaultPlaceholder() { return this.type === 'month' ? '选择月份' : this.type === 'datetime' ? '选择日期和时间' : '选择日期' },
    manualPlaceholder() { return this.type === 'month' ? 'YYYY-MM' : this.type === 'datetime' ? 'YYYY-MM-DD HH:mm' : 'YYYY-MM-DD' },
    displayValue() { const date = this.parseExternalValue(this.value); return date ? this.formatDate(date) : '' },
    calendarTitle() { return this.type === 'month' ? `${this.viewYear}年` : `${this.viewYear}年 ${this.viewMonth + 1}月` },
    firstWeekday() { return new Date(this.viewYear, this.viewMonth, 1).getDay() },
    daysInViewMonth() { return new Date(this.viewYear, this.viewMonth + 1, 0).getDate() }
  },
  mounted() {
    document.addEventListener('click', this.handleOutside)
    window.addEventListener('resize', this.updatePosition)
    window.addEventListener('scroll', this.updatePosition, true)
  },
  beforeDestroy() {
    this.closePopover()
    document.removeEventListener('click', this.handleOutside)
    window.removeEventListener('resize', this.updatePosition)
    window.removeEventListener('scroll', this.updatePosition, true)
  },
  methods: {
    handleEscape(event) {
      if (!this.open) return
      event.preventDefault()
      event.stopPropagation()
      this.closePopover()
      this.$refs.trigger?.focus()
    },
    parseExternalValue(value) {
      if (value === '' || value === undefined || value === null) return null
      if (value instanceof Date) return Number.isNaN(value.getTime()) ? null : value
      if (typeof value === 'number') { const date = new Date(value); return Number.isNaN(date.getTime()) ? null : date }
      return this.parseText(String(value))
    },
    parseText(text) {
      const pattern = this.type === 'month' ? /^(\d{4})-(\d{2})$/ : /^(\d{4})-(\d{2})-(\d{2})(?:[ T](\d{2}):(\d{2}))?$/
      const match = text.match(pattern)
      if (!match) return null
      const year = Number(match[1]); const month = Number(match[2]) - 1
      const day = this.type === 'month' ? 1 : Number(match[3]); const hour = Number(match[4] || 0); const minute = Number(match[5] || 0)
      if (month < 0 || month > 11 || hour > 23 || minute > 59) return null
      return localDate(year, month, day, hour, minute)
    },
    formatDate(date) {
      const datePart = `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
      if (this.type === 'month') return datePart.slice(0, 7)
      return this.type === 'datetime' ? `${datePart} ${pad(date.getHours())}:${pad(date.getMinutes())}` : datePart
    },
    syncDraft() {
      const value = this.parseExternalValue(this.value) || new Date()
      this.selectedDate = new Date(value.getTime()); this.viewYear = value.getFullYear(); this.viewMonth = value.getMonth()
      this.timeText = `${pad(value.getHours())}:${pad(value.getMinutes())}`; this.manualText = this.formatDate(value); this.manualError = false
    },
    togglePopover() { if (this.open) this.closePopover(); else this.openPopover() },
    openPopover() {
      if (this.disabled || this.open) return
      this.syncDraft(); this.open = true
      this.$nextTick(() => { this.updatePosition(); if (this.$refs.popover.showPopover) this.$refs.popover.showPopover(); this.$refs.manualInput?.focus() })
    },
    closePopover() {
      if (!this.open) return
      const hadFocus = this.$refs.popover?.contains(document.activeElement)
      this.open = false
      if (this.$refs.popover && this.$refs.popover.hidePopover) {
        try { this.$refs.popover.hidePopover() } catch (error) {
          // The browser may already have dismissed the popover.
        }
      }
      if (hadFocus) this.$refs.trigger?.focus()
      this.$emit('blur'); this.dispatch('LiquidFormItem', 'liquid.form.blur', [this.value])
    },
    updatePosition() {
      if (!this.open || !this.$refs.trigger) return
      const rect = this.$refs.trigger.getBoundingClientRect(); const width = Math.min(344, window.innerWidth - 24)
      const left = Math.min(Math.max(12, rect.left), window.innerWidth - width - 12)
      const roomBelow = window.innerHeight - rect.bottom - 12; const roomAbove = rect.top - 12
      const desiredHeight = this.type === 'month' ? 330 : this.type === 'datetime' ? 520 : 460
      const openAbove = roomBelow < desiredHeight && roomAbove > roomBelow
      const availableHeight = Math.max(120, openAbove ? roomAbove - 7 : roomBelow - 7)
      this.popoverStyle = { left: `${left}px`, top: openAbove ? 'auto' : `${rect.bottom + 7}px`, bottom: openAbove ? `${window.innerHeight - rect.top + 7}px` : 'auto', width: `${width}px`, maxHeight: `${Math.min(window.innerHeight - 24, availableHeight)}px` }
    },
    moveYear(delta) { this.viewYear += delta },
    moveMonth(delta) { const date = new Date(this.viewYear, this.viewMonth + delta, 1); this.viewYear = date.getFullYear(); this.viewMonth = date.getMonth() },
    isSelectedMonth(month) { return this.selectedDate && this.selectedDate.getFullYear() === this.viewYear && this.selectedDate.getMonth() === month },
    isSelectedDay(day) { return this.selectedDate && this.selectedDate.getFullYear() === this.viewYear && this.selectedDate.getMonth() === this.viewMonth && this.selectedDate.getDate() === day },
    isToday(day) { const now = new Date(); return now.getFullYear() === this.viewYear && now.getMonth() === this.viewMonth && now.getDate() === day },
    pickMonth(month) {
      this.viewMonth = month; const day = Math.min(this.selectedDate.getDate(), new Date(this.viewYear, month + 1, 0).getDate())
      this.selectedDate = localDate(this.viewYear, month, day); this.manualText = this.formatDate(this.selectedDate)
    },
    pickDay(day) {
      const [hour, minute] = this.timeText.split(':').map(Number)
      this.selectedDate = localDate(this.viewYear, this.viewMonth, day, hour || 0, minute || 0); this.manualText = this.formatDate(this.selectedDate)
    },
    pickToday() {
      const now = new Date(); this.selectedDate = now; this.viewYear = now.getFullYear(); this.viewMonth = now.getMonth()
      this.timeText = `${pad(now.getHours())}:${pad(now.getMinutes())}`; this.manualText = this.formatDate(now)
    },
    // WEB-019: single draft resolution shared by Enter and the confirm button.
    // When the typed text carries its own time it wins; otherwise the time
    // draft from the time control applies. Both paths produce the same date.
    resolveDraftSelection() {
      const parsed = this.parseText(this.manualText)
      if (!parsed) return null
      if (this.type === 'datetime' && /\d{1,2}:\d{2}/.test(this.manualText)) return parsed
      if (this.type === 'datetime') {
        const [hour, minute] = this.timeText.split(':').map(Number)
        parsed.setHours(hour || 0, minute || 0, 0, 0)
      }
      return parsed
    },
    outputValue(date) { return this.valueFormat === 'timestamp' ? date.getTime() : this.formatDate(date) },
    emitValue(value) { this.$emit('input', value); this.$emit('change', value); this.dispatch('LiquidFormItem', 'liquid.form.change', [value]) },
    confirmSelection() {
      const parsed = this.resolveDraftSelection()
      if (!parsed) { this.manualError = true; this.$nextTick(() => this.$refs.manualInput && this.$refs.manualInput.focus()); return }
      this.manualError = false; this.selectedDate = parsed
      this.emitValue(this.outputValue(this.selectedDate)); this.closePopover()
    },
    clearValue() { if (this.disabled) return; this.emitValue(''); this.closePopover(); this.$nextTick(() => this.$refs.trigger?.focus()) },
    handleOutside(event) {
      if (!this.open) return
      const inTrigger = this.$el && this.$el.contains(event.target); const inPopover = this.$refs.popover && this.$refs.popover.contains(event.target)
      if (!inTrigger && !inPopover) this.closePopover()
    }
  }
}
</script>

<style scoped>
.liquid-date-picker { position: relative; width: min(100%, var(--control-max-width)); max-width: var(--control-max-width); min-width: 0; }
.liquid-date-picker__trigger { display: flex; align-items: center; gap: 9px; width: 100%; min-height: var(--ui-control-size-height, 42px); padding: var(--ui-control-size-padding-y, 0) var(--ui-control-size-padding-x, 14px); border: 1px solid var(--control-border); border-radius: 14px; color: var(--ink-3); background: var(--control-fill); box-shadow: inset 0 1px 0 var(--spec-soft); font: inherit; text-align: left; }
.liquid-date-picker__trigger > span:nth-child(2) { flex: 1; min-width: 0; overflow: hidden; color: var(--ink); text-overflow: ellipsis; white-space: nowrap; }
.liquid-date-picker__trigger .is-placeholder { color: var(--ink-3); }
.liquid-date-picker.is-focused .liquid-date-picker__trigger { border-color: var(--accent); box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 18%, transparent), inset 0 1px 0 var(--spec-soft); }
.liquid-date-picker__arrow { margin-left: auto; }
.liquid-date-picker__clear { position: absolute; right: 12px; top: calc(50% - 12px); padding: 0; border: 0; color: var(--ink-3); display: grid; place-items: center; width: 24px; height: 24px; border-radius: 50%; background: var(--neutral-bg); }
.liquid-date-picker__popover { position: fixed; z-index: 5000; box-sizing: border-box; margin: 0; padding: 10px; border: 1px solid var(--rim); border-radius: var(--r-lg); color: var(--ink); background: linear-gradient(150deg, var(--spec-soft), transparent 46%), var(--glass-popover); box-shadow: var(--shadow), inset 0 1px 0 var(--spec); backdrop-filter: var(--ui-backdrop-surface); overflow: auto; }
.liquid-date-picker__manual { display: block; margin-bottom: 9px; }
.liquid-date-picker__manual span, .liquid-date-picker__time span { display: block; margin: 0 0 5px 3px; color: var(--form-label-ink); font-size: 11px; font-weight: 700; }
.liquid-date-picker__manual input, .liquid-date-picker__time input { width: 100%; min-height: 40px; padding: 0 12px; border: 1px solid var(--control-border); border-radius: 12px; outline: 0; color: var(--ink); background: var(--control-fill); font: inherit; }
.liquid-date-picker__manual.has-error input { border-color: var(--bad-fg); box-shadow: 0 0 0 3px var(--bad-bg); }
.liquid-date-picker__manual small { display: block; margin: 5px 3px 0; color: var(--bad-fg); font-size: 11px; }
.liquid-date-picker__calendar { padding: 10px; border: 1px solid var(--hairline); border-radius: var(--r-md); background: var(--glass-soft); }
.liquid-date-picker__calendar header { display: grid; grid-template-columns: 34px 34px 1fr 34px 34px; align-items: center; margin-bottom: 8px; }
.liquid-date-picker__calendar.is-month header { grid-template-columns: 34px 1fr 34px; }
.liquid-date-picker__calendar header strong { min-width: 0; text-align: center; white-space: nowrap; }
.liquid-date-picker__calendar button, .liquid-date-picker footer button { border: 0; color: var(--ink-2); background: transparent; }
.liquid-date-picker__calendar header button { width: 34px; height: 34px; border-radius: 10px; }
.liquid-date-picker__calendar header button:hover, .liquid-date-picker__days button:hover, .liquid-date-picker__months button:hover { color: var(--accent); background: var(--glass-strong); }
.liquid-date-picker__weekdays, .liquid-date-picker__days { display: grid; grid-template-columns: repeat(7, 1fr); gap: 3px; }
.liquid-date-picker__weekdays span { padding: 5px 0; color: var(--table-header-ink); font-size: 10px; text-align: center; }
.liquid-date-picker__days button { aspect-ratio: 1; border-radius: 11px; font: inherit; font-size: 12px; }
.liquid-date-picker__months { display: grid; grid-template-columns: repeat(3, 1fr); gap: 6px; }
.liquid-date-picker__months button { min-height: 42px; border-radius: 11px; font: inherit; }
.liquid-date-picker__days button.is-selected, .liquid-date-picker__months button.is-selected { color: var(--on-accent); background: var(--accent); box-shadow: inset 0 1px 0 var(--spec); }
.liquid-date-picker__days button.is-today:not(.is-selected) { color: var(--accent); box-shadow: inset 0 0 0 1px var(--accent); }
.liquid-date-picker__time { display: block; margin-top: 9px; }
.liquid-date-picker footer { display: grid; grid-template-columns: auto auto 1fr auto auto; gap: 5px; margin-top: 9px; }
.liquid-date-picker footer button { min-height: 34px; padding: 0 11px; border-radius: 999px; font: inherit; font-size: 12px; }
.liquid-date-picker footer button:hover { background: var(--glass-soft); }
.liquid-date-picker footer .is-primary { color: var(--on-accent); background: var(--accent); }
.liquid-date-picker.is-disabled { opacity: 0.58; }
</style>
