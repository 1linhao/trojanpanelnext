import axios from 'axios'
import { Message, MessageBox } from '@/utils/liquid-feedback'
import store from '@/store'
import { getToken } from '@/utils/auth'
import i18n from '@/lang'

const service = axios.create({
  baseURL: import.meta.env.VITE_BASE_API,
  timeout: 5000
})

service.interceptors.request.use(
  (config) => {
    if (store.getters.token) {
      config.headers['Authorization'] = getToken()
    }
    return config
  },
  (error) => {
    // console.log(error)
    return Promise.reject(error)
  }
)

service.interceptors.response.use(
  (response) => {
    const res = response.data
    if (res.code === 20000) {
      return res
    } else if (res instanceof Blob) {
      return response
    } else {
      Message({
        message: res.message || 'Error',
        type: 'error',
        duration: 5 * 1000
      })

      if (res.code === 50014 || res.code === 50008 || res.code === 50401) {
        MessageBox.confirm(i18n.t('confirm.logoutPrompt'), 'Confirm logout', {
          confirmButtonText: i18n.t('confirm.logoutConfirm'),
          cancelButtonText: i18n.t('confirm.cancel'),
          type: 'warning'
        }).then(() => {
          store.dispatch('account/resetToken').then(() => {
            location.reload()
          })
        })
      }
      return Promise.reject(new Error(res.message || 'Error'))
    }
  },
  (error) => {
    // console.log('err' + error)
    Message({
      message: error.message,
      type: 'error',
      duration: 5 * 1000
    })
    return Promise.reject(error)
  }
)

export default service
