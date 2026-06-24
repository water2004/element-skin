<template>
  <div class="app-shell" :class="{ 'is-home-layout': isHome, 'is-auth-layout': isAuthPage }">
    <el-header class="layout-header-wrap" v-if="!isAuthPage">
      <div class="layout-header">
        <!-- Logo -->
        <div class="logo" @click="go('/')">{{ siteName }}</div>

        <!-- Desktop Navigation -->
        <div class="desktop-nav">
          <el-menu
            mode="horizontal"
            :default-active="activeRoute"
            router
            :ellipsis="false"
            :default-openeds="defaultOpeneds"
          >
            <template v-for="(item, index) in navLinks" :key="item.path || item.index">
              <el-sub-menu v-if="item.type === 'group'" :index="item.index" :trigger="item.trigger">
                <template #title>
                  <span>{{ item.title }}</span>
                </template>
                <el-menu-item v-for="child in item.children" :key="child.path" :index="child.path">
                  <el-icon v-if="child.icon"><component :is="child.icon" /></el-icon>
                  <span>{{ child.title }}</span>
                </el-menu-item>
              </el-sub-menu>
              <el-menu-item
                v-else-if="!item.adminOnly || isAdmin"
                :index="item.path"
                :class="'nav-priority-' + (index + 1)"
              >
                <el-icon v-if="item.icon"><component :is="item.icon" /></el-icon>
                <span>{{ item.title }}</span>
              </el-menu-item>
            </template>
          </el-menu>
        </div>

        <div class="header-actions">
          <!-- Theme Toggle -->
          <el-button
            class="theme-toggle"
            :icon="isDark ? Sunny : Moon"
            circle
            text
            @click="toggleTheme"
          />

          <!-- Mobile Nav Trigger -->
          <div class="mobile-nav" v-if="authReady && isLogged">
            <el-button
              @click="drawer = true"
              :icon="MenuIcon"
              text
              circle
              class="mobile-menu-btn"
            />
          </div>

          <!-- Account Popover -->
          <el-popover
            v-if="authReady && isLogged"
            placement="bottom-end"
            :width="240"
            trigger="hover"
            popper-class="account-popover"
            :show-arrow="false"
            :offset="4"
          >
            <template #reference>
              <div class="account-trigger">
                <el-avatar
                  :shape="customAvatar ? 'square' : 'circle'"
                  size="small"
                  :class="[
                    'account-avatar',
                    {
                      'bg-gradient-to-br from-[#b37feb] to-[#8553cf]': !customAvatar,
                      'has-custom': !!customAvatar,
                    },
                  ]"
                  :src="customAvatar || ''"
                >
                  {{ !customAvatar ? avatarInitial : '' }}
                </el-avatar>
                <span class="account-name">{{ accountName }}</span>
              </div>
            </template>
            <div
              class="account-panel rounded-[16px] border border-[var(--color-border)] bg-[var(--color-card-background)] shadow-[0_4px_12px_rgba(0,0,0,0.05)]"
            >
              <div class="account-header">
                <el-avatar
                  :shape="customAvatar ? 'square' : 'circle'"
                  :size="48"
                  :class="[
                    'account-avatar',
                    {
                      'bg-gradient-to-br from-[#b37feb] to-[#8553cf]': !customAvatar,
                      'has-custom': !!customAvatar,
                    },
                  ]"
                  :src="customAvatar || ''"
                >
                  {{ !customAvatar ? avatarInitial : '' }}
                </el-avatar>
                <div class="account-meta">
                  <h4>{{ accountName }}</h4>
                  <p>{{ accountRoleLabel }}</p>
                </div>
              </div>
              <div class="account-actions">
                <UiButton variant="outline" @click="go('/dashboard')">
                  <span>个人面板</span>
                </UiButton>
                <UiButton v-if="isAdmin" variant="outline" @click="go('/admin')">
                  <span>管理面板</span>
                </UiButton>
                <UiButton variant="outline-danger" @click="logout">
                  <span>退出登录</span>
                </UiButton>
              </div>
            </div>
          </el-popover>

          <!-- Auth Buttons -->
          <template v-if="authReady && !isLogged">
            <el-button type="primary" @click="go('/login')">登录</el-button>
            <el-button @click="go('/register')" class="hero-register-btn ml-2"> 注册 </el-button>
          </template>
        </div>
      </div>
    </el-header>

    <!-- Mobile Drawer -->
    <el-drawer v-model="drawer" title="导航菜单" direction="ltr" size="280px" class="mobile-drawer">
      <el-menu :default-active="activeRoute" router @select="drawer = false" class="drawer-menu">
        <template v-for="(item, index) in drawerLinks" :key="index">
          <el-divider v-if="item.isDivider" class="nav-divider" />
          <el-menu-item v-else :index="item.path">
            <el-icon v-if="item.icon"><component :is="item.icon" /></el-icon>
            <span>{{ item.title }}</span>
          </el-menu-item>
        </template>
      </el-menu>
    </el-drawer>

    <main
      class="app-main"
      :style="{
        '--footer-height': footerHeight + 'px',
        '--home-center-offset': homeCenterOffset,
      }"
    >
      <slot />
    </main>

    <AppFooter
      v-if="showFooter"
      ref="footerRef"
      :variant="isHome ? 'home' : 'standard'"
      :footer-text="footerText"
      :filing-icp="filingIcp"
      :filing-icp-link="filingIcpLink"
      :filing-mps="filingMps"
      :filing-mps-link="filingMpsLink"
      :repo-url="repoUrl"
      :repo-label="repoLabel"
    />
  </div>
