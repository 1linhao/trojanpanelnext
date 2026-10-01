import request from '@/utils/request'

/**
 * 根据id查询服务器
 * @param data
 * @returns
 */
export function selectNodeServerById(data) {
  return request({
    url: '/nodeServer/selectNodeServerById',
    method: 'get',
    params: data
  })
}

/**
 * 创建服务器
 * @param data
 * @returns
 */
export function createNodeServer(data) {
  return request({
    url: '/nodeServer/createNodeServer',
    method: 'post',
    data
  })
}

export function nodeServerDeployment(id) {
  return request({
    url: '/nodeServer/deployment',
    method: 'get',
    params: { id }
  })
}

// Axios decodes all responses to Blob in download mode, including JSON errors.
// Verify the archive signature before allowing a browser download.
export async function decodeNodeDeploymentDownload(response) {
  const blob = response && response.data
  if (!(blob instanceof Blob) || !blob.size) throw new Error('Empty deployment archive')
  const prefix = new Uint8Array(await blob.slice(0, 2).arrayBuffer())
  if (prefix[0] === 0x1f && prefix[1] === 0x8b) return response
  let payload
  if (blob.size <= 1048576) {
    try { payload = JSON.parse(await blob.text()) } catch (_) { /* Not JSON. */ }
  }
  const error = new Error(payload && payload.message ? payload.message : 'Invalid deployment archive')
  if (payload && payload.code !== undefined) error.code = payload.code
  throw error
}

export async function downloadNodeDeployment(data) {
  try {
    const response = await request({
      url: '/nodeServer/downloadDeployment',
      method: 'post',
      responseType: 'blob',
      timeout: 30000,
      data
    })
    return await decodeNodeDeploymentDownload(response)
  } catch (error) {
    if (error.response && error.response.data instanceof Blob) {
      await decodeNodeDeploymentDownload(error.response)
    }
    throw error
  }
}

/**
 * 分页查询服务器
 * @param data
 * @returns
 */
export function selectNodeServerPage(data) {
  return request({
    url: '/nodeServer/selectNodeServerPage',
    method: 'get',
    params: data
  })
}

/**
 * 删除服务器
 * @param data
 * @returns
 */
export function deleteNodeServerById(data) {
  return request({
    url: '/nodeServer/deleteNodeServerById',
    method: 'post',
    timeout: 30000,
    data
  })
}

/**
 * 卸载目标 Node 宿主机，再移除 Web 记录
 */
export function uninstallNodeServerById(data) {
  return request({
    url: '/nodeServer/uninstallNodeServerById',
    method: 'post',
    timeout: 240000,
    data
  })
}

/**
 * 重置指定服务器的流量统计
 * @param data
 * @returns
 */
export function resetNodeServerTraffic(data) {
  return request({
    url: '/nodeServer/resetNodeServerTraffic',
    method: 'post',
    data
  })
}

/**
 * 更新服务器
 * @param data
 * @returns
 */
export function updateNodeServerById(data) {
  return request({
    url: '/nodeServer/updateNodeServerById',
    method: 'post',
    data
  })
}

/**
 * 查询服务器列表
 * @param data
 * @returns
 */
export function selectNodeServerList(data) {
  return request({
    url: '/nodeServer/selectNodeServerList',
    method: 'get',
    params: data
  })
}

/**
 * 查询服务器状态
 * @param data
 * @returns
 */
export function nodeServerState(data) {
  return request({
    url: '/nodeServer/nodeServerState',
    method: 'get',
    params: data
  })
}

/**
 * 导出服务器
 * @param data
 * @returns {*}
 */
export function exportNodeServer(data) {
  return request({
    url: '/nodeServer/exportNodeServer',
    method: 'post',
    data
  })
}

/**
 * 导出服务器
 * @param data
 * @returns {*}
 */
export function importNodeServer(data) {
  return request({
    url: '/nodeServer/importNodeServer',
    method: 'post',
    data
  })
}
