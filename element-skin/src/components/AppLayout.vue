<template>
  <div class="app-shell" :class="{ 'is-home-layout': isHome, 'is-auth-layout': isAuthPage }">
    <div v-if="!isAuthPage" ref="headerSpaceRef" class="layout-header-space">
      <el-header class="layout-header-wrap">
        <div class="layout-header" ref="headerRef">
          <div ref="headerBrandRef" class="header-brand">
            <!-- Logo -->
            <div class="logo" @click="go('/')">{{ siteName }}</div>

            <div class="mobile-nav" v-if="authReady && isLogged">
              <el-button
                @click="drawer = true"
                :icon="MenuIcon"
                text
                circle
                class="mobile-menu-btn"
                aria-label="打开导航菜单"
              />
            </div>
          </div>

          <!-- Desktop Navigation -->
          <div
            ref="desktopNavRef"
            class="desktop-nav"
            :class="{ 'is-hidden': !showDesktopNav }"
            :aria-hidden="!showDesktopNav"
          >
            <el-menu
              mode="horizontal"
              :default-active="activeRoute"
              router
              :ellipsis="false"
              :default-openeds="defaultOpeneds"
            >
              <template v-for="(item, index) in navLinks" :key="item.path || item.index">
                <el-sub-menu
                  v-if="item.type === 'group'"
                  :index="item.index"
                  :trigger="item.trigger"
                  :class="['nav-menu-entry', 'nav-priority-' + (index + 1)]"
                >
                  <template #title>
                    <span>{{ item.title }}</span>
                  </template>
                  <el-menu-item
                    v-for="child in item.children"
                    :key="child.path"
                    :index="child.path"
                  >
                    <el-icon v-if="child.icon"><component :is="child.icon" /></el-icon>
                    <span>{{ child.title }}</span>
                    <span v-if="shouldShowNotificationBadge(child)" class="notification-nav-dot" />
                  </el-menu-item>
                </el-sub-menu>
                <el-menu-item
                  v-else
                  :index="item.path"
                  :class="['nav-menu-entry', 'nav-priority-' + (index + 1)]"
                >
                  <el-icon v-if="item.icon"><component :is="item.icon" /></el-icon>
                  <span>{{ item.title }}</span>
                  <span v-if="shouldShowNotificationBadge(item)" class="notification-nav-dot" />
                </el-menu-item>
              </template>
            </el-menu>
          </div>

          <div ref="headerActionsRef" class="header-actions" :aria-busy="!authReady">
            <template v-if="authReady">
              <el-button
                v-if="canAccessNotifications"
                class="notification-toggle"
                :class="{ 'has-unread': shouldShowNotificationBadge({ path: '/notifications' }) }"
                :icon="Bell"
                circle
                text
                aria-label="通知中心"
                @click="go('/notifications')"
              />

              <!-- Theme Toggle -->
              <el-button
                class="theme-toggle"
                :icon="isDark ? Sunny : Moon"
                circle
                text
                :aria-label="isDark ? '切换浅色主题' : '切换深色主题'"
                @click="toggleTheme"
              />

              <AccountMenu
                v-if="isLogged"
                :avatar-src="customAvatar || ''"
                :avatar-configured="avatarConfigured"
                :avatar-loading="isAvatarLoading"
                :account-name="accountName"
                :role-label="accountRoleLabel"
                :can-access-admin="canAccessAdmin"
                :links="accountLinks"
                @navigate="go"
                @logout="logout"
              />

              <!-- Auth Buttons -->
              <template v-else>
                <el-button type="primary" @click="go('/login')">登录</el-button>
                <el-button @click="go('/register')" class="hero-register-btn"> 注册 </el-button>
              </template>
            </template>
          </div>
        </div>
      </el-header>
    </div>

    <!-- Mobile Drawer -->
    <el-drawer v-model="drawer" title="导航菜单" direction="ltr" size="280px" class="mobile-drawer">
      <el-menu :default-active="activeRoute" router @select="drawer = false" class="drawer-menu">
        <template v-for="(item, index) in drawerLinks" :key="item.index || item.path || index">
          <el-divider v-if="item.isDivider" class="nav-divider" />
          <template v-else-if="item.type === 'group' && item.collapsible === false">
            <div class="drawer-section-title">{{ item.title }}</div>
            <el-menu-item v-for="child in item.children" :key="child.path" :index="child.path!">
              <el-icon v-if="child.icon"><component :is="child.icon" /></el-icon>
              <span>{{ child.title }}</span>
              <span v-if="shouldShowNotificationBadge(child)" class="notification-nav-dot" />
            </el-menu-item>
          </template>
          <el-sub-menu v-else-if="item.type === 'group'" :index="item.index || String(index)">
            <template #title>
              <el-icon v-if="item.icon"><component :is="item.icon" /></el-icon>
              <span>{{ item.title }}</span>
            </template>
            <el-menu-item v-for="child in item.children" :key="child.path" :index="child.path!">
              <el-icon v-if="child.icon"><component :is="child.icon" /></el-icon>
              <span>{{ child.title }}</span>
              <span v-if="shouldShowNotificationBadge(child)" class="notification-nav-dot" />
            </el-menu-item>
          </el-sub-menu>
          <el-menu-item v-else :index="item.path!">
            <el-icon v-if="item.icon"><component :is="item.icon" /></el-icon>
            <span>{{ item.title }}</span>
            <span v-if="shouldShowNotificationBadge(item)" class="notification-nav-dot" />
          </el-menu-item>
        </template>
      </el-menu>
    </el-drawer>

    <main class="app-main">
      <slot />
    </main>

    <AppFooter
      v-if="showFooter"
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
import { Bell, Menu as MenuIcon, Moon, Sunny } from '@element-plus/icons-vue'
import { onMounted, onUnmounted, ref, watch } from 'vue'
import AppFooter from '@/components/layout/AppFooter.vue'
import AccountMenu from '@/components/layout/AccountMenu.vue'
import { useAppLayoutState } from '@/components/layout/useAppLayoutState'

