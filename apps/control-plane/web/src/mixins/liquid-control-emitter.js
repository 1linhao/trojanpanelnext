// Small parent-dispatch helper used by the native Liquid form controls.
export default {
  methods: {
    dispatch(componentName, eventName, params = []) {
      let parent = this.$parent || this.$root

      while (
        parent &&
        (!parent.$options || parent.$options.componentName !== componentName)
      ) {
        parent = parent.$parent
      }

      if (parent) parent.$emit(eventName, ...params)
    }
  }
}
