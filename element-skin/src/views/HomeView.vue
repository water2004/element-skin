<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'

const router = useRouter()
const siteName = ref('皮肤站')
const isLogged = ref(false)

onMounted(async () => {
  // 加载站点配置
  try {
    const res = await axios.get('/public/settings')
    if (res.data.site_name) {
      siteName.value = res.data.site_name
    }
  } catch (e) {
    console.warn('Failed to load site settings:', e)
  }

  // 检查登录状态
  const jwt = localStorage.getItem('jwt')
  if (jwt) {
    try {
      await axios.get('/me', { headers: { Authorization: 'Bearer ' + jwt } })
      isLogged.value = true
    } catch (e) {
      localStorage.removeItem('jwt')
      localStorage.removeItem('accessToken')
    }
  }
})

function goDashboard() {
  router.push('/dashboard')
}

function goLogin() {
  router.push('/login')
}

function goRegister() {
  router.push('/register')
}
</script>

<template>
  <div class="home-container">
    <div class="hero-section">
      <div class="hero-content">
        <h1 class="hero-title">{{ siteName }}</h1>
        <p class="hero-subtitle">为您的 Minecraft 角色管理皮肤和披风</p>
        <div class="hero-actions">
          <el-button v-if="isLogged" type="primary" size="large" @click="goDashboard">
            <el-icon><User /></el-icon>
            <span style="margin-left:8px">进入个人面板</span>
          </el-button>
          <template v-else>
            <el-button type="primary" size="large" @click="goLogin">
              登录
            </el-button>
            <el-button size="large" @click="goRegister" style="margin-left:16px">
              注册账号
            </el-button>
          </template>
        </div>
      </div>
    </div>

    <div class="features-section">
      <div class="container">
        <h2 class="section-title">核心功能</h2>
        <div class="features-grid">
          <div class="feature-card">
            <div class="feature-icon" style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%)">
              <el-icon :size="36"><Picture /></el-icon>
            </div>
            <h3>皮肤与披风</h3>
            <p>上传、预览、切换皮肤与披风，内置 3D 预览</p>
          </div>
          <div class="feature-card">
            <div class="feature-icon" style="background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%)">
              <el-icon :size="36"><Files /></el-icon>
            </div>
            <h3>个人材质库</h3>
            <p>一次上传，多角色复用，集中管理你的材质文件</p>
          </div>
          <div class="feature-card">
            <div class="feature-icon" style="background: linear-gradient(135deg, #43e97b 0%, #38f9d7 100%)">
              <el-icon :size="36"><Connection /></el-icon>
            </div>
            <h3>Yggdrasil 兼容</h3>
            <p>遵循 Yggdrasil 规范，兼容主流启动器和客户端</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.home-container {
  width: 100%;
  min-height: calc(100vh - 60px);
}

.hero-section {
  width: 100%;
  min-height: 420px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  padding: 48px 20px;
  border-radius: 16px;
  margin: 16px auto 0;
  max-width: 1280px;
}

.hero-content {
  text-align: center;
  max-width: 800px;
}

.hero-title {
  font-size: 48px;
  font-weight: 700;
  margin: 0 0 16px 0;
  text-shadow: 0 4px 12px rgba(0, 0, 0, 0.18);
}

.hero-subtitle {
  font-size: 18px;
  margin: 0 0 32px 0;
  opacity: 0.95;
  font-weight: 400;
}

.hero-actions {
  display: flex;
  gap: 16px;
  justify-content: center;
  align-items: center;
}

.features-section {
  width: 100%;
  padding: 48px 20px 64px;
  background: #f5f7fa;
}

.container {
  max-width: 1200px;
  margin: 0 auto;
}

.section-title {
  font-size: 32px;
  font-weight: 600;
  text-align: center;
  margin: 0 0 32px 0;
  color: #303133;
}

.features-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 32px;
}

.feature-card {
  background: #fff;
  border-radius: 12px;
  padding: 24px 20px;
  text-align: center;
  transition: all 0.25s ease;
  box-shadow: 0 1px 8px rgba(0, 0, 0, 0.06);
}

.feature-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 6px 18px rgba(0, 0, 0, 0.12);
}

.feature-icon {
  width: 64px;
  height: 64px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 auto 16px;
  color: #fff;
}

.feature-card h3 {
  font-size: 20px;
  font-weight: 600;
  margin: 0 0 16px 0;
  color: #303133;
}

.feature-card p {
  font-size: 15px;
  color: #606266;
  line-height: 1.6;
  margin: 0;
}

@media (max-width: 768px) {
  .hero-title {
    font-size: 42px;
  }

  .hero-subtitle {
    font-size: 18px;
  }

  .hero-actions {
    flex-direction: column;
    width: 100%;
  }

  .hero-actions .el-button {
    width: 100%;
  }

  .section-title {
    font-size: 32px;
  }

  .features-grid {
    grid-template-columns: 1fr;
  }
}
</style>
