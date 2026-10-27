<template>
  <el-tag size="small" effect="plain">{{
    type === 'cape' ? '披风' : model === 'slim' ? 'Alex' : 'Steve'
  }}</el-tag>
</template>
<script setup lang="ts">
defineProps<{ type?: string; model?: string | null }>()
</script>
