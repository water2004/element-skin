<template>
  <UiDialog v-model="open" destroy-on-close variant="viewer">
    <UiViewerLayout v-if="texture">
      <template #stage>
        <TexturePreviewStage :texture="texture" :textures-url="texturesUrl" />
      </template>

      <div v-loading="isLoading" class="flex min-h-0 flex-1 flex-col">
        <section class="border-b border-[var(--color-border)] py-3.5">
          <div class="flex items-center gap-2 pr-12">
            <el-button
              v-if="canEditMetadata"
              text
              circle
              class="title-action-button"
              aria-label="修改材质备注"
              @click="focusNoteInput"
            >
              <el-icon><Edit /></el-icon>
            </el-button>
            <el-input
              v-if="canEditMetadata"
              ref="noteInputRef"
              v-model="note"
              placeholder="未命名纹理"
              class="title-input-field"
              @blur="emit('updateNote')"
              @keyup.enter="emit('updateNote')"
            />
            <h2 v-else class="m-0 break-all text-2xl font-bold text-[var(--color-heading)]">
              {{ texture.note || '未命名纹理' }}
            </h2>
          </div>
        </section>

        <section class="border-b border-[var(--color-border)] py-3.5">
          <div class="flex items-center gap-2 pr-12">
            <span
              class="inline-flex h-7 max-w-full items-center rounded-full border border-[var(--color-border)] bg-[var(--color-background-soft)] px-3 text-xs whitespace-nowrap text-[var(--color-text)] transition"
              >{{ resolution || '--' }}px</span
            >
            <span
              class="inline-flex h-7 max-w-60 items-center overflow-hidden text-ellipsis whitespace-nowrap rounded-full border border-[var(--color-border)] bg-[var(--color-background-soft)] px-3 font-mono text-xs text-[var(--color-text)] transition"
              >{{ texture.hash }}</span
            >
          </div>
        </section>

        <section
          class="border-b border-[var(--color-border)] py-3.5"
          v-if="texture.type === 'skin'"
        >
          <div
            class="mb-2.5 text-xs font-bold uppercase tracking-[0.5px] text-[var(--color-text-light)]"
          >
            {{ canEditMetadata ? '模型选择' : '模型' }}
          </div>
          <UiSegmented
            v-if="canEditMetadata"
            :model-value="texture.model"
            @update:model-value="emit('updateModel', $event)"
          >
            <el-radio-button value="default">Default</el-radio-button>
            <el-radio-button value="slim">Slim</el-radio-button>
          </UiSegmented>
          <span v-else>{{ texture.model === 'slim' ? 'Slim' : 'Default' }}</span>
        </section>

        <section
          class="border-b border-[var(--color-border)] py-3.5"
          v-if="!isLoading && texture.is_public !== 2"
        >
          <div
            class="mb-2.5 text-xs font-bold uppercase tracking-[0.5px] text-[var(--color-text-light)]"
          >
            公开状态
          </div>
          <div class="flex items-center gap-3">
            <el-switch
              v-if="canEditVisibility"
              :model-value="texture.is_public"
              :active-value="1"
              :inactive-value="0"
              @change="emit('updatePublic', $event)"
            />
            <span class="text-[13px] text-[var(--el-text-color-secondary)]">
              {{
                texture.is_public === 1 ? '公开（其他用户可在皮肤库看到）' : '私有（仅自己可见）'
              }}
            </span>
          </div>
        </section>

        <section v-if="canApply" class="border-b border-[var(--color-border)] py-3.5">
          <div
            class="mb-2.5 text-xs font-bold uppercase tracking-[0.5px] text-[var(--color-text-light)]"
          >
            应用到角色
          </div>
          <div class="flex gap-2">
            <el-select v-model="profileId" placeholder="选择目标" class="gallery-select">
              <el-option v-for="p in profiles" :key="p.id" :label="p.name" :value="p.id" />
            </el-select>
            <el-button
              type="primary"
              class="gallery-apply-btn"
              @click="emit('apply')"
              :loading="isApplying"
            >
              确定
            </el-button>
          </div>
        </section>

        <section v-if="canDelete" class="mt-auto border-b-0 py-3.5 pb-0">
          <el-button type="danger" plain class="w-full rounded-lg" @click="emit('delete')">
            删除纹理
          </el-button>
        </section>
      </div>
    </UiViewerLayout>
  </UiDialog>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { Edit } from '@element-plus/icons-vue'

import type { Profile, Texture } from '@/api/types'
import TexturePreviewStage from '@/components/textures/TexturePreviewStage.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiSegmented from '@/components/ui/UiSegmented.vue'
import UiViewerLayout from '@/components/ui/UiViewerLayout.vue'

const open = defineModel<boolean>({ required: true })
const texture = defineModel<Texture | null>('texture', { required: true })
const note = defineModel<string>('note', { required: true })
const profileId = defineModel<string>('profileId', { required: true })

defineProps<{
  isLoading: boolean
  isApplying: boolean
  canEditMetadata: boolean
  canEditVisibility: boolean
  canApply: boolean
  canDelete: boolean
  profiles: Profile[]
  resolution?: number
  texturesUrl: (hash: string | null | undefined) => string
}>()

const emit = defineEmits<{
  updateNote: []
  updateModel: [value: string | number | boolean | null | undefined]
  updatePublic: [value: string | number | boolean]
  apply: []
  delete: []
}>()

const noteInputRef = ref<{ focus: () => void } | null>(null)

function focusNoteInput() {
  noteInputRef.value?.focus()
}
</script>

<style scoped>
.gallery-select {
  flex: 1;
}

.gallery-select :deep(.el-input__wrapper) {
  border-radius: 8px;
  border: 1px solid var(--color-border);
  background: var(--color-background-soft);
  box-shadow: none !important;
}

.gallery-apply-btn {
  min-width: 90px;
  border-radius: 8px;
}

.title-action-button {
  width: 32px !important;
  height: 32px !important;
  padding: 0 !important;
  display: flex !important;
  align-items: center;
  justify-content: center;
  border-radius: 50% !important;
  background: transparent !important;
  border: none !important;
  transition:
    color var(--motion-duration-fast) ease,
    background-color var(--motion-duration-fast) ease,
    border-color var(--motion-duration-fast) ease,
    box-shadow var(--motion-duration-fast) ease,
    opacity var(--motion-duration-fast) ease !important;
  flex-shrink: 0;
}

.title-action-button:is(:hover, :focus-visible) {
  background: var(--color-background-soft) !important;
  color: var(--el-color-primary) !important;
}

.title-action-button .el-icon {
  font-size: 18px;
  color: var(--color-text-light);
}

.title-action-button:is(:hover, :focus-visible) .el-icon {
  color: var(--el-color-primary);
}

.title-input-field {
  flex: 1;
}

.title-input-field :deep(.el-input__wrapper) {
  box-shadow: none !important;
  background: transparent !important;
  padding: 0 !important;
  border: none !important;
  transition: box-shadow var(--motion-duration-fast) ease;
}

.title-input-field :deep(.el-input__wrapper.is-focus) {
  box-shadow: 0 2px 0 var(--el-color-primary) !important;
  border-radius: 0 !important;
}

.title-input-field :deep(.el-input__inner) {
  height: 48px;
  font-size: 28px;
  font-weight: 700;
  color: var(--color-heading);
  line-height: 48px;
}
</style>
