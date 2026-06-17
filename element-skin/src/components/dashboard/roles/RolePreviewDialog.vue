<template>
  <el-dialog v-model="visible" destroy-on-close class="dialog-viewer" append-to-body>
    <div class="viewer-layout" v-if="profile">
      <div class="viewer-stage">
        <SkinViewer
          v-if="profile.skin_hash"
          :skinUrl="texturesUrl(profile.skin_hash)"
          :capeUrl="profile.cape_hash ? texturesUrl(profile.cape_hash) : null"
          :model="profile.model || 'default'"
          :width="320"
          :height="430"
        />
        <el-empty v-else description="未设置皮肤" />
      </div>

      <div class="viewer-info-panel">
        <section class="viewer-section title-section">
          <div class="viewer-title-row">
            <el-button text circle class="title-edit-btn" @click="focusNameInput">
              <el-icon><Edit /></el-icon>
            </el-button>
            <el-input
              ref="nameInputRef"
              v-model="localName"
              class="viewer-title-input"
              placeholder="角色名称"
              @change="$emit('rename', localName)"
            />
          </div>
        </section>

        <section class="viewer-section meta-section">
          <div class="viewer-section-label">角色信息</div>
          <div class="viewer-title-row">
            <span class="meta-chip">模型: {{ profile.model || 'default' }}</span>
          </div>
          <div class="hash-label">UUID: {{ formatUUID(profile.id) }}</div>
          <div class="hash-label" v-if="profile.skin_hash">皮肤 HASH: {{ profile.skin_hash }}</div>
          <div class="hash-label" v-if="profile.cape_hash">披风 HASH: {{ profile.cape_hash }}</div>
        </section>

        <section class="viewer-section" v-if="profile.skin_hash || profile.cape_hash">
          <div class="viewer-section-label">快捷操作</div>
          <div class="apply-row flex gap-2">
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
          </div>
        </section>

        <section class="viewer-section mt-auto">
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
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import type { InputInstance } from 'element-plus'
import { Edit } from '@element-plus/icons-vue'
import type { Profile } from '@/api/types'
import SkinViewer from '@/components/SkinViewer.vue'
import { formatUUID } from '@/utils/format'

const visible = defineModel<boolean>('visible', { required: true })
const props = defineProps<{
  profile: Profile | null
  texturesUrl: (hash: string | null | undefined) => string
}>()

defineEmits<{
  rename: [name: string]
  'set-avatar': [profile: Profile]
  'clear-skin': [profileId: string]
  'clear-cape': [profileId: string]
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
.apply-row .el-button {
  margin-left: 0 !important;
}
</style>
