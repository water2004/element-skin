<template>
  <el-table :data="profiles || []" size="small" max-height="320">
    <el-table-column prop="name" label="角色名称" />
    <el-table-column prop="model" label="模型" width="100">
      <template #default="{ row }">
        <el-tag size="small" :type="row.model === 'slim' ? 'success' : 'info'">
          {{ row.model }}
        </el-tag>
      </template>
    </el-table-column>
    <el-table-column prop="id" label="角色 UUID" min-width="260" />
  </el-table>
  <el-empty v-if="!profiles?.length" description="该用户暂无角色" :image-size="60" />
  <div class="pagination-container mt-4">
    <CursorPager
      v-if="profiles.length > 0"
      :count="profiles.length"
      :loading="loading"
      :disabled-prev="disabledPrev"
      :disabled-next="disabledNext"
      @prev="$emit('prev')"
      @next="$emit('next')"
    />
  </div>
</template>

<script setup lang="ts">
import CursorPager from '@/components/common/CursorPager.vue'
import type { Profile } from '@/api/types'

defineProps<{
  profiles: Profile[]
  loading: boolean
  disabledPrev: boolean
  disabledNext: boolean
}>()

defineEmits<{
  prev: []
  next: []
}>()
</script>
