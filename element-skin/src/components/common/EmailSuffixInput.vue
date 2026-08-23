<template>
  <el-input
    v-if="policy.mode === 'allowlist'"
    v-model="localPart"
    :maxlength="localPartMaxLength"
    :placeholder="placeholder"
    :prefix-icon="Message"
    @input="emitAllowlistEmail"
    @keyup.enter="emit('enter')"
  >
    <template #append>
      <el-select
        v-model="selectedSuffix"
        class="w-[170px]"
        filterable
        placeholder="选择邮箱后缀"
        @change="emitAllowlistEmail"
      >
        <el-option
          v-for="suffix in policy.suffixes"
          :key="suffix"
          :label="suffix"
          :value="suffix"
        />
      </el-select>
    </template>
  </el-input>
  <el-input
    v-else
    :model-value="modelValue"
    :maxlength="254"
    :placeholder="placeholder"
    :prefix-icon="Message"
    @update:model-value="emit('update:modelValue', String($event))"
    @keyup.enter="emit('enter')"
  />
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Message } from '@element-plus/icons-vue'
import type { PublicEmailSuffixPolicy } from '@/api/types'

const props = withDefaults(
  defineProps<{
    modelValue: string
    policy: PublicEmailSuffixPolicy
    placeholder?: string
  }>(),
  { placeholder: '请输入邮箱地址' },
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
  enter: []
}>()

const localPart = ref('')
const selectedSuffix = ref('')
const localPartMaxLength = computed(() => Math.max(1, 254 - selectedSuffix.value.length))

function syncAllowlistInput() {
  if (props.policy.mode !== 'allowlist') return
  const normalized = props.modelValue.trim().toLowerCase()
  const matchedSuffix = props.policy.suffixes.find((suffix) => normalized.endsWith(suffix))
  selectedSuffix.value = matchedSuffix || props.policy.suffixes[0] || ''
  if (matchedSuffix) {
    localPart.value = props.modelValue.slice(0, props.modelValue.length - matchedSuffix.length)
    return
  }
  const at = props.modelValue.indexOf('@')
  localPart.value = at >= 0 ? props.modelValue.slice(0, at) : props.modelValue
  emitAllowlistEmail()
}

function emitAllowlistEmail() {
  emit(
    'update:modelValue',
    localPart.value && selectedSuffix.value ? `${localPart.value}${selectedSuffix.value}` : '',
  )
}

watch(() => [props.policy.mode, props.policy.suffixes.join('\n')], syncAllowlistInput, {
  immediate: true,
})

watch(
  () => props.modelValue,
  (value) => {
    if (props.policy.mode !== 'allowlist') return
    const assembled = `${localPart.value}${selectedSuffix.value}`
    if (value !== assembled) syncAllowlistInput()
  },
)
</script>
