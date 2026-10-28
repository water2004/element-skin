<template>
  <div class="min-w-0 pr-12">
    <div v-if="!editing" class="flex min-w-0 items-start gap-2">
      <UiButton
        v-if="canEdit"
        variant="tertiary"
        icon-only
        :icon="Edit"
        :aria-label="`修改${label}`"
        :disabled="busy"
        @click="start"
      />
      <h2 class="m-0 min-w-0 break-all text-2xl font-bold text-[var(--color-heading)]">
        <button
          v-if="canEdit"
          type="button"
          class="cursor-pointer text-left"
          :disabled="busy"
          :aria-label="`编辑${label}：${value || placeholder}`"
          @click="start"
        >
          {{ value || placeholder }}
        </button>
        <template v-else>{{ value || placeholder }}</template>
      </h2>
    </div>
    <form v-else @submit.prevent="submit" @keydown="onKeydown">
      <el-input
        ref="input"
        v-model="draft"
        :aria-label="label"
        :placeholder="placeholder"
        :maxlength="maxLength"
        :disabled="busy || saving"
      />
      <ActionBar align="start" class="mt-2">
        <UiButton
          variant="primary"
          hover-effect
          size="small"
          :disabled="!dirty || busy || saving || !canEdit"
          :loading="saving"
          @click="submit"
          >保存名称</UiButton
        >
        <UiButton variant="secondary" size="small" :disabled="busy || saving" @click="cancel"
          >取消</UiButton
        >
      </ActionBar>
      <p v-if="error" role="alert" class="mt-2 text-sm text-[var(--el-color-danger)]">
        {{ error }}
      </p>
    </form>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { Edit } from '@element-plus/icons-vue'
import UiButton from '@/components/ui/UiButton.vue'
import ActionBar from '@/components/common/ActionBar.vue'

const props = withDefaults(
  defineProps<{
    value: string
    canEdit: boolean
    busy: boolean
    save: (value: string) => Promise<boolean>
    label?: string
    placeholder?: string
    allowEmpty?: boolean
    maxLength?: number
  }>(),
  { label: '名称', placeholder: '未命名材质', allowEmpty: true },
)
const emit = defineEmits<{ dirty: [value: boolean] }>()
const editing = ref(false)
const saving = ref(false)
const draft = ref(props.value)
const error = ref('')
const input = ref<{ focus: () => void } | null>(null)
const dirty = computed(() => editing.value && draft.value.trim() !== props.value)
watch(dirty, (value) => emit('dirty', value), { flush: 'sync' })
watch(
  () => props.value,
  (value) => {
    if (!editing.value) draft.value = value
  },
)
async function start() {
  if (!props.canEdit || props.busy) return
  draft.value = props.value
  error.value = ''
  editing.value = true
  await nextTick()
  input.value?.focus()
}
function cancel() {
  if (props.busy || saving.value) return
  draft.value = props.value
  editing.value = false
  error.value = ''
}
function onKeydown(event: KeyboardEvent) {
  if (event.isComposing || event.keyCode === 229) {
    if (event.key === 'Escape' || event.key === 'Enter') event.stopPropagation()
    return
  }
  if (event.key === 'Escape') {
    event.preventDefault()
    event.stopPropagation()
    cancel()
  } else if (event.key === 'Enter') {
    event.preventDefault()
    void submit()
  }
}
async function submit() {
  if (!props.canEdit || props.busy || saving.value || !dirty.value) return
  const value = draft.value.trim()
  if (!props.allowEmpty && !value) {
    error.value = '名称不能为空'
    return
  }
  saving.value = true
  error.value = ''
  try {
    if (await props.save(value)) editing.value = false
    else error.value = '名称未保存，请核对后重试或取消'
  } finally {
    saving.value = false
  }
}
</script>
