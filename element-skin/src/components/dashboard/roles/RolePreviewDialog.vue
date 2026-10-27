<template>
  <UiDialog
    v-model="visible"
    destroy-on-close
    variant="viewer"
    :busy="busy"
    :dirty="dirty"
    title="角色详情"
  >
    <UiViewerLayout v-if="profile">
      <template #stage
        ><ProfilePreviewStage :profile="profile" :textures-url="texturesUrl"
      /></template>
      <ViewerNameEditor
        :key="profile.id + String(revision)"
        :value="profile.name"
        :can-edit="canRename"
        :busy="busy"
        :save="saveName"
        :allow-empty="false"
        :max-length="16"
        label="角色名称"
        placeholder="角色名称"
        @dirty="dirty = $event"
      />
      <section class="flex flex-col items-start gap-4 border-b border-[var(--color-border)] py-5">
        <TextureModelTag :model="profile.model" />
        <ProfileIdentifiers :profile="profile" />
      </section>
      <section
        v-if="officialState !== 'ready' || officialBinding"
        aria-label="正版绑定"
        class="flex flex-col items-start gap-4 border-b border-[var(--color-border)] py-5"
      >
        <h3 class="m-0 text-sm text-[var(--color-text-light)]">正版绑定</h3>
        <div
          v-if="officialState !== 'ready'"
          class="text-sm text-[var(--color-text-light)]"
          role="status"
        >
          {{
            officialState === 'loading'
              ? '正版绑定加载中'
              : officialState === 'error'
                ? '正版绑定加载失败'
                : '无权读取正版绑定'
          }}
        </div>
        <template v-else-if="officialBinding">
          <div class="flex w-full min-w-0 flex-wrap items-center gap-x-3 gap-y-2">
            <ViewerIdentity :name="officialBinding.remote_name" />
            <span class="min-w-0 break-all text-sm text-[var(--color-text-light)]">{{
              officialBinding.identity.label || officialBinding.identity.provider_name
            }}</span>
            <OfficialBindingTag class="shrink-0" />
          </div>
          <IdentifierValue label="正版 UUID" :value="officialBinding.remote_uuid" />
          <ActionBar v-if="canSyncOfficialBinding || canDeleteOfficialBinding" align="start">
            <UiButton
              v-if="canSyncOfficialBinding"
              variant="secondary"
              :disabled="blocked"
              @click="$emit('sync-official', officialBinding)"
              >同步正版数据</UiButton
            >
            <UiButton
              v-if="canDeleteOfficialBinding"
              variant="secondary"
              tone="warning"
              :disabled="blocked"
              @click="$emit('unbind-official', officialBinding)"
              >解除绑定</UiButton
            >
          </ActionBar>
        </template>
      </section>
      <section
        v-if="hasCommonActions || canDelete"
        aria-label="通用操作"
        class="flex flex-col items-start gap-5 py-5"
      >
        <ActionBar v-if="hasCommonActions" align="start">
          <UiButton
            v-if="canSetAvatar && profile.skin_hash"
            variant="secondary"
            :disabled="blocked"
            @click="$emit('set-avatar', profile)"
            >用作头像</UiButton
          >
          <UiButton
            v-if="canClearTexture && profile.skin_hash"
            variant="secondary"
            tone="warning"
            :disabled="blocked"
            @click="$emit('clear-skin', profile.id)"
            >清除皮肤</UiButton
          >
          <UiButton
            v-if="canClearTexture && profile.cape_hash"
            variant="secondary"
            tone="warning"
            :disabled="blocked"
            @click="$emit('clear-cape', profile.id)"
            >清除披风</UiButton
          >
        </ActionBar>
        <div v-if="canDelete">
          <UiButton
            variant="secondary"
            tone="danger"
            :disabled="blocked"
            @click="$emit('delete', profile.id)"
            >删除角色</UiButton
          >
        </div>
      </section>
    </UiViewerLayout>
  </UiDialog>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import type { OfficialProfileBinding, Profile } from '@/api/types'
import OfficialBindingTag from '@/components/common/OfficialBindingTag.vue'
import ProfilePreviewStage from '@/components/common/ProfilePreviewStage.vue'
import ProfileIdentifiers from '@/components/common/ProfileIdentifiers.vue'
import ViewerIdentity from '@/components/common/ViewerIdentity.vue'
import ViewerNameEditor from '@/components/common/ViewerNameEditor.vue'
import IdentifierValue from '@/components/common/IdentifierValue.vue'
import TextureModelTag from '@/components/textures/TextureModelTag.vue'
import ActionBar from '@/components/common/ActionBar.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiViewerLayout from '@/components/ui/UiViewerLayout.vue'
const visible = defineModel<boolean>('visible', { required: true })
const dirty = defineModel<boolean>('dirty', { default: false })
const props = defineProps<{
  profile: Profile | null
  texturesUrl: (hash: string | null | undefined) => string
  officialBinding?: OfficialProfileBinding | null
  officialState: 'ready' | 'loading' | 'error' | 'forbidden'
  busy: boolean
  revision: number
  saveName: (value: string) => Promise<boolean>
  canRename: boolean
  canSetAvatar: boolean
  canClearTexture: boolean
  canDelete: boolean
  canSyncOfficialBinding: boolean
  canDeleteOfficialBinding: boolean
}>()
const blocked = computed(() => props.busy || dirty.value)
const hasCommonActions = computed(
  () =>
    !!props.profile &&
    ((props.canSetAvatar && !!props.profile.skin_hash) ||
      (props.canClearTexture && !!(props.profile.skin_hash || props.profile.cape_hash))),
)
defineEmits<{
  'set-avatar': [profile: Profile]
  'clear-skin': [profileId: string]
  'clear-cape': [profileId: string]
  'sync-official': [binding: OfficialProfileBinding]
  'unbind-official': [binding: OfficialProfileBinding]
  delete: [profileId: string]
}>()
</script>
