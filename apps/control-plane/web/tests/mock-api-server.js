const http = require('http')

const ok = (data) =>
  JSON.stringify({ code: 20000, type: 'success', message: '', data })
const page = (key, rows) => ({
  [key]: rows,
  pageNum: 1,
  pageSize: 20,
  total: rows.length
})

const account = {
  id: 2,
  username: 'glassdemo',
  email: 'demo@example.com',
  roleId: 3,
  roles: ['user'],
  deleted: 0,
  quota: 107374182400,
  download: 12884901888,
  upload: 4294967296,
  expireTime: Date.now() + 30 * 86400000,
  createTime: '2026-08-01T12:00:00+08:00'
}

const accounts = Array.from({ length: 36 }, (_, index) => ({
  ...account,
  id: index + 2,
  username: index === 0 ? account.username : `glassuser${index + 1}`,
  email: index === 0 ? account.email : `user${index + 1}@example.com`,
  download: account.download + index * 268435456,
  upload: account.upload + index * 67108864
}))

const captchaSvg =
  'data:image/svg+xml;charset=utf-8,' +
  encodeURIComponent(
    '<svg xmlns="http://www.w3.org/2000/svg" width="236" height="84" viewBox="0 0 236 84"><path d="M4 62C42 7 76 79 118 28s74 39 114-6" fill="none" stroke="#0a7cff" stroke-opacity=".28" stroke-width="3"/><path d="M8 24l218 42M18 72L216 14" stroke="#8d56d9" stroke-opacity=".2" stroke-width="2"/><text x="118" y="57" text-anchor="middle" font-family="ui-monospace,monospace" font-size="39" font-weight="700" font-style="italic" letter-spacing="10" fill="#1767ba">K7M4</text></svg>'
  )

const logoSvg = Buffer.from(
  '<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64"><rect width="64" height="64" rx="18" fill="#1767ba"/><text x="32" y="39" text-anchor="middle" font-family="Arial,sans-serif" font-size="22" font-weight="700" fill="white">TP</text></svg>'
)

const node = {
  id: 1,
  nodeServerId: 1,
  nodeSubId: 1,
  nodeTypeId: 1,
  name: 'Tokyo Reality',
  domain: 'jp.example.com',
  port: 443,
  priority: 100,
  clients: ['sing-box', 'clash-meta', 'v2ray', 'shadowrocket'],
  status: 1,
  createTime: '2026-08-01T12:00:00+08:00',
  serverTraffic: {
    period: 'month',
    limitMode: 'combined',
    totalLimit: 1099511627776,
    totalRemaining: 824633720832
  }
}

const nodes = [
  node,
  {
    ...node,
    id: 2,
    nodeServerId: 2,
    nodeSubId: 2,
    name: 'Singapore WebSocket',
    domain: 'sg.example.com',
    port: 8443,
    priority: 90,
    clients: ['sing-box', 'clash-meta', 'v2ray']
  },
  {
    ...node,
    id: 3,
    nodeServerId: 3,
    nodeSubId: 3,
    nodeTypeId: 5,
    name: 'Frankfurt Hysteria2 High Performance',
    domain: 'de.example.com',
    port: 2443,
    priority: 80,
    clients: ['sing-box', 'shadowrocket']
  },
  {
    ...node,
    id: 4,
    nodeServerId: 4,
    nodeSubId: 4,
    nodeTypeId: 4,
    name: 'San Francisco NaiveProxy',
    domain: 'us.example.com',
    port: 443,
    priority: 70,
    clients: ['sing-box', 'v2ray']
  },
  {
    ...node,
    id: 5,
    nodeServerId: 5,
    nodeSubId: 5,
    name: 'Hong Kong VLESS Reality',
    domain: 'hk.example.com',
    port: 10443,
    priority: 60,
    clients: ['sing-box', 'clash-meta', 'shadowrocket']
  }
]

