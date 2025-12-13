<template>
  <el-container style="min-height:100vh">
    <el-header class="layout-header-wrap">
      <div class="layout-header">
        <div class="logo" @click="go('/')">{{ siteName }}</div>

        <div class="header-actions">
          <el-button v-if="!isLogged" type="primary" @click="go('/login')">登录</el-button>
          <el-button v-if="!isLogged" @click="go('/register')" style="margin-left:8px">注册</el-button>
          <el-dropdown v-if="isLogged">
            <span class="el-dropdown-link">
              <el-avatar size="small" style="margin-right:8px">{{ emailInitial }}</el-avatar>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click.native.prevent="go('/dashboard')">个人面板</el-dropdown-item>
                <el-dropdown-item v-if="isAdmin" @click.native.prevent="go('/admin')">管理面板</el-dropdown-item>
                <el-dropdown-item divided @click.native.prevent="logout">退出</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </div>
    </el-header>

    <el-main style="padding:0">
      <slot />
    </el-main>
  </el-container>
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
const emailInitial = computed(() => payload.value?.sub?.slice(0,1).toUpperCase() || '?')

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
  // 加载站点配置
  try {
    const res = await axios.get('/public/settings')
    if (res.data.site_name) {
      siteName.value = res.data.site_name
      localStorage.setItem('site_name_cache', res.data.site_name)
    }
  } catch (e) {
    console.warn('Failed to load site settings:', e)
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
  }
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
}
.header-actions{display:flex;align-items:center; gap:8px}
.app-container{max-width:960px;margin:24px auto}

.layout-header-wrap{
  padding: 12px 20px;
  background: #f8f9fb;
  box-shadow: 0 1px 4px rgba(0,0,0,0.06);
}
</style>
