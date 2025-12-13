<template>
  <div class="app-shell">
    <el-header class="layout-header-wrap">
      <div class="layout-header">
        <div class="logo" @click="go('/')">{{ siteName }}</div>
        <div class="header-actions">
          <el-button v-if="!isLogged" type="primary" @click="go('/login')">登录</el-button>
          <el-button v-if="!isLogged" @click="go('/register')" style="margin-left:8px">注册</el-button>
          <el-popover v-if="isLogged" placement="bottom-end" :width="240" trigger="hover" popper-class="account-popover" :show-arrow="false" :offset="4">
            <template #reference>
              <div class="account-trigger">
                <el-avatar size="small" class="account-avatar">{{ avatarInitial }}</el-avatar>
                <span class="account-name">{{ accountName }}</span>
              </div>
            </template>
            <div class="account-panel">
              <div class="account-header">
                <el-avatar :size="48" class="account-avatar">{{ avatarInitial }}</el-avatar>
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
        </div>
      </div>
    </el-header>
    <main class="app-main">
      <slot />
    </main>
  </div>
</template>

<script setup>
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'

const router = useRouter()
const cachedSiteName = localStorage.getItem('site_name_cache') || '皮肤站'
const siteName = ref(cachedSiteName)

const jwtToken = ref(localStorage.getItem('jwt') || '')

function parseJwt(token) {
  if (!token) return null
  try {
    const payload = token.split('.')[1]
    const json = decodeURIComponent(atob(payload.replace(/-/g, '+').replace(/_/g, '/')).split('').map(function(c) {
      return '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2)
    }).join(''))
    return JSON.parse(json)
  } catch (e) {
    return null
  }
}

const isLogged = computed(() => !!jwtToken.value)
const payload = computed(() => parseJwt(jwtToken.value))
const isAdmin = computed(() => !!payload.value && !!payload.value.is_admin)
const accountName = ref('用户')
const avatarInitial = computed(() => (accountName.value || 'U').slice(0,1).toUpperCase())

let timer = null

function go(path){
  router.push(path)
}

function logout(){
  localStorage.removeItem('jwt')
  localStorage.removeItem('accessToken')
  jwtToken.value = ''
  router.push('/')
  setTimeout(() => window.location.reload(), 100)
}

// 监听 localStorage 变化
onMounted(async () => {
  try {
    const res = await axios.get('/public/settings')
    if (res.data.site_name) {
      siteName.value = res.data.site_name
      localStorage.setItem('site_name_cache', res.data.site_name)
    }
  } catch (e) {
    console.warn('Failed to load site settings:', e)
  }

  // 如果已登录，获取用户信息以显示昵称/邮箱（优先显示 display_name）
  if (isLogged.value) {
    try {
      const me = await axios.get('/me', { headers: authHeaders() })
      // 优先使用显示名，其次邮箱，最后使用 token 中的 sub
      const name = me.data?.display_name || me.data?.email || payload.value?.sub || '用户'
      accountName.value = name
    } catch (e) {
      accountName.value = payload.value?.sub || '用户'
    }
  }

  window.addEventListener('storage', checkAuth)
  // 定期检查 token 更新
  timer = setInterval(checkAuth, 500)
})

onBeforeUnmount(() => {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
  window.removeEventListener('storage', checkAuth)
})

function checkAuth() {
  const newToken = localStorage.getItem('jwt') || ''
  if (newToken !== jwtToken.value) {
    jwtToken.value = newToken
    // 刷新账户显示名
    if (isLogged.value) {
      axios.get('/me', { headers: authHeaders() }).then(me => {
        const name = me.data?.display_name || me.data?.email || payload.value?.sub || '用户'
        accountName.value = name
      }).catch(() => {
        accountName.value = payload.value?.sub || '用户'
      })
    } else {
      accountName.value = '用户'
    }
  }
}

function authHeaders() {
  const token = localStorage.getItem('jwt')
  return token ? { Authorization: 'Bearer ' + token } : {}
}
</script>

<style scoped>
.layout-header{
  display:flex;
  align-items:center;
  justify-content:space-between;
  gap: 12px;
}
.logo{
  font-weight:700;
  font-size:18px;
  color:var(--color-heading);
  cursor: pointer;
  user-select: none;
  transition: color 0.3s ease;
}
.logo:hover {
  color: #409eff;
}
.header-actions{display:flex;align-items:center; gap:8px}
.app-container{max-width:960px;margin:24px auto}

.layout-header-wrap{
  padding: 12px 20px;
  background: #f8f9fb;
  box-shadow: 0 1px 4px rgba(0,0,0,0.06);
}
.layout-header-wrap{
  padding: 12px 20px;
  background: #f8f9fb;
  box-shadow: 0 1px 4px rgba(0,0,0,0.06);
}

.app-shell {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

.app-container-flex {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.app-main {
  padding: 0;
  flex: 1;
  overflow: auto;
}

.account-trigger { display:flex; align-items:center; cursor:pointer; gap:8px; padding:6px 12px; border-radius:20px; transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1) }
.account-trigger:hover { background: #f0f2f5; transform: scale(1.02) }
.account-name { font-size:13px; color:#606266; font-weight:500 }

.account-popover { padding: 0 !important }
.account-panel {
  display:flex;
  flex-direction:column;
  padding: 20px;
  animation: fadeIn 0.2s ease-out;
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
  width: 100%;
  box-sizing: border-box;
}

.account-avatar {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
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
  box-sizing: border-box;
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
  padding: 0 !important;
  display: flex !important;
  align-items: center;
  justify-content: center;
  min-width: auto !important;
  margin: 0 !important;
  text-align: center !important;
}

.action-btn span {
  display: block;
  text-align: center;
  width: 100%;
  flex: 1;
}

/* 强制覆盖 Element Plus 按钮内部样式 */
:deep(.action-btn .el-button__text-wrapper) {
  display: flex;
  justify-content: center;
  align-items: center;
  width: 100%;
}

:deep(.action-btn span) {
  text-align: center !important;
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

/* Popover 样式 */
:deep(.account-popover) {
  box-shadow: 0 4px 16px rgba(0,0,0,0.12);
  border-radius: 12px;
  padding: 0 !important;
  border: 1px solid #e5e7eb;
  overflow: hidden;
}
:deep(.el-popper__arrow) { display:none }

@keyframes fadeIn {
  from { opacity:0; transform: translateY(6px) }
  to { opacity:1; transform: translateY(0) }
}

.el-dropdown-link {
  cursor: pointer;
  display: flex;
  align-items: center;
  transition: all 0.3s ease;
}

.el-dropdown-link:hover {
  opacity: 0.8;
}

:deep(.el-dropdown-menu__item) {
  padding: 10px 20px;
  font-size: 14px;
  transition: all 0.3s ease;
}

:deep(.el-dropdown-menu__item:hover) {
  background-color: #ecf5ff;
  color: #409eff;
}

:deep(.el-dropdown-menu__item--divided) {
  margin-top: 6px;
  border-top: 1px solid #ebeef5;
}

:deep(.el-dropdown-menu__item--divided:before) {
  display: none;
}
</style>
