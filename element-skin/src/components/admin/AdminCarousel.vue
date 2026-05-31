<template>
  <div class="admin-carousel animate-fade-in">
    <PageHeader title="首页图库管理" subtitle="上传并管理首页展示的轮播图片，建议使用高清横屏大图">
      <template #icon><PictureFilled /></template>
      <template #actions>
        <el-upload
          action="#"
          :http-request="uploadCarousel"
          :show-file-list="false"
          accept=".png,.jpg,.jpeg,.webp"
        >
          <el-button type="primary" :icon="Upload" size="large" class="hover-lift">上传图片</el-button>
        </el-upload>
      </template>
    </PageHeader>

    <el-alert
      title="配置建议"
      type="success"
      description="系统会自动循环展示所有上传的图片。为保证视觉效果，请确保图片比例一致（推荐 16:9），且文件大小不超过设置的上限。"
      show-icon
      :closable="false"
      class="mb-6"
    />

    <div class="carousel-grid" v-loading="loading">
      <div v-for="row in carouselImages" :key="row.filename" class="surface-card hover-lift carousel-item-card">
        <el-image 
          :src="getCarouselUrl(row.filename)" 
          fit="cover" 
          class="item-preview"
          :preview-src-list="[getCarouselUrl(row.filename)]"
          preview-teleported
        />
        <div class="item-info">
          <span class="filename" :title="row.filename">{{ row.filename }}</span>
          <el-button type="danger" :icon="Delete" size="small" @click="deleteCarousel(row)" plain circle />
        </div>
      </div>
      
      <div v-if="carouselImages.length === 0 && !loading" class="empty-placeholder">
        <el-empty description="图库暂无内容，首页将显示默认背景" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { UploadRequestOptions } from 'element-plus'
import { Delete, PictureFilled, Upload } from '@element-plus/icons-vue'
import { getPublicCarousel } from '@/api/public'
import { uploadCarousel as apiUploadCarousel, deleteCarousel as apiDeleteCarousel } from '@/api/admin/carousel'
import PageHeader from '@/components/common/PageHeader.vue'

const carouselImages = ref<{ filename: string }[]>([])
const loading = ref(false)

function getCarouselUrl(filename: string) {
  const base = import.meta.env.BASE_URL
  return `${base}static/carousel/${filename}`.replace(/\/+/g, '/')
}

async function fetchCarousel() {
  loading.value = true
  try {
    const res = await getPublicCarousel()
    carouselImages.value = res.data.map(f => ({ filename: f }))
  } catch (e) {
    ElMessage.error('获取图片列表失败')
  } finally {
    loading.value = false
  }
}

async function uploadCarousel({ file }: UploadRequestOptions) {
  const formData = new FormData()
  formData.append('file', file)
  try {
    await apiUploadCarousel(formData)
    ElMessage.success('上传成功')
    fetchCarousel()
  } catch (e: any) {
    ElMessage.error(e.response?.data?.detail || '上传失败')
  }
}

async function deleteCarousel(row: { filename: string }) {
  try {
    await ElMessageBox.confirm('确定要永久删除这张图片吗？', '确认删除', {
      type: 'warning',
      confirmButtonText: '确定删除',
      cancelButtonText: '取消'
    })
    await apiDeleteCarousel(row.filename)
    ElMessage.success('图片已删除')
    fetchCarousel()
  } catch (e) {}
}

onMounted(fetchCarousel)
</script>

<style scoped>
.admin-carousel { max-width: 1000px; margin: 0 auto; padding: 20px 0; }

.carousel-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 20px; }
.carousel-item-card { overflow: hidden; }

.item-preview { width: 100%; height: 160px; cursor: zoom-in; }
.item-info { padding: 12px 16px; display: flex; justify-content: space-between; align-items: center; background: var(--color-background-soft); }
.filename { font-size: 12px; color: var(--color-text-secondary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 180px; font-family: monospace; }

.empty-placeholder { grid-column: 1 / -1; padding: 40px 0; }

@media (max-width: 768px) {
  .carousel-grid { grid-template-columns: 1fr; }
}
</style>
