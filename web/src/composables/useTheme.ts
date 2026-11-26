import { ref, type Ref } from 'vue'

export type Theme = 'light' | 'dark'

const STORAGE_KEY = 'gh-theme'

function systemTheme(): Theme {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function savedTheme(): Theme | null {
  try {
    const v = localStorage.getItem(STORAGE_KEY)
    return v === 'light' || v === 'dark' ? v : null
  } catch {
    return null
  }
}

function apply(t: Theme): void {
  document.documentElement.dataset.theme = t
  const meta = document.querySelector('meta[name="theme-color"]')
  if (meta) meta.setAttribute('content', t === 'dark' ? '#1b1917' : '#f6f2ec')
}

// 模块级单例（仿 session.ts）。首屏脚本已在 HTML 里先行设好 data-theme，
// 这里只做兜底并接管后续切换。
const theme: Ref<Theme> = ref(savedTheme() ?? systemTheme())
apply(theme.value)

// 没有保存过偏好时跟随系统
const mql = window.matchMedia('(prefers-color-scheme: dark)')
mql.addEventListener('change', (e) => {
  if (!savedTheme()) {
    theme.value = e.matches ? 'dark' : 'light'
    apply(theme.value)
  }
})

export function useTheme(): { theme: Ref<Theme>; toggle: () => void } {
  function toggle(): void {
    theme.value = theme.value === 'light' ? 'dark' : 'light'
    try {
      localStorage.setItem(STORAGE_KEY, theme.value)
    } catch {
      /* 隐私模式等场景忽略 */
    }
    apply(theme.value)
  }
  return { theme, toggle }
}
