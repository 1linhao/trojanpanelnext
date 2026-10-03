import i18n from '@/lang'

export default function getPageTitle(pageTitle) {
  if (pageTitle) {
    const key = `route.${pageTitle}`
    return i18n.te(key) || i18n.te(key, 'en') ? i18n.t(key) : `${pageTitle}`
  }
  return ``
}
