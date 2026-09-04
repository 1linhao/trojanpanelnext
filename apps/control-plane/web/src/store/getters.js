const getters = {
  sidebar: (state) => state.app.sidebar,
  language: (state) => state.app.language,
  size: (state) => state.app.size,
  device: (state) => state.app.device,
  visitedViews: (state) => state.tagsView.visitedViews,
  cachedViews: (state) => state.tagsView.cachedViews,
  token: (state) => state.account.token,
  avatar: (state) => state.account.avatar,
  username: (state) => state.account.username,
  roles: (state) => state.account.roles,
  permission_routes: (state) => state.permission.routes
}
export default getters
