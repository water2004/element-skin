<template>
  <UiDialog
    v-model="visible"
    title="设置封禁时长"
    align-center
    :busy="banning"
    :dirty="hasChanges"
    @closed="resetForm"
  >
    <el-form :disabled="banning" class="ban-dialog-body" @submit.prevent>
      <UiSegmented v-model="durationType" class="mb-4">
        <el-radio-button value="preset">快速选择</el-radio-button>
        <el-radio-button value="custom">精确小时</el-radio-button>
      </UiSegmented>

      <div v-if="durationType === 'preset'" class="preset-durations mb-4">
        <el-button
          v-for="preset in presets"
          :key="preset.value"
          :type="presetDuration === preset.value ? 'primary' : ''"
          size="small"
          @click="presetDuration = preset.value"
        >
          {{ preset.label }}
        </el-button>
      </div>

      <div v-else class="custom-duration mb-4">
        <el-input-number v-model="customHours" :min="1" :max="8760" class="w-full" />
      </div>

      <div
        class="text-[13px] text-[var(--color-text-light)] p-3 bg-[var(--color-background-mute)] rounded-md"
      >
        解封时间：<span class="font-bold text-[var(--el-color-primary)]">{{ untilLabel }}</span>
      </div>

      <div class="mt-4">
        <div class="mb-2 text-sm font-medium text-[var(--color-text)]">封禁原因</div>
        <el-input
          v-model="reason"
          type="textarea"
          :rows="4"
          maxlength="500"
          show-word-limit
          placeholder="原因会作为站内通知发送给用户"
        />
      </div>
    </el-form>
    <template #footer="{ requestClose }">
      <el-button :disabled="banning" @click="requestClose">取消</el-button>
      <el-button type="danger" :loading="banning" @click="$emit('confirm')">确认封禁</el-button>
    </template>
  </UiDialog>
</template>

<script setup lang="ts">
import UiDialog from '@/components/ui/UiDialog.vue'
import UiSegmented from '@/components/ui/UiSegmented.vue'
import { computed, watch } from 'vue'
import { useDirtySnapshot } from '@/composables/useDirtySnapshot'

const visible = defineModel<boolean>('visible', { required: true })
const durationType = defineModel<string>('durationType', { required: true })
const presetDuration = defineModel<number>('presetDuration', { required: true })
const customHours = defineModel<number>('customHours', { required: true })
const reason = defineModel<string>('reason', { required: true })
const { hasChanges, capture } = useDirtySnapshot(
  computed(() => ({
    durationType: durationType.value,
    presetDuration: presetDuration.value,
    customHours: customHours.value,
    reason: reason.value,
  })),
)
watch(
  visible,
  (open) => {
    if (open) capture()
  },
  { immediate: true },
)

function resetForm() {
  durationType.value = 'preset'
  presetDuration.value = 24
  customHours.value = 24
  reason.value = ''
}

defineProps<{
  presets: Array<{ label: string; value: number }>
  untilLabel: string
  banning: boolean
}>()

defineEmits<{
  confirm: []
}>()
</script>
