import request from '@/utils/request'

/**
 * 分页查询黑名单
 * @param data
 * @returns
 */
export function selectBlackListPage(data) {
  return request({
    url: '/blackList/selectBlackListPage',
    method: 'get',
    params: data
  })
}

/**
 * 删除黑名单
 * @param data
 * @returns
 */
export function deleteBlackListByIp(data) {
  return request({
    url: '/blackList/deleteBlackListByIp',
    method: 'post',
    data
  })
}

/**
 * 删除黑名单
 * @param data
 * @returns
 */
export function createBlackList(data) {
  return request({
    url: '/blackList/createBlackList',
    method: 'post',
    data
  })
}
