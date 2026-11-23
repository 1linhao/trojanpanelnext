import { getAccountInfo, login, logout, register } from '@/api/account'
import { getLogoutVersion, getToken, isCurrentSession, removeToken, setToken } from '@/utils/auth'
import { resetRouter } from '@/router'
import i18n from '@/lang'

const getDefaultState = (token) => {
  return {
    token,
    logoutVersion: getLogoutVersion(),
    avatar: '', // 头像
    username: '', // 用户名
    roles: [] // 角色
  }
}

const state = getDefaultState(getToken())

const mutations = {
  RESET_STATE: (state) => {
    Object.assign(state, getDefaultState())
  },
  SET_TOKEN: (state, token) => {
    state.token = 'Bearer ' + token
    state.logoutVersion = getLogoutVersion()
  },
  SET_USERNAME: (state, username) => {
    state.username = username
  },
  SET_ROLES: (state, roles) => {
    state.roles = roles
  }
}

const actions = {
  // account login
  login({ commit }, accountInfo) {
    const { username, pass, captchaId, captchaCode } = accountInfo
    return new Promise((resolve, reject) => {
      login({
        username: username.trim(),
        pass: pass,
        captchaId: captchaId,
        captchaCode: captchaCode
      })
        .then((response) => {
          const { data } = response
          setToken('Bearer ' + data.token)
          commit('SET_TOKEN', data.token)
          resolve()
        })
        .catch((error) => {
          reject(error)
        })
    })
  },

  // account register
  register(...actionArguments) {
    const accountInfo = actionArguments[1]
    const { username, pass, captchaId, captchaCode } = accountInfo
    return new Promise((resolve, reject) => {
      register({
        username: username.trim(),
        pass: pass.trim(),
        captchaId: captchaId,
        captchaCode: captchaCode
      })
        .then(() => {
          resolve()
        })
        .catch((error) => {
          reject(error)
        })
    })
  },

  async getAccountInfo({ commit, state }) {
    const session = { token: state.token, logoutVersion: state.logoutVersion }
    const { data } = await getAccountInfo()
    if (!isCurrentSession(session) || state.token !== session.token) return null
    if (!data?.roles?.length) throw new Error(i18n.t('confirm.authFail'))
    commit('SET_USERNAME', data.username)
    commit('SET_ROLES', data.roles)
    return data
  },

  async logout({ dispatch, state }) {
    const session = { token: state.token, logoutVersion: state.logoutVersion }
    await logout()
    if (isCurrentSession(session)) await dispatch('resetToken')
  },

  resetToken({ dispatch }) {
    removeToken()
    return dispatch('clearSession')
  },

  clearSession({ commit }) {
    resetRouter()
    commit('RESET_STATE')
    commit('permission/CLEAR_ROUTES', null, { root: true })
  }
}

export default {
  namespaced: true,
  state,
  mutations,
  actions
}
