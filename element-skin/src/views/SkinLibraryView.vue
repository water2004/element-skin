<template>
  <div class="skin-library-container animate-fade-in">
    <div v-if="isDisabled" class="disabled-container">
      <el-empty description="皮肤库已关闭">
        <template #extra>
          <el-button type="primary" @click="$router.push('/')">返回首页</el-button>
        </template>
      </el-empty>
    </div>
    <template v-else>
      <div class="page-header">
        <div class="page-header-content">
          <div>
            <h1>皮肤库</h1>
            <p>探索并收藏精美材质</p>
          </div>
        </div>
        <div class="page-header-actions">
          <el-radio-group v-model="filterType" @change="handleFilterChange" size="large" class="capsule-radio">
            <el-radio-button value="">全部</el-radio-button>
            <el-radio-button value="skin">皮肤</el-radio-button>
            <el-radio-button value="cape">披风</el-radio-button>
          </el-radio-group>
        </div>
      </div>

    <div class="library-grid-container" v-loading="loading" element-loading-background="transparent">
      <div class="auto-grid" v-if="items.length > 0">
        <div 
          class="surface-card hoverable animate-card-slide clickable-card" 
          v-for="(item, index) in items" 
          :key="item.hash"
          :style="{ '--delay-index': index % 20 }"
          @click="openPreviewDialog(item)"
        >
          <div class="texture-preview" :style="{ background: isDark ? 'var(--color-background-hero-dark)' : 'var(--color-background-hero-light)' }">
            <SkinViewer
              v-if="item.type === 'skin'"
              :skinUrl="texturesUrl(item.hash)"
              :model="item.model || 'default'"
              :width="200"
              :height="280"
              is-static
            />
            <CapeViewer
              v-else
              :capeUrl="texturesUrl(item.hash)"
              :width="200"
              :height="280"
              is-static
            />
            <div
              v-if="item.type === 'skin' && textureResolutions.get(item.hash)"
              class="floating-badge"
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
          <div class="texture-actions" @click.stop>
            <el-button 
              class="btn-gradient btn-gradient-primary" 
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

      <!-- 预览对话框 -->
      <el-dialog
        v-model="showPreviewDialog"
        width="800px"
        destroy-on-close
        class="dialog-viewer"
        append-to-body
      >
        <div class="viewer-layout" v-if="selectedItem">
          <div class="viewer-stage">
            <SkinViewer
              v-if="selectedItem.type === 'skin'"
              :skinUrl="texturesUrl(selectedItem.hash)"
              :model="selectedItem.model || 'default'"
              :width="320"
              :height="430"
            />
            <CapeViewer
              v-else
              :capeUrl="texturesUrl(selectedItem.hash)"
              :width="320"
              :height="430"
            />
          </div>

          <div class="viewer-info-panel">
            <section class="viewer-section title-section">
              <div class="viewer-title-row">
                <h2 class="viewer-display-title">{{ selectedItem.name || '未命名纹理' }}</h2>
              </div>
            </section>

            <section class="viewer-section meta-section">
              <div class="viewer-title-row">
                <span class="meta-chip">{{ textureResolutions.get(selectedItem.hash) || '--' }}px</span>
                <span class="meta-chip" :class="selectedItem.type">
                  {{ selectedItem.type === 'skin' ? '皮肤' : '披风' }}
                </span>
              </div>
              <div class="hash-label">HASH: {{ selectedItem.hash }}</div>
            </section>

            <section class="viewer-section" v-if="selectedItem.uploader_name">
              <div class="viewer-section-label">上传者</div>
              <div class="uploader-info">
                <el-icon><User /></el-icon>
                <span>{{ selectedItem.uploader_name }}</span>
              </div>
            </section>

            <section class="viewer-section footer-section" style="margin-top: auto;">
              <el-button 
                type="primary" 
                size="large" 
                class="btn-gradient btn-gradient-primary" 
                style="width: 100%; border-radius: 12px; height: 50px;"
                @click="addToWardrobe(selectedItem.hash)"
                :disabled="!isLogged"
              >
                <el-icon><Plus /></el-icon>
                <span style="margin-left: 8px;">添加到我的衣柜</span>
              </el-button>
              <p v-if="!isLogged" class="login-hint">登录后即可收藏此纹理</p>
            </section>
          </div>
        </div>
      </el-dialog>

      <div class="pagination-container">
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
const showPreviewDialog = ref(false)
const selectedItem = ref(null)

function openPreviewDialog(item) {
  selectedItem.value = item
  showPreviewDialog.value = true
}

function texturesUrl(hash) {
  if (!hash) return ''
  const base = import.meta.env.BASE_URL
  return `${base}static/textures/${hash}.png`.replace(/\/+/g, '/')
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

<style>
/* Global Styles for Teleported Elements */
@import "@/assets/styles/dialogs.css";
@import "@/assets/styles/item-viewer.css";
@import "@/assets/styles/item-cards.css";
</style>

<style scoped>
@import "@/assets/styles/animations.css";
@import "@/assets/styles/layout.css";
@import "@/assets/styles/buttons.css";
@import "@/assets/styles/cards.css";
@import "@/assets/styles/tags.css";

.skin-library-container {
  margin: 0 0;
  padding: 0;
}

.disabled-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 60vh;
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
}

.texture-info {
  padding: 12px 16px;
  text-align: center;
  background: var(--color-card-background);
}

.texture-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-heading);
  margin-bottom: 6px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.texture-meta-info {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  color: var(--color-text-light);
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
}

.texture-actions .el-button {
  flex: 1;
}

.clickable-card {
  cursor: pointer;
}
</style>
