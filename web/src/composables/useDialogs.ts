import { reactive } from 'vue'
import router from '../router'

export interface PromptOptions {
  title: string
  label?: string
  value?: string
  placeholder?: string
  type?: string
  confirmText?: string
  /** 允许提交空值（对齐原生 prompt 可提交空串的语义，如 emoji 留空重置） */
  allowEmpty?: boolean
  autocomplete?: string
}

export interface ConfirmOptions {
  title: string
  message: string
  confirmText?: string
  cancelText?: string
  tone?: 'primary' | 'error'
}

interface DialogState {
  prompt: PromptOptions | null
  confirm: ConfirmOptions | null
  promptResolve: ((value: string | null) => void) | null
  confirmResolve: ((value: boolean) => void) | null
}

// 模块级单例：DialogHost 常驻渲染，视图用 await 取结果（替代原生 prompt/confirm）
const state = reactive<DialogState>({
  prompt: null,
  confirm: null,
  promptResolve: null,
  confirmResolve: null,
})

function resolvePrompt(value: string | null): void {
  state.promptResolve?.(value)
  state.prompt = null
  state.promptResolve = null
}

function resolveConfirm(value: boolean): void {
  state.confirmResolve?.(value)
  state.confirm = null
  state.confirmResolve = null
}

// 路由切换即取消未决对话框：原生 prompt/confirm 是模态的，用户不可能带着弹窗
// 离开页面；这里保证「应用内返回」后弹窗不会残留在上一个页面、确认后照发请求。
router.afterEach(() => {
  resolvePrompt(null)
  resolveConfirm(false)
})

export function useDialogs(): {
  state: DialogState
  prompt: (opts: PromptOptions) => Promise<string | null>
  confirm: (opts: ConfirmOptions) => Promise<boolean>
  resolvePrompt: (value: string | null) => void
  resolveConfirm: (value: boolean) => void
} {
  function prompt(opts: PromptOptions): Promise<string | null> {
    return new Promise((resolve) => {
      resolvePrompt(null)
      resolveConfirm(false)
      state.prompt = opts
      state.promptResolve = resolve
    })
  }

  function confirm(opts: ConfirmOptions): Promise<boolean> {
    return new Promise((resolve) => {
      resolvePrompt(null)
      resolveConfirm(false)
      state.confirm = opts
      state.confirmResolve = resolve
    })
  }

  return { state, prompt, confirm, resolvePrompt, resolveConfirm }
}
