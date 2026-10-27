<template>
  <el-tag
    v-if="dimensions && (!nonStandardOnly || (precision !== null && precision !== 1))"
    size="small"
    effect="dark"
    >{{ dimensions.width }}px</el-tag
  >
  <span v-else-if="!nonStandardOnly" class="text-sm text-[var(--color-text-light)]">{{
    loading ? '清晰度加载中' : '清晰度未知'
  }}</span>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  cacheTextureDimensions,
  texturePrecision,
  type TextureDimensions,
} from '@/components/textures/textureAssets'
const props = defineProps<{ hash: string; type?: string; nonStandardOnly?: boolean }>()
const cache = new Map<string, TextureDimensions>()
const dimensions = ref<TextureDimensions | null>(null)
const loading = ref(false)
const precision = computed(() =>
  dimensions.value ? texturePrecision(props.type || 'skin', dimensions.value) : null,
)
watch(
  () => props.hash,
  async (hash, _, cleanup) => {
    let current = true
    cleanup(() => {
      current = false
    })
    dimensions.value = null
    loading.value = true
    const value = await cacheTextureDimensions(hash, cache)
    if (current) {
      dimensions.value = value
      loading.value = false
    }
  },
  { immediate: true },
)
</script>
