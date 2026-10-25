<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import { getPublicSettings, getPublicHomepageMedia } from '@/api/public'
import { createHeroScene } from '@/composables/useHeroScene'
import { appStorage } from '@/storage'
import { useSessionStore } from '@/stores/session'
import { User } from '@element-plus/icons-vue'

const router = useRouter()
const siteName = ref(appStorage.siteSettings.getSiteName())
const siteSubtitle = ref(appStorage.siteSettings.getSiteSubtitle())
const sessionStore = useSessionStore()
const { user, authReady } = storeToRefs(sessionStore)
const isLogged = computed(() => authReady.value && !!user.value)
const homeActions = computed(() => {
  if (!authReady.value) return []
  return isLogged.value
    ? [{ path: '/dashboard', label: '进入个人面板', variant: 'primary', showIcon: true }]
    : [
        { path: '/login', label: '登录账号', variant: 'primary', showIcon: false },
        { path: '/register', label: '即刻注册', variant: 'secondary', showIcon: false },
      ]
})
const bgCanvasRef = ref<HTMLCanvasElement | null>(null)
const hasEntered = ref(false)
let entranceFrame = 0
let mounted = false

function startEntrance() {
  if (!mounted || !authReady.value || hasEntered.value || entranceFrame) return
  entranceFrame = requestAnimationFrame(() => {
    entranceFrame = requestAnimationFrame(() => {
      entranceFrame = 0
      if (mounted && authReady.value) hasEntered.value = true
    })
  })
}

watch(
  authReady,
  (ready) => {
    if (ready) startEntrance()
    else {
      cancelAnimationFrame(entranceFrame)
      entranceFrame = 0
    }
  },
  { flush: 'post' },
)

// Single source-of-truth renderer for the fixed hero background.
const scene = createHeroScene()

onMounted(async () => {
  scene.setTarget(bgCanvasRef.value)
  scene.start()

  // 会话确定后整组进入同一绘制周期，不等待头像、通知或背景媒体。
  mounted = true
  startEntrance()

  // 加载站点配置
  try {
    const res = await getPublicSettings()
    if (res.data.site_name) {
      siteName.value = res.data.site_name
      appStorage.siteSettings.setSiteName(res.data.site_name)
    }
    if (res.data.site_subtitle) {
      siteSubtitle.value = res.data.site_subtitle
      appStorage.siteSettings.setSiteSubtitle(res.data.site_subtitle)
    }
  } catch (e) {
    console.warn('Failed to load site settings:', e)
  }

  // 加载首页媒体
  try {
    const res = await getPublicHomepageMedia()
    scene.setMedia(res.data.items)
  } catch (e) {
    console.warn('Failed to load homepage media:', e)
  }
})

onBeforeUnmount(() => {
  mounted = false
  cancelAnimationFrame(entranceFrame)
  scene.destroy()
})
</script>

<template>
  <div class="home-container">
    <!-- Background is FIXED and outside of main content flow -->
    <canvas ref="bgCanvasRef" class="hero-bg-fixed" aria-hidden="true"></canvas>
    <div class="hero-content" :class="{ 'is-entered': hasEntered }">
      <div class="hero-copy">
        <h1 class="hero-title">{{ siteName }}</h1>
        <p class="hero-subtitle">{{ siteSubtitle }}</p>
      </div>
      <div class="hero-actions" :aria-busy="!authReady">
        <template v-for="action in homeActions" :key="action.path">
          <button
            type="button"
            class="home-action"
            :class="`home-action-${action.variant}`"
            @click="router.push(action.path)"
          >
            <span class="home-action-content">
              <el-icon v-if="action.showIcon"><User /></el-icon>
              <span>{{ action.label }}</span>
            </span>
          </button>
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped>
.home-container {
  position: relative;
  flex: 1;
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

/* Hero background is the only fixed layer — it is a backdrop, not layout */
.hero-bg-fixed {
  position: fixed;
  inset: 0;
  width: 100vw;
  height: 100vh;
  z-index: 0;
  display: block;
}

.hero-content {
  position: relative;
  z-index: 1;
  width: min(800px, calc(100vw - 40px));
  text-align: center;
  color: #fff;
  translate: 0 var(--motion-offset-content);
  transition: translate var(--motion-duration-slow) var(--motion-easing-standard);
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

.hero-actions {
  display: flex;
  min-height: 52px;
  gap: 16px;
  justify-content: center;
  flex-wrap: wrap;
}

.home-action {
  position: relative;
  isolation: isolate;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 148px;
  height: 52px;
  padding: 0 24px;
  color: #fff;
  font-size: 16px;
  font-weight: 600;
  border-radius: 14px;
  border: none;
  outline: none;
  background: transparent;
  transition: transform var(--motion-duration-fast) var(--motion-easing-standard);
  cursor: pointer;
  appearance: none;
  -webkit-appearance: none;
}

.home-action::before {
  content: '';
  position: absolute;
  inset: 0;
  z-index: -1;
  background: var(--home-action-bg, rgba(255, 255, 255, 0.08));
  backdrop-filter: blur(9px) saturate(180%);
  -webkit-backdrop-filter: blur(9px) saturate(180%);
  clip-path: inset(0 round 14px);
  pointer-events: none;
}

.home-action:is(:hover, :focus-visible) {
  transform: translateY(calc(-1 * var(--motion-offset-hover)));
}

.home-action:focus-visible {
  outline: 2px solid #fff;
  outline-offset: 4px;
}

.home-action-primary {
  --home-action-bg: rgba(64, 158, 255, 0.16);
}

.home-action-secondary {
  --home-action-bg: rgba(255, 255, 255, 0.12);
}

.home-action-content {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
}

/* 位移作用于同一流式容器；只对文案渐入，避免祖先 opacity 影响毛玻璃。 */
.hero-copy,
.home-action-content {
  opacity: 0;
  transition: opacity var(--motion-duration-slow) var(--motion-easing-standard);
}

.hero-content.is-entered {
  translate: 0 0;
}

.is-entered .hero-copy,
.is-entered .home-action-content {
  opacity: 1;
}

@media (prefers-reduced-motion: reduce) {
  .hero-content {
    translate: none;
    transition: none;
  }
  .hero-copy,
  .home-action-content {
    opacity: 1;
    transition: none;
  }
}

@media (max-width: 768px) {
  .hero-title {
    font-size: 36px;
  }
  .home-action {
    min-width: 0;
  }
}
</style>
