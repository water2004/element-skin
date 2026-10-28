<template>
  <UiDialog
    v-model="visible"
    title="新建角色"
    class="dialog-create-role"
    :busy="creating"
    :dirty="hasChanges"
    @closed="name = ''"
  >
    <el-form label-width="100px" :disabled="creating">
      <el-form-item label="角色名称">
        <div class="w-full">
          <el-input v-model="name" placeholder="请输入角色名称" maxlength="16" show-word-limit />
          <p class="mt-1 mb-0 text-xs leading-5 text-[var(--color-text-light)]">
            1–16 个字符，仅限英文字母、数字和下划线。
          </p>
        </div>
      </el-form-item>
    </el-form>
    <template #footer="{ requestClose }">
      <el-button :disabled="creating" @click="requestClose">取消</el-button>
      <UiButton hover-effect variant="primary" :icon="Check" :loading="creating" @click="submit">
        创建
      </UiButton>
    </template>
  </UiDialog>
</template>

<script setup lang="ts">
import UiButton from '@/components/ui/UiButton.vue'
import { Check } from '@element-plus/icons-vue'
import { computed, watch } from 'vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import { useDirtySnapshot } from '@/composables/useDirtySnapshot'

const visible = defineModel<boolean>('visible', { required: true })
const name = defineModel<string>('name', { required: true })
const props = defineProps<{ creating: boolean }>()
const { hasChanges, capture } = useDirtySnapshot(computed(() => name.value))
watch(
  visible,
  (open) => {
    if (open) capture()
  },
  { immediate: true },
)

const emit = defineEmits<{
  create: []
}>()

function submit() {
  if (props.creating) return
  emit('create')
}
</script>
