<template>
  <SkinViewer
    v-if="texture.type === 'skin'"
    :skin-url="texturesUrl(texture.hash)"
    :model="texture.model || 'default'"
    :width="width"
    :height="height"
  />
  <CapeViewer v-else :cape-url="texturesUrl(texture.hash)" :width="width" :height="height" />
</template>

<script setup lang="ts">
import SkinViewer from '@/components/SkinViewer.vue'
import CapeViewer from '@/components/CapeViewer.vue'
import type { Texture } from '@/api/types'

withDefaults(
  defineProps<{
    texture: Texture
    texturesUrl: (hash: string | null | undefined) => string
    width?: number
    height?: number
  }>(),
  {
    width: 280,
    height: 340,
  },
)
</script>