const nodeServers = [
  { id: 1, name: 'Tokyo', ip: 'jp.example.com', grpcPort: 8100, grpcTLSMode: 'mtls', grpcTLSServerName: 'core-jp.example.com', trafficPeriod: 'month', trafficLimitMode: 'combined', trafficTotalLimit: 1099511627776, trafficUploadLimit: 0, trafficDownloadLimit: 0, status: 1, trojanPanelCoreVersion: '2.3.0', kernelSummary: 'xray 25.8.3' },
  { id: 2, name: 'Singapore', ip: 'sg.example.com', grpcPort: 8101, grpcTLSMode: 'tls', grpcTLSServerName: 'core-sg.example.com', trafficPeriod: 'day', trafficLimitMode: 'separate', trafficTotalLimit: 0, trafficUploadLimit: 322122547200, trafficDownloadLimit: 536870912000, status: 1, trojanPanelCoreVersion: '2.3.0', kernelSummary: 'xray 25.8.3' },
  { id: 3, name: 'Frankfurt', ip: 'de.example.com', grpcPort: 8102, grpcTLSMode: 'mtls', grpcTLSServerName: 'core-de.example.com', trafficPeriod: 'year', trafficLimitMode: 'combined', trafficTotalLimit: 2199023255552, trafficUploadLimit: 0, trafficDownloadLimit: 0, status: 1, trojanPanelCoreVersion: '2.2.9', kernelSummary: 'hysteria2 2.6.3' },
  { id: 4, name: 'San Francisco', ip: 'us.example.com', grpcPort: 8103, grpcTLSMode: 'legacy', grpcTLSServerName: '', trafficPeriod: 'month', trafficLimitMode: 'combined', trafficTotalLimit: 879609302220, trafficUploadLimit: 0, trafficDownloadLimit: 0, status: 0, trojanPanelCoreVersion: '2.2.8', kernelSummary: 'naiveproxy 132.0' },
  { id: 5, name: 'Hong Kong', ip: 'hk.example.com', grpcPort: 8104, grpcTLSMode: 'mtls', grpcTLSServerName: 'core-hk.example.com', trafficPeriod: 'month', trafficLimitMode: 'combined', trafficTotalLimit: 1649267441664, trafficUploadLimit: 0, trafficDownloadLimit: 0, status: 1, trojanPanelCoreVersion: '2.3.0', kernelSummary: 'xray 25.8.3' }
]

const serverTrafficStatus = (server) => {
  if (server.trafficPeriod === 'none') return null
  const usage = [0.18, 0.42, 0.05, 0.73, 0.31]
  const ratio = usage[(server.id - 1) % usage.length]
  const separate = server.trafficLimitMode === 'separate'
  const uploadUsed = separate ? Math.round(server.trafficUploadLimit * ratio) : Math.round(server.trafficTotalLimit * ratio * 0.4)
  const downloadUsed = separate ? Math.round(server.trafficDownloadLimit * ratio) : Math.round(server.trafficTotalLimit * ratio * 0.6)
  const totalUsed = uploadUsed + downloadUsed
  const reached = ratio >= 1
  return {
    nodeServerId: server.id,
    nodeServerName: server.name,
    period: server.trafficPeriod,
    limitMode: server.trafficLimitMode,
    uploadUsed,
    downloadUsed,
    totalUsed,
    uploadLimit: server.trafficUploadLimit,
    downloadLimit: server.trafficDownloadLimit,
    totalLimit: server.trafficTotalLimit,
    uploadRemaining: Math.max(0, server.trafficUploadLimit - uploadUsed),
    downloadRemaining: Math.max(0, server.trafficDownloadLimit - downloadUsed),
    totalRemaining: Math.max(0, server.trafficTotalLimit - totalUsed),
    reached
  }
}

