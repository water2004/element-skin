<template>
  <div class="dashboard-container">
    <el-container style="height:100%">
      <el-aside width="220px" class="dashboard-sidebar">
        <div class="user-info">
          <el-avatar :size="60" class="user-avatar bg-gradient-purple">{{ emailInitial }}</el-avatar>
          <div class="user-name">{{ user?.display_name || user?.email || '用户' }}</div>
          <div class="user-status">
            <el-tag v-if="user?.is_admin" type="danger" size="small">管理员</el-tag>
            <el-tag v-else-if="getUserBanStatus()" type="warning" size="small">封禁</el-tag>
            <el-tag v-else type="info" size="small">用户</el-tag>
          </div>
        </div>
        <el-menu :default-active="activeRoute" mode="vertical" router class="sidebar-menu">
          <el-menu-item index="/dashboard/wardrobe">
            <el-icon><Box /></el-icon>
            <span>我的衣柜</span>
          </el-menu-item>
          <el-menu-item index="/dashboard/roles">
            <el-icon><User /></el-icon>
            <span>角色管理</span>
          </el-menu-item>
          <el-menu-item index="/dashboard/profile">
            <el-icon><Setting /></el-icon>
            <span>个人资料</span>
          </el-menu-item>
          <div v-if="user?.is_admin" class="menu-divider"></div>
          <el-menu-item v-if="user?.is_admin" index="/admin" class="admin-menu-item">
            <el-icon><Tools /></el-icon>
            <span>管理面板</span>
          </el-menu-item>
        </el-menu>
      </el-aside>

      <el-main class="dashboard-main">
        <router-view v-slot="{ Component }">
          <component
            :is="Component"
            :user="user"
            :user-profiles="user?.profiles"
            @refresh="fetchMe"
          />
        </router-view>
      </el-main>
    </el-container>
  </div>
</template>

<script setup>
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import { ElMessage } from 'element-plus'
import { Box, User, Setting, Tools } from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()
const user = ref(null)

const emailInitial = computed(() => {
  const email = user.value?.email || user.value?.display_name || 'U'
  return email.charAt(0).toUpperCase()
})

function getUserBanStatus() {
  if (!user.value?.banned_until) return false
  return Date.now() < user.value.banned_until
}

const activeRoute = computed(() => route.path)

watch(() => route.path, () => {
  if (!user.value) {
    fetchMe()
  }
}, { immediate: true })

function authHeaders() {
  const token = localStorage.getItem('jwt')
  return token ? { Authorization: 'Bearer ' + token } : {}
}

async function fetchMe() {
  try {
    const res = await axios.get('/me', { headers: authHeaders() })
    user.value = res.data
  } catch (e) {
    console.error('fetchMe error:', e)
    if (e.response?.status === 401 || e.response?.status === 403) {
      ElMessage.error('登录已过期，请重新登录')
      localStorage.removeItem('jwt')
      localStorage.removeItem('accessToken')
      setTimeout(() => {
        router.push('/login')
      }, 1000)
    } else {
      ElMessage.error('获取用户信息失败')
    }
  }
}

onMounted(async () => {
  try {
    const res = await axios.post('/me/refresh-token', {}, { headers: authHeaders() })
    if (res.data.token) {
      localStorage.setItem('token', res.data.token)
    }
  } catch (e) {
    console.warn('Failed to refresh token:', e)
  }

  await fetchMe()
})
</script>

<style scoped>
.dashboard-container {
  min-height: 100vh;
  background: #f5f7fa;
}

.dashboard-container :deep(.el-container) {
  min-height: 100vh;
}

.dashboard-sidebar {
  background: #fff;
  border-right: 1px solid #e4e7ed;
  padding: 20px 0;
  min-height: 100vh;
}

.user-info {
  text-align: center;
  padding: 20px;
  margin-bottom: 20px;
}

.user-avatar {
  color: #fff;
  font-weight: bold;
  font-size: 24px;
  margin-bottom: 12px;
  transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
  cursor: pointer;
}

.user-avatar:hover {
  transform: scale(1.15) rotate(8deg);
  box-shadow: 0 8px 24px rgba(102, 126, 234, 0.3);
}

.user-name {
  font-size: 16px;
  font-weight: 500;
  color: #303133;
  margin-top: 12px;
}

.user-status {
  margin-top: 8px;
  display: flex;
  justify-content: center;
}

.sidebar-menu {
  border: none;
}

.sidebar-menu .el-menu-item {
  height: 50px;
  line-height: 50px;
  margin: 4px 12px;
  border-radius: 8px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  overflow: hidden;
}

.sidebar-menu .el-menu-item::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0;
  height: 100%;
  width: 3px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  transform: translateX(-100%);
  transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.sidebar-menu .el-menu-item:hover::before {
  transform: translateX(0);
}

.sidebar-menu .el-menu-item:hover {
  background-color: #ecf5ff;
  transform: translateX(4px);
}

.sidebar-menu .el-menu-item.is-active {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: #fff;
  transform: translateX(0);
}

.menu-divider {
  height: 1px;
  background: #ebeef5;
  margin: 8px 12px;
}

.sidebar-menu .admin-menu-item:hover {
  background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);
  color: #fff;
}

.dashboard-main {
  padding: 30px;
  background: #f5f7fa;
  min-height: 100vh;
}
</style>