</template>

<script setup lang="ts">
import {
  computed,
  ref,
  onMounted,
  onUnmounted,
  provide,
  watch,
  nextTick,
  type Component,
} from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getPublicSettings } from '@/api/public'
import { getMe } from '@/api/me'
import { siteLogout } from '@/api/auth'
import type { User as UserType } from '@/api/types'
import {
  Menu as MenuIcon,
  Box,
  User,
  Setting,
  Tools,
  Back,
  Odometer,
  Link,
  Picture,
  Message,
  Moon,
  Sunny,
  MagicStick,
} from '@element-plus/icons-vue'

import { useAvatar } from '@/composables/useAvatar'
import { appStorage } from '@/storage'
import AppFooter from '@/components/layout/AppFooter.vue'
import UiButton from '@/components/ui/UiButton.vue'
import {
  cleanupEasterEgg,
  installEasterEggDevTools,
  refreshEasterEgg,
  setServerEasterEggConfig,
} from '@/easter-eggs'

interface NavLink {
  type?: 'item' | 'group'
  path?: string
  index?: string
  trigger?: 'hover' | 'click'
  title?: string
  icon?: Component
  adminOnly?: boolean
  children?: NavLink[]
}

interface DrawerLink {
  isDivider?: boolean
  path?: string
  title?: string
  icon?: Component
}

const { currentAvatarImg: customAvatar, initializeAvatar } = useAvatar()
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
const HOME_HEADER_HEIGHT = 64
const homeCenterOffset = computed(() => `${(HOME_HEADER_HEIGHT - footerHeight.value) / 2}px`)

const updateFooterHeight = () => {
  nextTick(() => {
    if (footerRef.value?.rootElement) footerHeight.value = footerRef.value.rootElement.offsetHeight
    else footerHeight.value = 0
  })
}

watch([() => route.path, footerText, filingIcp, filingMps], updateFooterHeight)

const isDark = ref(false)
function initTheme() {
  const savedTheme = appStorage.theme.get()
  if (savedTheme) isDark.value = savedTheme === 'dark'
  else isDark.value = window.matchMedia('(prefers-color-scheme: dark)').matches
  applyTheme()
}
function toggleTheme() {
  isDark.value = !isDark.value
  appStorage.theme.set(isDark.value ? 'dark' : 'light')
  applyTheme()
}
function applyTheme() {
  document.documentElement.classList.toggle('dark', isDark.value)
}

