<template>
  <div class="cape-viewer-wrapper" :style="{ width: width + 'px', height: height + 'px' }">
    <!-- Static Image Mode -->
    <img 
      v-if="isStatic && snapshotUrl" 
      :src="snapshotUrl" 
      class="cape-snapshot" 
      :style="{ width: width + 'px', height: height + 'px' }" 
    />

    <!-- Loading Placeholder -->
    <div v-if="isStatic && !snapshotUrl" class="cape-loader">
      <el-icon class="is-loading"><Loading /></el-icon>
    </div>
    
    <!-- Interactive Canvas Mode -->
    <div 
      v-if="!isStatic"
      ref="container" 
      class="cape-viewer-container"
      :style="{ width: width + 'px', height: height + 'px' }"
    ></div>
  </div>
</template>

<script>
// 全局渲染锁：披风渲染也排队
let globalCapeRenderLock = Promise.resolve();
</script>

<script setup>
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import * as skinview3d from 'skinview3d'
import { Loading } from '@element-plus/icons-vue'

const props = defineProps({
  capeUrl: { type: String, required: true },
  width: { type: Number, default: 200 },
  height: { type: Number, default: 280 },
  isStatic: { type: Boolean, default: false }
})

const container = ref(null)
const snapshotUrl = ref(null)
let viewer = null

async function initViewer() {
  if (viewer) {
    viewer.dispose()
    viewer = null
  }

  const config = {
    width: props.width,
    height: props.height,
    skin: null,
    cape: props.capeUrl,
    preserveDrawingBuffer: props.isStatic
  }

  if (props.isStatic) {
    globalCapeRenderLock = globalCapeRenderLock.then(async () => {
      if (snapshotUrl.value) return;

      const tempCanvas = document.createElement('canvas')
      let staticViewer = null;

      try {
        staticViewer = new skinview3d.SkinViewer({
          canvas: tempCanvas,
          ...config
        })

        if (staticViewer.playerObject) {
          staticViewer.playerObject.skin.visible = false
        }
        
        staticViewer.autoRotate = false
        staticViewer.camera.position.set(0, 10, -50)
        staticViewer.camera.lookAt(0, 15, 0)
        staticViewer.zoom = 1.3

        await staticViewer.loadCape(props.capeUrl)
        staticViewer.render()
        snapshotUrl.value = tempCanvas.toDataURL('image/png')
      } catch (e) {
        console.error('CapeViewer static render error:', e)
      } finally {
        if (staticViewer) {
          staticViewer.dispose()
          staticViewer = null
        }
      }
    });
    await globalCapeRenderLock;
  } else {
    await nextTick()
    if (!container.value) return
    container.value.innerHTML = ''
    
    const canvas = document.createElement('canvas')
    viewer = new skinview3d.SkinViewer({
      canvas: canvas,
      ...config
    })
    
    container.value.appendChild(viewer.canvas)
    if (viewer.playerObject) {
      viewer.playerObject.skin.visible = false
    }
    
    viewer.autoRotate = true
    viewer.autoRotateSpeed = 0.5
    viewer.zoom = 1.2
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

watch(() => [props.capeUrl, props.isStatic], () => {
  snapshotUrl.value = null
  initViewer()
}, { deep: true })
</script>

<style scoped>
.cape-viewer-wrapper {
  display: flex;
  justify-content: center;
  align-items: center;
  overflow: hidden;
  position: relative;
}

.cape-loader {
  font-size: 24px;
  color: var(--el-text-color-secondary);
  opacity: 0.5;
}

.cape-viewer-container {
  display: flex;
  justify-content: center;
  align-items: center;
}

.cape-snapshot {
  display: block;
  image-rendering: pixelated;
  object-fit: contain;
}
</style>
