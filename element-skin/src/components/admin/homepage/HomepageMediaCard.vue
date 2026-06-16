<template>
  <article
    class="homepage-media-card"
    :class="{
      'is-disabled': !item.enabled,
      'is-dirty': dirty,
      'is-dragging': dragging,
      'is-drag-over': dragOver,
    }"
    :data-homepage-media-id="item.id"
    draggable="true"
    @click="$emit('open')"
    @pointerdown="$emit('pressstart', $event)"
    @dragstart="$emit('dragstart', $event)"
    @dragenter.prevent="$emit('dragenter', $event)"
    @dragover.prevent="$emit('dragover', $event)"
    @drop.prevent="$emit('drop', $event)"
    @dragend="$emit('dragend', $event)"
  >
    <div class="media-base">
      <div class="media-actions" @click.stop @dragstart.stop @pointerdown.stop>
        <label class="state-control">
          <el-switch :model-value="item.enabled" @change="$emit('toggle-enabled')" />
          <span>{{ item.enabled ? '启用' : '停用' }}</span>
        </label>
        <el-button type="danger" :icon="Delete" circle plain @click="$emit('remove')" />
      </div>
    </div>

    <div class="media-cover">
      <img :src="previewUrl" :alt="item.title" class="media-image" draggable="false" />
      <el-tag
        class="media-kind"
        :type="item.type === 'panorama' ? 'warning' : 'success'"
        size="small"
        effect="dark"
      >
        {{ item.type === 'panorama' ? '全景图' : '静态图' }}
      </el-tag>
    </div>
  </article>
</template>

<script setup lang="ts">
import { Delete } from '@element-plus/icons-vue'
import type { HomepageMedia } from '@/api/types'

defineProps<{
  item: HomepageMedia
  previewUrl: string
  dirty: boolean
  dragging: boolean
  dragOver: boolean
}>()

defineEmits<{
  open: []
  'toggle-enabled': []
  remove: []
  pressstart: [event: PointerEvent]
  dragstart: [event: DragEvent]
  dragenter: [event: DragEvent]
  dragover: [event: DragEvent]
  drop: [event: DragEvent]
  dragend: [event: DragEvent]
}>()
</script>

<style scoped>
.homepage-media-card {
  position: relative;
  width: 240px;
  height: 222px;
  cursor: grab;
  user-select: none;
  -webkit-user-select: none;
  -webkit-touch-callout: none;
  touch-action: none;
  transition:
    opacity 0.2s ease,
    transform 0.2s ease,
    filter 0.2s ease;
}
.homepage-media-card:hover .media-cover,
.homepage-media-card.is-drag-over .media-cover {
  border-color: var(--el-color-primary-light-5);
  box-shadow: 0 10px 24px rgba(64, 158, 255, 0.12);
}
.homepage-media-card.is-dragging {
  cursor: grabbing;
  opacity: 0.36;
  transform: scale(0.96);
}
.homepage-media-card.is-disabled .media-cover {
  filter: grayscale(0.28);
}
.homepage-media-card.is-dirty .media-cover,
.homepage-media-card.is-dirty .media-base {
  border-color: var(--el-color-warning-light-5);
}
.media-base {
  position: absolute;
  inset: 14px 0 0 0;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-card-background);
  transition:
    border-color 0.2s ease,
    box-shadow 0.2s ease;
}
.media-cover,
.media-base {
  overflow: hidden;
}
.media-cover {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  z-index: 1;
  height: 150px;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-background-soft);
  transition:
    border-color 0.2s ease,
    box-shadow 0.2s ease;
}
.media-image {
  width: 100%;
  height: 100%;
  display: block;
  object-fit: cover;
}
.media-kind {
  position: absolute;
  top: 10px;
  left: 10px;
  box-shadow: 0 6px 14px rgba(0, 0, 0, 0.14);
}
.media-actions {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 72px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 18px 12px 12px;
  cursor: default;
}
.state-control {
  display: inline-flex;
  flex-direction: row;
  align-items: center;
  gap: 8px;
  color: var(--color-heading);
  font-size: 13px;
  font-weight: 600;
}

@media (max-width: 768px) {
  .homepage-media-card {
    width: 100%;
  }
  .media-cover {
    height: 180px;
  }
  .homepage-media-card {
    height: 252px;
  }
  .media-actions {
    height: 72px;
    padding: 18px 14px 12px;
  }
}
</style>
