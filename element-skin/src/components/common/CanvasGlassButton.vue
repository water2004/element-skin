<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref } from 'vue'
import { heroSceneKey } from '@/composables/useHeroScene'

const props = withDefaults(defineProps<{
  variant?: 'primary' | 'secondary'
  blur?: number
  disabled?: boolean
}>(), {
  variant: 'secondary',
  blur: 18,
  disabled: false,
})

const emit = defineEmits<{
  click: [event: MouseEvent]
}>()

const scene = inject(heroSceneKey, null)

const rootRef = ref<HTMLButtonElement | null>(null)
const canvasRef = ref<HTMLCanvasElement | null>(null)
let resizeObserver: ResizeObserver | null = null
let unsubscribe: (() => void) | null = null
let rafId = 0
let scrollTimer = 0
let disposed = false
let lastSceneDraw = 0

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
  rafId = requestAnimationFrame(drawGlass)
}

function requestSceneDraw() {
  const now = performance.now()
  if (now - lastSceneDraw < 33) return
  lastSceneDraw = now
  drawGlass()
}

function requestScrollDraw() {
  if (disposed) return
  window.clearTimeout(scrollTimer)
  scrollTimer = window.setTimeout(requestDraw, 120)
}

function drawFallback(ctx: CanvasRenderingContext2D, width: number, height: number) {
  const gradient = ctx.createLinearGradient(0, 0, width, height)
  gradient.addColorStop(0, '#1a1a1a')
  gradient.addColorStop(1, '#333333')
  ctx.fillStyle = gradient
  ctx.fillRect(0, 0, width, height)
}

// Copy a blurred crop of the shared scene canvas at this button's screen rect.
// Because the scene is the only renderer, the crop is always the exact frame
// shown behind the button — no second image load, no timing to match.
function drawGlass() {
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
  ctx.filter = `blur(${props.blur}px) saturate(180%)`

  const source = scene?.getCanvas()
  if (source && source.width > 0 && source.height > 0) {
    const sdpr = scene!.getDpr()
    const sx = (rect.left - pad) * sdpr
    const sy = (rect.top - pad) * sdpr
    const sw = cssWidth * sdpr
    const sh = cssHeight * sdpr
    ctx.drawImage(source, sx, sy, sw, sh, 0, 0, cssWidth, cssHeight)
  } else {
    drawFallback(ctx, cssWidth, cssHeight)
  }

  ctx.filter = 'none'
}

onMounted(() => {
  requestDraw()
  unsubscribe = scene?.subscribe(requestSceneDraw) ?? null
  // Debounce button-local triggers (move / resize) via rAF.
  window.addEventListener('resize', requestDraw)
  window.addEventListener('scroll', requestScrollDraw, { passive: true })
  if (window.ResizeObserver && rootRef.value) {
    resizeObserver = new ResizeObserver(requestDraw)
    resizeObserver.observe(rootRef.value)
  }
})

onBeforeUnmount(() => {
  disposed = true
  cancelAnimationFrame(rafId)
  window.clearTimeout(scrollTimer)
  unsubscribe?.()
  window.removeEventListener('resize', requestDraw)
  window.removeEventListener('scroll', requestScrollDraw)
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
