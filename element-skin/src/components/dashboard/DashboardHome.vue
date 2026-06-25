<template>
  <div class="flex flex-col animate-fade-in">
    <div class="page-header">
      <div class="page-header-content">
        <h1>我的主页</h1>
        <p>这里汇总了您的资源数量、启动器接入入口与备用服务的健康状态</p>
      </div>
    </div>

    <section class="flex flex-col gap-4 mb-8">
      <UiCard class="dashboard-stat-card" shadow="hover">
        <div class="stat-card-content">
          <el-statistic title="材质数量" :value="textureCount">
            <template #prefix>
              <el-icon><Box /></el-icon>
            </template>
          </el-statistic>
          <el-statistic title="角色数量" :value="profileCount">
            <template #prefix>
              <el-icon><User /></el-icon>
            </template>
          </el-statistic>
        </div>
      </UiCard>
    </section>

    <section class="flex flex-col gap-4 mb-8">
      <div class="flex justify-between items-baseline gap-3">
        <h2 class="m-0 text-lg font-semibold text-[var(--color-heading)]">快速接入启动器</h2>
      </div>
      <UiCard class="launcher-card" shadow="hover">
        <div class="launcher-access">
          <p class="launcher-copy">
            点击下方按钮复制 API 地址，或直接将其拖到支持 authlib-injector 的启动器窗口中。
          </p>
          <el-input v-model="apiUrl" readonly maxlength="256" class="api-url-input" />
          <a
            class="el-button el-button--primary drag-btn inline-flex items-center justify-center gap-2 font-medium whitespace-nowrap"
            :href="`authlib-injector:yggdrasil-server:${encodeURIComponent(apiUrl)}`"
            title="点击复制，或拖到启动器"
            @click.prevent="copyApiUrl"
          >
            <el-icon><CopyDocument /></el-icon>
            <span>复制或拖到启动器</span>
          </a>
        </div>
      </UiCard>
    </section>

    <section v-if="fallbackEntries.length" class="flex flex-col gap-4 mb-0">
      <div class="flex justify-between items-baseline gap-3">
        <h2 class="m-0 text-lg font-semibold text-[var(--color-heading)]">服务状态</h2>
        <el-button @click="loadFallbackStatus" :loading="isChecking" size="small" text>
          <el-icon><Refresh /></el-icon>
          <span>刷新</span>
        </el-button>
      </div>

      <div class="flex flex-col gap-4">
        <FallbackStatusCard v-for="entry in fallbackEntries" :key="entry.id" :entry="entry" />
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getPublicSettings, getPublicFallbackStatus } from '@/api/public'
import { getMe } from '@/api/me'
import { Box, User, CopyDocument, Refresh } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import type { FallbackStatusEntry } from '@/api/types'
import FallbackStatusCard from './FallbackStatusCard.vue'
import UiCard from '@/components/ui/UiCard.vue'

const textureCount = ref(0)
const profileCount = ref(0)
const apiUrl = ref('')

function getApiUrl() {
  const base = import.meta.env.VITE_API_BASE || ''
  if (base.startsWith('http')) {
    return base
  }
  const origin = window.location.origin
  const path = base.startsWith('/') ? base : '/' + base
  let full = origin + path
  if (full.endsWith('/') && full.length > 1) {
    full = full.slice(0, -1)
  }
  return full
}

function copyApiUrl() {
  if (!apiUrl.value) return
  navigator.clipboard
    .writeText(apiUrl.value)
    .then(() => {
      ElMessage.success('API 地址已复制')
    })
    .catch(() => {
      ElMessage.error('复制失败，请手动复制')
    })
}

const fallbackEntries = ref<FallbackStatusEntry[]>([])
const isChecking = ref(false)

async function loadFallbackStatus() {
  isChecking.value = true
  try {
    const res = await getPublicFallbackStatus()
    const list = (res.data.endpoints || []).slice()
    list.sort((a, b) => (a.priority || 0) - (b.priority || 0))
    fallbackEntries.value = list
  } catch {
    ElMessage.error('加载 Fallback 状态失败')
  } finally {
    isChecking.value = false
  }
}

onMounted(async () => {
  try {
    const res = await getPublicSettings()
    if (res.data.api_url) {
      apiUrl.value = res.data.api_url.endsWith('/')
        ? res.data.api_url.slice(0, -1)
        : res.data.api_url
    } else {
      apiUrl.value = getApiUrl()
    }
  } catch {
    apiUrl.value = getApiUrl()
  }

  await loadFallbackStatus()

  try {
    const res = await getMe()
    if (res.data) {
      profileCount.value = res.data.profile_count || 0
      textureCount.value = res.data.texture_count || 0
    }
  } catch (e) {
    console.error('Failed to load user stats', e)
  }
})
</script>

<style scoped>
.dashboard-stat-card :deep(.el-card__body) {
  padding: 26px 32px;
}

.stat-card-content {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 96px;
  min-height: 96px;
}

.dashboard-stat-card :deep(.el-statistic) {
  min-width: 160px;
  text-align: center;
}

.dashboard-stat-card :deep(.el-statistic__head) {
  margin-bottom: 8px;
  color: var(--color-text-light);
  font-size: 14px;
  font-weight: 600;
}

.dashboard-stat-card :deep(.el-statistic__content) {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: var(--color-heading);
  font-size: 30px;
  font-weight: 700;
}

.dashboard-stat-card :deep(.el-statistic__prefix) {
  display: inline-flex;
  align-items: center;
  color: var(--el-color-primary);
  font-size: 26px;
}

.launcher-card :deep(.el-card__body) {
  padding: 28px;
}

.launcher-access {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  text-align: center;
}

.launcher-copy {
  max-width: 720px;
  margin: 0;
  color: var(--color-text-light);
  font-size: 14px;
  line-height: 1.6;
}

.api-url-input {
  width: min(760px, 100%);
}

.drag-btn {
  text-decoration: none;
  min-width: 220px;
  min-height: 40px;
  padding: 0 20px;
  transition: transform 0.2s;
}

.drag-btn:hover {
  transform: translateY(-1px);
  color: white;
}

@media (max-width: 640px) {
  .dashboard-stat-card :deep(.el-card__body),
  .launcher-card :deep(.el-card__body) {
    padding: 22px 18px;
  }

  .stat-card-content {
    align-items: stretch;
    justify-content: center;
    flex-direction: column;
    gap: 18px;
    min-height: 88px;
  }

  .dashboard-stat-card :deep(.el-statistic) {
    min-width: 0;
  }

  .drag-btn {
    width: 100%;
    min-width: 0;
  }
}
</style>
