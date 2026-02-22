<template>
  <div class="skin-library-container">
    <div v-if="isDisabled" class="disabled-container">
      <el-empty description="皮肤库已关闭">
        <template #extra>
          <el-button type="primary" @click="$router.push('/')">返回首页</el-button>
        </template>
      </el-empty>
    </div>
    <template v-else>
      <div class="library-header">
      <div class="header-content">
        <h1>皮肤库</h1>
        <p>探索并收藏精美材质</p>
      </div>
      <div class="header-filters">
        <el-radio-group v-model="filterType" @change="handleFilterChange" size="large">
          <el-radio-button value="">全部</el-radio-button>
          <el-radio-button value="skin">皮肤</el-radio-button>
          <el-radio-button value="cape">披风</el-radio-button>
        </el-radio-group>
      </div>
    </div>

    <div class="library-grid-container" v-loading="loading">
      <div class="common-grid" v-if="items.length > 0">
        <div 
          class="common-card" 
          v-for="(item, index) in items" 
          :key="item.hash"
          :style="{ '--delay-index': index % 20 }"
        >
          <div class="texture-preview" :style="{ background: isDark ? 'var(--color-background-hero-dark)' : 'var(--color-background-hero-light)' }">
            <SkinViewer
              v-if="item.type === 'skin'"
              :skinUrl="texturesUrl(item.hash)"
              :model="item.model || 'default'"
              :width="200"
              :height="280"
              @load="handleTextureLoad(item.hash)"
            />
            <CapeViewer
              v-else
              :capeUrl="texturesUrl(item.hash)"
              :width="200"
              :height="280"
            />
            <div
              v-if="item.type === 'skin' && textureResolutions.get(item.hash)"
              class="resolution-badge"
              :style="getResolutionBadgeStyle(textureResolutions.get(item.hash))"
            >
              {{ textureResolutions.get(item.hash) }}x
            </div>
          </div>
          <div class="texture-info">
            <div class="texture-title">{{ item.name || '未命名材质' }}</div>
            <div class="texture-meta-info">
              <span class="uploader-name" v-if="item.uploader_name">
                <el-icon><User /></el-icon>
                {{ item.uploader_name }}
              </span>
              <span class="meta-separator" v-if="item.uploader_name">·</span>
              <span class="texture-date">
                {{ formatDate(item.created_at) }}
              </span>
            </div>
          </div>
          <div class="texture-actions">
            <el-button 
              class="action-btn action-btn-primary" 
              @click="addToWardrobe(item.hash)"
              :disabled="!isLogged"
            >
              <el-icon><Plus /></el-icon>
              <span>添加到衣柜</span>
            </el-button>
          </div>
        </div>
      </div>
      
      <el-empty v-else-if="!loading" description="库中暂无公开材质" />

      <div class="pagination-container" v-if="total > limit">
        <el-pagination
          background
          layout="prev, pager, next"
          :total="total"
          :page-size="limit"
          v-model:current-page="currentPage"
          @current-change="handlePageChange"
        />
      </div>
    </div>
    </template>
  </div>
</template>

<script setup>
import { ref, onMounted, inject, computed } from 'vue'
import axios from 'axios'
import { ElMessage } from 'element-plus'
import { Plus, User } from '@element-plus/icons-vue'
import SkinViewer from '@/components/SkinViewer.vue'
import CapeViewer from '@/components/CapeViewer.vue'

const isDark = inject('isDark')
const user = inject('user')
const isLogged = computed(() => !!user.value)

const items = ref([])
const total = ref(0)
const currentPage = ref(1)
const limit = 20
const loading = ref(false)
const isDisabled = ref(false)
const filterType = ref('')
const textureResolutions = ref(new Map())

function texturesUrl(hash) {
  if (!hash) return ''
  return (import.meta.env.VITE_API_BASE || '') + '/static/textures/' + hash + '.png'
}

function formatDate(ts) {
  if (!ts) return ''
  const date = new Date(ts)
  return date.toLocaleDateString()
}

async function fetchLibrary() {
  loading.value = true
  try {
    const params = {
      page: currentPage.value,
      limit: limit,
      texture_type: filterType.value || undefined
    }
    const res = await axios.get('/public/skin-library', { params })
    items.value = res.data.items
    total.value = res.data.total
    
    items.value.forEach(item => {
      if (item.type === 'skin') {
        loadTextureResolution(item.hash)
      }
    })
  } catch (e) {
    console.error('Fetch library error:', e)
    if (e.response?.status === 403) {
      isDisabled.value = true
    } else {
      ElMessage.error('加载皮肤库失败')
    }
  } finally {
    loading.value = false
  }
}