const {
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
  shouldShowNotificationBadge,
} = useAppLayoutState()

const headerRef = ref<HTMLElement | null>(null)
const headerSpaceRef = ref<HTMLElement | null>(null)
const headerBrandRef = ref<HTMLElement | null>(null)
const headerActionsRef = ref<HTMLElement | null>(null)
const desktopNavRef = ref<HTMLElement | null>(null)
const showDesktopNav = ref(false)
let navResizeObserver: ResizeObserver | null = null

function updateDesktopNavVisibility() {
  // Fixed 顶栏沿用流式占位的实际宽度，包含现有滚动槽策略；不计算滚动条宽度。
  const space = headerSpaceRef.value
  const width = space?.getBoundingClientRect().width
  if (space && width && width > 0) space.style.setProperty('--layout-header-width', `${width}px`)
  const header = headerRef.value
  const brand = headerBrandRef.value
  const actions = headerActionsRef.value
  const container = desktopNavRef.value
  const menu = container?.querySelector<HTMLElement>('.el-menu')
  if (!header || !brand || !actions || !container || !menu) return

  const headerRect = header.getBoundingClientRect()
  const brandRect = brand.getBoundingClientRect()
  const actionsRect = actions.getBoundingClientRect()
  const navWidth = menu.getBoundingClientRect().width
  const headerCenter = headerRect.left + headerRect.width / 2
  const navLeft = headerCenter - navWidth / 2
  const navRight = headerCenter + navWidth / 2
  const safeGap = 12

  showDesktopNav.value =
    window.innerWidth > 768 &&
    navLeft >= brandRect.right + safeGap &&
    navRight <= actionsRect.left - safeGap
}

onMounted(() => {
  const observer = new ResizeObserver(updateDesktopNavVisibility)
  navResizeObserver = observer
  watch(
    [headerRef, headerBrandRef, headerActionsRef, desktopNavRef, headerSpaceRef],
    (elements) => {
      observer.disconnect()
      for (const element of elements) {
        if (element) observer.observe(element)
      }
      updateDesktopNavVisibility()
    },
    { immediate: true, flush: 'post' },
  )
  window.addEventListener('resize', updateDesktopNavVisibility)
})

onUnmounted(() => {
  navResizeObserver?.disconnect()
  window.removeEventListener('resize', updateDesktopNavVisibility)
})
</script>

<style>
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
}
</style>

<style scoped>
.app-shell {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

/* Home Mode Shell：整页固定一屏，页头/内容/footer 在流内分布 */
.is-home-layout {
  height: 100vh;
  overflow: hidden;
  /* 与操作页视口保留同宽的滚动槽，同时允许 fixed 背景覆盖整个视口。 */
  scrollbar-gutter: stable;
}

.layout-header-wrap {
  position: fixed;
  top: 0;
  width: var(--layout-header-width, 100%);
  box-sizing: border-box;
  padding: 0 20px;
  background: var(--color-header-background);
  backdrop-filter: blur(8px);
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.08);
  border-bottom: 1px solid var(--color-border);
  height: var(--layout-header-height);
  z-index: 100;
  flex-shrink: 0;
}

