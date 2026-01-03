<template>
  <div class="app-shell" :class="{ 'is-home-layout': isHome }">
    <el-header class="layout-header-wrap">
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
          <!-- Mobile Navigation Trigger -->
          <div class="mobile-nav" v-if="isLogged">
            <el-button @click="drawer = true" :icon="MenuIcon" text circle />
          </div>

          <!-- Account Popover -->
          <el-popover v-if="isLogged" placement="bottom-end" :width="240" trigger="hover" popper-class="account-popover" :show-arrow="false" :offset="4">
            <template #reference>
              <div class="account-trigger">
                <el-avatar size="small" class="account-avatar bg-gradient-purple">{{ avatarInitial }}</el-avatar>
                <span class="account-name">{{ accountName }}</span>
              </div>
            </template>
            <div class="account-panel">
              <div class="account-header">
                <el-avatar :size="48" class="account-avatar bg-gradient-purple">{{ avatarInitial }}</el-avatar>
                <div class="account-meta">
                  <h4>{{ accountName }}</h4>
                  <p>{{ isAdmin ? '管理员' : '普通用户' }}</p>
                </div>
              </div>
              <div class="account-actions">
                <el-button class="action-btn" @click="go('/dashboard')">
                  <span>个人面板</span>
                </el-button>
                <el-button v-if="isAdmin" class="action-btn" @click="go('/admin')">
                  <span>管理面板</span>
                </el-button>
                <el-button class="action-btn danger-btn" @click="logout">
                  <span>退出登录</span>
                </el-button>
              </div>
            </div>
          </el-popover>

          <!-- Login/Register Buttons -->
          <template v-if="!isLogged">
            <el-button type="primary" @click="go('/login')">登录</el-button>
            <el-button @click="go('/register')" style="margin-left:8px">注册</el-button>
          </template>
        </div>
      </div>
    </el-header>

    <!-- Mobile Drawer -->
    <el-drawer v-model="drawer" title="Navigation" direction="ltr" size="240px" class="mobile-drawer">
      <el-menu :default-active="activeRoute" router @select="drawer = false">
        <template v-for="(item, index) in drawerLinks" :key="index">
            <el-divider v-if="item.isDivider" class="nav-divider" />
            <el-menu-item v-else :index="item.path">
              <el-icon v-if="item.icon"><component :is="item.icon" /></el-icon>
              <span>{{ item.title }}</span>
            </el-menu-item>
        </template>
      </el-menu>
    </el-drawer>

    <main class="app-main">
      <slot />
    </main>
  </div>
</template>

<script setup>
import { computed, ref, onMounted, onUnmounted, provide } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import {
  Menu as MenuIcon, Box, User, Setting, Tools, Back, Odometer, Link, Picture
} from '@element-plus/icons-vue'

const route = useRoute()
const { push } = useRouter()
const isHome = computed(() => route.path === '/')
const siteName = ref(localStorage.getItem('site_name_cache') || '皮肤站')
const jwtToken = ref(localStorage.getItem('jwt') || '')
const user = ref(null)
const drawer = ref(false)

// Provide user and fetch function to all children
provide('user', user)
provide('fetchMe', fetchMe)

// --- Navigation Links ---
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
  { path: '/admin/mojang', title: 'Mojang API', icon: Link },
  { path: '/admin/carousel', title: '首页图片', icon: Picture },
]

const navLinks = computed(() => {
  if (route.path.startsWith('/admin')) {
    return adminNavLinks
  }
  if (isLogged.value) {
    const links = [...dashboardLinks]
    if (isAdmin.value) {
      links.push({ path: '/admin', title: '管理面板', icon: Tools })
    }
    return links
  }
  return []
})

const drawerLinks = computed(() => {
  if (!isLogged.value) return []
  const links = [...dashboardLinks]
  if (isAdmin.value) {
    links.push({ isDivider: true })
    links.push(...adminNavLinks)
  }
  return links
})

const activeRoute = computed(() => route.path)


// --- Authentication and User State ---
function parseJwt(token) {
  if (!token) return null
  try {
    const payload = token.split('.')[1]
    const json = decodeURIComponent(atob(payload.replace(/-/g, '+').replace(/_/g, '/')).split('').map(c => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2)).join(''))
    return JSON.parse(json)
  } catch (e) { return null }
}

const isLogged = computed(() => !!jwtToken.value)
const payload = computed(() => parseJwt(jwtToken.value))
const isAdmin = computed(() => user.value?.is_admin || false)
const accountName = computed(() => user.value?.display_name || user.value?.email || '用户')
const avatarInitial = computed(() => (accountName.value || 'U').slice(0, 1).toUpperCase())

let authTimer = null

function go(path) {
  push(path)
  drawer.value = false
}

function logout() {
  localStorage.removeItem('jwt')
  localStorage.removeItem('accessToken')
  jwtToken.value = ''
  user.value = null
  push('/')
  setTimeout(() => window.location.reload(), 100)
}

function authHeaders() {
  const token = localStorage.getItem('jwt')
  return token ? { Authorization: 'Bearer ' + token } : {}
}

