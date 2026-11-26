// 统一 API 客户端：JSON、同源 Cookie、错误信封 → ApiError。
export class ApiError extends Error {
  status: number
  code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

export async function api<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: body !== undefined ? { 'Content-Type': 'application/json' } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
    credentials: 'same-origin',
  })
  if (!res.ok) {
    let code = 'unknown'
    let message = `HTTP ${res.status}`
    try {
      const data = await res.json()
      code = data?.error?.code ?? code
      message = data?.error?.message ?? message
    } catch {
      /* 非 JSON 错误体 */
    }
    // 会话过期：跳登录页（登录/注册接口自身的 401 交给表单提示）。
    // 注意：已在 /login、/register 等公开页时绝不能再跳——否则页面上
    // 路由守卫的 /api/me 探测 401 会把自己重定向到自己，造成整页刷新死循环。
    const onPublicPage =
      location.pathname.startsWith('/login') || location.pathname.startsWith('/register')
    if (res.status === 401 && !path.startsWith('/api/auth/') && !onPublicPage) {
      location.href = '/login'
    }
    throw new ApiError(res.status, code, message)
  }
  if (res.status === 204) {
    return undefined as T
  }
  return (await res.json()) as T
}

// errMsg 把错误码映射成中文文案。
export function errMsg(e: unknown): string {
  if (e instanceof ApiError) {
    const map: Record<string, string> = {
      duplicate_username: '用户名已被占用',
      bad_credentials: '用户名或密码不对',
      account_locked: '失败次数太多，请 10 分钟后再试',
      account_inactive: '账号已停用',
      rate_limited: '操作太频繁，稍后再试',
      invalid_username: '用户名需 3-32 位小写字母、数字或下划线',
      short_password: '密码至少 8 位',
      invalid_password: '密码至少 8 位',
      insufficient_points: '积分不够啦',
      no_active_prizes: '转盘还没有奖品',
      pending_choice_exists: '先选完上一局的奖品',
      already_fulfilled: '已经发放过了',
      not_found: '内容不存在',
    }
    return map[e.code] ?? e.message
  }
  return '网络异常，请重试'
}
