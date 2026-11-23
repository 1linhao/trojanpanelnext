import Cookies from 'js-cookie'

const TokenKey = 'Authorization'
export const logoutStorageKey = 'trojan-panel:logout'
let logoutVersion = null

export function getLogoutVersion() {
  try {
    return localStorage.getItem(logoutStorageKey) || logoutVersion
  } catch (_) {
    return logoutVersion
  }
}

export function isCurrentSession(session) {
  return Boolean(session.token) && session.token === getToken() &&
    session.logoutVersion === getLogoutVersion()
}

/**
 * 获取token
 * @returns {*}
 */
export function getToken() {
  return Cookies.get(TokenKey)
}

/**
 * 设置token
 * @param token
 * @returns {*}
 */
export function setToken(token) {
  return Cookies.set(TokenKey, token)
}

/**
 * 移除token
 * @returns {*}
 */
export function removeToken() {
  Cookies.remove(TokenKey)
  logoutVersion = `${Date.now()}-${Math.random()}`
  try {
    localStorage.setItem(logoutStorageKey, logoutVersion)
  } catch (_) {
    // Cookie removal still signs out this tab when storage is unavailable.
  }
}
