<template>
  <UiCard v-if="uri" class="p-5">
    <div class="text-sm font-semibold text-[var(--color-heading)]">
      请在身份提供方添加 Redirect URI
    </div>
    <el-text
      copyable
      class="mt-2 !flex min-w-0 rounded-lg bg-[var(--color-background-soft)] px-3 py-2 font-mono text-xs text-[var(--color-text)]"
    >
      <span class="min-w-0 break-all">{{ uri }}</span>
    </el-text>
  </UiCard>
</template>

<script setup lang="ts">
import UiCard from '@/components/ui/UiCard.vue'

defineProps<{
  uri: string
}>()
</script>
