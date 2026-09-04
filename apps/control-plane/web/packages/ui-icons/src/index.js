// Shared outline geometry for navigation, actions and form controls.
const icons = {
  home: [
    [
      'path',
      {
        d: 'M3 11.5 12 4l9 7.5V21h-6v-6H9v6H3z'
      }
    ]
  ],
  nodes: [
    [
      'path',
      {
        d: 'M5 5h4v4H5zm10 0h4v4h-4zM10 15h4v4h-4zM9 7h6m-3 2v6'
      }
    ]
  ],
  plus: [
    [
      'path',
      {
        d: 'M12 5v14M5 12h14'
      }
    ]
  ],
  minus: [
    [
      'path',
      {
        d: 'M5 12h14'
      }
    ]
  ],
  close: [
    [
      'path',
      {
        d: 'm6 6 12 12M18 6 6 18'
      }
    ]
  ],
  'circle-close': [
    [
      'path',
      {
        d: 'm6 6 12 12M18 6 6 18'
      }
    ]
  ],
  check: [
    [
      'path',
      {
        d: 'm5 12 4 4L19 6'
      }
    ]
  ],
  'arrow-down': [
    [
      'path',
      {
        d: 'm6 9 6 6 6-6'
      }
    ]
  ],
  'caret-bottom': [
    [
      'path',
      {
        d: 'm6 9 6 6 6-6'
      }
    ]
  ],
  'arrow-left': [
    [
      'path',
      {
        d: 'm15 18-6-6 6-6'
      }
    ]
  ],
  'arrow-right': [
    [
      'path',
      {
        d: 'm9 18 6-6-6-6'
      }
    ]
  ],
  'd-arrow-left': [
    [
      'path',
      {
        d: 'm13 17-5-5 5-5m6 10-5-5 5-5'
      }
    ]
  ],
  'd-arrow-right': [
    [
      'path',
      {
        d: 'm11 17 5-5-5-5m-6 10 5-5-5-5'
      }
    ]
  ],
  search: [
    [
      'circle',
      {
        cx: '10.5',
        cy: '10.5',
        r: '6.5'
      }
    ],
    [
      'path',
      {
        d: 'm16 16 4 4'
      }
    ]
  ],
  date: [
    [
      'rect',
      {
        x: '3.5',
        y: '5',
        width: '17',
        height: '15.5',
        rx: '3'
      }
    ],
    [
      'path',
      {
        d: 'M8 3v4m8-4v4M3.5 9.5h17M8 14h.01m4 0h.01m4 0h.01M8 17.5h.01m4 0h.01'
      }
    ]
  ],
  time: [
    [
      'circle',
      {
        cx: '12',
        cy: '12',
        r: '9'
      }
    ],
    [
      'path',
      {
        d: 'M12 7v5l3.5 2'
      }
    ]
  ],
  edit: [
    [
      'path',
      {
        d: 'M13.5 6.5 17.5 10.5M4 20l4.2-1 10.9-10.9a2.8 2.8 0 0 0-4-4L4.2 15Z'
      }
    ]
  ],
  delete: [
    [
      'path',
      {
        d: 'M4 7h16M9 7V4h6v3m3 0-1 13H7L6 7m4 4v5m4-5v5'
      }
    ]
  ],
  download: [
    [
      'path',
      {
        d: 'M12 3v12m-5-5 5 5 5-5M4 20h16'
      }
    ]
  ],
  upload2: [
    [
      'path',
      {
        d: 'M12 21V9m-5 5 5-5 5 5M4 4h16'
      }
    ]
  ],
  top: [
    [
      'path',
      {
        d: 'M12 21V9m-5 5 5-5 5 5M4 4h16'
      }
    ]
  ],
  refresh: [
    [
      'path',
      {
        d: 'M20 7v5h-5M4 17v-5h5M6.1 8a7 7 0 0 1 11.4-2L20 8m-16 8 2.5 2a7 7 0 0 0 11.4-2'
      }
    ]
  ],
  'refresh-left': [
    [
      'path',
      {
        d: 'M20 7v5h-5M4 17v-5h5M6.1 8a7 7 0 0 1 11.4-2L20 8m-16 8 2.5 2a7 7 0 0 0 11.4-2'
      }
    ]
  ],
  view: [
    [
      'path',
      {
        d: 'M2.5 12s3.5-6 9.5-6 9.5 6 9.5 6-3.5 6-9.5 6-9.5-6-9.5-6Z'
      }
    ],
    [
      'circle',
      {
        cx: '12',
        cy: '12',
        r: '2.5'
      }
    ]
  ],
  document: [
    [
      'path',
      {
        d: 'M6 3h8l4 4v14H6Z M14 3v5h4M9 12h6m-6 4h6'
      }
    ]
  ],
  'document-copy': [
    [
      'rect',
      {
        x: '8',
        y: '8',
        width: '12',
        height: '13',
        rx: '2'
      }
    ],
    [
      'path',
      {
        d: 'M16 8V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h3'
      }
    ]
  ],
  'full-screen': [
    [
      'path',
      {
        d: 'M8 3H3v5m13-5h5v5M8 21H3v-5m13 5h5v-5'
      }
    ]
  ],
  'warning-outline': [
    [
      'path',
      {
        d: 'M10.3 4.2 2.8 17.3A2 2 0 0 0 4.5 20h15a2 2 0 0 0 1.7-2.7L13.7 4.2a2 2 0 0 0-3.4 0Z M12 9v4m0 3.5h.01'
      }
    ]
  ],
  user: [
    [
      'circle',
      {
        cx: '12',
        cy: '8',
        r: '4'
      }
    ],
    [
      'path',
      {
        d: 'M4.5 21a7.5 7.5 0 0 1 15 0'
      }
    ]
  ],
  'user-solid': [
    [
      'circle',
      {
        cx: '12',
        cy: '8',
        r: '4'
      }
    ],
    [
      'path',
      {
        d: 'M4.5 21a7.5 7.5 0 0 1 15 0'
      }
    ]
  ],
  'user-plus': [
    [
      'circle',
      {
        cx: '9',
        cy: '7.5',
        r: '3.5'
      }
    ],
    [
      'path',
      {
        d: 'M2.8 20a6.2 6.2 0 0 1 12.4 0M18 8v6m-3-3h6'
      }
    ]
  ],
  connection: [
    [
      'circle',
      {
        cx: '6',
        cy: '12',
        r: '2.5'
      }
    ],
    [
      'circle',
      {
        cx: '18',
        cy: '6',
        r: '2.5'
      }
    ],
    [
      'circle',
      {
        cx: '18',
        cy: '18',
        r: '2.5'
      }
    ],
    [
      'path',
      {
        d: 'm8.3 10.8 7.4-3.6m-7.4 6 7.4 3.6'
      }
    ]
  ],
  cpu: [
    [
      'rect',
      {
        x: '6',
        y: '6',
        width: '12',
        height: '12',
        rx: '2'
      }
    ],
    [
      'rect',
      {
        x: '9',
        y: '9',
        width: '6',
        height: '6',
        rx: '1'
      }
    ],
    [
      'path',
      {
        d: 'M9 2v4m6-4v4M9 18v4m6-4v4M2 9h4m-4 6h4m12-6h4m-4 6h4'
      }
    ]
  ],
  monitor: [
    [
      'rect',
      {
        x: '3',
        y: '4',
        width: '18',
        height: '13',
        rx: '2.5'
      }
    ],
    [
      'path',
      {
        d: 'M8 21h8m-4-4v4'
      }
    ]
  ],
  tickets: [
    [
      'path',
      {
        d: 'M4 5h16v5a2 2 0 0 0 0 4v5H4v-5a2 2 0 0 0 0-4Z M9 8h6m-6 4h6m-6 4h4'
      }
    ]
  ],
  message: [
    [
      'path',
      {
        d: 'M4 5h16v12H9l-5 4Z M8 9h8m-8 4h5'
      }
    ]
  ],
  setting: [
    [
      'circle',
      {
        cx: '12',
        cy: '12',
        r: '3'
      }
    ],
    [
      'path',
      {
        d: 'M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1-2.8 2.8-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.6v.2h-4V21a1.7 1.7 0 0 0-1-1.6 1.7 1.7 0 0 0-1.9.3l-.1.1L4.2 17l.1-.1a1.7 1.7 0 0 0 .3-1.9A1.7 1.7 0 0 0 3 14H2.8v-4H3a1.7 1.7 0 0 0 1.6-1 1.7 1.7 0 0 0-.3-1.9L4.2 7 7 4.2l.1.1A1.7 1.7 0 0 0 9 4.6a1.7 1.7 0 0 0 1-1.6v-.2h4V3a1.7 1.7 0 0 0 1 1.6 1.7 1.7 0 0 0 1.9-.3l.1-.1L19.8 7l-.1.1a1.7 1.7 0 0 0-.3 1.9 1.7 1.7 0 0 0 1.6 1h.2v4H21a1.7 1.7 0 0 0-1.6 1Z'
      }
    ]
  ],
  'switch-button': [
    [
      'path',
      {
        d: 'M12 3v9m5.7-6.7a8 8 0 1 1-11.4 0'
      }
    ]
  ],
  'data-analysis': [
    [
      'path',
      {
        d: 'M4 20V10h4v10m4 0V4h4v16m4 0v-7h-4M2 20h20'
      }
    ]
  ],
  import: [
    [
      'path',
      {
        d: 'M12 3v11m-4-4 4 4 4-4M4 18v2h16v-2'
      }
    ]
  ],
  export: [
    [
      'path',
      {
        d: 'M12 16V5m-4 4 4-4 4 4M4 16v4h16v-4'
      }
    ]
  ],
  upgrade: [
    [
      'path',
      {
        d: 'm12 3 7 3v5c0 4.5-3 8-7 10-4-2-7-5.5-7-10V6Z M12 16V8m-3 3 3-3 3 3'
      }
    ]
  ],
  loading: [
    [
      'path',
      {
        d: 'M20 12a8 8 0 1 1-2.3-5.7'
      }
    ]
  ],
  dashboard: [
    [
      'rect',
      {
        x: '3.5',
        y: '3.5',
        width: '6.5',
        height: '6.5',
        rx: '1.8'
      }
    ],
    [
      'rect',
      {
        x: '14',
        y: '3.5',
        width: '6.5',
        height: '6.5',
        rx: '1.8'
      }
    ],
    [
      'rect',
      {
        x: '3.5',
        y: '14',
        width: '6.5',
        height: '6.5',
        rx: '1.8'
      }
    ],
    [
      'rect',
      {
        x: '14',
        y: '14',
        width: '6.5',
        height: '6.5',
        rx: '1.8'
      }
    ]
  ],
  account: [
    [
      'circle',
      {
        cx: '12',
        cy: '8',
        r: '3.5'
      }
    ],
    [
      'path',
      {
        d: 'M4.5 20c.7-4 3.2-6 7.5-6s6.8 2 7.5 6'
      }
    ]
  ],
  node: [
    [
      'circle',
      {
        cx: '12',
        cy: '12',
        r: '3'
      }
    ],
    [
      'circle',
      {
        cx: '5',
        cy: '5',
        r: '2'
      }
    ],
    [
      'circle',
      {
        cx: '19',
        cy: '5',
        r: '2'
      }
    ],
    [
      'circle',
      {
        cx: '19',
        cy: '19',
        r: '2'
      }
    ],
    [
      'path',
      {
        d: 'm7 6.5 3 3M14 10l3.2-3.3M14.2 14.1l3.1 3.1'
      }
    ]
  ],
  server: [
    [
      'rect',
      {
        x: '3.5',
        y: '3.5',
        width: '17',
        height: '7',
        rx: '2'
      }
    ],
    [
      'rect',
      {
        x: '3.5',
        y: '13.5',
        width: '17',
        height: '7',
        rx: '2'
      }
    ],
    [
      'path',
      {
        d: 'M7 7h.01M7 17h.01M11 7h6M11 17h6'
      }
    ]
  ],
  sysinfo: [
    [
      'rect',
      {
        x: '5',
        y: '5',
        width: '14',
        height: '14',
        rx: '3'
      }
    ],
    [
      'path',
      {
        d: 'M9 1.8v3.1M15 1.8v3.1M9 19.1v3.1M15 19.1v3.1M1.8 9h3.1M19.1 9h3.1M1.8 15h3.1M19.1 15h3.1M9 9h6v6H9z'
      }
    ]
  ],
  task: [
    [
      'path',
      {
        d: 'M7 3.5h10a2 2 0 0 1 2 2v15H5v-15a2 2 0 0 1 2-2Z'
      }
    ],
    [
      'path',
      {
        d: 'M9 3.5v3h6v-3M8.5 11h7M8.5 15h5'
      }
    ]
  ],
  email: [
    [
      'rect',
      {
        x: '3',
        y: '5',
        width: '18',
        height: '14',
        rx: '3'
      }
    ],
    [
      'path',
      {
        d: 'm5 7 7 6 7-6'
      }
    ]
  ],
  pass: [
    [
      'circle',
      {
        cx: '12',
        cy: '12',
        r: '9'
      }
    ],
    [
      'path',
      {
        d: 'm8.8 8.8 6.4 6.4'
      }
    ]
  ],
  system: [
    [
      'circle',
      {
        cx: '12',
        cy: '12',
        r: '3'
      }
    ],
    [
      'path',
      {
        d: 'M12 2.5v2M12 19.5v2M2.5 12h2M19.5 12h2M5.3 5.3l1.4 1.4M17.3 17.3l1.4 1.4M18.7 5.3l-1.4 1.4M6.7 17.3l-1.4 1.4'
      }
    ]
  ],
  username: [
    [
      'circle',
      {
        cx: '12',
        cy: '12',
        r: '9'
      }
    ],
    [
      'circle',
      {
        cx: '12',
        cy: '9',
        r: '2.7'
      }
    ],
    [
      'path',
      {
        d: 'M7.5 18c.7-2.7 2.2-4 4.5-4s3.8 1.3 4.5 4'
      }
    ]
  ],
  sun: [
    [
      'circle',
      {
        cx: '12',
        cy: '12',
        r: '3.5'
      }
    ],
    [
      'path',
      {
        d: 'M12 2.5v2M12 19.5v2M2.5 12h2M19.5 12h2'
      }
    ],
    [
      'path',
      {
        d: 'm5.3 5.3 1.4 1.4m10.6 10.6 1.4 1.4M18.7 5.3l-1.4 1.4M6.7 17.3l-1.4 1.4'
      }
    ]
  ],
  moon: [
    [
      'path',
      {
        d: 'M20.2 15.2A8.5 8.5 0 0 1 8.8 3.8a8.5 8.5 0 1 0 11.4 11.4Z'
      }
    ]
  ],
  eye: [
    [
      'path',
      {
        d: 'M2.5 12s3.5-6 9.5-6 9.5 6 9.5 6-3.5 6-9.5 6-9.5-6-9.5-6Z'
      }
    ],
    [
      'circle',
      {
        cx: '12',
        cy: '12',
        r: '2.5'
      }
    ]
  ],
  'eye-off': [
    [
      'path',
      {
        d: 'm3 3 18 18M9.8 6.3A10 10 0 0 1 12 6c6 0 9.5 6 9.5 6a18 18 0 0 1-3.2 3.8M6.2 6.8A19 19 0 0 0 2.5 12s3.5 6 9.5 6a11 11 0 0 0 4-.8M10.2 10.2a2.5 2.5 0 0 0 3.6 3.6'
      }
    ]
  ]
}

export const iconNames = Object.freeze(Object.keys(icons))

export function renderIcon(h, name, attrs = {}, data = {}) {
  if (!Object.prototype.hasOwnProperty.call(icons, name)) {
    throw new TypeError(`Unknown icon: ${name}`)
  }
  return h(
    'svg',
    {
      ...data,
      class: ['app-icon', `app-icon--${name}`, data.class],
      attrs: {
        width: '20',
        height: '20',
        viewBox: '0 0 24 24',
        fill: 'none',
        stroke: 'currentColor',
        'stroke-width': '2.35',
        'stroke-linecap': 'round',
        'stroke-linejoin': 'round',
        'aria-hidden': 'true',
        focusable: 'false',
        ...data.attrs,
        ...attrs
      }
    },
    icons[name].map(([tag, shape]) => h(tag, { attrs: shape }))
  )
}
