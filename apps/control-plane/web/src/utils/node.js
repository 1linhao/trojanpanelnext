export function getNodeTypeName(nodeType) {
  const nodeTypeMap = {
    1: 'xray',
    2: 'trojan-go',
    3: 'hysteria',
    4: 'naiveproxy',
    5: 'hysteria2'
  }
  return nodeTypeMap[nodeType]
}

export function isXray(temp) {
  return getNodeTypeName(temp.nodeTypeId) === 'xray'
}

export function isTrojanGo(temp) {
  return getNodeTypeName(temp.nodeTypeId) === 'trojan-go'
}

export function isHysteria(temp) {
  return getNodeTypeName(temp.nodeTypeId) === 'hysteria'
}

export function isHysteria2(temp) {
  return getNodeTypeName(temp.nodeTypeId) === 'hysteria2'
}

export function isNaiveProxy(temp) {
  return getNodeTypeName(temp.nodeTypeId) === 'naiveproxy'
}

export function isXrayVless(temp) {
  return isXray(temp) && temp.xrayProtocol === 'vless'
}

export function isXrayVmess(temp) {
  return isXray(temp) && temp.xrayProtocol === 'vmess'
}

export function isXrayTrojan(temp) {
  return isXray(temp) && temp.xrayProtocol === 'trojan'
}

export function isXrayShadowsocks(temp) {
  return isXray(temp) && temp.xrayProtocol === 'shadowsocks'
}

export function isXrayShadowsocksAEAD(temp) {
  return isXray(temp) && temp.xrayProtocol === 'shadowsocks-AEAD'
}

export function isXrayShadowsocks2022(temp) {
  return isXray(temp) && temp.xrayProtocol === 'shadowsocks-2022'
}

export function isXraySocks(temp) {
  return isXray(temp) && temp.xrayProtocol === 'socks'
}

export function isXrayWs(temp) {
  return isXray(temp) && temp.xrayStreamSettingsEntity.network === 'ws'
}

export function showXrayFlow(temp) {
  return (
    isXrayVless(temp) &&
    (temp.xrayStreamSettingsEntity.security === 'tls' ||
      temp.xrayStreamSettingsEntity.security === 'reality')
  )
}

export function showFallback(temp) {
  return (
    (isXrayVless(temp) || isXrayTrojan(temp)) &&
    temp.xrayStreamSettingsEntity.network === 'tcp' &&
    temp.xrayStreamSettingsEntity.security === 'tls'
  )
}

export function isTrojanGoEnableWebsocket(temp) {
  return isTrojanGo(temp) && temp.trojanGoWebsocketEnable === 1
}

export function isTrojanGoEnableSs(temp) {
  return isTrojanGo(temp) && temp.trojanGoSsEnable === 1
}

export function nodeTypeFind(nodeTypes, nodeTypeId) {
  let nodeType = nodeTypes.find((item) => item.id === nodeTypeId)
  if (nodeType && nodeType.name) {
    return nodeType.name
  }
  return ''
}

export function nodeServerFind(nodeServers, nodeServerId) {
  let nodeServer = nodeServers.find((item) => item.id === nodeServerId)
  if (nodeServer && nodeServer.name) {
    return nodeServer.name
  } else {
    return ''
  }
}

export function showUUID(temp) {
  return isXrayVless(temp) || isXrayVmess(temp)
}

export function showAlterId(temp) {
  return isXrayVmess(temp)
}

export function isXrayStreamSettingsSecurityTls(temp) {
  return isXray(temp) && temp.xrayStreamSettingsEntity.security === 'tls'
}

export function isXrayStreamSettingsSecurityReality(temp) {
  return isXray(temp) && temp.xrayStreamSettingsEntity.security === 'reality'
}

