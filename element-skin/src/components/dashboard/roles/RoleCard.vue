<template>
  <div
    class="role-card animate-card-slide clickable-card"
    :style="{ '--delay-index': delayIndex }"
    @click="$emit('preview', profile)"
  >
    <div class="card-clip">
      <div
        class="role-preview"
        :style="{
          background: isDark
            ? 'var(--color-background-hero-dark)'
            : 'var(--color-background-hero-light)',
        }"
      >
        <SkinViewer
          v-if="profile.skin_hash"
          :skinUrl="texturesUrl(profile.skin_hash)"
          :capeUrl="profile.cape_hash ? texturesUrl(profile.cape_hash) : null"
          :model="profile.model || 'default'"
          :width="200"
          :height="280"
          is-static
        />
        <el-empty v-else description="未设置皮肤" :image-size="120" />
      </div>

      <div class="role-info">
        <div class="role-name">{{ profile.name }}</div>
        <div class="role-model">模型: {{ profile.model || 'default' }}</div>
        <el-tag v-if="officialBinding" class="mt-2" size="small" type="success">
          正版：{{ officialBinding.remote_name }}
        </el-tag>
      </div>

      <CardActions>
        <UiButton
          variant="gradient-danger"
          icon-swap
          size="default"
          @click.stop="$emit('delete', profile.id)"
        >
          删除
          <template #icon>
            <el-icon><Delete /></el-icon>
          </template>
        </UiButton>

        <UiButton
          v-if="profile.skin_hash"
          variant="soft-warning"
          icon-swap
          size="default"
          @click.stop="$emit('clear-skin', profile.id)"
        >
          皮肤
          <template #icon>
            <el-icon><Close /></el-icon>
          </template>
        </UiButton>

        <UiButton
          v-if="profile.cape_hash"
          variant="soft-warning"
          icon-swap
          size="default"
          @click.stop="$emit('clear-cape', profile.id)"
        >
          披风
          <template #icon>
            <el-icon><Close /></el-icon>
          </template>
        </UiButton>

        <UiButton
          v-if="!officialBinding && canCreateOfficialBinding"
          variant="outline"
          size="default"
          @click.stop="$emit('bind-official', profile)"
        >
          绑定正版
        </UiButton>

        <UiButton
          v-if="officialBinding && canSyncOfficialBinding"
          variant="outline"
          size="default"
          @click.stop="$emit('sync-official', officialBinding)"
        >
          同步
        </UiButton>

        <UiButton
          v-if="officialBinding && canDeleteOfficialBinding"
          variant="soft-warning"
          size="default"
          @click.stop="$emit('unbind-official', officialBinding)"
        >
          解绑
        </UiButton>
      </CardActions>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Close, Delete } from '@element-plus/icons-vue'
import type { OfficialProfileBinding, Profile } from '@/api/types'
import SkinViewer from '@/components/SkinViewer.vue'
import CardActions from '@/components/common/CardActions.vue'
import UiButton from '@/components/ui/UiButton.vue'

defineProps<{
  profile: Profile
  delayIndex: number
  isDark: boolean
  texturesUrl: (hash: string | null | undefined) => string
  officialBinding?: OfficialProfileBinding | null
  canCreateOfficialBinding: boolean
  canSyncOfficialBinding: boolean
  canDeleteOfficialBinding: boolean
}>()

defineEmits<{
  preview: [profile: Profile]
  delete: [profileId: string]
  'clear-skin': [profileId: string]
  'clear-cape': [profileId: string]
  'bind-official': [profile: Profile]
  'sync-official': [binding: OfficialProfileBinding]
  'unbind-official': [binding: OfficialProfileBinding]
}>()
</script>

<style scoped>
.role-preview {
  width: 100%;
  height: 280px;
  display: flex;
  justify-content: center;
  align-items: center;
}

.role-info {
  padding: 16px;
  text-align: center;
  background: var(--color-card-background);
}

.role-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-heading);
  margin-bottom: 8px;
}

.role-model {
  font-size: 13px;
  color: var(--color-text-light);
  font-weight: 500;
}

.clickable-card {
  cursor: pointer;
}

.role-card {
  border: 1px solid var(--color-border);
  border-radius: 16px;
  background: var(--color-card-background);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
  overflow: hidden;
  transition:
    background-color 0.3s ease,
    border-color 0.3s ease,
    transform 0.3s ease,
    box-shadow 0.3s ease;
}

.role-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
}

.card-clip {
  border-radius: inherit;
  overflow: hidden;
}
</style>
