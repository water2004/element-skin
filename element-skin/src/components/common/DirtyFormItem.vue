<template>
  <ElFormItem :label="label" class="dirty-form-item">
    <template #label>
      <span>{{ label }}</span>
      <span v-if="dirty" class="field-change-hint" role="status">未保存</span>
    </template>
    <slot />
    <template v-if="$slots.error" #error="scope"><slot name="error" v-bind="scope" /></template>
  </ElFormItem>
</template>

<script setup lang="ts">
import { ElFormItem } from 'element-plus'

defineProps<{ label: string; dirty: boolean }>()
</script>

<style scoped>
.field-change-hint {
  margin-inline-start: var(--space-control-gap);
  color: var(--el-color-warning);
  font-size: var(--font-size-caption);
  font-weight: normal;
}

/* 未保存不是校验错误，也不覆盖输入焦点。错误发生时优先显示验证信息。 */
.dirty-form-item.is-error .field-change-hint {
  display: none;
}
</style>
