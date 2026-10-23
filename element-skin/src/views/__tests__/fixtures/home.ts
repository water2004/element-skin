import type { SiteSettings, User } from '@/api/types'

export const homeSettings: SiteSettings = {
  site_name: '测试皮肤站',
  site_subtitle: '首页入场测试',
}

export const homeUser: User = {
  id: 'home-user',
  email: 'home@example.com',
}

export const homeActionCases = [
  { loggedIn: false, label: '登录账号', path: '/login' },
  { loggedIn: false, label: '即刻注册', path: '/register' },
  { loggedIn: true, label: '进入个人面板', path: '/dashboard' },
]
