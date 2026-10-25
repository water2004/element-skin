<template>
  <div
    class="animate-card-slide clickable-card texture-card"
    :style="{ '--delay-index': delayIndex }"
    @click="$emit('preview', texture)"
  >
    <div class="card-clip">
      <div
        class="texture-card-preview relative flex h-[280px] w-full items-center justify-center overflow-hidden"
        :style="{ background: isDark ? 'var(--color-background-hero-dark)' : 'var(--color-background-hero-light)' }"
      >
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
        <div
          v-if="texture.type === 'skin' && resolution"
          class="absolute right-2 top-2 z-10 rounded-md px-2.5 py-1 text-xs font-semibold text-white backdrop-blur"
          :style="resolutionBadgeStyle"
        >
          {{ resolution }}x
        </div>
      </div>

      <div
        class="texture-card-info flex min-h-[82px] flex-col items-center gap-1.5 bg-[var(--color-card-background)] p-4 text-center"
      >
        <slot name="info" :texture="texture">
          <div
            v-if="showType"
            class="inline-flex rounded-lg px-2.5 py-1 text-xs font-bold uppercase leading-none tracking-[0.5px]"
            :class="
              texture.type === 'skin'
                ? 'bg-[rgba(64,158,255,0.1)] text-[#409eff]'
                : 'bg-[rgba(103,194,58,0.1)] text-[#67c23a]'
            "
          >
            {{ texture.type === 'skin' ? '皮肤' : '披风' }}
          </div>
          <div
            class="max-w-full overflow-hidden text-ellipsis whitespace-nowrap text-[15px] font-semibold text-[var(--color-heading)]"
          >
            {{ title || '未命名纹理' }}
          </div>
          <div v-if="subtitle" class="text-[13px] font-medium text-[var(--color-text-light)]">
            {{ subtitle }}
          </div>
        </slot>
      </div>

      <CardActions v-if="$slots.actions">
        <slot name="actions" :texture="texture" />
      </CardActions>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import SkinViewer from '@/components/SkinViewer.vue'
import CapeViewer from '@/components/CapeViewer.vue'
import CardActions from '@/components/common/CardActions.vue'
import type { Texture } from '@/api/types'

const props = withDefaults(defineProps<{
  texture: Texture
  delayIndex?: number
  isDark?: boolean
  texturesUrl: (hash: string | null | undefined) => string
  title?: string | null
  subtitle?: string | null
  resolution?: number
  showType?: boolean
}>(), {
  delayIndex: 0,
  isDark: false,
  title: '',
  subtitle: '',
  resolution: undefined,
  showType: false,
})

defineEmits<{
  preview: [texture: Texture]
}>()

const resolutionBadgeStyle = computed(() => {
  const resolution = props.resolution
  if (!resolution) return {}

  let hue = 0
  if (resolution <= 64) hue = 120
  else if (resolution <= 128) hue = 120 - ((resolution - 64) / 64) * 60
  else if (resolution <= 256) hue = 60 - ((resolution - 128) / 128) * 30
  else if (resolution <= 512) hue = 30 - ((resolution - 256) / 256) * 30
  else hue = 330

  return {
    background: `linear-gradient(135deg, hsl(${hue}, 58%, 65%), hsl(${hue + 15}, 53%, 62%))`,
    boxShadow: `0 2px 6px hsla(${hue}, 58%, 50%, 0.25)`,
  }
})
</script>

<style scoped>
.texture-card {
  cursor: pointer;
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

.texture-card:hover {
  transform: translateY(calc(-1 * var(--motion-offset-hover)));
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
}

.card-clip {
  border-radius: inherit;
  overflow: hidden;
}
</style>
