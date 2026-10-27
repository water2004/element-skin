<template>
  <UiDialog
    v-model="visible"
    destroy-on-close
    variant="viewer"
    :busy="busy"
    :dirty="dirty"
    title="管理角色详情"
  >
    <UiViewerLayout v-if="profile">
      <template #stage
        ><ProfilePreviewStage :profile="profile" :textures-url="texturesUrl"
      /></template>
      <ViewerNameEditor
        :key="profile.id + String(revision)"
        :value="profile.name"
        :can-edit="canUpdate"
        :busy="busy"
        :save="saveName"
        :allow-empty="false"
        :max-length="16"
        label="角色名称"
        placeholder="角色名称"
        @dirty="dirty = $event"
      />
      <section class="flex flex-col items-start gap-4 border-b border-[var(--color-border)] py-5">
        <div class="flex flex-wrap items-center gap-3">
          <span class="text-sm text-[var(--color-text-light)]">所有者</span
          ><ViewerIdentity :name="profile.owner_display_name" />
        </div>
        <ProfileIdentifiers :profile="profile" />
      </section>
      <ActionBar align="start" class="py-5">
        <UiButton
          v-if="canUpdate && profile.skin_hash"
          variant="secondary"
          tone="warning"
          :disabled="blocked"
          @click="$emit('clear-skin')"
          >清除皮肤</UiButton
        >
        <UiButton
          v-if="canUpdate && profile.cape_hash"
          variant="secondary"
          tone="warning"
          :disabled="blocked"
          @click="$emit('clear-cape')"
          >清除披风</UiButton
        >
      </ActionBar>
      <div v-if="canDelete" class="pb-5">
        <UiButton
          v-if="canDelete"
          variant="secondary"
          tone="danger"
          :disabled="blocked"
          @click="$emit('delete')"
          >删除角色</UiButton
        >
      </div>
    </UiViewerLayout>
  </UiDialog>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import type { Profile } from '@/api/types'
import ProfilePreviewStage from '@/components/common/ProfilePreviewStage.vue'
import ProfileIdentifiers from '@/components/common/ProfileIdentifiers.vue'
import ViewerIdentity from '@/components/common/ViewerIdentity.vue'
import ViewerNameEditor from '@/components/common/ViewerNameEditor.vue'
import ActionBar from '@/components/common/ActionBar.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiViewerLayout from '@/components/ui/UiViewerLayout.vue'
const visible = defineModel<boolean>('visible', { required: true })
const dirty = defineModel<boolean>('dirty', { default: false })
const props = defineProps<{
  profile: Profile | null
  texturesUrl: (hash: string | null | undefined) => string
  busy: boolean
  revision: number
  saveName: (value: string) => Promise<boolean>
  canUpdate: boolean
  canDelete: boolean
}>()
const blocked = computed(() => props.busy || dirty.value)
defineEmits<{ 'clear-skin': []; 'clear-cape': []; delete: [] }>()
</script>
