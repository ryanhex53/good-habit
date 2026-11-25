<script setup lang="ts">
import { onBeforeUnmount, onMounted, watch } from 'vue'
import type { Prize } from '../types'

const props = defineProps<{ prizes: Prize[] }>()

const emit = defineEmits<{ (e: 'spinstart'): void; (e: 'spinend'): void }>()

const PALETTE = ['#ff9a3c', '#ffd166', '#06d6a0', '#4cc9f0', '#f94144', '#b5179e', '#f8961e', '#43aa8b']

let rotation = 0
let raf = 0
let canvasEl: HTMLCanvasElement | null = null

function size(): number {
  const parent = canvasEl?.parentElement
  return Math.min(parent?.clientWidth ?? 320, 380)
}

function draw() {
  const c = canvasEl
  if (!c) return
  const dpr = window.devicePixelRatio || 1
  const s = size()
  c.width = s * dpr
  c.height = s * dpr
  c.style.width = `${s}px`
  c.style.height = `${s}px`
  const ctx = c.getContext('2d')
  if (!ctx) return
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  ctx.clearRect(0, 0, s, s)

  const cx = s / 2
  const cy = s / 2
  const R = s / 2 - 10
  const N = props.prizes.length
  if (N === 0) return

  const sector = (2 * Math.PI) / N
  for (let i = 0; i < N; i++) {
    const a0 = rotation + i * sector
    const a1 = a0 + sector
    ctx.beginPath()
    ctx.moveTo(cx, cy)
    ctx.arc(cx, cy, R, a0, a1)
    ctx.closePath()
    ctx.fillStyle = PALETTE[i % PALETTE.length]
    ctx.fill()
    ctx.strokeStyle = '#ffffff'
    ctx.lineWidth = 3
    ctx.stroke()

    // 文字沿半径方向（emoji 外圈 + 名称内圈）
    const mid = (a0 + a1) / 2
    ctx.save()
    ctx.translate(cx, cy)
    ctx.rotate(mid)
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'
    ctx.font = `${Math.round(s / 14)}px serif`
    ctx.fillText(props.prizes[i].emoji, R * 0.8, 0)
    ctx.shadowColor = 'rgba(0,0,0,0.3)'
    ctx.shadowBlur = 2
    const name = props.prizes[i].name
    const font = Math.max(11, Math.round(s / 30))
    ctx.font = `700 ${font}px system-ui, 'Microsoft YaHei'`
    if (name.length > 6) {
      const cut = Math.ceil(name.length / 2)
      ctx.fillText(name.slice(0, cut), R * 0.5, -font * 0.65)
      ctx.fillText(name.slice(cut), R * 0.5, font * 0.65)
    } else {
      ctx.fillText(name, R * 0.5, 0)
    }
    ctx.restore()
  }

  // 中心轴
  ctx.beginPath()
  ctx.arc(cx, cy, Math.max(18, s * 0.09), 0, 2 * Math.PI)
  ctx.fillStyle = '#ffffff'
  ctx.fill()
  ctx.lineWidth = 4
  ctx.strokeStyle = '#e8630a'
  ctx.stroke()

  // 顶部指针（固定，不随转盘旋转）
  ctx.beginPath()
  ctx.moveTo(cx, 6)
  ctx.lineTo(cx - 13, 34)
  ctx.lineTo(cx + 13, 34)
  ctx.closePath()
  ctx.fillStyle = '#e8630a'
  ctx.fill()
  ctx.strokeStyle = '#ffffff'
  ctx.lineWidth = 2.5
  ctx.stroke()
}

// cubic-bezier(0.15, 0.85, 0.25, 1) 的缓动求值（牛顿迭代解 x(t)=x）。
function cubicBezier(x1: number, y1: number, x2: number, y2: number, x: number): number {
  const cx = 3 * x1
  const bx = 3 * (x2 - x1) - cx
  const ax = 1 - cx - bx
  const cy = 3 * y1
  const by = 3 * (y2 - y1) - cy
  const ay = 1 - cy - by
  const fx = (t: number) => ((ax * t + bx) * t + cx) * t
  const fy = (t: number) => ((ay * t + by) * t + cy) * t
  const dfx = (t: number) => (3 * ax * t + 2 * bx) * t + cx
  let t = x
  for (let i = 0; i < 6; i++) {
    const err = fx(t) - x
    if (Math.abs(err) < 1e-6) break
    const d = dfx(t)
    if (Math.abs(d) < 1e-7) break
    t -= err / d
  }
  t = Math.min(1, Math.max(0, t))
  return fy(t)
}

function spin(targetIndex: number): Promise<void> {
  emit('spinstart')
  return new Promise<void>((resolve) => {
    const N = props.prizes.length
    if (N === 0) {
      resolve()
      return
    }
    const sector = (2 * Math.PI) / N
    const mod = (a: number, m: number) => ((a % m) + m) % m
    const TWO_PI = Math.PI * 2
    // 目标：target 扇区中线对准顶部指针（-π/2）
    const targetAbs = mod(-Math.PI / 2 - sector / 2 - targetIndex * sector, TWO_PI)
    const delta = mod(targetAbs - mod(rotation, TWO_PI), TWO_PI) + 5 * TWO_PI
    const start = rotation
    const final = start + delta
    const D = 4000
    const t0 = performance.now()

    const step = (now: number) => {
      const t = Math.min(1, (now - t0) / D)
      rotation = start + delta * cubicBezier(0.15, 0.85, 0.25, 1, t)
      draw()
      if (t < 1) {
        raf = requestAnimationFrame(step)
      } else {
        rotation = final
        draw()
        emit('spinend')
        resolve()
      }
    }
    raf = requestAnimationFrame(step)
  })
}

defineExpose<{ spin: (targetIndex: number) => Promise<void> }>({ spin })

function onResize() {
  draw()
}

onMounted(() => {
  canvasEl = document.querySelector('canvas.wheel-canvas')
  draw()
  window.addEventListener('resize', onResize)
})
onBeforeUnmount(() => {
  cancelAnimationFrame(raf)
  window.removeEventListener('resize', onResize)
})
watch(
  () => props.prizes,
  () => {
    rotation = 0
    draw()
  },
)
</script>

<template>
  <div class="wheel-wrap">
    <canvas class="wheel-canvas"></canvas>
    <p v-if="prizes.length === 0" class="empty">转盘还没有奖品</p>
  </div>
</template>

<style scoped>
.wheel-wrap {
  position: relative;
  display: flex;
  justify-content: center;
}
.empty {
  position: absolute;
  top: 45%;
  color: var(--c-sub);
  font-size: 14px;
}
</style>
