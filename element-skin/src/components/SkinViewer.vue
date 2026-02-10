<template>
  <div ref="container" class="skin-viewer-container"></div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import * as skinview3d from 'skinview3d'

const props = defineProps({
  skinUrl: {
    type: String,
    required: true
  },
  capeUrl: {
    type: String,
    default: null
  },
  width: {
    type: Number,
    default: 300
  },
  height: {
    type: Number,
    default: 400
  }
})

const container = ref(null)
let viewer = null

onMounted(() => {
  if (container.value && props.skinUrl) {
    viewer = new skinview3d.SkinViewer({
      canvas: createCanvas(),
      width: props.width,
      height: props.height,
      skin: props.skinUrl,
      cape: props.capeUrl,
    })

    container.value.appendChild(viewer.canvas)

    // 自动旋转
    viewer.autoRotate = true
    viewer.autoRotateSpeed = 0.5

    // 设置缩放
    viewer.zoom = 0.8

    // 设置动画
    viewer.animation = new skinview3d.WalkingAnimation()
    viewer.animation.speed = 0.5
  }
})

onUnmounted(() => {
  if (viewer) {
    viewer.dispose()
  }
})

watch(() => props.skinUrl, (newUrl) => {
  if (viewer && newUrl) {
    viewer.loadSkin(newUrl)
  }
})

watch(() => props.capeUrl, (newUrl) => {
  if (viewer) {
    if (newUrl) {
      viewer.loadCape(newUrl)
    } else {
      viewer.resetCape()
    }
  }
})

function createCanvas() {
  const canvas = document.createElement('canvas')
  return canvas
}
</script>

<style scoped>
.skin-viewer-container {
  display: flex;
  justify-content: center;
  align-items: center;
  /* background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); */
  border-radius: 8px;
  overflow: hidden;
}

.skin-viewer-container canvas {
  display: block;
}
</style>
