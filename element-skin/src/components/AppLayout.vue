<template>
  <el-container style="min-height:100vh">
    <el-header>
      <div class="layout-header">
        <div class="logo">皮肤站</div>
        <el-menu mode="horizontal" :default-active="active">
          <el-menu-item index="/">
            <router-link to="/">首页</router-link>
          </el-menu-item>
          <el-menu-item index="/about">
            <router-link to="/about">关于</router-link>
          </el-menu-item>
          <el-menu-item v-if="isLogged" index="/dashboard">
            <router-link to="/dashboard">个人面板</router-link>
          </el-menu-item>
          <el-menu-item v-if="isAdmin" index="/admin">
            <router-link to="/admin">管理面板</router-link>
          </el-menu-item>
        </el-menu>

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

    <el-main>
      <div class="app-container">
        <slot />
      </div>
    </el-main>
  </el-container>
</template>

<script setup>
import { computed } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()

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

const token = localStorage.getItem('jwt')
const payload = parseJwt(token)
const isLogged = computed(() => !!token)
const isAdmin = computed(() => !!payload && !!payload.is_admin)
const emailInitial = computed(() => payload?.sub?.slice(0,1).toUpperCase() || '?')

function go(path){
  router.push(path)
}

function logout(){
  localStorage.removeItem('jwt')
  localStorage.removeItem('accessToken')
  router.push('/')
  window.location.reload()
}

const active = router.currentRoute.value.path || '/'
</script>

<style scoped>
.layout-header{
  display:flex;
  align-items:center;
  justify-content:space-between;
}
.logo{
  font-weight:700;
  font-size:18px;
  color:var(--color-heading);
}
.header-actions{display:flex;align-items:center}
.app-container{max-width:960px;margin:24px auto}
</style>
