<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { User } from '@element-plus/icons-vue'
import axios from 'axios'

const router = useRouter()
const siteName = ref(localStorage.getItem('site_name_cache') || '皮肤站')
const siteSubtitle = ref(localStorage.getItem('site_subtitle_cache') || '简洁、高效、现代的 Minecraft 皮肤 management 站')
const isLogged = ref(false)
const carouselImages = ref([])

onMounted(async () => {
  // 加载站点配置
  try {
    const res = await axios.get('/public/settings')
    if (res.data.site_name) {
      siteName.value = res.data.site_name
      localStorage.setItem('site_name_cache', res.data.site_name)
    }
    if (res.data.site_subtitle) {
      siteSubtitle.value = res.data.site_subtitle
      localStorage.setItem('site_subtitle_cache', res.data.site_subtitle)
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

function goDashboard() { router.push('/dashboard') }
function goLogin() { router.push('/login') }
function goRegister() { router.push('/register') }

function getCarouselUrl(filename) {
  const base = import.meta.env.BASE_URL
  return `${base}static/carousel/${filename}`.replace(/\/+/g, '/')
}
</script>

<template>
  <div class="home-container">
    <!-- Background is FIXED and outside of main content flow -->
    <div v-if="carouselImages.length > 0" class="hero-bg-fixed">
      <el-carousel height="100%" indicator-position="none" arrow="never" :interval="5000">
        <el-carousel-item v-for="img in carouselImages" :key="img">
          <div class="carousel-img-wrap">
            <img :src="getCarouselUrl(img)" class="carousel-img" />
            <div class="carousel-overlay"></div>
          </div>
        </el-carousel-item>
      </el-carousel>
    </div>
    <div v-else class="hero-bg-fixed is-gradient"></div>

    <!-- Main Content -->
    <div class="hero-section">
      <div class="hero-content animate-fade-in">
        <h1 class="hero-title">{{ siteName }}</h1>
        <p class="hero-subtitle">{{ siteSubtitle }}</p>
        <div class="hero-actions">
          <el-button v-if="isLogged" size="large" @click="goDashboard" class="btn-glass btn-glass-primary hero-btn">
            <el-icon><User /></el-icon>
            <span>进入个人面板</span>
          </el-button>
          <template v-else>
            <el-button size="large" @click="goLogin" class="btn-glass btn-glass-primary hero-btn">
              登录账号
            </el-button>
            <el-button size="large" @click="goRegister" class="btn-glass hero-btn">
              即刻注册
            </el-button>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import "@/assets/styles/animations.css";
@import "@/assets/styles/buttons.css";

.home-container { 
  width: 100%; 
  flex: 1; 
  display: flex; 
  flex-direction: column; 
  position: relative;
  overflow: hidden; /* Prevent any accidental content overflow */
}

/* FIXED Background logic - Using 100% instead of 100vw to avoid scrollbar calculation issues */
.hero-bg-fixed {
  position: fixed; top: 0; left: 0; right: 0; bottom: 0; z-index: 0;
}
.hero-bg-fixed.is-gradient {
  background: linear-gradient(135deg, #1a1a1a 0%, #333333 100%);
}

.hero-bg-fixed :deep(.el-carousel) {
  height: 100%;
}

.carousel-img-wrap { width: 100%; height: 100%; position: relative; }
.carousel-img { width: 100%; height: 100%; object-fit: cover; }
.carousel-overlay { 
  position: absolute; top: 0; left: 0; width: 100%; height: 100%; 
  background: rgba(0, 0, 0, 0.45); 
}

.hero-section {
  position: relative; z-index: 1; flex: 1; display: flex; align-items: center; justify-content: center; color: #fff; padding: 0 20px;
}

.hero-content { text-align: center; max-width: 800px; }
.hero-title { font-size: 56px; font-weight: 800; margin: 0 0 16px 0; letter-spacing: -1.5px; text-shadow: 0 2px 10px rgba(0,0,0,0.3); }
.hero-subtitle { font-size: 20px; margin: 0 0 32px 0; opacity: 0.95; font-weight: 400; }

.hero-actions { display: flex; gap: 16px; justify-content: center; }
.hero-btn { height: 52px; padding: 0 36px; font-size: 16px; font-weight: 600; border-radius: 14px; }

@media (max-width: 768px) {
  .hero-title { font-size: 36px; }
  .hero-actions { flex-direction: column; gap: 12px; }
  .hero-btn { width: 100%; }
}
</style>
