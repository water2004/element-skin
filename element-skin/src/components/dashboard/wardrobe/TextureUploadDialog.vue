<template>
  <UiDialog v-model="open" destroy-on-close variant="viewer" class="texture-upload-panel">
    <UiViewerLayout>
      <template #stage>
        <el-empty
          v-if="!previewUrl"
          description="选择文件后即可在此预览"
          class="preview-placeholder"
        />
        <div v-else-if="previewFailed" class="preview-fallback">
          <el-empty description="无法渲染此纹理，仍可尝试上传" />
        </div>
        <SkinViewer
          v-else-if="form.texture_type === 'skin'"
          :key="previewUrl"
          :skin-url="previewUrl"
          :model="form.model"
          :width="320"
          :height="430"
          @error="onPreviewError"
        />
        <CapeViewer
          v-else
          :key="previewUrl"
          :cape-url="previewUrl"
          :width="320"
          :height="430"
          @error="onPreviewError"
        />
      </template>

      <div class="flex min-h-0 flex-1 flex-col">
        <section class="border-b border-[var(--color-border)] py-3.5">
          <div
            class="mb-2.5 text-xs font-bold uppercase tracking-[0.5px] text-[var(--color-text-light)]"
          >
            选择文件
          </div>
          <el-upload
            ref="uploadRef"
            :auto-upload="false"
            :limit="1"
            accept=".png"
            :on-change="handleFileChange"
            :on-remove="handleFileRemove"
            drag
            class="upload-wrapper"
          >
            <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
            <div class="el-upload__text">将 PNG 文件拖到此处，或<em>点击上传</em></div>
            <template #tip>
              <div class="el-upload__tip">仅支持 PNG 格式的皮肤或披风文件</div>
            </template>
          </el-upload>
        </section>

        <section class="border-b border-[var(--color-border)] py-3.5">
          <div
            class="mb-2.5 text-xs font-bold uppercase tracking-[0.5px] text-[var(--color-text-light)]"
          >
            纹理类型
          </div>
          <UiSegmented v-model="form.texture_type">
            <el-radio-button value="skin">皮肤 (Skin)</el-radio-button>
            <el-radio-button value="cape">披风 (Cape)</el-radio-button>
          </UiSegmented>
        </section>

        <section
          class="border-b border-[var(--color-border)] py-3.5"
          v-if="form.texture_type === 'skin'"
        >
          <div
            class="mb-2.5 text-xs font-bold uppercase tracking-[0.5px] text-[var(--color-text-light)]"
          >
            皮肤模型
          </div>
          <UiSegmented v-model="form.model">
            <el-radio-button value="default">普通 (4px 手臂)</el-radio-button>
            <el-radio-button value="slim">纤细 (3px 手臂)</el-radio-button>
          </UiSegmented>
        </section>

        <section class="border-b border-[var(--color-border)] py-3.5">
          <div
            class="mb-2.5 text-xs font-bold uppercase tracking-[0.5px] text-[var(--color-text-light)]"
          >
            备注
          </div>
          <el-input v-model="form.note" placeholder="给这个纹理添加备注（可选）" />
        </section>

        <section class="border-b border-[var(--color-border)] py-3.5">
          <div
            class="mb-2.5 text-xs font-bold uppercase tracking-[0.5px] text-[var(--color-text-light)]"
          >
            公开状态
          </div>
          <div class="flex items-center gap-3">
            <el-switch v-model="form.is_public" />
            <span class="text-[13px] text-[var(--el-text-color-secondary)]">
              公开后其他用户可以在皮肤库中看到并使用
            </span>
          </div>
        </section>

        <section class="mt-auto flex gap-2 pt-3.5">
          <el-button class="flex-1 rounded-lg" @click="open = false">取消</el-button>
          <el-button type="primary" class="flex-1 rounded-lg" @click="emit('submit')">
            <el-icon><Upload /></el-icon>
            确认上传
          </el-button>
        </section>
      </div>
    </UiViewerLayout>
  </UiDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { Upload, UploadFilled } from '@element-plus/icons-vue'
import type { UploadFile, UploadInstance } from 'element-plus'

import CapeViewer from '@/components/CapeViewer.vue'
import SkinViewer from '@/components/SkinViewer.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiSegmented from '@/components/ui/UiSegmented.vue'
import UiViewerLayout from '@/components/ui/UiViewerLayout.vue'
import type { TextureUploadForm } from '@/components/dashboard/wardrobe/uploadForm'

const open = defineModel<boolean>({ required: true })
const form = defineModel<TextureUploadForm>('form', { required: true })

const props = defineProps<{
  previewUrl: string | null
}>()

const emit = defineEmits<{
  fileChange: [file: UploadFile]
  fileRemove: []
  submit: []
}>()

const uploadRef = ref<UploadInstance | null>(null)
const previewFailed = ref(false)

watch(
  () => [props.previewUrl, form.value.texture_type],
  () => {
    previewFailed.value = false
  },
)

function handleFileChange(file: UploadFile) {
  emit('fileChange', file)
}

function handleFileRemove() {
  emit('fileRemove')
}

function onPreviewError() {
  previewFailed.value = true
}

function clearFiles() {
  uploadRef.value?.clearFiles()
}

defineExpose({ clearFiles })
</script>

<style scoped>
.texture-upload-panel :deep(.el-upload-dragger) {
  width: 100%;
}

.upload-wrapper {
  width: 100%;
}

.preview-fallback {
  padding: 24px;
}
</style>