window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
  if (!appStorage.theme.hasUserPreference()) {
    isDark.value = e.matches
    applyTheme()
  }
})

provide('user', user)
provide('fetchMe', fetchMe)
provide('authReady', authReady)
provide('isDark', isDark)
provide('footerHeight', footerHeight)

const dashboardLinks: NavLink[] = [
  { path: '/dashboard/home', title: '仪表盘', icon: Odometer },
  { path: '/dashboard/wardrobe', title: '我的衣柜', icon: Box },
  { path: '/dashboard/roles', title: '角色管理', icon: User },
  { path: '/dashboard/profile', title: '个人资料', icon: Setting },
]
const adminNavLinks: NavLink[] = [
  { path: '/dashboard', title: '返回面板', icon: Back },
  { path: '/admin/users', title: '用户管理', icon: User },
  { path: '/admin/roles', title: '角色管理', icon: User },
  { path: '/admin/textures', title: '材质管理', icon: Box },
  { path: '/admin/invites', title: '邀请码管理', icon: Tools },
  { path: '/admin/settings', title: '站点设置', icon: Setting },
  { path: '/admin/email', title: '邮件服务', icon: Message },
  { path: '/admin/mojang', title: 'Fallback 服务', icon: Link },
  { path: '/admin/homepage-media', title: '首页图片', icon: Picture },
  { path: '/admin/easter-eggs', title: '彩蛋列表', icon: MagicStick },
]

const adminNavItems = computed<NavLink[]>(() => [
  { type: 'item', path: '/dashboard', title: '返回面板', icon: Back },
  {
    type: 'group',
    index: 'admin-content-group',
    title: '用户与内容',
    trigger: 'click',
    children: [
      { path: '/admin/users', title: '用户管理', icon: User },
      { path: '/admin/roles', title: '角色管理', icon: User },
      { path: '/admin/textures', title: '材质管理', icon: Box },
    ],
  },
  { type: 'item', path: '/admin/invites', title: '邀请码管理', icon: Tools },
  { type: 'item', path: '/admin/settings', title: '站点设置', icon: Setting },
  {
    type: 'group',
    index: 'admin-config-group',
    title: '更多设置',
    trigger: 'click',
    children: [
      { path: '/admin/email', title: '邮件服务', icon: Message },
      { path: '/admin/mojang', title: 'Fallback 服务', icon: Link },
      { path: '/admin/homepage-media', title: '首页图片', icon: Picture },
      { path: '/admin/easter-eggs', title: '彩蛋列表', icon: MagicStick },
    ],
  },
])

const defaultOpeneds = computed(() => {
  const path = route.path
  const opened: string[] = []
  if (['/admin/users', '/admin/roles', '/admin/textures'].some((p) => path.startsWith(p))) {
    opened.push('admin-content-group')
  }
  if (
    ['/admin/email', '/admin/mojang', '/admin/homepage-media', '/admin/easter-eggs'].some((p) =>
      path.startsWith(p),
    )
  ) {
    opened.push('admin-config-group')
  }
  return opened
})

const navLinks = computed<NavLink[]>(() => {
  if (route.path.startsWith('/admin')) return adminNavItems.value
  const links: NavLink[] = []
  if (isLogged.value) {
    if (enableSkinLibrary.value)
      links.push({ path: '/skin-library', title: '皮肤库', icon: Picture })
    links.push(...dashboardLinks)
    if (isAdmin.value) links.push({ path: '/admin', title: '管理面板', icon: Tools })
  }
  return links
})

const drawerLinks = computed<DrawerLink[]>(() => {
  const links: DrawerLink[] = []
  if (isLogged.value) {
    if (enableSkinLibrary.value)
      links.push({ path: '/skin-library', title: '皮肤库', icon: Picture })
    links.push({ isDivider: true })
    links.push(...dashboardLinks)
    if (isAdmin.value) {
      links.push({ isDivider: true })
      links.push(...adminNavLinks)
    }
  }
  return links
})

