<template>
  <el-tag v-if="width" size="small" effect="plain">{{ width }}px</el-tag>
  <span v-else class="text-sm text-[var(--color-text-light)]">{{
    loading ? '清晰度加载中' : '清晰度未知'
  }}</span>
</template>
<script setup lang="ts">
import { ref, watch } from 'vue'
import { cacheTextureWidth } from '@/components/textures/textureAssets'
const props = defineProps<{ hash: string }>()
const cache = new Map<string, number>()
const width = ref<number | null>(null)
const loading = ref(false)
watch(
  () => props.hash,
  async (hash, _, cleanup) => {
    let current = true
    cleanup(() => {
      current = false
    })
    width.value = null
    loading.value = true
    const value = await cacheTextureWidth(hash, cache)
    if (current) {
      width.value = value
      loading.value = false
    }
  },
  { immediate: true },
)
</script>
