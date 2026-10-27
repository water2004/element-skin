<template>
  <div class="flex flex-col items-start gap-3">
    <IdentifierValue label="角色 UUID" :value="profile.id" />
    <IdentifierValue v-if="profile.skin_hash" label="皮肤 Hash" :value="profile.skin_hash" />
    <IdentifierValue v-if="profile.cape_hash" label="披风 Hash" :value="profile.cape_hash" />
  </div>
</template>
<script setup lang="ts">
import type { Profile } from '@/api/types'
import IdentifierValue from '@/components/common/IdentifierValue.vue'
defineProps<{ profile: Profile }>()
</script>
