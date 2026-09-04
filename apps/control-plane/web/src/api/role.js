import request from '@/utils/request'

/**
 * 查询角色列表
 * @param data
 * @returns
 */
export function selectRoleList(data) {
  return request({
    url: '/role/selectRoleList',
    method: 'get',
    params: data
  })
}
