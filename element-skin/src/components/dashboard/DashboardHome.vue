<template>
  <div class="dashboard-home animate-fade-in">
    <!-- Stats Section -->
    <div class="stats-section">
      <el-row :gutter="20">
        <el-col :xs="24" :sm="12">
          <el-card shadow="hover" class="surface-card">
            <div class="stats-card-content">
              <div class="stats-card-icon bg-gradient-blue">
                <el-icon><Box /></el-icon>
              </div>
              <div class="stats-card-info">
                <div class="stats-card-label">材质数量</div>
                <div class="stats-card-value">{{ textureCount }}</div>
              </div>
            </div>
          </el-card>
        </el-col>
        <el-col :xs="24" :sm="12">
          <el-card shadow="hover" class="surface-card">
            <div class="stats-card-content">
              <div class="stats-card-icon bg-gradient-purple">
                <el-icon><User /></el-icon>
              </div>
              <div class="stats-card-info">
                <div class="stats-card-label">角色数量</div>
                <div class="stats-card-value">{{ profileCount }}</div>
              </div>
            </div>
          </el-card>
        </el-col>
      </el-row>
    </div>

    <!-- Quick Config Section -->
    <div class="config-section">
      <el-card shadow="hover" class="surface-card config-card">
        <template #header>
          <div class="card-header">
            <span>快速配置启动器</span>
          </div>
        </template>
        <div class="config-content">
          <p class="config-desc">
            将下方的 API 地址复制到您的启动器，或直接拖动“添加到启动器”按钮到支持 authlib-injector 的启动器窗口中。
          </p>
          <div class="api-url-box">
            <el-input v-model="apiUrl" readonly>
              <template #append>
                <el-button @click="copyApiUrl">
                  <el-icon><CopyDocument /></el-icon> 复制
                </el-button>
              </template>
            </el-input>
          </div>
          <div class="drag-action">
            <a 
              class="el-button el-button--primary is-round drag-btn" 
              :href="`authlib-injector:yggdrasil-server:${encodeURIComponent(apiUrl)}`"
              title="拖动我到启动器"
            >
              <el-icon><Pointer /></el-icon>
              <span>拖拽添加到启动器</span>
            </a>
          </div>
        </div>
      </el-card>
    </div>

    <!-- Mojang Status Section -->
    <div class="mojang-status-section">
      <div class="section-header">
        <h2>Mojang 服务状态</h2>
        <el-button @click="checkMojangStatus" :loading="isChecking" size="small">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
      </div>

      <div class="status-container">
        <div v-if="mojangStatusUrls">
          <el-row :gutter="20">
            <el-col :xs="24" :sm="8" v-for="(url, key) in mojangStatusUrls" :key="key">
              <el-card shadow="hover" class="surface-card status-card-mojang">
                <div class="status-item">
                  <div class="status-label">{{ key.toUpperCase() }} API</div>
                  <div class="status-tag" :class="getMojangStatus(key)">
                    <el-icon v-if="getMojangStatus(key) === 'online'"><Check /></el-icon>
                    <el-icon v-else-if="getMojangStatus(key) === 'checking'"><Loading /></el-icon>
                    <el-icon v-else><Warning /></el-icon>
                    <span>{{ formatStatusText(getMojangStatus(key)) }}</span>
                  </div>
                </div>
              </el-card>
            </el-col>
          </el-row>
        </div>
        <el-empty v-else description="无法获取状态配置" />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import axios from 'axios'
