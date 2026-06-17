<template>
  <el-dialog v-model="visible" title="设置封禁时长" class="dialog-form" align-center append-to-body>
    <div class="ban-dialog-body">
      <el-radio-group v-model="durationType" class="mb-4 capsule-radio">
        <el-radio-button value="preset">快速选择</el-radio-button>
        <el-radio-button value="custom">精确小时</el-radio-button>
      </el-radio-group>

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

      <div class="text-13 text-light p-3 bg-mute rounded-md">
        解封时间：<span class="font-bold text-primary">{{ untilLabel }}</span>
      </div>
    </div>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="danger" :loading="banning" @click="$emit('confirm')">确认封禁</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
const visible = defineModel<boolean>('visible', { required: true })
const durationType = defineModel<string>('durationType', { required: true })
const presetDuration = defineModel<number>('presetDuration', { required: true })
const customHours = defineModel<number>('customHours', { required: true })

defineProps<{
  presets: Array<{ label: string; value: number }>
  untilLabel: string
  banning: boolean
}>()

defineEmits<{
  confirm: []
}>()
</script>
