<template>
  <div class="skin-viewer-wrapper" :style="{ width: width + 'px', height: height + 'px' }">
    <!-- 静态模式：显示快照 -->
    <img 
      v-if="isStatic && snapshotUrl" 
      :src="snapshotUrl" 
      class="skin-snapshot" 
      :style="{ width: width + 'px', height: height + 'px' }" 
    />
    
    <!-- 加载中占位 (可选) -->
    <div v-if="isStatic && !snapshotUrl" class="skin-loader">
      <el-icon class="is-loading"><Loading /></el-icon>
    </div>

    <!-- 交互模式：显示 Canvas -->
    <div 
      v-if="!isStatic"
      ref="container" 
      class="skin-viewer-container"
      :style="{ width: width + 'px', height: height + 'px' }"
    ></div>
  </div>
</template>

<script lang="ts">
// 全局渲染锁：确保全站同一时间只有一个静态渲染任务
let globalRenderLock: Promise<void> = Promise.resolve()
</script>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import * as skinview3d from 'skinview3d'
import { Loading } from '@element-plus/icons-vue'

const props = withDefaults(
  defineProps<{
    skinUrl: string
    capeUrl?: string | null
    model?: string
    width?: number
    height?: number
    isStatic?: boolean
  }>(),
  { capeUrl: null, model: 'default', width: 300, height: 400, isStatic: false }
)

const container = ref<HTMLDivElement | null>(null)
const snapshotUrl = ref<string | null>(null)
let viewer: skinview3d.SkinViewer | null = null

async function initViewer() {
  if (viewer) {
    viewer.dispose()
    viewer = null
  }

  const config: skinview3d.SkinViewerOptions = {
    width: props.width,
    height: props.height,
    skin: props.skinUrl,
    cape: props.capeUrl ?? undefined,
    model: props.model === 'slim' ? 'slim' : 'default',
    preserveDrawingBuffer: props.isStatic,
  }

  if (props.isStatic) {
    // 使用全局锁排队执行，防止 WebGL 上下文溢出
    globalRenderLock = globalRenderLock.then(async () => {
      // 再次检查快照是否已存在（防止重复渲染）
      if (snapshotUrl.value) return

      const tempCanvas = document.createElement('canvas')
      let staticViewer: skinview3d.SkinViewer | null = null

      try {
        staticViewer = new skinview3d.SkinViewer({
          canvas: tempCanvas,
          ...config,
        })

        // 静态视角设置
        staticViewer.autoRotate = false
        staticViewer.animation = null
        staticViewer.camera.position.set(0, 10, 500)
        staticViewer.camera.lookAt(0, 15, 0)
        staticViewer.zoom = 0.8

        staticViewer.playerObject.skin.leftArm.rotation.z = 0.05
        staticViewer.playerObject.skin.rightArm.rotation.z = -0.05
        staticViewer.playerObject.skin.leftLeg.rotation.z = 0
        staticViewer.playerObject.skin.rightLeg.rotation.z = 0

        // 等待资源加载
        await staticViewer.loadSkin(props.skinUrl, { model: props.model === 'slim' ? 'slim' : 'default' })
        if (props.capeUrl) await staticViewer.loadCape(props.capeUrl)

        staticViewer.render()
        snapshotUrl.value = tempCanvas.toDataURL('image/png')
      } catch (e) {
        console.error('SkinViewer static render error:', e)
      } finally {
        if (staticViewer) {
          staticViewer.dispose()
          staticViewer = null
        }
      }
    })
    await globalRenderLock
  } else {
    // 交互模式逻辑
    await nextTick()
    if (!container.value) return
    container.value.innerHTML = ''

    const canvas = document.createElement('canvas')
    viewer = new skinview3d.SkinViewer({
      canvas,
      ...config,
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

watch(
  () => [props.skinUrl, props.model, props.isStatic, props.capeUrl],
  () => {
    // 如果是贴图变了，清空旧快照
    snapshotUrl.value = null
    initViewer()
  },
  { deep: true }
)
</script>

<style scoped>
.skin-viewer-wrapper {
  display: flex;
  justify-content: center;
  align-items: center;
  overflow: hidden;
  position: relative;
}

.skin-loader {
  font-size: 24px;
  color: var(--el-text-color-secondary);
  opacity: 0.5;
}

.skin-snapshot {
  display: block;
  image-rendering: pixelated;
  object-fit: contain;
}
</style>
