<template>
  <div class="admin-carousel">
    <div class="section-header">
      <h2>首页轮播图管理</h2>
      <el-upload
        action="#"
        :http-request="uploadCarousel"
        :show-file-list="false"
        accept=".png,.jpg,.jpeg,.webp"
      >
        <el-button type="success" size="large">
          <el-icon><Plus /></el-icon>
          <span style="margin-left: 8px">上传新图片</span>
        </el-button>
      </el-upload>
    </div>

    <el-alert
      title="使用建议"
      type="info"
      description="建议上传 1920x1080 或更高分辨率的横屏图片，以获得最佳全屏展示效果。图片将自动填充整个首页背景。"
      show-icon
      :closable="false"
      class="mb-4"
    />

    <el-card class="list-card" shadow="hover">
      <el-table :data="carouselImages" style="width: 100%" v-loading="loading">
        <el-table-column label="预览" width="200">
          <template #default="{ row }">
            <el-image 
              :src="getCarouselUrl(row.filename)" 
              fit="cover" 
              style="width: 160px; height: 90px; border-radius: 8px; cursor: pointer"
              :preview-src-list="[getCarouselUrl(row.filename)]"
              preview-teleported
            />
          </template>
        </el-table-column>
        <el-table-column prop="filename" label="文件名" min-width="200" />
        <el-table-column label="操作" width="120" align="right" fixed="right">
          <template #default="{ row }">
            <el-button type="danger" @click="deleteCarousel(row)" link>
              <el-icon><Delete /></el-icon>
              <span>删除</span>
            </el-button>
          </template>
        </el-table-column>
      </el-table>
      
      <el-empty v-if="carouselImages.length === 0 && !loading" description="暂无轮播图，首页将显示默认渐变背景" />
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Delete } from '@element-plus/icons-vue'

const carouselImages = ref([])
const loading = ref(false)

function authHeaders() {
  const token = localStorage.getItem('jwt')
  return token ? { Authorization: 'Bearer ' + token } : {}
}

function getCarouselUrl(filename) {
  const base = import.meta.env.VITE_API_BASE || ''
  return `${base}/static/carousel/${filename}`
}

async function fetchCarousel() {
  loading.value = true
  try {
    const res = await axios.get('/public/carousel')
    carouselImages.value = res.data.map(f => ({ filename: f }))
  } catch (e) {
    console.error('Fetch carousel error:', e)
    ElMessage.error('获取列表失败')
  } finally {
    loading.value = false
  }
}

async function uploadCarousel({ file }) {
  const formData = new FormData()
  formData.append('file', file)
  try {
    await axios.post('/admin/carousel', formData, {
      headers: { ...authHeaders(), 'Content-Type': 'multipart/form-data' }
    })
    ElMessage.success('上传成功')
    fetchCarousel()
  } catch (e) {
    ElMessage.error('上传失败: ' + (e.response?.data?.detail || e.message))
  }
}

async function deleteCarousel(row) {
  try {
    await ElMessageBox.confirm('确定要删除这张轮播图吗？', '确认删除', {
      type: 'warning',
      confirmButtonText: '确定删除',
      cancelButtonText: '取消'
    })
    await axios.delete(`/admin/carousel/${row.filename}`, { headers: authHeaders() })
    ElMessage.success('已删除')
    fetchCarousel()
  } catch (e) {
    if (e !== 'cancel') ElMessage.error('删除失败')
  }
}

onMounted(() => {
  fetchCarousel()
})
</script>

<style scoped>
.admin-carousel {
  max-width: 1000px;
  margin: 0 auto;
  width: 100%;
  animation: fadeIn 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.section-header h2 {
  font-weight: 600;
  color: var(--color-heading);
  margin: 0;
}

.list-card {
  border: 1px solid var(--color-border);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
  background: var(--color-card-background);
  animation: cardSlideIn 0.5s cubic-bezier(0.4, 0, 0.2, 1);
}

.mb-4 {
  margin-bottom: 16px;
}

@media (max-width: 768px) {
  .section-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 16px;
  }
}
</style>