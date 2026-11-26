<script setup lang="ts">
// 积分数字：变化时按 600ms 三次缓出滚到新值，落定时 bump 一下。
// - 起点必须在 setup 里同步算出（不能先渲染旧值、再在 onMounted 里改）：
//   enter 且首个大于 0 的值，首帧就是 round(value * 0.7)，数据没回来（0）不算。
// - 滚动按时间推进（rAF 时间戳相对起点），冻结或节流时也能一次落定。
// - 减少动态效果时直接显示最终值；卸载时取消未完成的 rAF。
import { onBeforeUnmount, ref, watch } from 'vue'

const props = defineProps<{ value: number; enter?: boolean }>()
const DURATION = 600

const prefersReduced = () =>
  window.matchMedia('(prefers-reduced-motion: reduce)').matches

let entered = false // 「首个大于 0 的值」从 70% 滚起，只生效一次
if (props.enter && props.value > 0) entered = true

// 起点同步算出：首帧渲染的就是 70%，不是先渲染 value 再改
const shown = ref(
  entered && !prefersReduced() ? Math.round(props.value * 0.7) : props.value,
)

const root = ref<HTMLElement | null>(null)
let rafId = 0

function stopRaf() {
  if (rafId) cancelAnimationFrame(rafId)
  rafId = 0
}

// 落定时播一次 bump：先移除类、强制回流、再加回，连续变化也能重新触发
function playBump() {
  const el = root.value
  if (!el) return
  el.classList.remove('animate-bump')
  void el.offsetWidth
  el.classList.add('animate-bump')
}

function animate(from: number, to: number) {
  stopRaf()
  if (from === to || prefersReduced()) {
    shown.value = to
    return
  }
  shown.value = from
  const startAt = performance.now()
  const step = (ts: number) => {
    const t = Math.min(1, Math.max(0, ts - startAt) / DURATION)
    shown.value = Math.round(from + (to - from) * (1 - (1 - t) ** 3))
    if (t >= 1) {
      shown.value = to
      playBump()
    } else {
      rafId = requestAnimationFrame(step)
    }
  }
  rafId = requestAnimationFrame(step)
}

// 挂载即有值时（enter 且 > 0）从起点滚到位；rAF 异步启动，不影响首帧的起点值
if (shown.value !== props.value) animate(shown.value, props.value)

watch(
  () => props.value,
  (v) => {
    const firstPositive = props.enter && !entered && v > 0
    if (firstPositive) entered = true
    animate(firstPositive ? Math.round(v * 0.7) : shown.value, v)
  },
)

onBeforeUnmount(stopRaf)
</script>

<template>
  <span ref="root" class="inline-block">
    <span class="sr-only">{{ value }}</span>
    <span aria-hidden="true">{{ shown }}</span>
  </span>
</template>
