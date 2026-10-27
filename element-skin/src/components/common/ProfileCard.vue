<template>
  <PreviewCard
    :class="admin ? 'admin-role-card' : 'role-card'"
    :title="profile.name"
    :delay-index="delayIndex"
    @preview="$emit('preview', profile)"
  >
    <template #preview
      ><ProfilePreviewStage
        :profile="profile"
        :textures-url="texturesUrl"
        :width="200"
        :height="280"
        is-static
      />
      <div v-if="official" class="absolute right-2 top-2 flex">
        <OfficialBindingTag />
      </div>
    </template>
    <template v-if="admin" #info><ViewerIdentity :name="profile.owner_display_name" /></template>
    <template v-if="$slots.actions" #actions><slot name="actions" /></template>
  </PreviewCard>
</template>
<script setup lang="ts">
import type { Profile } from '@/api/types'
import PreviewCard from '@/components/common/PreviewCard.vue'
import ProfilePreviewStage from '@/components/common/ProfilePreviewStage.vue'
import ViewerIdentity from '@/components/common/ViewerIdentity.vue'
import OfficialBindingTag from '@/components/common/OfficialBindingTag.vue'
defineProps<{
  profile: Profile
  delayIndex: number
  admin?: boolean
  official?: boolean
  texturesUrl: (hash: string | null | undefined) => string
}>()
defineEmits<{ preview: [profile: Profile] }>()
</script>
