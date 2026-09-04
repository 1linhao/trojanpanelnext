// Compatibility aliases are resolved at the application seam; packages use sm/md/lg.
export default {
  props: {
    size: {
      type: String,
      default: 'md',
      validator: (value) => ['mini', 'small', 'medium', 'large', 'default', 'sm', 'md', 'lg'].includes(value)
    }
  },
  computed: {
    controlSize() {
      return ({ mini: 'sm', small: 'sm', large: 'lg', medium: 'md', default: 'md' })[this.size] || this.size
    }
  }
}