.layout-header-space {
  height: var(--layout-header-height);
  flex-shrink: 0;
  min-width: 0;
}

.is-home-layout .layout-header-wrap {
  position: relative;
  width: 100%;
  background: transparent;
  border-bottom: none;
  box-shadow: none;
  backdrop-filter: none;
  z-index: 20;
}

/* Home Layout UI Enforcement - Scoped to .layout-header */
.is-home-layout .layout-header .logo,
.is-home-layout .layout-header :deep(.account-name),
.is-home-layout .layout-header .theme-toggle,
.is-home-layout .layout-header .notification-toggle,
.is-home-layout .layout-header .mobile-menu-btn,
.is-home-layout .layout-header :deep(.el-menu-item),
.is-home-layout .layout-header :deep(.el-sub-menu__title) {
  color: #fff !important;
}

.is-home-layout .layout-header :deep(.account-trigger:hover),
.is-home-layout .layout-header .logo:hover,
.is-home-layout .layout-header .theme-toggle:hover,
.is-home-layout .layout-header .notification-toggle:hover,
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
.drawer-section-title {
  padding: 16px 20px 6px;
  color: var(--color-text-light);
  font-size: 12px;
  font-weight: 600;
}

.layout-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  position: relative;
  height: 100%;
  gap: 12px;
}
.logo {
  font-weight: 700;
  font-size: 20px;
  color: var(--color-heading);
  cursor: pointer;
  border-radius: 8px;
  padding: 4px 8px;
  transition: background-color var(--motion-duration-fast);
  flex-shrink: 0;
}
.logo:hover {
  color: var(--el-color-primary);
}

.header-brand {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  gap: 4px;
}

.desktop-nav {
  display: flex;
  justify-content: center;
  position: absolute;
  left: 50%;
  transform: translateX(-50%);
  width: max-content;
  max-width: 100%;
  height: 100%;
  overflow: hidden;
}

.desktop-nav.is-hidden {
  visibility: hidden;
  pointer-events: none;
}

.mobile-nav {
  display: block;
}
.desktop-nav .el-menu {
  flex-shrink: 0;
  border-bottom: none;
  height: 100%;
  background: transparent;
  max-width: none;
  overflow: visible;
}

.desktop-nav :deep(.nav-menu-entry) {
  flex-shrink: 0;
}

.desktop-nav :deep(.el-sub-menu__title) {
  border-bottom: 2px solid transparent;
  transition:
    color var(--motion-duration-fast),
    border-color var(--motion-duration-fast);
}
.desktop-nav :deep(.el-sub-menu__title:hover) {
  color: var(--el-color-primary);
}
.notification-nav-dot,
.notification-toggle.has-unread::after {
  width: 8px;
  height: 8px;
  margin-left: 2px;
  border-radius: 999px;
  background: var(--el-color-danger);
  box-shadow: 0 0 0 2px var(--color-header-background);
  flex-shrink: 0;
}
.is-home-layout .notification-nav-dot,
.is-home-layout .notification-toggle.has-unread::after {
  box-shadow: 0 0 0 2px rgba(255, 255, 255, 0.28);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 32px;
  flex-shrink: 0;
}
.header-actions > .el-button {
  margin-left: 0;
  transition-property: color, background-color, border-color, box-shadow;
}
.theme-toggle {
  font-size: 20px;
  border-radius: 8px;
}
.notification-toggle {
  position: relative;
  font-size: 20px;
  border-radius: 8px;
}
.notification-toggle.has-unread::after {
  content: '';
  position: absolute;
  top: 5px;
  right: 5px;
  margin: 0;
  pointer-events: none;
}

.app-main {
  padding: var(--space-page-gutter);
  min-width: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  background-color: var(--color-background);
}
.is-home-layout .app-main {
  position: relative;
  z-index: 1;
  padding: 0;
  background: transparent;
  min-height: 0;
}
.is-auth-layout .app-main {
  padding: 0 !important;
}

.filing-icon {
  width: 13px;
}

@media (max-width: 1100px) {
  :deep(.account-name) {
    display: none;
  }
}

@media (max-width: 768px) {
  .desktop-nav {
    visibility: hidden;
    pointer-events: none;
  }
}
</style>
