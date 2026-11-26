import { computed, ref, type ComputedRef, type Ref } from 'vue'
import router from '../router'

export interface ToastState {
  id: number
  message: string
}

// 模块级单例：整个应用同一时刻只有一条 toast（与原各页面的单字符串 ref 等价）。
const current = ref<ToastState | null>(null)
const active = computed(() => current.value !== null)

let timer = 0
let seq = 0

/** 展示一条 toast；新消息顶掉旧的并重置计时（默认 2200ms，与原实现一致）。 */
function show(message: string, duration = 2200): void {
  clearTimeout(timer)
  current.value = { id: ++seq, message }
  timer = window.setTimeout(() => {
    current.value = null
  }, duration)
}

function dismiss(): void {
  clearTimeout(timer)
  current.value = null
}

// 路由切换即清掉（现状：页面一走 toast 就没了）
router.afterEach(() => dismiss())

export function useToast(): {
  current: Ref<ToastState | null>
  active: ComputedRef<boolean>
  show: (message: string, duration?: number) => void
  dismiss: () => void
} {
  return { current, active, show, dismiss }
}
