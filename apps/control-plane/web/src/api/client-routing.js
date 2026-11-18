import request from '@/utils/request'

const path = '/system/clientRouting'

export function routingSettings(id) {
  return request({ url: path, method: 'get', params: id ? { id } : {} })
}

export function publishRouting(profile) {
  return request({ url: path + '/publish', method: 'post', data: profile })
}

export function activateRouting(id) {
  return request({ url: path + '/activate', method: 'post', data: { id } })
}

export function deleteRouting(id) {
  return request({ url: path + '/delete', method: 'post', data: { id } })
}
