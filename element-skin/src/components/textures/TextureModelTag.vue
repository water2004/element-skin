<template>
  <el-tag size="small" :type="type === 'cape' ? 'success' : 'primary'" effect="light" round>{{
    type === 'cape' ? '披风' : model === 'slim' ? 'Alex' : 'Steve'
  }}</el-tag>
</template>
<script setup lang="ts">
defineProps<{ type?: string; model?: string | null }>()
</script>
