<template>
  <UiDialog v-model="visible" destroy-on-close variant="viewer" :busy="!!clearingTexture">
    <UiViewerLayout v-if="profile">
      <template #stage>
        <SkinViewer
          v-if="profile.skin_hash"
          :skin-url="texturesUrl(profile.skin_hash)"
          :cape-url="profile.cape_hash ? texturesUrl(profile.cape_hash) : null"
          :model="profile.texture_model || profile.model || 'default'"
          :width="320"
          :height="430"
        />
        <el-empty v-else description="未设置皮肤" />
      </template>

      <div class="flex min-h-0 flex-1 flex-col">
        <section class="border-b border-[var(--color-border)] py-3.5">
          <el-input
            v-model="name"
            placeholder="角色名称"
            :disabled="!!clearingTexture"
            @blur="$emit('rename')"
            @keyup.enter="$emit('rename')"
          />
        </section>

        <section class="border-b border-[var(--color-border)] py-3.5">
          <div
            class="mb-2.5 text-xs font-bold uppercase tracking-[0.5px] text-[var(--color-text-light)]"
          >
            皮肤绑定
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <div
              aria-label="皮肤绑定"
              class="min-w-0 flex-1 basis-40 rounded-lg bg-[var(--color-background-soft)] px-3 py-2 text-xs text-[var(--color-text-light)]"
            >
              <code v-if="profile.skin_hash" class="break-all select-text font-mono">{{
                profile.skin_hash
              }}</code>
              <span v-else>未绑定</span>
            </div>
            <UiButton
              variant="secondary"
              tone="warning"
              :disabled="!profile.skin_hash || !!clearingTexture"
              :loading="clearingTexture === 'skin'"
              class="shrink-0"
              @click="$emit('clear-skin')"
            >
              清除皮肤
            </UiButton>
          </div>
        </section>

        <section class="border-b border-[var(--color-border)] py-3.5">
          <div
            class="mb-2.5 text-xs font-bold uppercase tracking-[0.5px] text-[var(--color-text-light)]"
          >
            披风绑定
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <div
              aria-label="披风绑定"
              class="min-w-0 flex-1 basis-40 rounded-lg bg-[var(--color-background-soft)] px-3 py-2 text-xs text-[var(--color-text-light)]"
            >
              <code v-if="profile.cape_hash" class="break-all select-text font-mono">{{
                profile.cape_hash
              }}</code>
              <span v-else>未绑定</span>
            </div>
            <UiButton
              variant="secondary"
              tone="warning"
              :disabled="!profile.cape_hash || !!clearingTexture"
              :loading="clearingTexture === 'cape'"
              class="shrink-0"
              @click="$emit('clear-cape')"
            >
              清除披风
            </UiButton>
          </div>
        </section>

        <section class="mt-auto py-3.5">
          <el-button
            type="danger"
            plain
            class="w-full rounded-lg"
            :disabled="!!clearingTexture"
            @click="$emit('delete')"
          >
            删除角色
          </el-button>
        </section>
      </div>
    </UiViewerLayout>
  </UiDialog>
</template>

<script setup lang="ts">
import type { Profile } from '@/api/types'
import SkinViewer from '@/components/SkinViewer.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiViewerLayout from '@/components/ui/UiViewerLayout.vue'

const visible = defineModel<boolean>('visible', { required: true })
const name = defineModel<string>('name', { required: true })

defineProps<{
  profile: Profile | null
  texturesUrl: (hash: string | null | undefined) => string
  clearingTexture: 'skin' | 'cape' | null
}>()

defineEmits<{
  rename: []
  'clear-skin': []
  'clear-cape': []
  delete: []
}>()
</script>
