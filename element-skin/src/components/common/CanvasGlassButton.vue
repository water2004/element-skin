<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

const props = withDefaults(defineProps<{
  backgroundUrl?: string
  variant?: 'primary' | 'secondary'
  blur?: number
  disabled?: boolean
  overlayColor?: string
}>(), {
  backgroundUrl: '',
  variant: 'secondary',
  blur: 18,
  disabled: false,
  overlayColor: 'rgba(0, 0, 0, 0.45)',
})

const emit = defineEmits<{
  click: [event: MouseEvent]
}>()

const rootRef = ref<HTMLButtonElement | null>(null)
const canvasRef = ref<HTMLCanvasElement | null>(null)
const imageCache = new Map<string, HTMLImageElement>()
let resizeObserver: ResizeObserver | null = null
let rafId = 0
let disposed = false

const bleed = computed(() => Math.ceil(props.blur * 2))
const canvasStyle = computed(() => ({
  left: `-${bleed.value}px`,
  top: `-${bleed.value}px`,
  width: `calc(100% + ${bleed.value * 2}px)`,
  height: `calc(100% + ${bleed.value * 2}px)`,
}))

function handleClick(event: MouseEvent) {
  if (!props.disabled) emit('click', event)
}

function requestDraw() {
  if (disposed) return
  cancelAnimationFrame(rafId)
  rafId = requestAnimationFrame(() => {
    void drawGlass()
  })
}

function loadImage(url: string) {
  const cached = imageCache.get(url)
  if (cached?.complete && cached.naturalWidth > 0) return Promise.resolve(cached)

  return new Promise<HTMLImageElement>((resolve, reject) => {
    const img = cached || new Image()
    imageCache.set(url, img)
    img.onload = () => {
      requestDraw()
      resolve(img)
    }
    img.onerror = reject
    if (!cached) img.src = url
  })
}

function drawFallback(ctx: CanvasRenderingContext2D, width: number, height: number) {
  const gradient = ctx.createLinearGradient(0, 0, width, height)
  gradient.addColorStop(0, '#1a1a1a')
  gradient.addColorStop(1, '#333333')
  ctx.fillStyle = gradient
  ctx.fillRect(0, 0, width, height)
}

async function drawGlass() {
  const root = rootRef.value
  const canvas = canvasRef.value
  const ctx = canvas?.getContext('2d')
  if (!root || !canvas || !ctx) return

  const rect = root.getBoundingClientRect()
  if (rect.width <= 0 || rect.height <= 0) return

  const dpr = Math.max(window.devicePixelRatio || 1, 1)
  const pad = bleed.value
  const cssWidth = Math.ceil(rect.width + pad * 2)
  const cssHeight = Math.ceil(rect.height + pad * 2)
  const pixelWidth = Math.ceil(cssWidth * dpr)
  const pixelHeight = Math.ceil(cssHeight * dpr)

  if (canvas.width !== pixelWidth) canvas.width = pixelWidth
  if (canvas.height !== pixelHeight) canvas.height = pixelHeight

  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  ctx.clearRect(0, 0, cssWidth, cssHeight)
  ctx.save()
  ctx.filter = `blur(${props.blur}px) saturate(180%)`

  const imageUrl = props.backgroundUrl
  if (imageUrl) {
    try {
      const img = await loadImage(imageUrl)
      if (disposed) return

      const viewportWidth = window.innerWidth
      const viewportHeight = window.innerHeight
      const scale = Math.max(viewportWidth / img.naturalWidth, viewportHeight / img.naturalHeight)
      const drawnWidth = img.naturalWidth * scale
      const drawnHeight = img.naturalHeight * scale
      const offsetX = (viewportWidth - drawnWidth) / 2
      const offsetY = (viewportHeight - drawnHeight) / 2
      const cropX = rect.left - pad
      const cropY = rect.top - pad

      ctx.drawImage(img, offsetX - cropX, offsetY - cropY, drawnWidth, drawnHeight)
    } catch {
      drawFallback(ctx, cssWidth, cssHeight)
    }
  } else {
    drawFallback(ctx, cssWidth, cssHeight)
  }

  ctx.restore()
  ctx.fillStyle = props.overlayColor
  ctx.fillRect(0, 0, cssWidth, cssHeight)
}

watch(() => props.backgroundUrl, () => {
  nextTick(requestDraw)
})

onMounted(() => {
  nextTick(requestDraw)
  window.addEventListener('resize', requestDraw)
  window.addEventListener('scroll', requestDraw, { passive: true })

  if (window.ResizeObserver && rootRef.value) {
    resizeObserver = new ResizeObserver(requestDraw)
    resizeObserver.observe(rootRef.value)
  }
})

onBeforeUnmount(() => {
  disposed = true
  cancelAnimationFrame(rafId)
  window.removeEventListener('resize', requestDraw)
  window.removeEventListener('scroll', requestDraw)
  resizeObserver?.disconnect()
})
</script>

<template>
  <button
    ref="rootRef"
    type="button"
    class="canvas-glass-button"
    :class="`is-${variant}`"
    :disabled="disabled"
    @click="handleClick"
  >
    <canvas ref="canvasRef" class="glass-canvas" :style="canvasStyle" aria-hidden="true"></canvas>
    <span class="glass-tint"></span>
    <span class="glass-content">
      <slot />
    </span>
  </button>
</template>

<style scoped>
.canvas-glass-button {
  position: relative;
  isolation: isolate;
  overflow: hidden;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border-radius: 14px;
  border: 1px solid rgba(255, 255, 255, 0.25);
  color: #fff;
  background: rgba(255, 255, 255, 0.12);
  cursor: pointer;
  font: inherit;
  white-space: nowrap;
  transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1), box-shadow 0.3s cubic-bezier(0.4, 0, 0.2, 1), border-color 0.3s ease;
}

.canvas-glass-button:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.15);
}

.canvas-glass-button:disabled {
  cursor: not-allowed;
  opacity: 0.65;
}

.canvas-glass-button.is-primary {
  border-color: rgba(64, 158, 255, 0.35);
  background: rgba(64, 158, 255, 0.18);
}

.glass-canvas {
  position: absolute;
  z-index: -2;
  pointer-events: none;
}

.glass-tint {
  position: absolute;
  inset: 0;
  z-index: -1;
  background: rgba(255, 255, 255, 0.12);
  pointer-events: none;
}

.is-primary .glass-tint {
  background: rgba(64, 158, 255, 0.2);
}

.glass-content {
  position: relative;
  z-index: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}
</style>