async function fetchMe() {
  if (!isLogged.value) {
    user.value = null
    return
  }
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
  if (newToken !== jwtToken.value) {
    jwtToken.value = newToken
    fetchMe()
  }
}

onMounted(async () => {
  // Fetch site settings
  try {
    const res = await axios.get('/public/settings')
    if (res.data.site_name) {
      siteName.value = res.data.site_name
      localStorage.setItem('site_name_cache', res.data.site_name)
      document.title = res.data.site_name
    }
  } catch (e) {
    console.warn('Failed to load site settings:', e)
  }

  // Fetch user data
  await fetchMe()

  // Listen for auth changes
  window.addEventListener('storage', checkAuth)
  authTimer = setInterval(checkAuth, 1000)
})

onUnmounted(() => {
  if (authTimer) clearInterval(authTimer)
  window.removeEventListener('storage', checkAuth)
})
</script>

<style scoped>
.layout-header-wrap {
  padding: 0 20px;
  background: #fff;
  box-shadow: 0 1px 4px rgba(0,0,0,0.08);
  border-bottom: 1px solid #dcdfe6;
  height: 64px;
  z-index: 100;
  transition: all 0.3s;
}

.is-home-layout .layout-header-wrap {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  background: transparent;
  border-bottom: none;
  box-shadow: none;
}

.is-home-layout .logo, 
.is-home-layout :deep(.el-menu-item),
.is-home-layout .account-name,
.is-home-layout .mobile-nav :deep(.el-button) {
  color: #fff !important;
}

.is-home-layout :deep(.el-menu) {
  background: transparent !important;
  border-bottom: none !important;
}

.is-home-layout :deep(.el-menu-item:hover),
.is-home-layout :deep(.el-menu-item.is-active) {
  background-color: rgba(255, 255, 255, 0.15) !important;
  color: #fff !important;
}

.is-home-layout .account-trigger:hover,
.is-home-layout .mobile-nav :deep(.el-button:hover) {
  background: rgba(255, 255, 255, 0.15) !important;
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
  user-select: none;
  transition: color 0.3s ease;
}
.logo:hover {
  color: #409eff;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.app-main {
  padding: 20px;
  flex: 1;
  overflow: auto;
}

.is-home-layout .app-main {
  padding: 0;
}

/* --- Desktop Navigation --- */
.desktop-nav {
  flex-grow: 1;
  display: flex;
  justify-content: center;
  height: 100%;
}
.desktop-nav .el-menu {
  border-bottom: none;
  height: 100%;
}
.desktop-nav .el-menu-item {
  font-size: 15px;
  height: 100%;
}

/* --- Mobile Navigation --- */
.mobile-nav {
  display: none;
}

/* --- Account Popover --- */
.account-trigger { display:flex; align-items:center; cursor:pointer; gap:8px; padding:6px 12px; border-radius:20px; transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1) }
.account-trigger:hover { background: #f0f2f5; }
.account-name { font-size:14px; color:#303133; font-weight:500 }
.account-popover { padding: 0 !important }

.account-panel {
  display:flex;
  flex-direction:column;
  padding: 20px;
  box-sizing: border-box;
  width: 100%;
}
.account-header {
  display:flex;
  align-items:center;
  gap:12px;
  margin-bottom:16px;
  padding-bottom: 16px;
  border-bottom: 1px solid #f0f0f0;
}
.account-avatar {
  color:#fff;
  font-weight:600;
  font-size: 18px;
}
.account-meta { flex: 1; min-width: 0 }
.account-meta h4 {
  margin:0;
  font-size:14px;
  font-weight:600;
  color:#303133;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.account-meta p {
  margin:4px 0 0;
  font-size:12px;
  color:#909399;
}
.account-actions {
  display:flex;
  flex-direction:column;
  gap:8px;
  width: 100%;
}
.action-btn {
  width: 100% !important;
  height: 38px;
  border: 1px solid #e5e7eb;
  background: #fff;
  color: #606266;
  border-radius: 8px;
  font-size: 14px;
  font-weight: 500;
  transition: all 0.2s ease;
  justify-content: center;
  margin: 0 !important;
}
.action-btn:hover {
  background: #f7f8fa;
  border-color: #409eff;
  color: #409eff;
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(64, 158, 255, 0.2);
}
.action-btn.danger-btn:hover {
  background: #fef0f0;
  border-color: #f56c6c;
  color: #f56c6c;
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(245, 108, 108, 0.2);
}

/* --- Responsive Breakpoint --- */
@media (max-width: 1200px) {
  .nav-priority-6 { display: none !important; }
  .mobile-nav { display: block; }
}
@media (max-width: 1100px) {
  .nav-priority-5 { display: none !important; }
}
@media (max-width: 1000px) {
  .nav-priority-4 { display: none !important; }
}
@media (max-width: 900px) {
  .nav-priority-3 { display: none !important; }
}

@media (max-width: 768px) {
  .desktop-nav {
    display: none;
  }
  .layout-header {
    justify-content: space-between;
  }
}
</style>