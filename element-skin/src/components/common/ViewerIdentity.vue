<template>
  <span class="inline-flex min-w-0 items-center gap-1 text-sm" :title="name || '未知用户'">
    <el-icon class="shrink-0"><User /></el-icon
    ><span class="truncate">{{ name || '未知用户' }}</span>
  </span>
</template>
<script setup lang="ts">
import { User } from '@element-plus/icons-vue'
defineProps<{ name?: string | null }>()
</script>
