<template>
  <div
    class="admin-role-card animate-card-slide clickable-card"
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
          :skin-url="texturesUrl(profile.skin_hash)"
          :cape-url="profile.cape_hash ? texturesUrl(profile.cape_hash) : null"
          :model="profile.texture_model || profile.model || 'default'"
          :width="200"
          :height="280"
          is-static
        />
        <el-empty v-else description="未设置皮肤" :image-size="120" />
      </div>

      <div class="role-info">
        <div class="role-name">{{ profile.name }}</div>
        <div class="role-owner">
          所属: {{ profile.owner_display_name || profile.owner_email || '-' }}
        </div>
        <div class="role-model">
          模型: {{ profile.texture_model || profile.model || 'default' }}
        </div>
      </div>

      <CardActions>
        <UiButton variant="primary" :icon="Edit" @click="$emit('preview', profile)">
          编辑
        </UiButton>
      </CardActions>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Edit } from '@element-plus/icons-vue'
import type { Profile } from '@/api/types'
import SkinViewer from '@/components/SkinViewer.vue'
import CardActions from '@/components/common/CardActions.vue'
import UiButton from '@/components/ui/UiButton.vue'

defineProps<{
  profile: Profile
  delayIndex: number
  isDark: boolean
  texturesUrl: (hash: string | null | undefined) => string
}>()

defineEmits<{
  preview: [profile: Profile]
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
  margin-bottom: 4px;
}

.role-owner {
  font-size: 13px;
  color: var(--color-text-light);
  margin-bottom: 4px;
}

.role-model {
  font-size: 13px;
  color: var(--color-text-light);
  font-weight: 500;
}

.clickable-card {
  cursor: pointer;
}

.admin-role-card {
  border: 1px solid var(--color-border);
  border-radius: 16px;
  background: var(--color-card-background);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
  overflow: hidden;
  transition:
    background-color var(--motion-duration-base) ease,
    border-color var(--motion-duration-base) ease,
    transform var(--motion-duration-base) ease,
    box-shadow var(--motion-duration-base) ease;
}

.admin-role-card:hover {
  transform: translateY(calc(-1 * var(--motion-offset-hover)));
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
}

.card-clip {
  border-radius: inherit;
  overflow: hidden;
}
</style>
