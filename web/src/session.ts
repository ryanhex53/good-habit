import { ref, type Ref } from 'vue'
import { api } from './api'

export interface Me {
  id: number
  username: string
  displayName: string
  role: 'parent' | 'child'
  familyId: number
}

// 模块级单例：整个应用共享一份会话状态。
const me: Ref<Me | null> = ref(null)
let inflight: Promise<void> | null = null

export function useSession() {
  async function refresh(): Promise<void> {
    if (inflight) return inflight
    inflight = (async () => {
      try {
        const res = await api<{ user: Me }>('GET', '/api/me')
        me.value = res.user
      } catch {
        me.value = null
      } finally {
        inflight = null
      }
    })()
    return inflight
  }

  async function logout(): Promise<void> {
    try {
      await api('POST', '/api/auth/logout')
    } finally {
      me.value = null
    }
  }

  return { me, refresh, logout }
}
