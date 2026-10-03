import request from '@/utils/request'

export function containerInventory(nodeServerId) {
  return request({
    url: '/container/inventory', method: 'get',
    params: { nodeServerId }, timeout: 15000, silentError: true
  })
}

export function updateNodeContainer(data) {
  return request({
    url: '/container/update', method: 'post',
    data: { nodeServerId: data.nodeServerId }, timeout: 15000, silentError: true
  })
}
