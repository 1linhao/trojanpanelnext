import { getAccountInfo, login, logout, register } from '@/api/account'
import { getToken, removeToken, setToken } from '@/utils/auth'
import { resetRouter } from '@/router'

const getDefaultState = () => {
  return {
    token: getToken(),
    avatar: '', // 头像
    username: '', // 用户名
    roles: [] // 角色
  }
}

const state = getDefaultState()

const mutations = {
  RESET_STATE: (state) => {
    Object.assign(state, getDefaultState())
  },
  SET_TOKEN: (state, token) => {
    state.token = 'Bearer ' + token
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
          commit('SET_TOKEN', data.token)
          setToken('Bearer ' + data.token)
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

  // get account info
  getAccountInfo({ commit }) {
    return new Promise((resolve, reject) => {
      getAccountInfo()
        .then((response) => {
          const { data } = response

          if (!data) {
            return reject(this.$t('confirm.authFail'))
          }

          const { username, roles } = data

          commit('SET_USERNAME', username)
          commit('SET_ROLES', roles)
          resolve(data)
        })
        .catch((error) => {
          reject(error)
        })
    })
  },

  // account logout
  logout({ commit }) {
    return new Promise((resolve, reject) => {
      logout()
        .then(() => {
          removeToken()
          resetRouter()
          commit('RESET_STATE')
          resolve()
        })
        .catch((error) => {
          reject(error)
        })
    })
  },

  // remove token
  resetToken({ commit }) {
    return new Promise((resolve) => {
      removeToken()
      commit('RESET_STATE')
      resolve()
    })
  }
}

export default {
  namespaced: true,
  state,
  mutations,
  actions
}
