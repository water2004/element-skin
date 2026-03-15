<template>
  <div class="skin-viewer-wrapper" :style="{ width: width + 'px', height: height + 'px' }">
    <!-- Static Image Mode -->
    <img 
      v-if="isStatic && snapshotUrl" 
      :src="snapshotUrl" 
      class="skin-snapshot" 
      :style="{ width: width + 'px', height: height + 'px' }" 
    />
    
    <!-- Interactive Canvas Mode: Only render if not static or snapshot not ready -->
    <div 
      v-if="!isStatic"
      ref="container" 
      class="skin-viewer-container"
      :style="{ width: width + 'px', height: height + 'px' }"
    ></div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import * as skinview3d from 'skinview3d'

const props = defineProps({
  skinUrl: { type: String, required: true },
  capeUrl: { type: String, default: null },
  model: { type: String, default: 'default' },
  width: { type: Number, default: 300 },
  height: { type: Number, default: 400 },
  isStatic: { type: Boolean, default: false }
})

const container = ref(null)
const snapshotUrl = ref(null)
let viewer = null

async function initViewer() {
  // Dispose existing viewer
  if (viewer) {
    viewer.dispose()
    viewer = null
  }

  const config = {
    width: props.width,
    height: props.height,
    skin: props.skinUrl,
    cape: props.capeUrl,
    model: props.model === 'slim' ? 'slim' : 'steve',
    preserveDrawingBuffer: props.isStatic
  }

  if (props.isStatic) {
    // 1. Create a temporary off-screen canvas for snapshot
    const tempCanvas = document.createElement('canvas')
    const staticViewer = new skinview3d.SkinViewer({
      canvas: tempCanvas,
      ...config
    })

    try {
      staticViewer.autoRotate = false
      staticViewer.animation = null
      staticViewer.camera.position.set(0, 10, 500)
      staticViewer.camera.lookAt(0, 15, 0)
      staticViewer.zoom = 0.8

      staticViewer.playerObject.skin.leftArm.rotation.z = 0.05
      staticViewer.playerObject.skin.rightArm.rotation.z = -0.05
      staticViewer.playerObject.skin.leftLeg.rotation.z = 0
      staticViewer.playerObject.skin.rightLeg.rotation.z = 0

      await staticViewer.loadSkin(props.skinUrl, { model: props.model === 'slim' ? 'slim' : 'steve' })
      if (props.capeUrl) await staticViewer.loadCape(props.capeUrl)
      
      staticViewer.render()
      snapshotUrl.value = tempCanvas.toDataURL('image/png')
    } catch (e) {
      console.error('SkinViewer static render error:', e)
    } finally {
      staticViewer.dispose()
    }
  } else {
    // 2. Interactive mode
    await nextTick() // Wait for container ref to be available via v-if
    if (!container.value) return
    
    const canvas = document.createElement('canvas')
    viewer = new skinview3d.SkinViewer({
      canvas: canvas,
      ...config
    })
    
    container.value.appendChild(viewer.canvas)
    
    viewer.autoRotate = true
    viewer.autoRotateSpeed = 0.5
    viewer.zoom = 0.8
    viewer.animation = new skinview3d.WalkingAnimation()
    viewer.animation.speed = 0.5
  }
}

onMounted(() => {
  initViewer()
})

onUnmounted(() => {
  if (viewer) {
    viewer.dispose()
    viewer = null
  }
})

watch(() => [props.skinUrl, props.model, props.isStatic, props.capeUrl], () => {
  initViewer()
}, { deep: true })
</script>

<style scoped>
.skin-viewer-wrapper {
  display: flex;
  justify-content: center;
  align-items: center;
  overflow: hidden;
  position: relative;
}

.skin-viewer-container {
  display: flex;
  justify-content: center;
  align-items: center;
}

.skin-snapshot {
  display: block;
  image-rendering: pixelated;
  object-fit: contain;
}
</style>
