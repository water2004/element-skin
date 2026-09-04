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

    <!-- Toggle Button for Cape/Elytra -->
    <div v-if="!isStatic" class="equipment-toggle">
      <el-radio-group v-model="backEquipment" size="large">
        <el-radio-button value="cape">披风</el-radio-button>
        <el-radio-button value="elytra">鞘翅</el-radio-button>
      </el-radio-group>
    </div>
  </div>
</template>

<script lang="ts">
// 全局渲染锁：披风渲染也排队
let globalCapeRenderLock: Promise<void> = Promise.resolve()
</script>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import * as skinview3d from 'skinview3d'
import { Loading } from '@element-plus/icons-vue'
import {
  canvasToBlob,
  capeSnapshotCacheKey,
  getCachedImageUrl,
  setCachedImageUrl,
} from '@/storage/renderCache'

const props = withDefaults(
  defineProps<{
    capeUrl: string
    width?: number
    height?: number
    isStatic?: boolean
  }>(),
  { width: 200, height: 280, isStatic: false },
)

const container = ref<HTMLDivElement | null>(null)
const snapshotUrl = ref<string | null>(null)
let viewer: skinview3d.SkinViewer | null = null
let generation = 0

const emit = defineEmits<{
  error: [reason: unknown]
}>()

const backEquipment = ref<'cape' | 'elytra'>('cape')

async function initViewer() {
  const gen = ++generation
  if (viewer) {
    viewer.dispose()
    viewer = null
  }

  const config: skinview3d.SkinViewerOptions = {
    width: props.width,
    height: props.height,
    preserveDrawingBuffer: props.isStatic,
  }

  if (props.isStatic) {
    globalCapeRenderLock = globalCapeRenderLock.then(async () => {
      if (gen !== generation) return

      const cacheKey = capeSnapshotCacheKey({
        capeUrl: props.capeUrl,
        width: props.width,
        height: props.height,
      })

      if (snapshotUrl.value) return
      const cachedUrl = await getCachedImageUrl('viewer-snapshot', cacheKey)
      if (gen !== generation) return
      if (cachedUrl) {
        snapshotUrl.value = cachedUrl
        return
      }

      const tempCanvas = document.createElement('canvas')
      let staticViewer: skinview3d.SkinViewer | null = null

      try {
        staticViewer = new skinview3d.SkinViewer({
          canvas: tempCanvas,
          ...config,
        })

        if (staticViewer.playerObject) {
          staticViewer.playerObject.skin.visible = false
        }

        staticViewer.autoRotate = false
        staticViewer.camera.position.set(0, 10, -50)
        staticViewer.camera.lookAt(0, 15, 0)
        staticViewer.zoom = 1.3

        await staticViewer.loadCape(props.capeUrl, { backEquipment: 'cape' })
        if (gen !== generation) return
        staticViewer.render()
        const blob = await canvasToBlob(tempCanvas)
        if (gen !== generation) return
        const storedUrl = blob ? await setCachedImageUrl('viewer-snapshot', cacheKey, blob) : null
        if (gen !== generation) return
        snapshotUrl.value = storedUrl ?? tempCanvas.toDataURL('image/png')
      } catch (e) {
        console.error('CapeViewer static render error:', e)
      } finally {
        if (staticViewer) {
          staticViewer.dispose()
          staticViewer = null
        }
      }
    })
    await globalCapeRenderLock
  } else {
    await nextTick()
    if (gen !== generation || !container.value) return
    container.value.innerHTML = ''

    const canvas = document.createElement('canvas')
    const localViewer = new skinview3d.SkinViewer({
      canvas,
      width: props.width,
      height: props.height,
    })

    try {
      await localViewer.loadCape(props.capeUrl, { makeVisible: false })
      if (gen !== generation) {
        localViewer.dispose()
        return
      }
      localViewer.playerObject.backEquipment = backEquipment.value
    } catch (e) {
      localViewer.dispose()
      if (gen === generation) emit('error', e)
      return
    }

    if (gen !== generation || !container.value) {
      localViewer.dispose()
      return
    }

    viewer = localViewer
    container.value.appendChild(localViewer.canvas)
    if (viewer.playerObject) {
      viewer.playerObject.skin.visible = false
    }

    viewer.autoRotate = true
    viewer.autoRotateSpeed = 0.5
    viewer.zoom = 1
    viewer.playerWrapper.position.y = 4
  }
}

onMounted(() => {
  initViewer()
})

onUnmounted(() => {
  generation++
  if (viewer) {
    viewer.dispose()
    viewer = null
  }
})

watch(
  () => [props.capeUrl, props.isStatic],
  () => {
    snapshotUrl.value = null
    initViewer()
  },
  { deep: true },
)

watch(backEquipment, (value) => {
  if (viewer && !props.isStatic) {
    viewer.playerObject.backEquipment = value
  }
})
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

.equipment-toggle {
  position: absolute;
  bottom: 10px;
  z-index: 10;
  display: flex;
  justify-content: center;
  width: 100%;
}
</style>
