import axios from 'axios'
import { Message, MessageBox } from '@/utils/liquid-feedback'
import store from '@/store'
import { isCurrentSession } from '@/utils/auth'
import i18n from '@/lang'

const service = axios.create({
  baseURL: import.meta.env.VITE_BASE_API,
  timeout: 5000
})

service.interceptors.request.use(
  (config) => {
    const { token, logoutVersion } = store.state.account
    if (token) {
      config.accountSession = { token, logoutVersion }
      if (isCurrentSession(config.accountSession)) config.headers['Authorization'] = token
    }
    return config
  },
  (error) => {
    // console.log(error)
    return Promise.reject(error)
  }
)

const isStaleRequest = (config) => config?.accountSession && !isCurrentSession(config.accountSession)

service.interceptors.response.use(
  async (response) => {
    let res = response.data
    if (res instanceof Blob && response.config.jsonAttachment) {
      const disposition = response.headers['content-disposition'] || ''
      if (/^attachment; filename=.+$/i.test(disposition)) return response
      // Authentication and application errors still use the standard JSON envelope.
      try {
        res = JSON.parse(await res.text()) || {}
      } catch (_) {
        res = {}
      }
    }
    if (res.code === 20000) {
      return res
    } else if (res instanceof Blob) {
      return response
    } else {
      if (isStaleRequest(response.config)) return Promise.reject(new Error(res.message || 'Error'))
      if (!response.config || !response.config.silentError) {
        Message({
          message: res.message || 'Error',
          type: 'error',
          duration: 5 * 1000
        })
      }

      if (response.config.accountSession && [50014, 50008, 50401].includes(res.code)) {
        MessageBox.confirm(i18n.t('confirm.logoutPrompt'), 'Confirm logout', {
          confirmButtonText: i18n.t('confirm.logoutConfirm'),
          cancelButtonText: i18n.t('confirm.cancel'),
          type: 'warning'
        }).then(async () => {
          if (isStaleRequest(response.config)) return
          await store.dispatch('account/resetToken')
          location.reload()
        }).catch(() => {})
      }
      return Promise.reject(Object.assign(new Error(res.message || 'Error'), { code: res.code }))
    }
  },
  (error) => {
    if (axios.isCancel(error) || isStaleRequest(error.config)) return Promise.reject(error)
    if (!error.config || !error.config.silentError) {
      Message({
        message: error.message,
        type: 'error',
        duration: 5 * 1000
      })
    }
    return Promise.reject(error)
  }
)

export default service
