<template>
  <article
    class="preview-card animate-card-slide clickable-card"
    :style="{ '--delay-index': delayIndex }"
    @click.self="$emit('preview')"
  >
    <div
      role="button"
      tabindex="0"
      :aria-label="`查看${title}`"
      @click="$emit('preview')"
      @keydown.enter.self.prevent="$emit('preview')"
      @keydown.space.self.prevent="$emit('preview')"
    >
      <div
        class="flex h-[280px] w-full items-center justify-center overflow-hidden bg-[image:var(--color-background-hero-light)] dark:bg-[image:var(--color-background-hero-dark)]"
      >
        <slot name="preview" />
      </div>
      <div
        class="flex min-w-0 flex-col items-center gap-2 bg-[var(--color-card-background)] p-4 text-center"
      >
        <div
          class="w-full truncate text-base font-semibold text-[var(--color-heading)]"
          :title="title"
        >
          {{ title }}
        </div>
        <slot name="info" />
      </div>
    </div>
    <CardActions v-if="$slots.actions"><slot name="actions" /></CardActions>
  </article>
</template>
<script setup lang="ts">
import CardActions from '@/components/common/CardActions.vue'
withDefaults(defineProps<{ title: string; delayIndex?: number }>(), { delayIndex: 0 })
defineEmits<{ preview: [] }>()
</script>
<style scoped>
.preview-card {
  cursor: pointer;
  border: 1px solid var(--color-border);
  border-radius: 16px;
  background: var(--color-card-background);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
  overflow: hidden;
  transition:
    background-color var(--motion-duration-base) ease,
    border-color var(--motion-duration-base) ease,
    transform var(--motion-duration-base) ease,
    box-shadow var(--motion-duration-base) ease;
}
.preview-card:hover {
  transform: translateY(calc(-1 * var(--motion-offset-hover)));
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
}
</style>
