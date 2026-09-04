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
