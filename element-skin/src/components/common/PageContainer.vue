<template>
  <div class="page-container" :class="`page-container--${variant}`"><slot /></div>
</template>

<script setup lang="ts">
withDefaults(defineProps<{ variant?: 'form' | 'editor' | 'wide' | 'gallery' }>(), {
  variant: 'wide',
})
</script>

<style scoped>
.page-container {
  width: 100%;
  min-width: 0;
  margin-inline: auto;
  padding-block: var(--space-page-block);
}
.page-container--form {
  max-width: var(--page-width-form);
}
.page-container--editor {
  max-width: var(--page-width-editor);
}
.page-container--wide {
  max-width: var(--page-width-wide);
}
.page-container--gallery {
  max-width: none;
}

.page-container :deep(.el-form-item__content) {
  min-width: 0;
  overflow-wrap: anywhere;
}

.page-container :deep(.el-table),
.page-container :deep(.el-input-number) {
  max-width: 100%;
}
</style>
