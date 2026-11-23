import router from '@/router'
import store from '@/store'
import { Message } from '@/utils/liquid-feedback'
import NProgress from 'nprogress'
import 'nprogress/nprogress.css'
import { isCurrentSession, logoutStorageKey } from '@/utils/auth'
import getPageTitle from '@/utils/get-page-title'
import i18n from '@/lang'

NProgress.configure({ showSpinner: false })

const publicPaths = ['/login', '/register']
const loginLocation = (redirect) => ({ path: '/login', query: { redirect }, replace: true })

function clearStaleSession() {
  const session = store.state.account
  if (!session.token || isCurrentSession(session)) return false
  store.dispatch('account/clearSession')
  Message.info(i18n.t('navbar.loggedOutElsewhere'))
  return true
}

function syncOpenTab() {
  if (!clearStaleSession()) return
  const redirect = router.currentRoute.matched.length
    ? router.currentRoute.fullPath
    : window.location.hash.slice(1) || '/'
  router.replace(loginLocation(redirect)).catch(() => {})
}

window.addEventListener('storage', (event) => {
  if (event.key === logoutStorageKey) syncOpenTab()
})
window.addEventListener('pageshow', syncOpenTab)
document.addEventListener('visibilitychange', () => {
  if (!document.hidden) syncOpenTab()
})

function isLoginDestination(route) {
  return route.matched.length > 0 && route.path !== '/404' &&
    !publicPaths.includes(route.path) && route.path !== '/redirect' && !route.path.startsWith('/redirect/')
}

router.beforeEach(async (to, from, next) => {
  NProgress.start()
  document.title = getPageTitle(to.meta.title)
  clearStaleSession()
  const { token, logoutVersion, roles } = store.state.account

  if (!token) {
    next(publicPaths.includes(to.path) ? undefined : loginLocation(to.fullPath))
    return
  }
  if (to.path === '/login') {
    next({ path: '/' })
    return
  }

  const session = { token, logoutVersion }
  if (roles.length === 0) {
    const abandonStaleNavigation = () => {
      if (clearStaleSession()) next(loginLocation(to.fullPath))
      else next(false)
    }
    try {
      const info = await store.dispatch('account/getAccountInfo')
      if (!info || !isCurrentSession(session)) return abandonStaleNavigation()
      const accessRoutes = await store.dispatch('permission/generateRoutes', info.roles)
      if (!isCurrentSession(session)) return abandonStaleNavigation()
      accessRoutes.forEach((route) => router.addRoute(route))
    } catch (error) {
      if (!isCurrentSession(session)) return abandonStaleNavigation()
      await store.dispatch('account/resetToken')
      Message.error(error.message || i18n.t('confirm.authFail'))
      next(loginLocation(to.fullPath))
      return
    }
  }

  const destination = to.redirectedFrom || to.fullPath
  if (from.path === '/login' && !isLoginDestination(router.resolve(destination).route)) {
    next({ path: '/', replace: true })
  } else if (roles.length === 0) {
    next({ path: destination, replace: true })
  } else {
    next()
  }
})

router.afterEach(() => NProgress.done())
router.onError(() => NProgress.done())
