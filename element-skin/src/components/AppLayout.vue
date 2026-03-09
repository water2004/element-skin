<template>
  <div class="app-shell" :class="{ 'is-home-layout': isHome, 'is-auth-layout': isAuthPage }">
    <el-header class="layout-header-wrap" v-if="!isAuthPage">
      <div class="layout-header">
        <!-- Logo -->
        <div class="logo" @click="go('/')">{{ siteName }}</div>

        <!-- Desktop Navigation -->
        <div class="desktop-nav">
          <el-menu mode="horizontal" :default-active="activeRoute" router :ellipsis="false">
            <template v-for="(item, index) in navLinks" :key="item.path">
              <el-menu-item 
                :index="item.path" 
                v-if="!item.adminOnly || isAdmin"
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
          <div class="mobile-nav" v-if="isLogged">
            <el-button @click="drawer = true" :icon="MenuIcon" text circle class="mobile-menu-btn" />
          </div>

          <!-- Account Popover -->
          <el-popover v-if="isLogged" placement="bottom-end" :width="240" trigger="hover" popper-class="account-popover" :show-arrow="false" :offset="4">
            <template #reference>
              <div class="account-trigger">
                <el-avatar size="small" class="account-avatar bg-gradient-purple">{{ avatarInitial }}</el-avatar>
                <span class="account-name">{{ accountName }}</span>
              </div>
            </template>
            <div class="account-panel surface-card">
              <div class="account-header">
                <el-avatar :size="48" class="account-avatar bg-gradient-purple">{{ avatarInitial }}</el-avatar>
                <div class="account-meta">
                  <h4>{{ accountName }}</h4>
                  <p>{{ isAdmin ? '管理员' : '普通用户' }}</p>
                </div>
              </div>
              <div class="account-actions">
                <el-button class="btn-outline" @click="go('/dashboard')">
                  <span>个人面板</span>
                </el-button>
                <el-button v-if="isAdmin" class="btn-outline" @click="go('/admin')">
                  <span>管理面板</span>
                </el-button>
                <el-button class="btn-outline btn-outline-danger" @click="logout">
                  <span>退出登录</span>
                </el-button>
              </div>
            </div>
          </el-popover>

          <!-- Auth Buttons -->
          <template v-if="!isLogged">
            <el-button type="primary" @click="go('/login')">登录</el-button>
            <el-button @click="go('/register')" style="margin-left:8px" class="hero-register-btn">
              注册
            </el-button>
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

    <main class="app-main" :style="{ '--footer-height': footerHeight + 'px' }">
      <slot />
    </main>

    <!-- Unified Footer -->
    <footer 
      v-if="showFooter" 
      ref="footerRef" 
      class="footer-container" 
      :class="isHome ? 'footer-home' : 'footer-standard'"
    >
      <div class="footer-content">
        <div class="footer-row">
          <span v-if="footerText" class="footer-text-item">{{ footerText }}</span>
          
          <template v-if="filingIcp">
            <span class="footer-separator">|</span>
            <a :href="filingIcpLink || '#'" target="_blank" class="footer-link-item">{{ filingIcp }}</a>
          </template>

          <template v-if="filingMps">
            <span class="footer-separator">|</span>
            <a :href="filingMpsLink || '#'" target="_blank" class="footer-link-item">
              <img src="https://www.beian.gov.cn/img/ghs.png" style="width:13px; margin-right:4px;" />
              {{ filingMps }}
            </a>
          </template>
        </div>
        <div class="footer-credits">
          Powered by <a :href="repoUrl" target="_blank" class="footer-link-item">{{ repoLabel }}</a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup>