const activeRoute = computed(() => route.path)
const showFooter = computed(() => !isAuthPage.value)
const repoUrl = 'https://github.com/water2004/element-skin'
// REPAIRED: Correct version number display
const repoLabel = `Element Skin ${typeof __APP_VERSION__ !== 'undefined' ? __APP_VERSION__ : 'v1.3.0'}`

const isLogged = computed(() => !!user.value)
const isAdmin = computed(() => user.value?.is_admin || false)
const isSuperAdmin = computed(() => user.value?.is_super_admin || false)
const accountRoleLabel = computed(() =>
  isSuperAdmin.value ? '超级管理员' : isAdmin.value ? '管理员' : '普通用户',
)
const accountName = computed(() => user.value?.display_name || user.value?.email || '用户')
const avatarInitial = computed(() => (accountName.value || 'U').slice(0, 1).toUpperCase())

let resizeObserver: ResizeObserver | null = null

function go(path: string) {
  push(path)
  drawer.value = false
}
async function logout() {
  try {
    await siteLogout()
  } catch {}
  user.value = null
  authReady.value = true
  push('/')
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
  } finally {
    authReady.value = true
  }
}

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
  cleanupEasterEgg()
})
</script>

<style>
.app-shell :where(.page-header) {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  margin-bottom: 40px;
  flex-wrap: wrap;
  gap: 20px;
}

.app-shell :where(.page-header-content h1) {
  font-size: 32px;
  margin: 0 0 8px 0;
  background: linear-gradient(135deg, var(--color-heading) 0%, #409eff 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.app-shell :where(.page-header-content p) {
  margin: 0;
  color: var(--color-text-light);
  font-size: 16px;
  transition: color 0.3s ease;
}

.app-shell :where(.page-header-actions) {
  display: flex;
  gap: 12px;
}

.app-shell :where(.form-tip) {
  font-size: 12px;
  color: var(--color-text-light);
  margin-top: 6px;
  line-height: 1.4;
}

.app-shell :where(.pagination-container) {
  margin-top: 32px;
  padding-bottom: 8px;
  display: flex;
  justify-content: center;
  align-items: center;
  width: 100%;
  animation: fadeIn 0.6s ease;
}
</style>

<style scoped>
.app-shell {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

/* Home Mode Shell */
.is-home-layout {
  min-height: 100vh;
  position: fixed;
  inset: 0;
  overflow: hidden;
}

.layout-header-wrap {
  padding: 0 20px;
  background: var(--color-header-background);
  backdrop-filter: blur(8px);
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.08);
  border-bottom: 1px solid var(--color-border);
  height: 64px;
  z-index: 100;
  flex-shrink: 0;
}

.is-home-layout .layout-header-wrap {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  background: transparent;
  border-bottom: none;
  box-shadow: none;
  backdrop-filter: none;
  z-index: 20;
}

/* Home Layout UI Enforcement - Scoped to .layout-header */
.is-home-layout .layout-header .logo,
.is-home-layout .layout-header .account-name,
.is-home-layout .layout-header .theme-toggle,
.is-home-layout .layout-header .mobile-menu-btn,
.is-home-layout .layout-header :deep(.el-menu-item),
.is-home-layout .layout-header :deep(.el-sub-menu__title) {
  color: #fff !important;
}

.is-home-layout .layout-header .account-trigger:hover,
.is-home-layout .layout-header .logo:hover,
.is-home-layout .layout-header .theme-toggle:hover,
.is-home-layout .layout-header .mobile-menu-btn:hover,
.is-home-layout .layout-header :deep(.el-menu-item:hover),
.is-home-layout .layout-header :deep(.el-menu-item.is-active),
.is-home-layout .layout-header :deep(.el-sub-menu__title:hover),
.is-home-layout .layout-header :deep(.el-sub-menu__title.is-active) {
  background-color: rgba(255, 255, 255, 0.15) !important;
  color: #fff !important;
}

.is-home-layout .header-actions :deep(.el-button--primary) {
  background: rgba(64, 158, 255, 0.3) !important;
  border: 1px solid rgba(64, 158, 255, 0.4) !important;
  color: #fff !important;
  border-radius: 8px;
}
.is-home-layout .hero-register-btn {
  background: rgba(255, 255, 255, 0.15) !important;
  border: 1px solid rgba(255, 255, 255, 0.25) !important;
  color: #fff !important;
  border-radius: 8px;
  height: 32px;
  padding: 0 15px;
  font-size: 14px;
}

/* Mobile Drawer reset - Respect Global Theme */
.mobile-drawer :deep(.el-menu) {
  border-right: none;
  background: transparent;
}
.mobile-drawer :deep(.el-menu-item) {
  color: var(--color-text);
  border-radius: 8px;
  margin: 4px 8px;
  height: 44px;
  line-height: 44px;
}
.mobile-drawer :deep(.el-menu-item.is-active) {
  background-color: rgba(64, 158, 255, 0.1);
  color: var(--el-color-primary);
  font-weight: 600;
}

.layout-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 100%;
}
.logo {
  font-weight: 700;
  font-size: 20px;
  color: var(--color-heading);
  cursor: pointer;
  border-radius: 8px;
  padding: 4px 8px;
  transition: background-color 0.2s;
}
.logo:hover {
  color: var(--el-color-primary);
}

