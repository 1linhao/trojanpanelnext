import request from '@/utils/request'

/**
 * 仪表板
 * @param data
 * @returns
 */
export function panelGroup(data) {
  return request({
    url: '/dashboard/panelGroup',
    method: 'get',
    params: data
  })
}

/**
 * 流量排行榜
 * @param data
 * @returns
 */
export function trafficRank(data) {
  return request({
    url: '/dashboard/trafficRank',
    method: 'get',
    params: data
  })
}

export function serverTrafficUsage(data) {
  return request({
    url: '/dashboard/serverTrafficUsage',
    method: 'get',
    params: data
  })
}

export function serverTrafficUserUsage(data) {
  return request({
    url: '/dashboard/serverTrafficUserUsage',
    method: 'get',
    params: data
  })
}