import { computed, ref, onMounted, onUnmounted, provide, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import {
  Menu as MenuIcon, Box, User, Setting, Tools, Back, Odometer, Link, Picture, Message, Moon, Sunny
} from '@element-plus/icons-vue'

const route = useRoute()
const { push } = useRouter()
const isHome = computed(() => route.path === '/')
const isAuthPage = computed(() => ['/login', '/register', '/reset-password'].includes(route.path))
const siteName = ref(localStorage.getItem('site_name_cache') || '皮肤站')
const enableSkinLibrary = ref(localStorage.getItem('enable_skin_library_cache') === 'true' || localStorage.getItem('enable_skin_library_cache') === null)
const jwtToken = ref(localStorage.getItem('jwt') || '')
const user = ref(null)
const drawer = ref(false)
const footerText = ref('')
const filingIcp = ref('')
const filingIcpLink = ref('')
const filingMps = ref('')
const filingMpsLink = ref('')
const footerHeight = ref(0)
const footerRef = ref(null)

const updateFooterHeight = () => {
  nextTick(() => {
    if (footerRef.value) footerHeight.value = footerRef.value.offsetHeight
    else footerHeight.value = 0
  })
}

watch([() => route.path, footerText, filingIcp, filingMps], updateFooterHeight)

const isDark = ref(false)
function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (savedTheme) isDark.value = savedTheme === 'dark'
  else isDark.value = window.matchMedia('(prefers-color-scheme: dark)').matches
  applyTheme()
}
function toggleTheme() {
  isDark.value = !isDark.value
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
  applyTheme()
}
function applyTheme() {
  document.documentElement.classList.toggle('dark', isDark.value)
}

window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
  if (!localStorage.getItem('theme')) {
    isDark.value = e.matches
    applyTheme()
  }
})

provide('user', user)
provide('fetchMe', fetchMe)
provide('isDark', isDark)
provide('footerHeight', footerHeight)

const dashboardLinks = [
  { path: '/dashboard/home', title: '仪表盘', icon: Odometer },
  { path: '/dashboard/wardrobe', title: '我的衣柜', icon: Box },
  { path: '/dashboard/roles', title: '角色管理', icon: User },
  { path: '/dashboard/profile', title: '个人资料', icon: Setting },
]
const adminNavLinks = [
  { path: '/dashboard', title: '返回面板', icon: Back },
  { path: '/admin/users', title: '用户管理', icon: User },
  { path: '/admin/invites', title: '邀请码管理', icon: Tools },
  { path: '/admin/settings', title: '站点设置', icon: Setting },
  { path: '/admin/email', title: '邮件服务', icon: Message },
  { path: '/admin/mojang', title: 'Fallback 服务', icon: Link },
  { path: '/admin/carousel', title: '首页图片', icon: Picture },
]

const navLinks = computed(() => {
  if (route.path.startsWith('/admin')) return adminNavLinks
  const links = []
  if (isLogged.value) {
    if (enableSkinLibrary.value) links.push({ path: '/skin-library', title: '皮肤库', icon: Picture })
    links.push(...dashboardLinks)
    if (isAdmin.value) links.push({ path: '/admin', title: '管理面板', icon: Tools })
  }
  return links
})

const drawerLinks = computed(() => {
  const links = []
  if (isLogged.value) {
    if (enableSkinLibrary.value) links.push({ path: '/skin-library', title: '皮肤库', icon: Picture })
    links.push({ isDivider: true })
    links.push(...dashboardLinks)
    if (isAdmin.value) { links.push({ isDivider: true }); links.push(...adminNavLinks); }
  }
  return links
})

const activeRoute = computed(() => route.path)
const showFooter = computed(() => !isAuthPage.value)
const repoUrl = 'https://github.com/water2004/element-skin'
// REPAIRED: Correct version number display
const repoLabel = `Element Skin ${typeof __APP_VERSION__ !== 'undefined' ? __APP_VERSION__ : 'v1.3.0'}`

function parseJwt(token) {
  if (!token) return null
  try {
    const payload = token.split('.')[1]
    const json = decodeURIComponent(atob(payload.replace(/-/g, '+').replace(/_/g, '/')).split('').map(c => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2)).join(''))
    return JSON.parse(json)
  } catch (e) { return null }
}

