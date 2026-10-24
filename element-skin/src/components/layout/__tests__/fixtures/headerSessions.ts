export const headerSessions = [
  {
    label: 'an account with notification access',
    isLogged: true,
    canAccessNotifications: true,
    expectedButtons: ['通知中心', '切换深色主题'],
  },
  {
    label: 'an account without notification access',
    isLogged: true,
    canAccessNotifications: false,
    expectedButtons: ['切换深色主题'],
  },
  {
    label: 'an anonymous session',
    isLogged: false,
    canAccessNotifications: false,
    expectedButtons: ['切换深色主题', '登录', '注册'],
  },
]

export const headerAccountLinks = [
  { path: '/dashboard/profile', title: '个人资料' },
  { path: '/dashboard/identities', title: '身份管理' },
]
