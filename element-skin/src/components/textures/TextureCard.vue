<template>
  <PreviewCard
    class="texture-card"
    :title="title || '未命名材质'"
    :delay-index="delayIndex"
    @preview="$emit('preview', texture)"
  >
    <template #preview>
      <SkinViewer
        v-if="texture.type === 'skin'"
        :skin-url="texturesUrl(texture.hash)"
        :model="texture.model || 'default'"
        :width="200"
        :height="280"
        is-static
      />
      <CapeViewer
        v-else
        :cape-url="texturesUrl(texture.hash)"
        :width="200"
        :height="280"
        is-static
      />
    </template>
    <template #info
      ><slot name="info" :texture="texture"
        ><TextureModelTag :type="texture.type" :model="texture.model" /></slot
    ></template>
    <template v-if="$slots.actions" #actions><slot name="actions" :texture="texture" /></template>
  </PreviewCard>
</template>
<script setup lang="ts">
import SkinViewer from '@/components/SkinViewer.vue'
import CapeViewer from '@/components/CapeViewer.vue'
import PreviewCard from '@/components/common/PreviewCard.vue'
import TextureModelTag from '@/components/textures/TextureModelTag.vue'
import type { Texture } from '@/api/types'
defineProps<{
  texture: Texture
  delayIndex?: number
  texturesUrl: (hash: string | null | undefined) => string
  title?: string | null
}>()
defineEmits<{ preview: [texture: Texture] }>()
</script>
