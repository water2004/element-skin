<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { getPublicSettings, getPublicHomepageMedia } from '@/api/public'
import { getMe } from '@/api/me'
import { createHeroScene } from '@/composables/useHeroScene'
import { User } from '@element-plus/icons-vue'

const router = useRouter()
const siteName = ref(localStorage.getItem('site_name_cache') || '皮肤站')
const siteSubtitle = ref(
  localStorage.getItem('site_subtitle_cache') || '简洁、高效、现代的 Minecraft 皮肤 management 站',
)
const isLogged = ref(false)
const bgCanvasRef = ref<HTMLCanvasElement | null>(null)

// Single source-of-truth renderer for the fixed hero background.
const scene = createHeroScene()

onMounted(async () => {
  scene.setTarget(bgCanvasRef.value)
  scene.start()

  // 加载站点配置
  try {
    const res = await getPublicSettings()
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

  // 加载首页媒体
  try {
    const res = await getPublicHomepageMedia()
    scene.setMedia(res.data)
  } catch (e) {
    console.warn('Failed to load homepage media:', e)
  }

  // 检查登录状态（cookie 自动携带）
  try {
    await getMe()
    isLogged.value = true
  } catch {}
})

onBeforeUnmount(() => {
  scene.destroy()
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
    <!-- Background is FIXED and outside of main content flow -->
    <canvas ref="bgCanvasRef" class="hero-bg-fixed" aria-hidden="true"></canvas>
    <button
      v-if="isLogged"
      type="button"
      class="home-fixed-button home-fixed-primary home-fixed-single probe-fade-in"
      @click="goDashboard"
    >
      <el-icon class="home-fixed-icon"><User /></el-icon>
      <span class="home-fixed-label">进入个人面板</span>
    </button>
    <button
      v-else
      type="button"
      class="home-fixed-button home-fixed-primary probe-fade-in"
      @click="goLogin"
    >
      <span class="home-fixed-label">登录账号</span>
    </button>
    <button
      v-if="!isLogged"
      type="button"
      class="home-fixed-button home-fixed-secondary probe-fade-in"
      @click="goRegister"
    >
      <span class="home-fixed-label">即刻注册</span>
    </button>
    <!-- Main Content -->
    <div class="hero-section">
      <div class="hero-content home-title-fade-in">
        <h1 class="hero-title">{{ siteName }}</h1>
        <p class="hero-subtitle">{{ siteSubtitle }}</p>
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
  position: fixed;
  inset: 0;
  overflow: hidden;
}

/* FIXED Background logic — single canvas, drawn by the hero scene */
.hero-bg-fixed {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  z-index: 0;
  display: block;
}

.home-fixed-button {
  position: fixed;
  top: calc(50vh + 44px);
  z-index: 10;
  isolation: isolate;
  overflow: hidden;
  width: 148px;
  height: 52px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  color: #fff;
  font-size: 16px;
  font-weight: 600;
  border-radius: 14px;
  border: none;
  background: var(--home-action-bg, rgba(255, 255, 255, 0.08));
  backdrop-filter: blur(9px) saturate(180%);
  -webkit-backdrop-filter: blur(9px) saturate(180%);
  box-shadow:
    0 14px 28px rgba(0, 0, 0, 0),
    inset 0 0 0 1px var(--home-action-ring, rgba(255, 255, 255, 0.38)),
    inset 0 1px 0 rgba(255, 255, 255, 0);
  transition:
    top 0.3s cubic-bezier(0.4, 0, 0.2, 1),
    box-shadow 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  cursor: pointer;
  font: inherit;
  appearance: none;
  -webkit-appearance: none;
}

.home-fixed-icon,
.home-fixed-label {
  position: relative;
  z-index: 1;
}

.home-fixed-icon {
  flex: 0 0 auto;
  font-size: 20px;
}

.home-fixed-button:hover {
  box-shadow:
    0 14px 28px rgba(0, 0, 0, 0.18),
    inset 0 0 0 1px var(--home-action-ring, rgba(255, 255, 255, 0.38)),
    inset 0 1px 0 rgba(255, 255, 255, 0.18);
}

.home-fixed-primary {
  left: calc(50vw - 156px);
  top: calc(50vh + 44px);
  --home-action-ring: rgba(64, 158, 255, 0.45);
  --home-action-bg: rgba(64, 158, 255, 0.16);
}

.home-fixed-single {
  left: calc(50vw - 97px);
  width: 194px;
}

.home-fixed-secondary {
  left: calc(50vw + 8px);
  top: calc(50vh + 44px);
  --home-action-ring: rgba(255, 255, 255, 0.34);
  --home-action-bg: rgba(255, 255, 255, 0.12);
}

.home-fixed-primary:hover,
.home-fixed-secondary:hover {
  top: calc(50vh + 40px);
}

.probe-fade-in {
  animation: homeActionFadeIn 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}

.home-title-fade-in {
  animation: homeActionFadeIn 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}

@keyframes homeActionFadeIn {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}

.hero-section {
  position: fixed;
  inset: 0;
  z-index: 1;
  color: #fff;
  pointer-events: none;
}

.hero-content {
  position: fixed;
  left: 50%;
  top: calc(50vh - 116px);
  width: min(800px, calc(100vw - 40px));
  transform: translateX(-50%);
  text-align: center;
}
.hero-title {
  font-size: 56px;
  font-weight: 800;
  margin: 0 0 16px 0;
  letter-spacing: -1.5px;
  text-shadow: 0 2px 10px rgba(0, 0, 0, 0.3);
}
.hero-subtitle {
  font-size: 20px;
  margin: 0 0 32px 0;
  opacity: 0.95;
  font-weight: 400;
}

@media (max-width: 768px) {
  .hero-title {
    font-size: 36px;
  }
  .home-fixed-button {
    left: 32px;
    right: 32px;
    width: auto;
  }
  .home-fixed-primary {
    top: calc(50vh + 36px);
  }
  .home-fixed-single {
    left: 32px;
  }
  .home-fixed-secondary {
    top: calc(50vh + 100px);
  }
  .home-fixed-primary:hover {
    top: calc(50vh + 32px);
  }
  .home-fixed-secondary:hover {
    top: calc(50vh + 96px);
  }
}
</style>
