import { createShellModel } from '@tp-ui/contracts'

export const ADMIN_GROUPS = Object.freeze([
  { key: 'overview', label: '概览', items: [
    { key: '/dashboard/index', label: '仪表板', mobileLabel: '首页', icon: 'dashboard' }
  ] },
  { key: 'manage', label: '管理', items: [
    { key: '/account-manage/account-list', label: '账号管理', mobileLabel: '账号', icon: 'account', roles: ['sysadmin', 'admin'] },
    { key: '/node-manage/node-list', label: '节点管理', mobileLabel: '节点', icon: 'node' },
    { key: '/server-manage/server-list', label: '服务器', mobileLabel: '服务器', icon: 'server', roles: ['sysadmin', 'admin'] },
    { key: '/server-manage/kernel-upgrade', label: '内核升级', mobileLabel: '内核', icon: 'sysinfo', roles: ['sysadmin'] }
  ] },
  { key: 'operations', label: '运维', items: [
    { key: '/taskManage/task-list', label: '文件任务', mobileLabel: '任务', icon: 'task', roles: ['sysadmin'] },
    { key: '/emailManage/email-record', label: '邮件记录', mobileLabel: '邮件', icon: 'email', roles: ['sysadmin', 'admin'] },
    { key: '/system/black-list', label: '黑名单', mobileLabel: '黑名单', icon: 'pass', roles: ['sysadmin'] }
  ] },
  { key: 'system', label: '系统', items: [
    { key: '/system/base-config', label: '系统配置', mobileLabel: '设置', icon: 'system', roles: ['sysadmin'] },
    { key: '/modify/index', label: '个人资料', mobileLabel: '我的', icon: 'username' }
  ] }
])

export const USER_GROUPS = Object.freeze([
  { key: 'mine', label: '我的', items: [
    { key: '/dashboard/index', label: '我的首页', mobileLabel: '首页', icon: 'dashboard' },
    { key: '/node-manage/node-list', label: '我的节点', mobileLabel: '节点', icon: 'node' },
    { key: '/modify/index', label: '个人资料', mobileLabel: '我的', icon: 'username' }
  ] }
])

export const PAGE_TITLES = Object.freeze({
  '/dashboard/index': '仪表板',
  '/account-manage/account-list': '账号管理',
  '/node-manage/node-list': '节点管理',
  '/server-manage/server-list': '服务器管理',
  '/server-manage/kernel-upgrade': '内核升级',
  '/taskManage/task-list': '文件任务',
  '/emailManage/email-record': '邮件记录',
  '/system/black-list': '黑名单',
  '/system/base-config': '系统配置',
  '/modify/index': '个人资料'
})

export function createTrojanPanelShellModel({ roles = [], username, activePath, pageTitle, branding }) {
  const admin = roles.some((role) => role === 'sysadmin' || role === 'admin')
  const source = admin ? ADMIN_GROUPS : USER_GROUPS
  const groups = source.map((group) => ({
    ...group,
    items: group.items.filter((item) => !item.roles || item.roles.some((role) => roles.includes(role)))
  })).filter((group) => group.items.length)
  return createShellModel({
    brand: { name: branding.systemName, subtitle: '', mark: Array.from(branding.systemName || 'T')[0] },
    activeKey: activePath,
    pageTitle,
    user: { name: username || 'Trojan Panel', label: username || 'Trojan Panel' },
    groups
  })
}
