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
        <el-input v-model="name" placeholder="请输入角色名称" maxlength="32" show-word-limit />
      </el-form-item>
    </el-form>
    <template #footer="{ requestClose }">
      <el-button :disabled="creating" @click="requestClose">取消</el-button>
      <el-button type="primary" :loading="creating" @click="submit">
        <el-icon><Check /></el-icon>
        创建
      </el-button>
    </template>
  </UiDialog>
</template>

<script setup lang="ts">
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
