<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { User, Picture, Files, Connection } from '@element-plus/icons-vue'
import axios from 'axios'

const router = useRouter()
const siteName = ref('皮肤站')
const isLogged = ref(false)
const carouselImages = ref([])

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

  // 加载轮播图
  try {
    const res = await axios.get('/public/carousel')
    carouselImages.value = res.data
  } catch (e) {
    console.warn('Failed to load carousel images:', e)
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

function getCarouselUrl(filename) {
  const base = import.meta.env.VITE_API_BASE || ''
  return `${base}/static/carousel/${filename}`
}
</script>

<template>
  <div class="home-container">
    <div class="hero-wrapper">
      <!-- Background Carousel -->
      <div v-if="carouselImages.length > 0" class="hero-carousel-bg">
        <el-carousel height="100%" indicator-position="none" arrow="never" :interval="5000">
          <el-carousel-item v-for="img in carouselImages" :key="img">
            <div class="carousel-img-wrap">
              <img :src="getCarouselUrl(img)" class="carousel-img" />
              <div class="carousel-overlay"></div>
            </div>
          </el-carousel-item>
        </el-carousel>
      </div>
      <div v-else class="hero-gradient-bg"></div>

      <!-- Hero Content -->
      <div class="hero-section">
        <div class="hero-content">
          <h1 class="hero-title">{{ siteName }}</h1>
          <p class="hero-subtitle">简洁、高效、现代的 Minecraft 皮肤管理站</p>
          <div class="hero-actions">
            <el-button v-if="isLogged" type="primary" size="large" @click="goDashboard" class="hero-btn">
              <el-icon><User /></el-icon>
              <span>进入个人面板</span>
            </el-button>
            <template v-else>
              <el-button type="primary" size="large" @click="goLogin" class="hero-btn">
                登录账号
              </el-button>
              <el-button size="large" @click="goRegister" class="hero-btn secondary">
                即刻注册
              </el-button>
            </template>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.home-container {
  width: 100%;
  height: 100vh;
  display: flex;
  flex-direction: column;
}

.hero-wrapper {
  position: relative;
  width: 100%;
  flex: 1;
  overflow: hidden;
}

.hero-carousel-bg, .hero-gradient-bg, :deep(.el-carousel) {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  z-index: 1;
}

.hero-gradient-bg {
  background: var(--color-background-hero-light);
  transition: background 0.3s ease;
}

.carousel-img-wrap {
  width: 100%;
  height: 100%;
  position: relative;
}

.carousel-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.carousel-overlay {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(0, 0, 0, 0.4); /* Darken for text readability */
}

.hero-section {
  position: relative;
  z-index: 2;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  padding: 0 20px;
}

.hero-content {
  text-align: center;
  max-width: 800px;
  animation: fadeIn 0.8s ease-out;
}

.hero-title {
  font-size: 56px;
  font-weight: 800;
  margin: 0 0 16px 0;
  letter-spacing: -1px;
}

.hero-subtitle {
  font-size: 20px;
  margin: 0 0 32px 0;
  opacity: 0.9;
  font-weight: 300;
}

.hero-actions {
  display: flex;
  gap: 16px;
  justify-content: center;
}

.hero-btn {
  height: 48px;
  padding: 0 32px;
  font-size: 16px;
  font-weight: 600;
  border-radius: 12px;
  transition: all 0.3s;
}

.hero-btn.secondary {
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.3);
  color: #fff;
}

.hero-btn.secondary:hover {
  background: rgba(255, 255, 255, 0.2);
  border-color: #fff;
}

.features-section {
  padding: 80px 20px;
  background: transparent;
}

.container {
  max-width: 1100px;
  margin: 0 auto;
}

.features-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 40px;
}

.feature-card {
  text-align: center;
  padding: 20px;
}

.feature-icon-simple {
  font-size: 40px;
  margin-bottom: 20px;
}

.feature-card h3 {
  font-size: 22px;
  font-weight: 700;
  margin: 0 0 12px 0;
  color: #2c3e50;
}

.feature-card p {
  font-size: 16px;
  color: #7f8c8d;
  line-height: 1.6;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(20px); }
  to { opacity: 1; transform: translateY(0); }
}

@media (max-width: 768px) {
  .hero-wrapper {
    height: 400px;
    border-radius: 0;
    margin-top: 0;
  }
  .hero-title {
    font-size: 36px;
  }
  .features-grid {
    grid-template-columns: 1fr;
    gap: 20px;
  }
  .hero-actions {
    flex-direction: column;
    gap: 12px;
  }
  .hero-btn {
    width: 100%;
  }
}
</style>
