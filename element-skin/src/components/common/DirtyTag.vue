<template>
  <el-tag
    v-if="visible"
    size="small"
    type="warning"
    effect="dark"
    :class="rootClass"
  >
    有未保存更改
  </el-tag>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    visible: boolean
    /** add left margin when placed inline next to a heading */
    spaced?: boolean
  }>(),
  {
    spaced: true,
  },
)

const rootClass = computed(() => (props.spaced ? 'ml-2' : ''))
</script>
