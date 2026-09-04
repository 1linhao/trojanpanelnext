import request from '@/utils/request'

/**
 * 根据id查询节点
 * @param data
 * @returns
 */
export function selectNodeById(data) {
  return request({
    url: '/node/selectNodeById',
    method: 'get',
    params: data
  })
}

/**
 * 查询节点连接信息
 * @param data
 * @returns
 */
export function selectNodeInfo(data) {
  return request({
    url: '/node/selectNodeInfo',
    method: 'get',
    params: data
  })
}

/**
 * 创建节点
 * @param data
 * @returns
 */
export function createNode(data) {
  return request({
    url: '/node/createNode',
    method: 'post',
    data
  })
}

/**
 * 分页查询节点
 * @param data
 * @returns
 */
export function selectNodePage(data) {
  return request({
    url: '/node/selectNodePage',
    method: 'get',
    params: data
  })
}

/**
 * 删除节点
 * @param data
 * @returns
 */
export function deleteNodeById(data) {
  return request({
    url: '/node/deleteNodeById',
    method: 'post',
    data
  })
}

/**
 * 更新节点
 * @param data
 * @returns
 */
export function updateNodeById(data) {
  return request({
    url: '/node/updateNodeById',
    method: 'post',
    data
  })
}

/**
 * 获取节点二维码
 * @param data
 * @returns
 */
export function nodeQRCode(data) {
  return request({
    url: '/node/nodeQRCode',
    method: 'post',
    data
  })
}

/**
 * 获取节点URL
 * @param data
 * @returns
 */
export function nodeURL(data) {
  return request({
    url: '/node/nodeURL',
    method: 'post',
    data
  })
}

/**
 * 节点部分属性的默认值
 * @param data
 * @returns
 */
export function nodeDefault(data) {
  return request({
    url: '/node/nodeDefault',
    method: 'get',
    params: data
  })
}
