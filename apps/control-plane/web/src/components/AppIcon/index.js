import { renderIcon } from '@tp-ui/icons'

export default {
  name: 'AppIcon',
  functional: true,
  props: {
    name: { type: String, required: true }
  },
  render(h, { props, data }) {
    return renderIcon(h, props.name, {}, data)
  }
}
