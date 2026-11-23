function iconSlot(component, h, item) {
  return component.$scopedSlots.icon
    ? component.$scopedSlots.icon({ name: item.icon, item })
    : []
}

function navButton(component, h, item, mobile = false) {
  return h(
    'button',
    {
      key: `${mobile ? 'mobile' : 'desktop'}:${item.key}`,
      class: [
        'tp-ui-shell__nav-item',
        { 'is-active': component.model.activeKey === item.key }
      ],
      attrs: {
        type: 'button',
        disabled: item.disabled,
        'data-ui-state':
          component.model.activeKey === item.key ? 'selected' : 'idle'
      },
      on: { click: () => component.$emit('navigate', item.key, item) }
    },
    [
      h(
        'span',
        { class: 'tp-ui-shell__nav-icon', attrs: { 'aria-hidden': 'true' } },
        iconSlot(component, h, item)
      ),
      h('span', mobile ? item.mobileLabel : item.label)
    ]
  )
}

function brandButton(component, h, compact = false) {
  const { brand } = component.model
  return h(
    'button',
    {
      class: ['tp-ui-shell__brand', { 'tp-ui-shell__topbar-brand': compact }],
      attrs: { type: 'button', 'aria-label': compact ? brand.name : undefined },
      on: { click: () => component.$emit('action', 'brand') }
    },
    component.$scopedSlots.brand
      ? component.$scopedSlots.brand({ brand, compact })
      : [
          h('span', { class: 'tp-ui-shell__brand-mark' }, brand.mark),
          h('span', { class: 'tp-ui-shell__brand-label' }, [
            h('strong', brand.name),
            h('small', brand.subtitle)
          ])
        ]
  )
}

export const UiAppShell = {
  name: 'UiAppShell',
  props: {
    model: { type: Object, required: true },
    showUser: { type: Boolean, default: true },
    showMobileBrand: { type: Boolean, default: false },
    labels: {
      type: Object,
      default: () => ({
        navigation: 'Application navigation',
        profile: 'Profile',
        logout: 'Log out'
      })
    }
  },
  mounted() {
    this.shellResizeView = this.$el?.ownerDocument?.defaultView
    this.shellResizeHandler = () => this.revealActiveMobileItem()
    this.shellResizeView?.addEventListener?.('resize', this.shellResizeHandler)
    this.revealActiveMobileItem()
  },
  beforeDestroy() {
    this.shellResizeView?.removeEventListener?.(
      'resize',
      this.shellResizeHandler
    )
  },
  updated() {
    this.revealActiveMobileItem()
  },
  methods: {
    revealActiveMobileItem() {
      const nav = this.$refs.mobileNav
      const activeItem = nav?.querySelector?.('.is-active')
      if (!activeItem || nav.scrollWidth <= nav.clientWidth) return
      nav.scrollTo({
        left: Math.max(
          0,
          activeItem.offsetLeft - (nav.clientWidth - activeItem.offsetWidth) / 2
        )
      })
    }
  },
  render(h) {
    const allItems = this.model.groups.flatMap((group) => group.items)
    return h(
      'div',
      {
        class: 'tp-ui-shell',
        attrs: {
          'data-ui-component': 'app-shell',
          'data-ui-surface': 'canvas',
          'data-ui-density': 'comfortable'
        }
      },
      [
        h(
          'aside',
          {
            class: 'tp-ui-shell__side',
            attrs: {
              'data-ui-surface': 'navigation',
              'data-ui-tone': 'neutral'
            }
          },
          [
            brandButton(this, h),
            ...this.model.groups.flatMap((group) => [
              h(
                'div',
                { key: `${group.key}:label`, class: 'tp-ui-shell__nav-label' },
                group.label
              ),
              ...group.items.map((item) => navButton(this, h, item))
            ])
          ]
        ),
        h('main', { class: 'tp-ui-shell__main' }, [
          h(
            'header',
            {
              class: 'tp-ui-shell__topbar',
              attrs: { 'data-ui-surface': 'raised', 'data-ui-tone': 'neutral' }
            },
            [
              this.showMobileBrand ? brandButton(this, h, true) : null,
              h('h1', this.model.pageTitle),
              h('div', { class: 'tp-ui-shell__actions' }, [
                ...(this.$slots.actions || []),
                this.$scopedSlots.user
                  ? this.$scopedSlots.user({ user: this.model.user })
                  : this.showUser && this.model.user
                  ? h(
                      'button',
                      {
                        class: 'tp-ui-shell__user',
                        attrs: {
                          type: 'button',
                          'aria-label': this.labels.logout
                        },
                        on: { click: () => this.$emit('logout') }
                      },
                      this.model.user.label || this.model.user.name || ''
                    )
                  : null
              ])
            ]
          ),
          h('section', { class: 'tp-ui-shell__content' }, this.$slots.default)
        ]),
        h(
          'nav',
          {
            ref: 'mobileNav',
            class: [
              'tp-ui-shell__mobile-nav',
              { 'is-scrollable': allItems.length > 5 }
            ],
            attrs: {
              'data-ui-surface': 'navigation',
              'aria-label': this.labels.navigation
            }
          },
          allItems.map((item) => navButton(this, h, item, true))
        )
      ]
    )
  }
}

export function createAppShell() {
  return Object.freeze({
    install(Vue) {
      Vue.component('UiAppShell', UiAppShell)
    }
  })
}
