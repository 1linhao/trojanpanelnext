import request from '@/utils/request'

export function kernelReleases(params) {
  return request({ url: '/kernel/releases', method: 'get', params })
}

export function kernelInventory(params) {
  return request({ url: '/kernel/inventory', method: 'get', params })
}

export function createKernelTask(data) {
  return request({ url: '/kernel/createTask', method: 'post', data })
}

export function selectKernelTaskPage(params) {
  return request({ url: '/kernel/selectTaskPage', method: 'get', params })
}

export function selectKernelTaskById(params) {
  return request({ url: '/kernel/selectTaskById', method: 'get', params })
}

export function retryKernelTask(data) {
  return request({ url: '/kernel/retryTask', method: 'post', data })
}

export function probeKernelMTLS(data) {
  return request({ url: '/kernel/probeMTLS', method: 'post', data })
}
