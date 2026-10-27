<template>
  <el-tag
    size="default"
    :type="type === 'cape' ? 'success' : 'primary'"
    effect="light"
    round
    class="font-bold"
    >{{ type === 'cape' ? '披风' : model === 'slim' ? 'Alex' : 'Steve' }}</el-tag
  >
</template>
<script setup lang="ts">
defineProps<{ type?: string; model?: string | null }>()
</script>
