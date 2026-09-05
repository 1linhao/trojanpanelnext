module.exports = {
  head: [['link', { rel: 'icon', href: '/logo.png' }]],
  title: 'TrojanPanel Next',
  description: '支持 Xray、Hysteria2 和 NaiveProxy 的多用户 Web 管理面板',
  plugins: [['@vuepress/back-to-top'], ['vuepress-plugin-code-copy', true]],
  themeConfig: {
    sidebarDepth: 3,
    logo: '/logo.png',
    lastUpdated: '最后更新',
    nav: [
      { text: '新手起步', link: '/start/introduce' },
      { text: '安装', link: '/install-tutorial/installation' },
      { text: '使用教程', link: '/tutorial/using-tutorials' },
      { text: 'API', link: '/api/api' },
      { text: 'FAQ', link: '/faq/faq' },
      { text: 'English', link: '/README_EN.html' },
      { text: 'GitHub', link: 'https://github.com/1linhao/trojanpanelnext' }
    ],
    sidebar: {
      '/start/': ['introduce', 'system-structure'],
      '/tutorial/': [
        'using-tutorials',
        'des-of-related-doc',
        'using-cdn',
        'client-config',
        'recommend-tool',
        'performance-tuning',
        'performance-testing'
      ],
      '/install-tutorial/': ['installation'],
      '/api/': ['api'],
      '/sdk/': ['sdk'],
      '/faq/': ['faq'],
      '/change/': ['change-log']
    }
  },
  dest: 'docs'
}
