<template>
  <div class="dashboard-home">
    <!-- Stats Section -->
    <div class="stats-section">
      <el-row :gutter="20">
        <el-col :xs="24" :sm="12">
          <el-card shadow="hover" class="stats-card">
            <div class="stats-content">
              <div class="stats-icon bg-blue">
                <el-icon><Box /></el-icon>
              </div>
              <div class="stats-info">
                <div class="stats-label">材质数量</div>
                <div class="stats-value">{{ textureCount }}</div>
              </div>
            </div>
          </el-card>
        </el-col>
        <el-col :xs="24" :sm="12">
          <el-card shadow="hover" class="stats-card">
            <div class="stats-content">
              <div class="stats-icon bg-purple">
                <el-icon><User /></el-icon>
              </div>
              <div class="stats-info">
                <div class="stats-label">角色数量</div>
                <div class="stats-value">{{ profileCount }}</div>
              </div>
            </div>
          </el-card>
        </el-col>
      </el-row>
    </div>

    <!-- Quick Config Section -->
    <div class="config-section">
      <el-card shadow="hover" class="config-card">
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
              <el-card shadow="hover" class="status-card-mojang">
                <div class="status-item">
                  <div class="status-label">{{ key.toUpperCase() }} API</div>
                  <div class="status-indicator" :class="getMojangStatus(key)">
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
.dashboard-home {
  animation: fadeIn 0.4s cubic-bezier(0.4, 0, 0.2, 1);
  display: flex;
  flex-direction: column;
  gap: 24px;
}

/* Stats Section */
.stats-card {
  border-radius: 12px;
  overflow: hidden;
  border: 1px solid var(--color-border);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
  background: var(--color-card-background);
}
.stats-content {
  display: flex;
  align-items: center;
  padding: 20px;
}
.stats-icon {
  width: 64px;
  height: 64px;
  border-radius: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 32px;
  color: white;
  margin-right: 20px;
}
.bg-blue { background: linear-gradient(135deg, #409eff, #337ecc); }
.bg-purple { background: linear-gradient(135deg, #a0cfff, #8c9eff); /* Adjusted to match theme potentially */ background: linear-gradient(135deg, #b37feb, #8553cf); }

.stats-info {
  display: flex;
  flex-direction: column;
}
.stats-label {
  font-size: 14px;
  color: var(--color-text-light);
  margin-bottom: 4px;
}
.stats-value {
  font-size: 28px;
  font-weight: 700;
  color: var(--color-heading);
}

/* Config Section */
.config-card {
  border-radius: 12px;
  border: 1px solid var(--color-border);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
  background: var(--color-card-background);
}
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
  color: white; /* Ensure text stays white on hover if primary */
}
.drag-btn:active {
  transform: translateY(0);
}

/* Mojang Status Section */
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
.status-card-mojang {
  margin-bottom: 20px;
  border-radius: 12px;
  border: 1px solid var(--color-border);
  box-shadow: 0 2px 8px rgba(0,0,0,0.04);
  background: var(--color-card-background);
}
.status-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px;
}
.status-label {
  font-size: 15px;
  color: var(--color-text);
  font-weight: 600;
}
.status-indicator {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 500;
  padding: 4px 12px;
  border-radius: 16px;
  background: var(--color-background-soft);
}
.status-indicator.online { color: #67c23a; background: rgba(103, 194, 58, 0.1); }
.status-indicator.checking { color: #409eff; background: rgba(64, 158, 255, 0.1); }
.status-indicator.offline { color: #f56c6c; background: rgba(245, 108, 108, 0.1); }
</style>