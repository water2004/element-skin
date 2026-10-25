import { computed, onMounted, onUnmounted, provide, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'
import { getPublicSettings } from '@/api/public'
import { siteLogout } from '@/api/auth'
import { useSessionStore } from '@/stores/session'
import { useAvatar } from '@/composables/useAvatar'
import { useNotificationIndicator } from '@/composables/useNotificationIndicator'
import { useTheme } from '@/composables/useTheme'
import { appStorage } from '@/storage'
import { canAccessSitePath } from '@/permissions/sitePages'
import {
  buildAccountLinks,
  buildDefaultOpeneds,
  buildDrawerLinks,
  buildNavLinks,
  canAccessAdmin as canAccessAdminPanel,
  type DrawerLink,
  type NavLink,
} from '@/components/layout/appNavigation'
import {
  cleanupEasterEgg,
  installEasterEggDevTools,
  refreshEasterEgg,
  setServerEasterEggConfig,
} from '@/easter-eggs'

export function useAppLayoutState() {
  const {
    currentAvatarImg: customAvatar,
    avatarHash,
    isAvatarLoading,
    initializeAvatar,
  } = useAvatar()
  const { hasUnreadNotifications, refreshUnreadNotifications, clearUnreadNotifications } =
    useNotificationIndicator()
  const { isDark, initTheme, toggleTheme } = useTheme()
  const route = useRoute()
  const { push } = useRouter()

  const isHome = computed(() => route.path === '/')
  const isAuthPage = computed(() => ['/login', '/register', '/reset-password'].includes(route.path))
  const siteName = ref(appStorage.siteSettings.getSiteName())
  const enableSkinLibrary = ref(appStorage.siteSettings.getEnableSkinLibrary())
  const sessionStore = useSessionStore()
  const { user, authReady } = storeToRefs(sessionStore)
  const drawer = ref(false)
  const footerText = ref('')
  const filingIcp = ref('')
  const filingIcpLink = ref('')
  const filingMps = ref('')
  const filingMpsLink = ref('')

  const isLogged = computed(() => !!user.value)
  const avatarConfigured = computed(() => !!(user.value?.avatar_hash || avatarHash.value))
  const userPermissions = computed(() => user.value?.permissions || [])
  const canAccessAdmin = computed(() => canAccessAdminPanel(userPermissions.value))
  const canAccessNotifications = computed(
    () => isLogged.value && canAccessSitePath('/notifications', userPermissions.value),
  )
  const accountLinks = computed(() => buildAccountLinks(userPermissions.value))
  const defaultOpeneds = computed(() => buildDefaultOpeneds(route.path))
  const navLinks = computed<NavLink[]>(() =>
    buildNavLinks({
      path: route.path,
      isLogged: isLogged.value,
      enableSkinLibrary: enableSkinLibrary.value,
      userPermissions: userPermissions.value,
    }),
  )
  const drawerLinks = computed<DrawerLink[]>(() =>
    buildDrawerLinks({
      isLogged: isLogged.value,
      enableSkinLibrary: enableSkinLibrary.value,
      userPermissions: userPermissions.value,
    }),
  )
  const activeRoute = computed(() => route.path)
  const showFooter = computed(() => !isAuthPage.value)
  const repoUrl = 'https://github.com/water2004/element-skin'
  const repoLabel = `Element Skin ${typeof __APP_VERSION__ !== 'undefined' ? __APP_VERSION__ : 'v1.3.0'}`
  const isSuperAdmin = computed(() =>
    userPermissions.value.includes('permission_protected.manage.any'),
  )
  const accountRoleLabel = computed(() =>
    isSuperAdmin.value ? '超级管理员' : canAccessAdmin.value ? '管理员' : '普通用户',
  )
  const accountName = computed(() => user.value?.display_name || user.value?.email || '用户')

  let unreadRefreshTimer: number | null = null

  function shouldShowNotificationBadge(item: NavLink | DrawerLink) {
    return (
      canAccessNotifications.value && item.path === '/notifications' && hasUnreadNotifications.value
    )
  }

  function startUnreadRefreshTimer() {
    if (unreadRefreshTimer !== null) return
    unreadRefreshTimer = window.setInterval(() => {
      if (canAccessNotifications.value) void refreshUnreadNotifications()
    }, 60_000)
  }

  function stopUnreadRefreshTimer() {
    if (unreadRefreshTimer === null) return
    window.clearInterval(unreadRefreshTimer)
    unreadRefreshTimer = null
  }

  function go(path: string) {
    void push(path)
    drawer.value = false
  }

  function clearSessionState() {
    sessionStore.clearSession()
    stopUnreadRefreshTimer()
    drawer.value = false
  }

  async function logout() {
    try {
      await siteLogout()
    } catch {}
    clearSessionState()
    await push('/')
    window.location.reload()
  }

  watch(
    [() => user.value?.id, () => user.value?.avatar_hash],
    ([, hash]) => {
      if (user.value) void initializeAvatar(hash)
    },
    { immediate: true },
  )

  provide('isDark', isDark)

  watch(
    [() => user.value?.id, canAccessNotifications],
    ([, canRead]) => {
      clearUnreadNotifications()
      stopUnreadRefreshTimer()
      if (canRead) {
        void refreshUnreadNotifications()
        startUnreadRefreshTimer()
      }
    },
    { immediate: true },
  )

  onMounted(async () => {
    appStorage.cleanupUnusedKeys()
    initTheme()
    installEasterEggDevTools()
    void refreshEasterEgg()
    void sessionStore.fetchMe()
    try {
      const res = await getPublicSettings()
      if (res.data.site_name) {
        siteName.value = res.data.site_name
        appStorage.siteSettings.setSiteName(res.data.site_name)
        document.title = res.data.site_name
      }
      if (res.data.enable_skin_library !== undefined) {
        enableSkinLibrary.value = res.data.enable_skin_library
        appStorage.siteSettings.setEnableSkinLibrary(res.data.enable_skin_library)
      }
      if (res.data.footer_text !== undefined) footerText.value = res.data.footer_text
      if (res.data.filing_icp !== undefined) filingIcp.value = res.data.filing_icp
      if (res.data.filing_icp_link !== undefined) filingIcpLink.value = res.data.filing_icp_link
      if (res.data.filing_mps !== undefined) filingMps.value = res.data.filing_mps
      if (res.data.filing_mps_link !== undefined) filingMpsLink.value = res.data.filing_mps_link
      setServerEasterEggConfig(res.data.easter_eggs)
    } catch (e) {
      console.warn('Failed to load site settings:', e)
    }
  })

  onUnmounted(() => {
    stopUnreadRefreshTimer()
    clearUnreadNotifications()
    cleanupEasterEgg()
  })

  return {
    customAvatar,
    avatarConfigured,
    isAvatarLoading,
    isDark,
    toggleTheme,
    isHome,
    isAuthPage,
    siteName,
    authReady,
    drawer,
    footerText,
    filingIcp,
    filingIcpLink,
    filingMps,
    filingMpsLink,
    isLogged,
    canAccessAdmin,
    canAccessNotifications,
    accountLinks,
    defaultOpeneds,
    navLinks,
    drawerLinks,
    activeRoute,
    showFooter,
    repoUrl,
    repoLabel,
    accountRoleLabel,
    accountName,
    go,
    logout,
    clearSessionState,
    shouldShowNotificationBadge,
  }
}