export function handleNodeDetail(nodeDetail, responseData) {
  nodeDetail.password = responseData.password
  nodeDetail.clients = Array.isArray(responseData.clients)
    ? responseData.clients
    : ['sing-box', 'clash-meta', 'v2ray', 'shadowrocket']
  if (nodeDetail.nodeTypeId === 1) {
    nodeDetail.xrayProtocol = responseData.xrayProtocol
    nodeDetail.xraySettings = responseData.xraySettings
    nodeDetail.xraySettingsEntity = Object.assign(
      nodeDetail.xraySettingsEntity,
      responseData.xraySettingsEntity
    )
    nodeDetail.xrayStreamSettingsEntity = Object.assign(
      nodeDetail.xrayStreamSettingsEntity,
      responseData.xrayStreamSettingsEntity
    )
    nodeDetail.xrayTag = responseData.xrayTag
    nodeDetail.xraySniffing = responseData.xraySniffing
    nodeDetail.xrayAllocate = responseData.xrayAllocate
    nodeDetail.uuid = responseData.uuid
    nodeDetail.alterId = responseData.alterId
    nodeDetail.xrayFlow = responseData.xrayFlow
    nodeDetail.xraySSMethod = responseData.xraySSMethod
    nodeDetail.xrayUotEnable = responseData.xrayUotEnable || 0
    nodeDetail.xrayUotVersion = responseData.xrayUotVersion || 2
    nodeDetail.xrayXudpEnable = responseData.xrayXudpEnable || 0
    nodeDetail.xrayMuxEnable = responseData.xrayMuxEnable || 0
    nodeDetail.realityPbk = responseData.realityPbk
  }
  if (nodeDetail.nodeTypeId === 2) {
    nodeDetail.trojanGoSni = responseData.trojanGoSni
    nodeDetail.trojanGoMuxEnable = responseData.trojanGoMuxEnable
    nodeDetail.trojanGoWebsocketEnable = responseData.trojanGoWebsocketEnable
    nodeDetail.trojanGoWebsocketPath = responseData.trojanGoWebsocketPath
    nodeDetail.trojanGoWebsocketHost = responseData.trojanGoWebsocketHost
    nodeDetail.trojanGoSsEnable = responseData.trojanGoSsEnable
    nodeDetail.trojanGoSsMethod = responseData.trojanGoSsMethod
    nodeDetail.trojanGoSsPassword = responseData.trojanGoSsPassword
  }
  if (nodeDetail.nodeTypeId === 3) {
    nodeDetail.hysteriaProtocol = responseData.hysteriaProtocol
    nodeDetail.hysteriaObfs = responseData.hysteriaObfs
    nodeDetail.hysteriaUpMbps = responseData.hysteriaUpMbps
    nodeDetail.hysteriaDownMbps = responseData.hysteriaDownMbps
    nodeDetail.hysteriaServerName = responseData.hysteriaServerName
    nodeDetail.hysteriaInsecure = responseData.hysteriaInsecure
    nodeDetail.hysteriaFastOpen = responseData.hysteriaFastOpen
  }
  if (nodeDetail.nodeTypeId === 4) {
    nodeDetail.naiveProxyUsername = responseData.naiveProxyUsername
    nodeDetail.naiveUotEnable = responseData.naiveUotEnable
    nodeDetail.naiveUotVersion = responseData.naiveUotVersion || 2
  }
  if (nodeDetail.nodeTypeId === 5) {
    nodeDetail.hysteria2ObfsPassword = responseData.hysteria2ObfsPassword
    nodeDetail.hysteria2UpMbps = responseData.hysteria2UpMbps
    nodeDetail.hysteria2DownMbps = responseData.hysteria2DownMbps
    nodeDetail.hysteria2ServerName = responseData.hysteria2ServerName
    nodeDetail.hysteria2PortHopping = responseData.hysteria2PortHopping
    nodeDetail.hysteria2HopInterval =
      responseData.hysteria2HopInterval || undefined
    nodeDetail.hysteria2Insecure = responseData.hysteria2Insecure
  }
  return nodeDetail
}

