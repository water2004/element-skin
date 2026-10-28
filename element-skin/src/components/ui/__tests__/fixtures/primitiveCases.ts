export const buttonVariants = [
  { variant: 'primary', tone: 'neutral', type: 'primary', plain: false, text: false },
  { variant: 'secondary', tone: 'neutral', type: 'default', plain: false, text: false },
  { variant: 'tertiary', tone: 'neutral', type: 'default', plain: false, text: true },
  { variant: 'primary', tone: 'success', type: 'success', plain: false, text: false },
  { variant: 'secondary', tone: 'danger', type: 'danger', plain: true, text: false },
  { variant: 'tertiary', tone: 'warning', type: 'warning', plain: false, text: true },
] as const

export const cardPaddings = [
  { padding: 'none', pixels: '0px' },
  { padding: 'compact', pixels: '16px' },
  { padding: 'default', pixels: '20px' },
  { padding: 'spacious', pixels: '24px' },
] as const

export const buttonAccents = [
  { tone: 'neutral', color: '#409eff' },
  { tone: 'primary', color: '#409eff' },
  { tone: 'success', color: '#67c23a' },
  { tone: 'warning', color: '#e6a23c' },
  { tone: 'danger', color: '#f56c6c' },
] as const

export const launcherLink = {
  href: 'authlib-injector:yggdrasil-server:https%3A%2F%2Fskin.example.test%2Fapi',
  label: '复制或拖到启动器',
}
