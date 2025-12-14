<template>
  <div ref="container" class="cape-viewer-container"></div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import * as skinview3d from 'skinview3d'

const props = defineProps({
  capeUrl: {
    type: String,
    required: true
  },
  width: {
    type: Number,
    default: 200
  },
  height: {
    type: Number,
    default: 280
  }
})

const container = ref(null)
let viewer = null

onMounted(() => {
  if (container.value && props.capeUrl) {
    viewer = new skinview3d.SkinViewer({
      canvas: createCanvas(),
      width: props.width,
      height: props.height,
      skin: null,  // 不显示皮肤
      cape: props.capeUrl,
    })

    container.value.appendChild(viewer.canvas)

    // 隐藏角色模型，只显示披风
    if (viewer.playerObject) {
      viewer.playerObject.skin.visible = false
    }

    // 自动旋转，不加动画
    viewer.autoRotate = true
    viewer.autoRotateSpeed = 0.5
    viewer.zoom = 1.2
  }
})

onUnmounted(() => {
  if (viewer) {
    viewer.dispose()
  }
})

watch(() => props.capeUrl, (newUrl) => {
  if (viewer && newUrl) {
    viewer.loadCape(newUrl)
  }
})

function createCanvas() {
  const canvas = document.createElement('canvas')
  return canvas
}
</script>

<style scoped>
.cape-viewer-container {
  display: flex;
  justify-content: center;
  align-items: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border-radius: 8px;
  overflow: hidden;
}

.cape-viewer-container canvas {
  display: block;
}
</style>
