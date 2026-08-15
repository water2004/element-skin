<template>
  <UiDialog v-model="visible" destroy-on-close variant="viewer">
    <UiViewerLayout v-if="profile">
      <template #stage>
        <SkinViewer
          v-if="profile.skin_hash"
          :skinUrl="texturesUrl(profile.skin_hash)"
          :capeUrl="profile.cape_hash ? texturesUrl(profile.cape_hash) : null"
          :model="profile.model || 'default'"
          :width="320"
          :height="430"
        />
        <el-empty v-else description="未设置皮肤" />
      </template>

      <div class="flex min-h-0 flex-1 flex-col">
        <section class="border-b border-[var(--color-border)] py-3.5">
          <div class="flex items-center gap-2 pr-12">
            <el-button text circle class="title-action-button" @click="focusNameInput">
              <el-icon><Edit /></el-icon>
            </el-button>
            <el-input
              ref="nameInputRef"
              v-model="localName"
              class="title-input-field"
              placeholder="角色名称"
              @change="$emit('rename', localName)"
            />
          </div>
        </section>

        <section class="border-b border-[var(--color-border)] py-3.5">
          <div
            class="mb-2.5 text-xs font-bold uppercase tracking-[0.5px] text-[var(--color-text-light)]"
          >
            角色信息
          </div>
          <div class="flex items-center gap-2 pr-12">
            <span
              class="inline-flex h-7 max-w-full items-center rounded-full border border-[var(--color-border)] bg-[var(--color-background-soft)] px-3 text-xs whitespace-nowrap text-[var(--color-text)] transition"
              >模型: {{ profile.model || 'default' }}</span
            >
          </div>
          <div
            class="mt-3 break-all rounded bg-[var(--color-background-soft)] px-2 py-1 font-mono text-[11px] text-[var(--el-text-color-secondary)]"
          >
            UUID: {{ formatUUID(profile.id) }}
          </div>
          <div
            class="mt-3 break-all rounded bg-[var(--color-background-soft)] px-2 py-1 font-mono text-[11px] text-[var(--el-text-color-secondary)]"
            v-if="profile.skin_hash"
          >
            皮肤 HASH: {{ profile.skin_hash }}
          </div>
          <div
            class="mt-3 break-all rounded bg-[var(--color-background-soft)] px-2 py-1 font-mono text-[11px] text-[var(--el-text-color-secondary)]"
            v-if="profile.cape_hash"
          >
            披风 HASH: {{ profile.cape_hash }}
          </div>
        </section>

        <section v-if="officialBinding" class="border-b border-[var(--color-border)] py-3.5">
          <div
            class="mb-2.5 text-xs font-bold uppercase tracking-[0.5px] text-[var(--color-text-light)]"
          >
            正版角色
          </div>
          <div class="rounded-xl bg-[var(--color-background-soft)] p-3">
            <div class="flex items-center justify-between gap-3">
              <div class="min-w-0">
                <div class="truncate text-sm font-semibold text-[var(--color-heading)]">
                  {{ officialBinding.remote_name }}
                </div>
                <div class="mt-1 truncate text-xs text-[var(--color-text-light)]">
                  {{ officialBinding.identity.label || officialBinding.identity.provider_name }}
                </div>
              </div>
              <el-tag type="success" effect="dark" size="small">正版</el-tag>
            </div>
            <div
              class="mt-3 break-all rounded bg-[var(--color-card-background)] px-2 py-1 font-mono text-[11px] text-[var(--el-text-color-secondary)]"
            >
              UUID: {{ formatUUID(officialBinding.remote_uuid) }}
            </div>
            <ActionBar full>
              <el-button
                v-if="canSyncOfficialBinding"
                type="success"
                plain
                class="flex-1 rounded-lg"
                @click="$emit('sync-official', officialBinding)"
              >
                同步正版数据
              </el-button>
              <el-button
                v-if="canDeleteOfficialBinding"
                type="warning"
                plain
                class="flex-1 rounded-lg"
                @click="$emit('unbind-official', officialBinding)"
              >
                解除绑定
              </el-button>
            </ActionBar>
          </div>
        </section>

        <section
          class="border-b border-[var(--color-border)] py-3.5"
          v-if="profile.skin_hash || profile.cape_hash"
        >
          <div
            class="mb-2.5 text-xs font-bold uppercase tracking-[0.5px] text-[var(--color-text-light)]"
          >
            快捷操作
          </div>
          <ActionBar full>
            <el-button
              v-if="profile.skin_hash"
              type="primary"
              plain
              class="flex-1 rounded-lg"
              @click="$emit('set-avatar', profile)"
            >
              用作头像
            </el-button>
            <el-button
              v-if="profile.skin_hash"
              type="warning"
              plain
              class="flex-1 rounded-lg"
              @click="$emit('clear-skin', profile.id)"
            >
              清除皮肤
            </el-button>
            <el-button
              v-if="profile.cape_hash"
              type="warning"
              plain
              class="flex-1 rounded-lg"
              @click="$emit('clear-cape', profile.id)"
            >
              清除披风
            </el-button>
          </ActionBar>
        </section>

        <section class="mt-auto py-3.5">
          <el-button
            type="danger"
            plain
            class="w-full rounded-lg"
            @click="$emit('delete', profile.id)"
          >
            删除此角色
          </el-button>
        </section>
      </div>
    </UiViewerLayout>
  </UiDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import type { InputInstance } from 'element-plus'
import { Edit } from '@element-plus/icons-vue'
import type { OfficialProfileBinding, Profile } from '@/api/types'
import SkinViewer from '@/components/SkinViewer.vue'
import ActionBar from '@/components/common/ActionBar.vue'
import { formatUUID } from '@/utils/format'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiViewerLayout from '@/components/ui/UiViewerLayout.vue'

const visible = defineModel<boolean>('visible', { required: true })
const props = defineProps<{
  profile: Profile | null
  texturesUrl: (hash: string | null | undefined) => string
  officialBinding?: OfficialProfileBinding | null
  canSyncOfficialBinding: boolean
  canDeleteOfficialBinding: boolean
}>()

defineEmits<{
  rename: [name: string]
  'set-avatar': [profile: Profile]
  'clear-skin': [profileId: string]
  'clear-cape': [profileId: string]
  'sync-official': [binding: OfficialProfileBinding]
  'unbind-official': [binding: OfficialProfileBinding]
  delete: [profileId: string]
}>()

const localName = ref('')
const nameInputRef = ref<InputInstance | null>(null)

watch(
  () => props.profile?.name,
  (name) => {
    localName.value = name || ''
  },
  { immediate: true },
)

function focusNameInput() {
  nameInputRef.value?.focus()
}
</script>

<style scoped>
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
  transition: all 0.2s ease !important;
  flex-shrink: 0;
}

.title-action-button:hover {
  background: var(--color-background-soft) !important;
  color: var(--el-color-primary) !important;
  transform: scale(1.1);
}

.title-action-button .el-icon {
  font-size: 18px;
  color: var(--color-text-light);
}

.title-action-button:hover .el-icon {
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
  transition: box-shadow 0.2s ease;
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
