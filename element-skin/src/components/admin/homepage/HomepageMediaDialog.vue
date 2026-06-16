<template>
  <el-dialog
    v-model="visible"
    :title="item?.type === 'panorama' ? 'Panorama 配置' : '图片配置'"
    width="720px"
    destroy-on-close
    append-to-body
  >
    <div v-if="item" class="detail-layout">
      <div class="detail-preview">
        <el-image
          :src="previewUrl"
          fit="cover"
          class="detail-preview-image"
          :preview-src-list="[previewUrl]"
          preview-teleported
        />
      </div>

      <el-form label-width="96px" class="detail-form">
        <el-form-item label="文件名">
          <el-input v-model="item.title" />
        </el-form-item>
        <el-form-item label="类型">
          <el-tag :type="item.type === 'panorama' ? 'warning' : 'success'">
            {{ item.type === 'panorama' ? '全景图' : '静态图' }}
          </el-tag>
        </el-form-item>
        <el-form-item label="时长">
          <el-input-number
            v-model="item.duration_ms"
            :min="1000"
            :max="60000"
            :step="500"
            controls-position="right"
          />
        </el-form-item>
        <el-form-item label="浅色遮罩">
          <el-slider v-model="item.overlay_opacity_light" :min="0" :max="0.9" :step="0.05" />
        </el-form-item>
        <el-form-item label="深色遮罩">
          <el-slider v-model="item.overlay_opacity_dark" :min="0" :max="0.9" :step="0.05" />
        </el-form-item>
        <template v-if="item.type === 'panorama'">
          <el-form-item v-for="field in panoramaFields" :key="field.key" :label="field.label">
            <el-input-number
              v-model="item[field.key]"
              :min="field.min"
              :max="field.max"
              :step="field.step"
              controls-position="right"
            />
          </el-form-item>
        </template>
      </el-form>
    </div>
    <template #footer>
      <el-button @click="visible = false">完成</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import type { HomepageMedia } from '@/api/types'

type PanoramaField = keyof Pick<
  HomepageMedia,
  'start_yaw' | 'start_pitch' | 'yaw_speed_dps' | 'pitch_speed_dps'
>

defineProps<{
  item?: HomepageMedia
  previewUrl: string
}>()

const visible = defineModel<boolean>('visible', { required: true })

const panoramaFields = [
  { key: 'start_yaw', label: '起始 yaw', min: -360, max: 360, step: 1 },
  { key: 'start_pitch', label: '起始 pitch', min: -89, max: 89, step: 1 },
  { key: 'yaw_speed_dps', label: 'yaw 速度', min: -90, max: 90, step: 0.1 },
  { key: 'pitch_speed_dps', label: 'pitch 速度', min: -90, max: 90, step: 0.1 },
] satisfies Array<{
  key: PanoramaField
  label: string
  min: number
  max: number
  step: number
}>
</script>

<style scoped>
.detail-layout {
  display: grid;
  grid-template-columns: 240px minmax(0, 1fr);
  gap: 22px;
  align-items: start;
}
.detail-preview {
  width: 240px;
  height: 150px;
  overflow: hidden;
  border-radius: 8px;
  background: var(--color-background-soft);
}
.detail-preview-image {
  width: 100%;
  height: 100%;
  display: block;
}
.detail-form {
  min-width: 0;
}

@media (max-width: 768px) {
  .detail-layout {
    grid-template-columns: 1fr;
  }
  .detail-preview {
    width: 100%;
    height: 180px;
  }
}
</style>
