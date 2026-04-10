<template>
  <div class="cursor-pager" v-if="visible">
    <el-button
      class="pager-arrow"
      circle
      :icon="ArrowLeftBold"
      :disabled="disabledPrev || loading"
      @click="$emit('prev')"
    />
    <span class="pager-count" v-if="showCount">{{ count }} 项</span>
    <el-button
      class="pager-arrow"
      circle
      :icon="ArrowRightBold"
      :disabled="disabledNext || loading"
      @click="$emit('next')"
    />
  </div>
</template>

<script setup lang="ts">
import { ArrowLeftBold, ArrowRightBold } from '@element-plus/icons-vue'

interface Props {
  visible?: boolean
  loading?: boolean
  disabledPrev?: boolean
  disabledNext?: boolean
  showCount?: boolean
  count?: number
}

withDefaults(defineProps<Props>(), {
  visible: true,
  loading: false,
  disabledPrev: false,
  disabledNext: false,
  showCount: true,
  count: 0,
})

defineEmits<{
  prev: []
  next: []
}>()
</script>

<style scoped>
.cursor-pager {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.pager-arrow {
  border-color: var(--color-border);
  background: var(--color-card-background);
}

.pager-count {
  min-width: 0;
  padding: 0 4px;
  text-align: center;
  font-size: 13px;
  color: var(--color-text-light);
}
</style>