const server = http.createServer((req, res) => {
  const url = new URL(req.url, 'http://127.0.0.1')
  const path = url.pathname.replace(/^\/api/, '')
  const trafficRankDate = url.searchParams.get('date') || 'total'
  const isUserSession = req.headers.authorization === 'Bearer user-token'
  const isUserLogin = (req.headers.referer || '').startsWith(
    'http://localhost:'
  )
  res.setHeader('Access-Control-Allow-Origin', '*')
  if (path === '/image/logo') {
    res.setHeader('Content-Type', 'image/svg+xml')
    res.setHeader('Content-Length', logoSvg.length)
    res.end(logoSvg)
    return
  }
  res.setHeader('Content-Type', 'application/json; charset=utf-8')

  const responses = {
    '/auth/setting': {
      registerEnable: 1,
      registerQuota: 10240,
      registerExpireDays: 30,
      trafficRankEnable: 1,
      captchaEnable: 1,
      emailEnable: 0,
      systemName: 'Trojan Panel'
    },
    '/auth/generateCaptcha/': { captchaId: 'mock', captchaImg: captchaSvg },
    '/auth/generateCaptcha': { captchaId: 'mock', captchaImg: captchaSvg },
    '/auth/login': { token: isUserLogin ? 'user-token' : 'mock-token' },
    '/auth/register': null,
    '/account/getAccountInfo': isUserSession
      ? { id: account.id, username: account.username, roles: account.roles }
      : {
          id: 1,
          username: 'sysadmin',
          roles: ['sysadmin', 'admin', 'user']
        },
    '/account/logout': null,
    '/account/selectAccountPage': page('accounts', accounts),
    '/account/selectAccountById': account,
    '/account/exportOptions': [
      {
        id: 'sing-box',
        name: 'sing-box',
        templates: [{ id: 'tun', name: 'TUN' }],
        formats: ['url']
      }
    ],
    '/role/selectRoleList': [
      { id: 1, name: 'sysadmin', desc: 'System Admin' },
      { id: 2, name: 'admin', desc: 'Admin' },
      { id: 3, name: 'user', desc: 'User' }
    ],
    '/dashboard/panelGroup': {
      quota: -1,
      residualFlow: -1,
      nodeCount: nodes.length,
      expireTime: 4078656000000,
      accountCount: accounts.length,
      cpuUsed: 28,
      memUsed: 43,
      diskUsed: 37,
      resetDownloadAndUploadMonth: 1
    },
    '/dashboard/trafficRank': [
      {
        username: `rank-${trafficRankDate}`,
        upload: 4294967296,
        download: 12884901888,
        trafficUsed: 17179869184
      }
    ],
    '/dashboard/serverTrafficUsage': {
      rows: nodeServers.map((item, index) => ({
          nodeServerId: item.id,
          nodeServerName: item.name,
          upload: 7516192768 + index * 1073741824,
          download: 19327352832 + index * 2147483648,
          total: 26843545600 + index * 3221225472
      })),
      pageNum: 1,
      pageSize: 20,
      total: nodeServers.length
    },
    '/dashboard/serverTrafficUserUsage': {
      rows: [
        {
          accountId: 2,
          username: 'glassdemo',
          upload: 4294967296,
          download: 12884901888,
          total: 17179869184
        },
        {
          accountId: 3,
          username: 'aurora',
          upload: 2147483648,
          download: 4294967296,
          total: 6442450944
        },
        {
          accountId: 4,
          username: 'seaglass',
          upload: 1073741824,
          download: 2147483648,
          total: 3221225472
        }
      ],
      pageNum: 1,
      pageSize: 20,
      total: 3
    },
    '/node/selectNodePage': page('nodes', nodes),
    '/node/selectNodeById': Object.assign({}, node, {
      password: 'demo',
      uuid: '00000000-0000-0000-0000-000000000000',
      alterId: 0,
      xrayProtocol: 'vless',
      xraySettingsEntity: {
        fallbacks: [],
        network: 'tcp',
        accounts: [],
        udp: true
      },
      xrayStreamSettingsEntity: {
        network: 'tcp',
        security: 'reality',
        tlsSettings: {},
        realitySettings: {
          dest: 'www.apple.com:443',
          xver: 0,
          serverNames: ['www.apple.com'],
          fingerprint: 'chrome',
          privateKey: 'mock',
          shortIds: ['abcd'],
          spiderX: '/'
        },
        wsSettings: { path: '/', headers: {} }
      }
    }),
    '/node/selectNodeInfo': Object.assign({}, node, {
      password: 'demo',
      uuid: '00000000-0000-0000-0000-000000000000',
      xrayProtocol: 'vless',
      xraySettingsEntity: { fallbacks: [] },
      xrayStreamSettingsEntity: {
        network: 'tcp',
        security: 'reality',
        tlsSettings: {},
        realitySettings: {},
        wsSettings: {}
      }
    }),
    '/node/nodeDefault': {
      publicKey: 'mock-public',
      privateKey: 'mock-private',
      shortId: 'abcd1234',
      spiderX: '/'
    },
    '/nodeType/selectNodeTypeList': [
      { id: 1, name: 'xray' },
      { id: 2, name: 'trojan-go' },
      { id: 3, name: 'hysteria' },
      { id: 4, name: 'naiveproxy' },
      { id: 5, name: 'hysteria2' }
    ],
    '/nodeServer/selectNodeServerList': nodeServers.map(({ id, name }) => ({ id, name })),
    '/nodeServer/selectNodeServerPage': page(
      'nodeServers',
      nodeServers.map((server) => ({ ...server, trafficStatus: serverTrafficStatus(server) }))
    ),
    '/nodeServer/selectNodeServerById': Object.assign({}, nodeServers[0], { trafficStatus: serverTrafficStatus(nodeServers[0]) }),
    '/nodeServer/nodeServerState': { cpuUsed: 28, memUsed: 43, diskUsed: 37 },
    '/nodeServer/resetNodeServerTraffic': { deletedRows: 24 },
    '/kernel/releases': {
      releases: [
        { version: '25.8.3', channel: 'stable' },
        { version: '25.7.26', channel: 'stable' }
      ]
    },
    '/kernel/inventory': {
      os: 'linux',
      arch: 'amd64',
      kernels: {
        xray: { version: '25.8.3', sha256: 'mock-xray', inUse: true },
        hysteria2: { version: '2.6.3', sha256: 'mock-hysteria2', inUse: false }
      }
    },
    '/kernel/selectTaskPage': page('tasks', []),
    '/emailRecord/selectEmailRecordPage': page('emailRecords', []),
    '/fileTask/selectFileTaskPage': page('fileTasks', []),
    '/blackList/selectBlackListPage': page('blackLists', []),
    '/system/selectSystemByName': {
      id: 1,
      registerEnable: 1,
      registerQuota: 10240,
      registerExpireDays: 30,
      resetDownloadAndUploadMonth: 0,
      trafficRankEnable: 1,
      captchaEnable: 1,
      expireWarnEnable: 0,
      expireWarnDay: 0,
      emailEnable: 0,
      emailHost: '',
      emailPort: 25,
      emailUsername: '',
      emailPassword: '',
      systemName: 'Trojan Panel',
      clashRule: '',
      singBoxTun: '{}',
      singBoxOutbound: '{}',
      xrayTemplate: '{}',
      clashTemplateName: 'Default',
      singBoxTunTemplateName: 'TUN',
      singBoxOutboundTemplateName: 'Outbound',
      xrayTemplateName: 'Default'
    }
  }

  const data = Object.prototype.hasOwnProperty.call(responses, path)
    ? responses[path]
    : null
  res.end(ok(data))
})

const port = Number(process.env.MOCK_API_PORT || 8081)
server.listen(port, '127.0.0.1', () => {
  process.stdout.write(`Mock API listening on http://127.0.0.1:${port}\n`)
})
