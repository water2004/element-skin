<template>
  <div class="ui-viewer-layout">
    <div class="ui-viewer-stage">
      <slot name="stage" />
    </div>
    <div class="ui-viewer-info">
      <slot />
    </div>
  </div>
</template>

<style scoped>
.ui-viewer-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  min-height: 560px;
}

.ui-viewer-stage {
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 560px;
  background: var(--color-background-hero-light);
  transition: background var(--motion-duration-base) ease;
}

html.dark .ui-viewer-stage {
  background: var(--color-background-hero-dark);
}

.ui-viewer-info {
  min-width: 0;
  background: var(--color-card-background);
  padding: var(--space-panel-padding);
  display: flex;
  flex-direction: column;
  min-height: 560px;
  border-left: 1px solid var(--color-border);
  position: relative;
}

@media (max-width: 900px) {
  .ui-viewer-layout {
    grid-template-columns: 1fr;
    min-height: auto;
  }

  .ui-viewer-stage {
    min-height: 340px;
  }

  .ui-viewer-info {
    min-height: auto;
    border-left: 0;
    border-top: 1px solid var(--color-border);
    padding: var(--space-content-padding);
  }
}
</style>
