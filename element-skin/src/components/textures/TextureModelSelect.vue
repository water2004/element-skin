<template>
  <UiSegmented v-model="selection" :disabled="disabled" @change="$emit('change', String($event))">
    <el-radio-button value="default">Steve</el-radio-button>
    <el-radio-button value="slim">Alex</el-radio-button>
  </UiSegmented>
</template>
<script setup lang="ts">
import { ref, watch } from 'vue'
import UiSegmented from '@/components/ui/UiSegmented.vue'
const props = defineProps<{ model?: string; busy: boolean; disabled: boolean }>()
defineEmits<{ change: [value: string] }>()
const selection = ref<string | number | boolean | null | undefined>(props.model || 'default')
// Keep the pending choice visible; a completed or failed write restores the confirmed value.
watch([() => props.model, () => props.busy], () => {
  if (!props.busy) selection.value = props.model || 'default'
})
</script>
