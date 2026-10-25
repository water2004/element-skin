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
    <span v-if="showPageSize" class="pager-size-label">每页展示</span>
    <el-select
      v-if="showPageSize"
      class="pager-size"
      size="small"
      filterable
      allow-create
      :model-value="pageSize"
      :placeholder="String(pageSize)"
      :disabled="loading"
      @change="handlePageSizeChange"
    >
      <el-option
        v-for="option in pageSizeOptions"
        :key="option"
        :label="String(option)"
        :value="option"
      />
    </el-select>
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
import { PAGE_SIZE_OPTIONS } from '@/composables/useCursorPagination'
import { normalizePageSize } from '@/storage/pageSize'

interface Props {
  visible?: boolean
  loading?: boolean
  disabledPrev?: boolean
  disabledNext?: boolean
  showCount?: boolean
  count?: number
  showPageSize?: boolean
  pageSize?: number
  pageSizeOptions?: readonly number[]
}

const props = withDefaults(defineProps<Props>(), {
  visible: true,
  loading: false,
  disabledPrev: false,
  disabledNext: false,
  showCount: true,
  count: 0,
  showPageSize: true,
  pageSize: 20,
  pageSizeOptions: () => PAGE_SIZE_OPTIONS,
})

const emit = defineEmits<{
  prev: []
  next: []
  'page-size-change': [value: number]
}>()

function handlePageSizeChange(value: unknown) {
  const next = normalizePageSize(value, props.pageSize)
  if (next !== props.pageSize) emit('page-size-change', next)
}
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

.pager-size-label {
  color: var(--color-text-light);
  font-size: 13px;
  white-space: nowrap;
}

.pager-size {
  width: 54px;
}
</style>