function loadTextureResolution(hash) {
  if (textureResolutions.value.has(hash)) return
  const img = new Image()
  img.crossOrigin = 'anonymous'
  img.onload = () => {
    textureResolutions.value.set(hash, img.width)
  }
  img.src = texturesUrl(hash)
}

function handleTextureLoad(hash) {}

function getResolutionBadgeStyle(resolution) {
  let hue = 0
  if (resolution <= 64) hue = 120
  else if (resolution <= 128) hue = 120 - ((resolution - 64) / 64) * 60
  else if (resolution <= 256) hue = 60 - ((resolution - 128) / 128) * 30
  else if (resolution <= 512) hue = 30 - ((resolution - 256) / 256) * 30
  else hue = 330

  return {
    background: `linear-gradient(135deg, hsl(${hue}, 58%, 65%), hsl(${hue + 15}, 53%, 62%))`,
    boxShadow: `0 2px 6px hsla(${hue}, 58%, 50%, 0.25)`
  }
}

function handlePageChange(page) {
  currentPage.value = page
  fetchLibrary()
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

function handleFilterChange() {
  currentPage.value = 1
  fetchLibrary()
}

function authHeaders() {
  const token = localStorage.getItem('jwt')
  return token ? { Authorization: 'Bearer ' + token } : {}
}

async function addToWardrobe(hash) {
  try {
    await axios.post(`/me/textures/${hash}/add`, {}, { headers: authHeaders() })
    ElMessage.success('已成功添加到我的衣柜')
  } catch (e) {
    ElMessage.error('添加失败: ' + (e.response?.data?.detail || e.message))
  }
}

onMounted(() => {
  fetchLibrary()
})
</script>

<style scoped>
.skin-library-container {
  max-width: 1400px;
  margin: 0 auto;
  padding: 20px;
  animation: fadeIn 0.4s ease;
}

.disabled-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 60vh;
}

.library-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  margin-bottom: 40px;
  flex-wrap: wrap;
  gap: 20px;
}

.header-content h1 {
  font-size: 32px;
  margin: 0 0 8px 0;
  background: linear-gradient(135deg, var(--color-heading) 0%, #409eff 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.header-content p {
  margin: 0;
  color: var(--color-text-light);
  font-size: 16px;
  transition: color 0.3s ease;
}

.library-grid-container {
  min-height: 400px;
}

.texture-preview {
  width: 100%;
  height: 280px;
  display: flex;
  justify-content: center;
  align-items: center;
  position: relative;
  overflow: hidden;
  transition: background 0.3s ease, color 0.3s ease;
}

.resolution-badge {
  position: absolute;
  top: 8px;
  right: 8px;
  padding: 4px 10px;
  border-radius: 6px;
  color: #fff;
  font-size: 12px;
  font-weight: 600;
  backdrop-filter: blur(4px);
  z-index: 10;
}

.texture-info {
  padding: 12px 16px;
  text-align: center;
  background: var(--color-card-background);
  transition: background-color 0.3s ease, color 0.3s ease;
}

.texture-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-heading);
  margin-bottom: 6px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  transition: color 0.3s ease;
}

.texture-meta-info {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  color: var(--color-text-light);
  transition: color 0.3s ease;
}

.meta-separator {
  opacity: 0.5;
}

.uploader-name {
  font-size: 12px;
  display: flex;
  align-items: center;
  gap: 3px;
  max-width: 100px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.texture-date {
  font-size: 12px;
}

.texture-actions {
  display: flex;
  padding: 12px 16px;
  border-top: 1px solid var(--color-border);
  background: var(--color-background-soft);
  transition: background-color 0.3s ease, border-color 0.3s ease;
}

.texture-actions .el-button {
  flex: 1;
}

.action-btn {
  border: none;
  font-weight: 500;
  transition: all 0.3s ease;
}

.action-btn-primary {
  background: linear-gradient(135deg, #409eff 0%, #5cadff 100%);
  color: #fff;
}

.action-btn-primary:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(64, 158, 255, 0.4);
}

.pagination-container {
  margin-top: 40px;
  display: flex;
  justify-content: center;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}

@media (max-width: 768px) {
  .library-header {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
