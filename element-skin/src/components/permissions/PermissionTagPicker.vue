<template>
  <div class="space-y-3">
    <div v-if="groups.length" class="flex flex-wrap gap-2">
      <PermissionToneTag
        v-for="group in groups"
        :key="group.resource"
        :label="group.resourceDescription"
        :tone="group.tone"
        :count="group.items.length"
        :active="selectedResource === group.resource"
        variant="category"
        clickable
        @click="selectedResource = group.resource"
      />
    </div>
    <div class="rounded-lg bg-[var(--color-background-soft)] px-3 py-3">
      <div v-if="selectedGroup" class="space-y-2">
        <div class="flex items-center gap-2">
          <span class="text-sm font-semibold text-[var(--color-heading)]">
            {{ selectedGroup.resourceDescription }}
          </span>
          <span class="text-xs text-[var(--color-text-light)]"
            >{{ selectedGroup.items.length }} 项</span
          >
        </div>
        <div class="flex flex-wrap gap-2">
          <PermissionToneTag
            v-for="item in selectedGroup.items"
            :key="item.code"
            :label="item.label"
            :title="item.code"
            :tone="selectedGroup.tone"
            :badge-label="selectedSet.has(item.code) ? '已选' : ''"
            :clickable="!disabled"
            @click="toggle(item.code)"
          />
        </div>
      </div>
      <el-text v-else type="info" size="small">暂无可选权限</el-text>
    </div>
    <div class="space-y-2">
      <div class="text-sm font-semibold text-[var(--color-heading)]">已选择</div>
      <div v-if="selectedItems.length" class="flex flex-wrap gap-2">
        <PermissionToneTag
          v-for="item in selectedItems"
          :key="item.code"
          :label="item.label"
          :title="item.code"
          :tone="toneFor(item.resource)"
          :removable="!disabled"
          @remove="remove(item.code)"
        />
      </div>
      <el-text v-else type="info" size="small">还没有选择权限</el-text>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { PermissionDefinition } from '@/api/types'
import PermissionToneTag from '@/components/admin/users/PermissionToneTag.vue'
import {
  createPermissionDisplayItem,
  groupPermissionItems,
  normalizeSelectedResource,
  permissionTone,
  selectedPermissionGroup,
} from '@/components/permissions/permissionDisplay'

const props = defineProps<{
  modelValue: string[]
  permissions: PermissionDefinition[]
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string[]]
}>()

const selectedResource = ref('')

const permissionByCode = computed(() => {
  const out = new Map<string, PermissionDefinition>()
  for (const item of props.permissions) out.set(item.code, item)
  return out
})

const groups = computed(() =>
  groupPermissionItems(
    props.permissions.map((item) => createPermissionDisplayItem(item.code, item)),
  ),
)
const selectedGroup = computed(() => selectedPermissionGroup(groups.value, selectedResource.value))
const selectedSet = computed(() => new Set(props.modelValue))
const selectedItems = computed(() =>
  props.modelValue
    .map((code) => createPermissionDisplayItem(code, permissionByCode.value.get(code)))
    .sort(
      (a, b) =>
        a.resourceDescription.localeCompare(b.resourceDescription) || a.code.localeCompare(b.code),
    ),
)

watch(
  groups,
  (next) => {
    selectedResource.value = normalizeSelectedResource(selectedResource.value, next)
  },
  { immediate: true },
)

function toggle(code: string) {
  if (props.disabled) return
  if (selectedSet.value.has(code)) remove(code)
  else emit('update:modelValue', [...props.modelValue, code].sort())
}

function remove(code: string) {
  if (props.disabled) return
  emit(
    'update:modelValue',
    props.modelValue.filter((item) => item !== code),
  )
}

function toneFor(resource: string) {
  return permissionTone(resource)
}
</script>
