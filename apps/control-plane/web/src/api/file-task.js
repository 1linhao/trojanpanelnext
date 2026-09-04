import request from '@/utils/request'

/**
 * 分页查询文件任务
 * @param data
 * @returns
 */
export function selectFileTaskPage(data) {
  return request({
    url: '/fileTask/selectFileTaskPage',
    method: 'get',
    params: data
  })
}

/**
 * 删除文件任务
 * @param data
 * @returns
 */
export function deleteFileTaskById(data) {
  return request({
    url: '/fileTask/deleteFileTaskById',
    method: 'post',
    data
  })
}

/**
 * 下载文件任务的文件
 * @param data
 * @returns
 */
export function downloadFileTask(data) {
  return request({
    url: '/fileTask/downloadFileTask',
    method: 'post',
    responseType: 'blob',
    data
  })
}

/**
 * 获取文件模板
 * @param data
 * @returns
 */
export function downloadTemplate(data) {
  return request({
    url: '/fileTask/downloadTemplate',
    method: 'post',
    responseType: 'blob',
    data
  })
}