.desktop-nav {
  flex-grow: 1;
  display: flex;
  justify-content: center;
  height: 100%;
}
.desktop-nav .el-menu {
  border-bottom: none;
  height: 100%;
  background: transparent;
}

.desktop-nav :deep(.el-sub-menu__title) {
  border-bottom: 2px solid transparent;
  transition:
    color 0.2s,
    border-color 0.2s;
}
.desktop-nav :deep(.el-sub-menu__title:hover) {
  color: var(--el-color-primary);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.theme-toggle {
  font-size: 20px;
  border-radius: 8px;
}

.app-main {
  --header-height: 64px;
  padding: 20px;
  flex: 1;
  display: flex;
  flex-direction: column;
  background-color: var(--color-background);
  transition: padding 0.3s ease;
}
.is-home-layout .app-main {
  position: fixed;
  inset: 0;
  z-index: 0;
  padding: 0;
  flex: none;
  height: 100vh;
  min-height: 100vh;
  background: transparent;
}
.is-auth-layout .app-main {
  padding: 0 !important;
}

/* Account */
.account-trigger {
  display: flex;
  align-items: center;
  cursor: pointer;
  gap: 8px;
  padding: 6px 12px;
  border-radius: 20px;
  transition: background-color 0.2s;
}
.account-trigger:hover {
  background: var(--color-background-soft);
}
.account-name {
  font-size: 14px;
  color: var(--color-text);
  font-weight: 500;
}
.account-popover {
  padding: 0 !important;
  background: var(--color-popover-background) !important;
  border: 1px solid var(--color-border) !important;
}

.account-panel {
  padding: 20px;
  width: 100%;
  border: none !important;
}
.account-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--color-border);
}
.account-meta h4 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-heading);
}
.account-meta p {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--color-text-light);
}
.account-actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}
.account-actions :deep(.el-button) {
  width: 100%;
}
.account-actions :deep(.el-button + .el-button) {
  margin-left: 0 !important;
}

.account-avatar.has-custom {
  background: transparent !important;
}
.account-avatar.has-custom :deep(img) {
  object-fit: contain;
}

.filing-icon {
  width: 13px;
}

@media (max-width: 1200px) {
  .mobile-nav {
    display: block;
  }
}
@media (max-width: 768px) {
  .desktop-nav {
    display: none;
  }
}
</style>
