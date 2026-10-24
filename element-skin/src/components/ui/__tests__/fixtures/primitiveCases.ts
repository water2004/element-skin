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