export function handleNodeUpdate(temp, responseData) {
  temp.clients = Array.isArray(responseData.clients)
    ? responseData.clients
    : ['sing-box', 'clash-meta', 'v2ray', 'shadowrocket']
  if (temp.nodeTypeId === 1) {
    temp.xrayProtocol = responseData.xrayProtocol
    temp.xraySettings = responseData.xraySettings
    temp.xraySettingsEntity = Object.assign(
      temp.xraySettingsEntity,
      responseData.xraySettingsEntity
    )
    temp.xrayStreamSettingsEntity = Object.assign(
      temp.xrayStreamSettingsEntity,
      responseData.xrayStreamSettingsEntity
    )
    temp.xrayTag = responseData.xrayTag
    temp.xraySniffing = responseData.xraySniffing
    temp.xrayAllocate = responseData.xrayAllocate
    temp.uuid = responseData.uuid
    temp.alterId = responseData.alterId
    temp.xrayFlow = responseData.xrayFlow
    temp.xraySSMethod = responseData.xraySSMethod
    temp.xrayUotEnable = responseData.xrayUotEnable || 0
    temp.xrayUotVersion = responseData.xrayUotVersion || 2
    temp.xrayXudpEnable = responseData.xrayXudpEnable || 0
    temp.xrayMuxEnable = responseData.xrayMuxEnable || 0
    temp.realityPbk = responseData.realityPbk
  }
  if (temp.nodeTypeId === 2) {
    temp.trojanGoSni = responseData.trojanGoSni
    temp.trojanGoMuxEnable = responseData.trojanGoMuxEnable
    temp.trojanGoWebsocketEnable = responseData.trojanGoWebsocketEnable
    temp.trojanGoWebsocketPath = responseData.trojanGoWebsocketPath
    temp.trojanGoWebsocketHost = responseData.trojanGoWebsocketHost
    temp.trojanGoSsEnable = responseData.trojanGoSsEnable
    temp.trojanGoSsMethod = responseData.trojanGoSsMethod
    temp.trojanGoSsPassword = responseData.trojanGoSsPassword
  }
  if (temp.nodeTypeId === 3) {
    temp.hysteriaProtocol = responseData.hysteriaProtocol
    temp.hysteriaObfs = responseData.hysteriaObfs
    temp.hysteriaUpMbps = responseData.hysteriaUpMbps
    temp.hysteriaDownMbps = responseData.hysteriaDownMbps
    temp.hysteriaServerName = responseData.hysteriaServerName
    temp.hysteriaInsecure = responseData.hysteriaInsecure
    temp.hysteriaFastOpen = responseData.hysteriaFastOpen
  }
  if (temp.nodeTypeId === 4) {
    temp.naiveProxyUsername = responseData.naiveProxyUsername
    temp.naiveUotEnable = responseData.naiveUotEnable
    temp.naiveUotVersion = responseData.naiveUotVersion || 2
  }
  if (temp.nodeTypeId === 5) {
    temp.hysteria2ObfsPassword = responseData.hysteria2ObfsPassword
    temp.hysteria2UpMbps = responseData.hysteria2UpMbps
    temp.hysteria2DownMbps = responseData.hysteria2DownMbps
    temp.hysteria2ServerName = responseData.hysteria2ServerName
    temp.hysteria2PortHopping = responseData.hysteria2PortHopping
    temp.hysteria2HopInterval = responseData.hysteria2HopInterval || undefined
    temp.hysteria2Insecure = responseData.hysteria2Insecure
  }
  return temp
}

export const fingerprints = [
  'chrome',
  'firefox',
  'safari',
  'ios',
  'android',
  'edge',
  '360',
  'qq',
  'random',
  'randomized'
]

// 处理Xray节点配置XraySettings字段 去除掉无效配置
export function handleXraySettings(nodeProps) {
  if (isXrayVless(nodeProps)) {
    nodeProps.xraySettingsEntity.decryption = 'none'
  }
  if (isXraySocks(nodeProps)) {
    nodeProps.xraySettingsEntity.auth = 'password'
    nodeProps.xraySettingsEntity.ip = '127.0.0.1'
  } else {
    nodeProps.xraySettingsEntity.udp = undefined
    nodeProps.xraySettingsEntity.ip = undefined
    nodeProps.xraySettingsEntity.auth = undefined
    nodeProps.xraySettingsEntity.accounts = undefined
  }
  if (!isXrayShadowsocks(nodeProps)) {
    nodeProps.xraySettingsEntity.network = undefined
  }

  if (!showFallback(nodeProps)) {
    nodeProps.xraySettingsEntity.fallbacks = undefined
  }
  return JSON.stringify(nodeProps.xraySettingsEntity)
}

// 处理Xray节点配置StreamSettings字段 去除掉无效配置
export function handleXrayStreamSettings(nodeProps) {
  if (nodeProps.xrayStreamSettingsEntity.security !== 'tls') {
    nodeProps.xrayStreamSettingsEntity.tlsSettings = undefined
  } else if (isXrayShadowsocks(nodeProps)) {
    nodeProps.xrayStreamSettingsEntity.tlsSettings.serverName = undefined
    nodeProps.xrayStreamSettingsEntity.tlsSettings.alpn = undefined
    nodeProps.xrayStreamSettingsEntity.tlsSettings.allowInsecure = undefined
    nodeProps.xrayStreamSettingsEntity.tlsSettings.fingerprint = undefined
  }
  if (nodeProps.xrayStreamSettingsEntity.security !== 'reality') {
    nodeProps.xrayStreamSettingsEntity.realitySettings = undefined
  }
  if (nodeProps.xrayStreamSettingsEntity.network !== 'ws') {
    nodeProps.xrayStreamSettingsEntity.wsSettings = undefined
  }
  return JSON.stringify(nodeProps.xrayStreamSettingsEntity)
}
