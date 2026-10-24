<template>
  <div class="search-bar-container">
    <ElInput
      v-model="query"
      :placeholder="placeholder"
      :aria-label="label"
      clearable
      :size="size"
      :disabled="disabled"
      @clear="emit('clear')"
      @keyup.enter="search"
    >
      <template #prefix
        ><ElIcon><Search /></ElIcon
      ></template>
      <template #append>
        <UiButton
          variant="primary"
          :icon="Search"
          :size="size"
          :disabled="disabled"
          :loading="loading"
          @click="search"
          >搜索</UiButton
        >
      </template>
    </ElInput>
  </div>
</template>

<script setup lang="ts">
import { ElInput, ElIcon } from 'element-plus'
import { Search } from '@element-plus/icons-vue'
import UiButton from '@/components/ui/UiButton.vue'

const query = defineModel<string>({ required: true })
const props = withDefaults(
  defineProps<{
    placeholder?: string
    label?: string
    size?: 'small' | 'default' | 'large'
    disabled?: boolean
    loading?: boolean
  }>(),
  {
    placeholder: '搜索',
    label: '搜索内容',
    size: 'large',
    disabled: false,
    loading: false,
  },
)
const emit = defineEmits<{ search: []; clear: [] }>()

function search(event: KeyboardEvent | MouseEvent) {
  if (props.disabled || props.loading || ('isComposing' in event && event.isComposing)) return
  emit('search')
}
</script>

<style scoped>
.search-bar-container {
  display: flex;
  min-width: 0;
}

.search-bar-container :deep(.el-input) {
  --el-input-border-radius: var(--radius-control);
}

.search-bar-container :deep(.el-input-group__append) {
  padding: 0;
  display: flex;
  align-items: stretch;
  overflow: hidden;
}

.search-bar-container :deep(.el-input-group__append button.ui-button) {
  margin: 0;
  height: auto;
  align-self: stretch;
  border-radius: 0;
  color: var(--el-button-text-color);
  background-color: var(--el-button-bg-color);
  border-color: var(--el-button-border-color);
}

.search-bar-container :deep(.el-input-group__append button.ui-button:hover:not(:disabled)) {
  color: var(--el-button-hover-text-color);
  background-color: var(--el-button-hover-bg-color);
  border-color: var(--el-button-hover-border-color);
}

.search-bar-container :deep(.el-input-group__append button.ui-button:disabled) {
  color: var(--el-button-disabled-text-color);
  background-color: var(--el-button-disabled-bg-color);
  border-color: var(--el-button-disabled-border-color);
}
</style>
