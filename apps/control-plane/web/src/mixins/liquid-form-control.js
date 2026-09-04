// Public form-control seam: labels and validation follow the nearest form item.
// Explicit caller IDs/names win, including controls used outside LiquidForm.
export default {
  inject: { liquidFormItem: { default: null } },
  computed: {
    controlAttrs() {
      const attrs = { ...this.$attrs, id: this.$attrs.id || `liquid-control-${this._uid}` }
      const item = this.liquidFormItem
      if (!item) return attrs
      if (item.label != null && !attrs['aria-label'] && !attrs['aria-labelledby']) {
        attrs['aria-labelledby'] = item.labelId
      }
      if (item.error) {
        attrs['aria-describedby'] = [attrs['aria-describedby'], item.errorId].filter(Boolean).join(' ')
        attrs['aria-invalid'] = 'true'
      }
      return attrs
    }
  },
  mounted() { this.liquidFormItem?.addControl(this) },
  beforeDestroy() { this.liquidFormItem?.removeControl(this) }
}
