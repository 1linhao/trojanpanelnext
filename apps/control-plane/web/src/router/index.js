import Vue from 'vue'
import Router from 'vue-router'

const Layout = () =>
  import('@/layout')

Vue.use(Router)

export const constantRoutes = [
  {
    path: '/redirect',
    component: Layout,
    hidden: true,
    children: [
      {
        path: '/redirect/:path(.*)',
        component: () =>
          import('@/views/redirect/index')
      }
    ]
  },

  {
    path: '/login',
    component: () =>
      import('@/views/login/index'),
    hidden: true
  },

  {
    path: '/register',
    component: () =>
      import('@/views/register/index'),
    hidden: true
  },

  {
    path: '/',
    redirect: '/dashboard/index',
    hidden: true
  },

  {
    path: '/dashboard',
    component: Layout,
    redirect: '/dashboard/index',
    meta: { title: 'dashboard', name: 'dashboard', icon: 'dashboard' },
    children: [
      {
        path: 'index',
        name: 'index',
        component: () =>
          import('@/views/dashboard'),
        meta: {
          title: 'dashboard',
          name: 'dashboard',
          icon: 'dashboard'
        }
      }
    ]
  },

  {
    path: '/modify',
    component: Layout,
    hidden: true,
    redirect: '/modify/index',
    meta: { title: 'modify', name: 'modify' },
    children: [
      {
        path: 'index',
        name: 'index',
        component: () =>
          import('@/views/account/modify'),
        meta: {
          title: 'modify',
          name: 'modify'
        }
      }
    ]
  },

  {
    path: '/node-manage',
    name: 'nodeManage',
    component: Layout,
    redirect: '/node-manage/node-list',
    meta: { title: 'nodeManage', name: 'nodeManage', icon: 'node' },
    children: [
      {
        path: 'node-list',
        name: 'nodeList',
        component: () =>
          import('@/views/node/list'),
        meta: {
          title: 'nodeList',
          name: 'nodeList'
        }
      }
    ]
  },

  {
    path: '/404',
    component: () =>
      import('@/views/404'),
    hidden: true
  }
]

export const asyncRoutes = [
  {
    path: '/account-manage',
    name: 'accountManage',
    component: Layout,
    redirect: '/account-manage/account-list',
    meta: {
      title: 'accountManage',
      name: 'accountManage',
      icon: 'account',
      roles: ['sysadmin', 'admin']
    },
    children: [
      {
        path: 'account-list',
        name: 'accountList',
        component: () =>
          import('@/views/account/list'),
        meta: {
          title: 'accountList',
          name: 'accountList',
          roles: ['sysadmin', 'admin']
        }
      }
    ]
  },

  {
    path: '/server-manage',
    name: 'serverManage',
    component: Layout,
    redirect: '/server-manage/server-list',
    meta: {
      title: 'serverManage',
      name: 'serverManage',
      icon: 'server',
      roles: ['sysadmin', 'admin']
    },
    children: [
      {
        path: 'server-list',
        name: 'serverList',
        component: () =>
          import('@/views/node-server/list'),
        meta: {
          title: 'serverList',
          name: 'serverList',
          roles: ['sysadmin', 'admin']
        }
      },
      {
        path: 'kernel-upgrade',
        name: 'kernelUpgrade',
        component: () =>
          import('@/views/kernel-upgrade'),
        hidden: true,
        meta: {
          title: 'kernelUpgrade',
          name: 'kernelUpgrade',
          roles: ['sysadmin']
        }
      }
    ]
  },

  {
    path: '/emailManage',
    name: 'emailManage',
    component: Layout,
    redirect: '/emailManage/email-record',
    meta: {
      title: 'emailManage',
      name: 'emailManage',
      icon: 'email',
      roles: ['sysadmin', 'admin']
    },
    children: [
      {
        path: 'email-record',
        name: 'emailRecord',
        component: () =>
          import('@/views/email/record'),
        meta: {
          title: 'emailRecord',
          name: 'emailRecord',
          roles: ['sysadmin', 'admin']
        }
      }
    ]
  },

  {
    path: '/taskManage',
    name: 'taskManage',
    component: Layout,
    redirect: '/taskManage/task-list',
    meta: {
      title: 'taskManage',
      name: 'taskManage',
      icon: 'task',
      roles: ['sysadmin', 'admin']
    },
    children: [
      {
        path: 'task-list',
        name: 'taskList',
        component: () =>
          import('@/views/task-manage/list'),
        meta: {
          title: 'taskList',
          name: 'taskList',
          roles: ['sysadmin']
        }
      }
    ]
  },

  {
    path: '/system',
    name: 'system',
    component: Layout,
    redirect: '/system/base-config',
    meta: {
      title: 'system',
      name: 'system',
      icon: 'system',
      roles: ['sysadmin']
    },
    children: [
      {
        path: 'base-config',
        name: 'baseConfig',
        component: () =>
          import('@/views/system/base'),
        meta: {
          title: 'baseConfig',
          name: 'baseConfig',
          roles: ['sysadmin']
        }
      },
      {
        path: 'black-list',
        name: 'blackList',
        component: () =>
          import('@/views/system/black'),
        meta: {
          title: 'blackList',
          name: 'blackList',
          roles: ['sysadmin']
        }
      }
    ]
  },

  { path: '*', redirect: '/404', hidden: true }
]

const createRouter = () =>
  new Router({
    scrollBehavior: () => ({ y: 0 }),
    routes: constantRoutes
  })

const router = createRouter()

export function resetRouter() {
  const newRouter = createRouter()
  router.matcher = newRouter.matcher
}

export default router
