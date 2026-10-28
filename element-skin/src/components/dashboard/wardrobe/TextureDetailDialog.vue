<template>
  <UiDialog
    v-model="open"
    destroy-on-close
    variant="viewer"
    :busy="busy"
    :dirty="dirty"
    title="材质详情"
  >
    <UiViewerLayout v-if="texture">
      <template #stage
        ><TexturePreviewStage :texture="texture" :textures-url="texturesUrl"
      /></template>
      <ViewerNameEditor
        :key="texture.hash + texture.type + String(revision)"
        :value="texture.note || ''"
        :can-edit="canEditMetadata && ready"
        :busy="busy"
        :save="saveName"
        label="材质名称"
        @dirty="dirty = $event"
      />
      <el-alert v-if="error" :title="error" type="error" :closable="false" class="mt-4" />
      <div v-if="isLoading" role="status" class="py-4">详情加载中</div>
      <section class="flex flex-col items-start gap-3 border-b border-[var(--color-border)] py-5">
        <div class="flex flex-wrap gap-2">
          <TextureResolutionTag :hash="texture.hash" :type="texture.type" /><TextureModelTag
            :type="texture.type"
            :model="texture.model"
          />
        </div>
        <IdentifierValue label="Hash" :value="texture.hash" />
      </section>
      <div v-if="ready" class="flex flex-col items-start gap-5 py-5">
        <div v-if="texture.type === 'skin' && canEditMetadata" class="w-full">
          <div class="mb-2 text-sm text-[var(--color-text-light)]">设置模型</div>
          <TextureModelSelect
            :model="texture.model"
            :busy="busy"
            :disabled="blocked"
            @change="$emit('updateModel', $event)"
          />
        </div>
        <div v-if="texture.is_public !== 2" class="flex flex-wrap items-center gap-3">
          <span class="text-sm text-[var(--color-text-light)]">公开状态</span>
          <el-switch
            v-if="canEditVisibility"
            :model-value="texture.is_public"
            :active-value="1"
            :inactive-value="0"
            :disabled="blocked"
            aria-label="公开状态"
            @change="$emit('updatePublic', $event)"
          />
          <span>{{ texture.is_public === 1 ? '公开' : '私有' }}</span>
        </div>
        <div v-if="canApply" class="w-full">
          <div class="mb-2 text-sm text-[var(--color-text-light)]">应用到角色</div>
          <div class="flex flex-wrap gap-2">
            <el-select
              v-model="profileId"
              placeholder="选择角色"
              class="min-w-0 flex-1 basis-40"
              :disabled="busy || profilesLoading || !!profilesError"
              :loading="profilesLoading"
            >
              <el-option v-for="p in profiles" :key="p.id" :label="p.name" :value="p.id" />
            </el-select>
            <UiButton
              variant="primary"
              hover-effect
              :disabled="blocked || !profileId || profilesLoading || !!profilesError"
              @click="$emit('apply')"
              >应用到角色</UiButton
            >
          </div>
          <p v-if="profilesError" role="alert" class="mt-2 text-sm text-[var(--el-color-danger)]">
            {{ profilesError }}
          </p>
        </div>
        <UiButton
          v-if="canDelete"
          variant="secondary"
          tone="danger"
          :disabled="blocked"
          @click="$emit('delete')"
          >删除纹理</UiButton
        >
      </div>
    </UiViewerLayout>
  </UiDialog>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import type { Profile, Texture } from '@/api/types'
import TextureModelSelect from '@/components/textures/TextureModelSelect.vue'
import TexturePreviewStage from '@/components/textures/TexturePreviewStage.vue'
import TextureModelTag from '@/components/textures/TextureModelTag.vue'
import TextureResolutionTag from '@/components/textures/TextureResolutionTag.vue'
import ViewerNameEditor from '@/components/common/ViewerNameEditor.vue'
import IdentifierValue from '@/components/common/IdentifierValue.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiViewerLayout from '@/components/ui/UiViewerLayout.vue'
const open = defineModel<boolean>({ required: true })
const dirty = defineModel<boolean>('dirty', { default: false })
const profileId = defineModel<string>('profileId', { required: true })
const props = defineProps<{
  texture: Texture | null
  isLoading: boolean
  ready: boolean
  error: string
  busy: boolean
  revision: number
  saveName: (value: string) => Promise<boolean>
  canEditMetadata: boolean
  canEditVisibility: boolean
  canApply: boolean
  canDelete: boolean
  profiles: Profile[]
  profilesLoading: boolean
  profilesError: string
  texturesUrl: (hash: string | null | undefined) => string
}>()
const blocked = computed(() => props.busy || dirty.value)
defineEmits<{
  updateModel: [value: string | number | boolean | undefined]
  updatePublic: [value: string | number | boolean]
  apply: []
  delete: []
}>()
</script>
