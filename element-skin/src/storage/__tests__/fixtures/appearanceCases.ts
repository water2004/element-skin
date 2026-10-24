export const themeCases = [
  { label: 'saved light over system dark', saved: 'light', systemDark: true, expected: 'light' },
  { label: 'saved dark over system light', saved: 'dark', systemDark: false, expected: 'dark' },
  { label: 'system dark without a preference', saved: null, systemDark: true, expected: 'dark' },
  { label: 'system light without a preference', saved: null, systemDark: false, expected: 'light' },
  { label: 'invalid preference', saved: 'sepia', systemDark: true, expected: 'dark' },
]

export const appearancePaths = [
  { base: '/', path: '/', home: true },
  { base: '/', path: '/dashboard/home', home: false },
  { base: '/skin/', path: '/skin/', home: true },
  { base: '/skin/', path: '/skin', home: true },
  { base: '/skin/', path: '/skin/dashboard/home', home: false },
  { base: '/skin/', path: '/skin-other/', home: false },
]
