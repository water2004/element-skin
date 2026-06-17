<template>
  <el-dialog v-model="visible" destroy-on-close class="dialog-viewer" append-to-body>
    <div class="viewer-layout" v-if="profile">
      <div class="viewer-stage">
        <SkinViewer
          v-if="profile.skin_hash"
          :skin-url="texturesUrl(profile.skin_hash)"
          :cape-url="profile.cape_hash ? texturesUrl(profile.cape_hash) : null"
          :model="profile.texture_model || profile.model || 'default'"
          :width="320"
          :height="430"
        />
        <el-empty v-else description="未设置皮肤" />
      </div>

      <div class="viewer-info-panel">
        <section class="viewer-section title-section">
          <el-input
            v-model="name"
            placeholder="角色名称"
            @blur="$emit('rename')"
            @keyup.enter="$emit('rename')"
          />
        </section>

        <section class="viewer-section">
          <div class="viewer-section-label">皮肤绑定</div>
          <el-input :model-value="profile.skin_hash || '未绑定'" disabled>
            <template #append>
              <el-button :disabled="!profile.skin_hash" @click="$emit('clear-skin')"
                >清除</el-button
              >
            </template>
          </el-input>
        </section>

        <section class="viewer-section">
          <div class="viewer-section-label">披风绑定</div>
          <el-input :model-value="profile.cape_hash || '未绑定'" disabled>
            <template #append>
              <el-button :disabled="!profile.cape_hash" @click="$emit('clear-cape')"
                >清除</el-button
              >
            </template>
          </el-input>
        </section>

        <section class="viewer-section mt-auto">
          <el-button type="danger" plain class="w-full rounded-lg" @click="$emit('delete')">
            删除角色
          </el-button>
        </section>
      </div>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import type { Profile } from '@/api/types'
import SkinViewer from '@/components/SkinViewer.vue'

const visible = defineModel<boolean>('visible', { required: true })
const name = defineModel<string>('name', { required: true })

defineProps<{
  profile: Profile | null
  texturesUrl: (hash: string | null | undefined) => string
}>()

defineEmits<{
  rename: []
  'clear-skin': []
  'clear-cape': []
  delete: []
}>()
</script>
