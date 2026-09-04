import request from '@/utils/request'

/**
 * 登录
 * @param data
 * @returns
 */
export function login(data) {
  return request({
    url: '/auth/login',
    method: 'post',
    data
  })
}

/**
 * 注册账户
 * @param data
 * @returns
 */
export function register(data) {
  return request({
    url: '/auth/register',
    method: 'post',
    data
  })
}

/**
 * 注册账户
 * @param data
 * @returns
 */
export function logout(data) {
  return request({
    url: '/account/logout',
    method: 'post',
    data
  })
}

/**
 * 获取用户信息
 * @returns
 */
export function getAccountInfo() {
  return request({
    url: '/account/getAccountInfo',
    method: 'get'
  })
}

/**
 * 分页查询账户
 * @param data
 * @returns
 */
export function selectAccountPage(data) {
  return request({
    url: '/account/selectAccountPage',
    method: 'get',
    params: data
  })
}

/**
 * 通过id删除账户
 * @param data
 * @returns
 */
export function deleteAccountById(data) {
  return request({
    url: '/account/deleteAccountById',
    method: 'post',
    data
  })
}

/**
 * 修改密码
 * @param data
 * @returns
 */
export function updateAccountPass(data) {
  return request({
    url: '/account/updateAccountPass',
    method: 'post',
    data
  })
}

/**
 * 修改用户名或者邮箱
 * @param data
 * @returns
 */
export function updateAccountProperty(data) {
  return request({
    url: '/account/updateAccountProperty',
    method: 'post',
    data
  })
}

/**
 * 查询单个账户
 * @param data
 * @returns
 */
export function selectAccountById(data) {
  return request({
    url: '/account/selectAccountById',
    method: 'get',
    params: data
  })
}

/**
 * 创建账户
 * @param data
 * @returns
 */
export function createAccount(data) {
  return request({
    url: '/account/createAccount',
    method: 'post',
    data
  })
}

/**
 * 修改账户
 * @param data
 * @returns
 */
export function updateAccountById(data) {
  return request({
    url: '/account/updateAccountById',
    method: 'post',
    data
  })
}

export function exportOptions() {
  return request({
    url: '/account/exportOptions',
    method: 'get'
  })
}

export function exportSubscribe(data) {
  return request({
    url: '/account/exportSubscribe',
    method: 'get',
    params: data
  })
}

export function exportQRCode(data) {
  return request({
    url: '/account/exportQRCode',
    method: 'get',
    params: data
  })
}

/**
 * 重设下载和上传流量
 * @param data
 * @returns
 */
export function resetAccountDownloadAndUpload(data) {
  return request({
    url: '/account/resetAccountDownloadAndUpload',
    method: 'post',
    data
  })
}

/**
 * 导出用户
 * @param data
 * @returns {*}
 */
export function exportAccount(data) {
  return request({
    url: '/account/exportAccount',
    method: 'post',
    data
  })
}

/**
 * 导入用户
 * @param data
 * @returns {*}
 */
export function importAccount(data) {
  return request({
    url: '/account/importAccount',
    method: 'post',
    data
  })
}

/**
 * 生成验证码
 * @param data
 * @returns
 */
export function generateCaptcha() {
  return request({
    url: '/auth/generateCaptcha/',
    method: 'get'
  })
}

/**
 * 批量生成账户
 * @param data
 * @returns {*}
 */

export function createAccountBatch(data) {
  return request({
    url: '/account/createAccountBatch',
    method: 'post',
    data
  })
}

/**
 * 导出未使用的账户
 * @param data
 * @returns {*}
 */

export function exportAccountUnused(data) {
  return request({
    url: '/account/exportAccountUnused',
    method: 'post',
    data
  })
}