const isLogged = computed(() => !!jwtToken.value)
const isAdmin = computed(() => user.value?.is_admin || false)
const accountName = computed(() => user.value?.display_name || user.value?.email || '用户')
const avatarInitial = computed(() => (accountName.value || 'U').slice(0, 1).toUpperCase())

let authTimer = null
let resizeObserver = null

function go(path) { push(path); drawer.value = false; }
function logout() {
  localStorage.removeItem('jwt'); localStorage.removeItem('accessToken');
  jwtToken.value = ''; user.value = null; push('/');
  setTimeout(() => window.location.reload(), 100)
}

function authHeaders() {
  const token = localStorage.getItem('jwt')
  return token ? { Authorization: 'Bearer ' + token } : {}
}

async function fetchMe() {
  if (!isLogged.value) { user.value = null; return; }
  try {
    const res = await axios.get('/me', { headers: authHeaders() })
    user.value = res.data
  } catch (e) {
    user.value = null
    console.error('Failed to fetch user data in AppLayout:', e)
  }
}

function checkAuth() {
  const newToken = localStorage.getItem('jwt') || ''
  if (newToken !== jwtToken.value) { jwtToken.value = newToken; fetchMe(); }
}

onMounted(async () => {
  initTheme()
  try {
    const res = await axios.get('/public/settings')
    if (res.data.site_name) {
      siteName.value = res.data.site_name
      localStorage.setItem('site_name_cache', res.data.site_name); document.title = res.data.site_name;
    }
    if (res.data.enable_skin_library !== undefined) {
      enableSkinLibrary.value = res.data.enable_skin_library
      localStorage.setItem('enable_skin_library_cache', res.data.enable_skin_library.toString())
    }
    if (res.data.footer_text !== undefined) footerText.value = res.data.footer_text
    if (res.data.filing_icp !== undefined) filingIcp.value = res.data.filing_icp
    if (res.data.filing_icp_link !== undefined) filingIcpLink.value = res.data.filing_icp_link
    if (res.data.filing_mps !== undefined) filingMps.value = res.data.filing_mps
    if (res.data.filing_mps_link !== undefined) filingMpsLink.value = res.data.filing_mps_link
    updateFooterHeight()
  } catch (e) { console.warn('Failed to load site settings:', e) }

  await fetchMe()
  window.addEventListener('storage', checkAuth)
  authTimer = setInterval(checkAuth, 1000)

  if (window.ResizeObserver) {
    resizeObserver = new ResizeObserver(() => updateFooterHeight())
    nextTick(() => { if (footerRef.value) resizeObserver.observe(footerRef.value) })
  }
  window.addEventListener('resize', updateFooterHeight)
})

onUnmounted(() => {
  if (authTimer) clearInterval(authTimer)
  window.removeEventListener('storage', checkAuth)
  window.removeEventListener('resize', updateFooterHeight)
  if (resizeObserver) resizeObserver.disconnect()
})
</script>

<style scoped>
@import "@/assets/styles/animations.css";
@import "@/assets/styles/layout.css";
@import "@/assets/styles/buttons.css";
@import "@/assets/styles/cards.css";
@import "@/assets/styles/footers.css";

.app-shell { min-height: 100vh; display: flex; flex-direction: column; overflow-x: hidden; }

/* Home Mode Shell - Strict首屏，防止滚动 */
.is-home-layout { height: 100vh; overflow: hidden; }

.layout-header-wrap {
  padding: 0 20px; background: var(--color-header-background); backdrop-filter: blur(8px);
  box-shadow: 0 1px 4px rgba(0,0,0,0.08); border-bottom: 1px solid var(--color-border);
  height: 64px; z-index: 100; flex-shrink: 0;
}

.is-home-layout .layout-header-wrap {
  position: absolute; top: 0; left: 0; right: 0; background: transparent; border-bottom: none; box-shadow: none; backdrop-filter: none;
}