import { 
  Box, User, CopyDocument, Pointer, 
  Check, Loading, Warning, Refresh 
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

// --- Stats & Config ---
const textureCount = ref(0)
const profileCount = ref(0)
const apiUrl = ref('')

function getApiUrl() {
    const base = import.meta.env.VITE_API_BASE || ''
    if (base.startsWith('http')) {
        return base
    }
    // Remove trailing slash from origin if base starts with slash to avoid double slash, 
    // actually window.location.origin usually has no trailing slash.
    // But VITE_API_BASE might not have leading slash?
    // Let's safe join.
    const origin = window.location.origin
    const path = base.startsWith('/') ? base : '/' + base
    // Remove trailing slash from result to be clean, Yggdrasil usually handles it but clean is better.
    let full = origin + path
    if (full.endsWith('/') && full.length > 1) {
        full = full.slice(0, -1)
    }
    // If path was just '/', full is origin.
    return full
}

function copyApiUrl() {
  if (!apiUrl.value) return
  navigator.clipboard.writeText(apiUrl.value).then(() => {
    ElMessage.success('API 地址已复制')
  }).catch(() => {
    ElMessage.error('复制失败，请手动复制')
  })
}

// --- Mojang Status ---
const mojangStatusUrls = ref(null)
const mojangHealth = ref({})
const isChecking = ref(false)

async function checkMojangStatus() {
  if (!mojangStatusUrls.value) return
  isChecking.value = true

  for (const [key, url] of Object.entries(mojangStatusUrls.value)) {
    mojangHealth.value[key] = 'checking'
    try {
      const controller = new AbortController()
      const timeoutId = setTimeout(() => controller.abort(), 5000)

      await fetch(url, { mode: 'no-cors', signal: controller.signal })
      clearTimeout(timeoutId)
      mojangHealth.value[key] = 'online'
    } catch (e) {
      mojangHealth.value[key] = 'offline'
    }
  }
  isChecking.value = false
}

function getMojangStatus(key) {
  return mojangHealth.value[key] || 'checking'
}

function formatStatusText(status) {
  if (status === 'online') return '在线'
  if (status === 'checking') return '检查中...'
  return '连接超时'
}

// --- Lifecycle ---
onMounted(async () => {
  // Load Settings for Mojang Status and API URL
  try {
    const res = await axios.get('/public/settings')
    
    // Set API URL from backend settings, fallback to calculated one if empty
    if (res.data.site_url) {
      apiUrl.value = res.data.site_url.endsWith('/') ? res.data.site_url.slice(0, -1) : res.data.site_url
    } else {
      apiUrl.value = getApiUrl()
    }

    if (res.data.mojang_status_urls) {
      mojangStatusUrls.value = res.data.mojang_status_urls
      checkMojangStatus()
    }
  } catch (e) {
    console.warn('Failed to load public settings')
    apiUrl.value = getApiUrl()
  }

  // Load User Stats (Textures)
  const token = localStorage.getItem('jwt')
  if (token) {
      const headers = { Authorization: 'Bearer ' + token }
      
      // Get Textures
      try {
          const res = await axios.get('/me/textures', { headers })
          if (Array.isArray(res.data)) {
              textureCount.value = res.data.length
          }
      } catch (e) {
          console.error('Failed to load textures count', e)
      }

      // Get Profiles (from /me)
      try {
          const res = await axios.get('/me', { headers })
          if (res.data && res.data.profiles) {
              profileCount.value = res.data.profiles.length
          }
      } catch (e) {
          console.error('Failed to load profiles count', e)
      }
  }
})
</script>

<style scoped>
@import "@/assets/styles/animations.css";
@import "@/assets/styles/cards.css";
@import "@/assets/styles/tags.css";

.dashboard-home {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

/* Config Section Specifics */
.card-header {
  font-weight: 600;
  font-size: 18px;
  color: var(--color-heading);
}
.config-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
  align-items: center;
  padding: 10px 0;
}
.config-desc {
  font-size: 14px;
  color: var(--color-text);
  text-align: center;
  margin: 0;
}
.api-url-box {
  width: 100%;
  max-width: 500px;
}
.drag-action {
  margin-top: 8px;
}
.drag-btn {
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 40px;
  padding: 0 20px;
  font-weight: 500;
  transition: transform 0.2s;
}
.drag-btn:hover {
  transform: translateY(-2px);
  color: white;
}
.drag-btn:active {
  transform: translateY(0);
}

/* Mojang Status Section Specifics */
.mojang-status-section {
    margin-top: 12px;
}
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.section-header h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: var(--color-heading);
}
.status-card-mojang :deep(.el-card__body) {
  padding: 0;
}
.status-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
}
.status-label {
  font-size: 15px;
  color: var(--color-text);
  font-weight: 600;
}
</style>