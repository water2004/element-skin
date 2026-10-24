import { onUnmounted, ref } from 'vue'

import { appStorage } from '@/storage'
import { applyTheme, resolveTheme } from '@/storage/appearance'

export function useTheme() {
  const isDark = ref(resolveTheme() === 'dark')
  let mediaQuery: MediaQueryList | null = null

  function applyCurrentTheme() {
    applyTheme(isDark.value ? 'dark' : 'light')
  }

  function handlePreferenceChange(event: MediaQueryListEvent) {
    if (!appStorage.theme.hasUserPreference()) {
      isDark.value = event.matches
      applyCurrentTheme()
    }
  }

  function startSystemPreferenceWatcher() {
    if (mediaQuery || typeof window === 'undefined') return
    mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
    mediaQuery.addEventListener('change', handlePreferenceChange)
  }

  function stopSystemPreferenceWatcher() {
    if (!mediaQuery) return
    mediaQuery.removeEventListener('change', handlePreferenceChange)
    mediaQuery = null
  }

  function initTheme() {
    isDark.value = resolveTheme() === 'dark'
    applyCurrentTheme()
    startSystemPreferenceWatcher()
  }

  function toggleTheme() {
    isDark.value = !isDark.value
    appStorage.theme.set(isDark.value ? 'dark' : 'light')
    applyCurrentTheme()
  }

  onUnmounted(stopSystemPreferenceWatcher)

  return {
    isDark,
    initTheme,
    toggleTheme,
  }
}