/* Home Layout UI Enforcement - Scoped to .layout-header */
.is-home-layout .layout-header .logo,
.is-home-layout .layout-header .account-name,
.is-home-layout .layout-header .theme-toggle,
.is-home-layout .layout-header .mobile-menu-btn,
.is-home-layout .layout-header :deep(.el-menu-item) {
  color: #fff !important;
}

.is-home-layout .layout-header .account-trigger:hover,
.is-home-layout .layout-header .logo:hover,
.is-home-layout .layout-header .theme-toggle:hover,
.is-home-layout .layout-header .mobile-menu-btn:hover,
.is-home-layout .layout-header :deep(.el-menu-item:hover),
.is-home-layout .layout-header :deep(.el-menu-item.is-active) {
  background-color: rgba(255, 255, 255, 0.15) !important;
  color: #fff !important;
}

.is-home-layout .header-actions :deep(.el-button--primary) {
  background: rgba(64, 158, 255, 0.3) !important; border: 1px solid rgba(64, 158, 255, 0.4) !important;
  color: #fff !important; backdrop-filter: blur(8px); -webkit-backdrop-filter: blur(8px);
  border-radius: 8px;
}
.is-home-layout .hero-register-btn {
  background: rgba(255, 255, 255, 0.15) !important; border: 1px solid rgba(255, 255, 255, 0.25) !important;
  color: #fff !important; backdrop-filter: blur(8px); -webkit-backdrop-filter: blur(8px);
  border-radius: 8px; height: 32px; padding: 0 15px; font-size: 14px;
}

/* Mobile Drawer reset - Respect Global Theme */
.mobile-drawer :deep(.el-menu) { border-right: none; background: transparent; }
.mobile-drawer :deep(.el-menu-item) { color: var(--color-text); border-radius: 8px; margin: 4px 8px; height: 44px; line-height: 44px; }
.mobile-drawer :deep(.el-menu-item.is-active) { background-color: rgba(64, 158, 255, 0.1); color: var(--el-color-primary); font-weight: 600; }

.layout-header { display: flex; align-items: center; justify-content: space-between; height: 100%; }
.logo { font-weight: 700; font-size: 20px; color: var(--color-heading); cursor: pointer; border-radius: 8px; padding: 4px 8px; transition: background-color 0.2s; }
.logo:hover { color: var(--el-color-primary); }

.desktop-nav { flex-grow: 1; display: flex; justify-content: center; height: 100%; }
.desktop-nav .el-menu { border-bottom: none; height: 100%; background: transparent; }

.header-actions { display: flex; align-items: center; gap: 8px; }
.theme-toggle { font-size: 20px; border-radius: 8px; }

.app-main { padding: 20px; flex: 1; display: flex; flex-direction: column; background-color: var(--color-background); transition: padding 0.3s ease; }
.is-home-layout .app-main { padding: 0; flex: 1; height: 0; min-height: 0; }
.is-auth-layout .app-main { padding: 0 !important; }

/* Account */
.account-trigger { display:flex; align-items:center; cursor:pointer; gap:8px; padding:6px 12px; border-radius:20px; transition: background-color 0.2s; }
.account-trigger:hover { background: var(--color-background-soft); }
.account-name { font-size:14px; color: var(--color-text); font-weight:500; }
.account-popover { padding: 0 !important; background: var(--color-popover-background) !important; border: 1px solid var(--color-border) !important; }

.account-panel { padding: 20px; width: 100%; border: none !important; }
.account-header { display:flex; align-items:center; gap:12px; margin-bottom:16px; padding-bottom: 16px; border-bottom: 1px solid var(--color-border); }
.account-meta h4 { margin:0; font-size:14px; font-weight:600; color: var(--color-heading); }
.account-meta p { margin:4px 0 0; font-size:12px; color: var(--color-text-light); }
.account-actions { display:flex; flex-direction:column; gap:8px; width: 100%; }

@media (max-width: 1200px) { .nav-priority-6 { display: none !important; } .mobile-nav { display: block; } }
@media (max-width: 768px) { .desktop-nav { display: none; } }
</style>
