<template>
  <SkinViewer
    v-if="profile.skin_hash"
    :skin-url="texturesUrl(profile.skin_hash)"
    :cape-url="profile.cape_hash ? texturesUrl(profile.cape_hash) : null"
    :model="profile.texture_model || profile.model || 'default'"
    :width="width"
    :height="height"
    :is-static="isStatic"
  />
  <CapeViewer
    v-else-if="profile.cape_hash"
    :cape-url="texturesUrl(profile.cape_hash)"
    :width="width"
    :height="height"
    :is-static="isStatic"
  />
  <el-empty v-else description="未设置材质" :image-size="120" />
</template>
<script setup lang="ts">
import type { Profile } from '@/api/types'
import SkinViewer from '@/components/SkinViewer.vue'
import CapeViewer from '@/components/CapeViewer.vue'
withDefaults(
  defineProps<{
    profile: Profile
    texturesUrl: (hash: string | null | undefined) => string
    width?: number
    height?: number
    isStatic?: boolean
  }>(),
  { width: 280, height: 340, isStatic: false },
)
</script>
