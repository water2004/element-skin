import { computed, nextTick, onMounted, onUnmounted, provide, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getPublicSettings } from '@/api/public'
import { getMe } from '@/api/me'
import { siteLogout } from '@/api/auth'
import type { User as UserType } from '@/api/types'
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
import type AppFooter from '@/components/layout/AppFooter.vue'
import {
  cleanupEasterEgg,
  installEasterEggDevTools,
  refreshEasterEgg,
  setServerEasterEggConfig,
} from '@/easter-eggs'

const HOME_HEADER_HEIGHT = 64

export function useAppLayoutState() {
  const { currentAvatarImg: customAvatar, initializeAvatar } = useAvatar()
  const { hasUnreadNotifications, refreshUnreadNotifications, clearUnreadNotifications } =
    useNotificationIndicator()
  const { isDark, initTheme, toggleTheme } = useTheme()
  const route = useRoute()
  const { push } = useRouter()

  const isHome = computed(() => route.path === '/')
  const isAuthPage = computed(() => ['/login', '/register', '/reset-password'].includes(route.path))
  const siteName = ref(appStorage.siteSettings.getSiteName())
  const enableSkinLibrary = ref(appStorage.siteSettings.getEnableSkinLibrary())
  const user = ref<UserType | null>(null)
  const authReady = ref(false)
  const drawer = ref(false)
  const footerText = ref('')
  const filingIcp = ref('')
  const filingIcpLink = ref('')
  const filingMps = ref('')
  const filingMpsLink = ref('')
  const footerHeight = ref(0)
  const footerRef = ref<InstanceType<typeof AppFooter> | null>(null)
  const homeCenterOffset = computed(() => `${(HOME_HEADER_HEIGHT - footerHeight.value) / 2}px`)
  const homeContentCenterY = computed(() => `calc(50vh + ${homeCenterOffset.value})`)

  const isLogged = computed(() => !!user.value)
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

  let resizeObserver: ResizeObserver | null = null
  let unreadRefreshTimer: number | null = null

  function updateFooterHeight() {
    nextTick(() => {
      if (footerRef.value?.rootElement)
        footerHeight.value = footerRef.value.rootElement.offsetHeight
      else footerHeight.value = 0
    })
  }

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

  async function logout() {
    try {
      await siteLogout()
    } catch {}
    user.value = null
    clearUnreadNotifications()
    stopUnreadRefreshTimer()
    authReady.value = true
    void push('/')
    setTimeout(() => window.location.reload(), 100)
  }

  async function fetchMe() {
    try {
      const res = await getMe()
      user.value = res.data
      if (res.data.avatar_hash) {
        initializeAvatar(res.data.avatar_hash)
      }
    } catch {
      user.value = null
      clearUnreadNotifications()
      stopUnreadRefreshTimer()
    } finally {
      authReady.value = true
    }
  }

  watch([() => route.path, footerText, filingIcp, filingMps], updateFooterHeight)

  provide('user', user)
  provide('fetchMe', fetchMe)
  provide('authReady', authReady)
  provide('isDark', isDark)
  provide('footerHeight', footerHeight)

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
    void fetchMe()
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
      updateFooterHeight()
    } catch (e) {
      console.warn('Failed to load site settings:', e)
    }

    if (window.ResizeObserver) {
      resizeObserver = new ResizeObserver(() => updateFooterHeight())
      nextTick(() => {
        if (footerRef.value?.rootElement) resizeObserver!.observe(footerRef.value.rootElement)
      })
    }
    window.addEventListener('resize', updateFooterHeight)
  })

  onUnmounted(() => {
    window.removeEventListener('resize', updateFooterHeight)
    if (resizeObserver) resizeObserver.disconnect()
    stopUnreadRefreshTimer()
    clearUnreadNotifications()
    cleanupEasterEgg()
  })

  return {
    customAvatar,
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
    footerHeight,
    footerRef,
    homeCenterOffset,
    homeContentCenterY,
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
    shouldShowNotificationBadge,
  }
}
