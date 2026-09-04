import request from '@/utils/request'

/**
 * 分页查询发送记录
 * @param data
 * @returns
 */
export function selectEmailRecordPage(data) {
  return request({
    url: '/emailRecord/selectEmailRecordPage',
    method: 'get',
    params: data
  })
}
